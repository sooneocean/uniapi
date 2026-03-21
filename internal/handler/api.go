package handler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/cache"
	"github.com/sooneocean/uniapi/internal/memory"
	"github.com/sooneocean/uniapi/internal/metrics"
	"github.com/sooneocean/uniapi/internal/plugin"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/quota"
	"github.com/sooneocean/uniapi/internal/rag"
	"github.com/sooneocean/uniapi/internal/router"
	"github.com/sooneocean/uniapi/internal/usage"
	"github.com/sooneocean/uniapi/internal/webhook"
)

type APIHandler struct {
	router      *router.Router
	recorder    *usage.Recorder
	webhookMgr  *webhook.Manager
	respCache   *ResponseCache
	aliasCache  *cache.MemCache
	db          *sql.DB
	ragMgr      *rag.Manager
	pluginMgr   *plugin.Manager
	memMgr      *memory.Manager
	quotaEngine *quota.Engine
}

func (h *APIHandler) SetRAGManager(m *rag.Manager) {
	h.ragMgr = m
}

// SetQuotaEngine wires the quota engine into the chat handler.
func (h *APIHandler) SetQuotaEngine(e *quota.Engine) {
	h.quotaEngine = e
}

func (h *APIHandler) SetPluginManager(m *plugin.Manager) {
	h.pluginMgr = m
}

// RouterModelCount returns the number of models available across all registered providers.
func (h *APIHandler) RouterModelCount() int {
	return len(h.router.AllModels())
}

func NewAPIHandler(r *router.Router, rec *usage.Recorder) *APIHandler {
	return &APIHandler{router: r, recorder: rec}
}

func NewAPIHandlerFull(r *router.Router, rec *usage.Recorder, webhookMgr *webhook.Manager, respCache *ResponseCache, db *sql.DB) *APIHandler {
	return &APIHandler{router: r, recorder: rec, webhookMgr: webhookMgr, respCache: respCache, db: db}
}

func NewAPIHandlerWithCache(r *router.Router, rec *usage.Recorder, webhookMgr *webhook.Manager, respCache *ResponseCache, db *sql.DB, mc *cache.MemCache) *APIHandler {
	return &APIHandler{router: r, recorder: rec, webhookMgr: webhookMgr, respCache: respCache, db: db, aliasCache: mc, memMgr: memory.NewManager(8000)}
}

type chatToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type chatTool struct {
	Type     string            `json:"type"`
	Function chatToolFunction  `json:"function"`
}

type chatCompletionRequest struct {
	Model       string     `json:"model" binding:"required"`
	Messages    []chatMsg  `json:"messages" binding:"required"`
	Tools       []chatTool `json:"tools,omitempty"`
	MaxTokens   int        `json:"max_tokens,omitempty"`
	Temperature *float64   `json:"temperature,omitempty"`
	Stream      bool       `json:"stream,omitempty"`
	Provider    string     `json:"provider,omitempty"`
}

type chatMsg struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // string or []ContentBlock
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  interface{} `json:"tool_calls,omitempty"`
}

func parseContentBlocks(raw interface{}) []provider.ContentBlock {
	switch v := raw.(type) {
	case string:
		return []provider.ContentBlock{{Type: "text", Text: v}}
	case []interface{}:
		var blocks []provider.ContentBlock
		for _, item := range v {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			cb := provider.ContentBlock{}
			if t, ok := block["type"].(string); ok {
				cb.Type = t
			}
			if t, ok := block["text"].(string); ok {
				cb.Text = t
			}
			if iu, ok := block["image_url"].(map[string]interface{}); ok {
				if url, ok := iu["url"].(string); ok {
					cb.ImageURL = url
				}
			}
			blocks = append(blocks, cb)
		}
		return blocks
	default:
		return []provider.ContentBlock{{Type: "text", Text: ""}}
	}
}

func (h *APIHandler) resolveAlias(model, userID string) string {
	cacheKey := "alias:" + model
	if h.aliasCache != nil {
		if cached, ok := h.aliasCache.Get(cacheKey); ok {
			if resolved, ok := cached.(string); ok && resolved != "" {
				return resolved
			}
			return model
		}
	}
	if h.db == nil {
		return model
	}
	var resolved string
	err := h.db.QueryRow(
		"SELECT model_id FROM model_aliases WHERE alias = ? AND (user_id IS NULL OR user_id = ?) ORDER BY user_id DESC LIMIT 1",
		model, userID,
	).Scan(&resolved)
	if err == nil {
		if h.aliasCache != nil {
			h.aliasCache.Set(cacheKey, resolved, 5*time.Minute)
		}
		return resolved
	}
	if h.aliasCache != nil {
		h.aliasCache.Set(cacheKey, "", 5*time.Minute)
	}
	return model
}

func (h *APIHandler) ChatCompletions(c *gin.Context) {
	var req chatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": err.Error()}})
		return
	}
	messages := make([]provider.Message, len(req.Messages))
	for i, m := range req.Messages {
		// Handle tool role messages (tool results)
		if m.Role == "tool" {
			content := ""
			switch v := m.Content.(type) {
			case string:
				content = v
			}
			toolResult := &struct {
				ToolUseID string `json:"tool_use_id"`
				Content   string `json:"content"`
			}{
				ToolUseID: m.ToolCallID,
				Content:   content,
			}
			messages[i] = provider.Message{
				Role: "tool",
				Content: []provider.ContentBlock{{
					Type:       "tool_result",
					ToolResult: toolResult,
				}},
			}
			continue
		}
		// Handle assistant messages with tool_calls
		if m.Role == "assistant" && m.ToolCalls != nil {
			var blocks []provider.ContentBlock
			// Parse text content if present
			if v, ok := m.Content.(string); ok && v != "" {
				blocks = append(blocks, provider.ContentBlock{Type: "text", Text: v})
			}
			// Parse tool_calls
			if tcList, ok := m.ToolCalls.([]interface{}); ok {
				for _, tc := range tcList {
					if tcMap, ok := tc.(map[string]interface{}); ok {
						id, _ := tcMap["id"].(string)
					tcType, _ := tcMap["type"].(string)
					call := &provider.ToolCall{
						ID:   id,
						Type: tcType,
					}
					if fn, ok := tcMap["function"].(map[string]interface{}); ok {
						call.Function.Name, _ = fn["name"].(string)
						call.Function.Arguments, _ = fn["arguments"].(string)
					}
						blocks = append(blocks, provider.ContentBlock{
							Type:    "tool_use",
							ToolUse: call,
						})
					}
				}
			}
			messages[i] = provider.Message{Role: "assistant", Content: blocks}
			continue
		}
		if v, ok := m.Content.(string); ok && len(v) > 1_000_000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "message content too large"}})
			return
		}
		messages[i] = provider.Message{Role: m.Role, Content: parseContentBlocks(m.Content)}
	}
	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if u, ok := uid.(string); ok {
			userID = u
		}
	}

	chatReq := &provider.ChatRequest{
		Model: req.Model, Messages: messages, MaxTokens: req.MaxTokens,
		Temperature: req.Temperature, Stream: req.Stream, Provider: req.Provider,
	}
	if len(req.Tools) > 0 {
		chatReq.Tools = make([]provider.Tool, len(req.Tools))
		for i, t := range req.Tools {
			chatReq.Tools[i] = provider.Tool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			}
		}
	}

	// Resolve model alias
	chatReq.Model = h.resolveAlias(chatReq.Model, userID)

	// Quota check
	if userID != "" {
		if h.quotaEngine != nil {
			result := h.quotaEngine.Check(userID)
			if !result.Allowed {
				c.JSON(429, gin.H{"error": gin.H{"type": "quota_exceeded", "message": result.Message}})
				return
			}
			if result.Warning {
				c.Header("X-Quota-Warning", result.Message)
			}
		} else if err := h.checkQuota(userID); err != nil {
			c.JSON(429, gin.H{"error": gin.H{"type": "quota_exceeded", "message": err.Error()}})
			return
		}
	}

	// Memory compaction
	if h.memMgr != nil {
		chatReq.Messages = h.memMgr.CompactMessages(c.Request.Context(), chatReq.Messages,
			func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
				return h.router.Route(ctx, req, userID)
			})
	}

	// RAG: inject knowledge base context
	if h.ragMgr != nil {
		queryText := ""
		for _, m := range chatReq.Messages {
			if m.Role == "user" && len(m.Content) > 0 {
				queryText = m.Content[0].Text
				break
			}
		}
		if queryText != "" {
			chunks, _ := h.ragMgr.Search(userID, queryText, 3)
			if len(chunks) > 0 {
				context := "Relevant context from knowledge base:\n\n"
				for _, ch := range chunks {
					context += ch.Content + "\n---\n"
				}
				sysMsg := provider.Message{Role: "system", Content: []provider.ContentBlock{{Type: "text", Text: context}}}
				chatReq.Messages = append([]provider.Message{sysMsg}, chatReq.Messages...)
			}
		}
	}

	// Plugins: inject as tools
	if h.pluginMgr != nil {
		pluginTools, _ := h.pluginMgr.ToTools(userID)
		chatReq.Tools = append(chatReq.Tools, pluginTools...)
	}

	if req.Stream {
		h.handleStream(c, chatReq)
		return
	}

	// Check response cache (non-streaming only)
	if h.respCache != nil {
		cacheKey := h.respCache.Key(chatReq.Model, chatReq.Messages)
		if cached, ok := h.respCache.Get(cacheKey); ok {
			c.JSON(http.StatusOK, gin.H{
				"id": "chatcmpl-" + uuid.New().String()[:8], "object": "chat.completion",
				"created": time.Now().Unix(), "model": cached.Model,
				"choices": []gin.H{{"index": 0, "message": gin.H{"role": "assistant", "content": cached.Content}, "finish_reason": "stop"}},
				"usage": gin.H{"prompt_tokens": cached.TokensIn, "completion_tokens": cached.TokensOut, "total_tokens": cached.TokensIn + cached.TokensOut},
				"x_uniapi": gin.H{"cached": true},
			})
			return
		}
	}

	// Enforce maximum request timeout to prevent hung connections
	reqCtx, reqCancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer reqCancel()

	start := time.Now()
	resp, err := h.router.Route(reqCtx, chatReq, userID)
	latency := time.Since(start)
	if err != nil {
		metrics.ProviderRequestsTotal.WithLabelValues(req.Model, req.Model, "error").Inc()
		if h.webhookMgr != nil {
			h.webhookMgr.Fire("provider_error", map[string]interface{}{
				"model": req.Model, "error": err.Error(), "user_id": userID,
			})
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "api_error", "message": err.Error()}})
		return
	}
	content := ""
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	// Store in response cache (only when no tool calls)
	if h.respCache != nil && len(resp.ToolCalls) == 0 {
		cacheKey := h.respCache.Key(chatReq.Model, chatReq.Messages)
		h.respCache.Set(cacheKey, &CachedResponse{
			Content:   content,
			Model:     resp.Model,
			TokensIn:  resp.TokensIn,
			TokensOut: resp.TokensOut,
		})
	}

	if h.recorder != nil {
		if uid, exists := c.Get("user_id"); exists {
			if userID, ok := uid.(string); ok {
				cost := usage.CalculateCost(resp.Model, resp.TokensIn, resp.TokensOut)
				go h.recorder.RecordUsage(usage.UsageRecord{ //nolint:errcheck
					UserID:    userID,
					Model:     resp.Model,
					Provider:  "",
					TokensIn:  resp.TokensIn,
					TokensOut: resp.TokensOut,
					Cost:      cost,
					LatencyMs: int(latency.Milliseconds()),
				})
			}
		}
	}

	metrics.ProviderRequestsTotal.WithLabelValues(resp.Model, resp.Model, "success").Inc()
	metrics.ProviderLatency.WithLabelValues(resp.Model, resp.Model).Observe(latency.Seconds())
	metrics.TokensProcessed.WithLabelValues("input", resp.Model).Add(float64(resp.TokensIn))
	metrics.TokensProcessed.WithLabelValues("output", resp.Model).Add(float64(resp.TokensOut))

	finishReason := "stop"
	if resp.StopReason == "tool_use" {
		finishReason = "tool_calls"
	}
	message := gin.H{"role": "assistant", "content": content}
	if len(resp.ToolCalls) > 0 {
		message["tool_calls"] = resp.ToolCalls
	}
	c.JSON(http.StatusOK, gin.H{
		"id": "chatcmpl-" + uuid.New().String()[:8], "object": "chat.completion",
		"created": time.Now().Unix(), "model": resp.Model,
		"choices": []gin.H{{"index": 0, "message": message, "finish_reason": finishReason}},
		"usage": gin.H{"prompt_tokens": resp.TokensIn, "completion_tokens": resp.TokensOut, "total_tokens": resp.TokensIn + resp.TokensOut},
		"x_uniapi": gin.H{"latency_ms": latency.Milliseconds()},
	})
}

// streamChunk is the SSE JSON payload for a single streaming chunk.
type streamChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []streamDelta `json:"choices"`
}

// streamDelta is a single choice delta within a streamChunk.
type streamDelta struct {
	Index        int               `json:"index"`
	Delta        map[string]string `json:"delta"`
	FinishReason *string           `json:"finish_reason"`
}

func (h *APIHandler) handleStream(c *gin.Context, req *provider.ChatRequest) {
	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if u, ok := uid.(string); ok {
			userID = u
		}
	}
	// Enforce maximum streaming timeout
	streamCtx, streamCancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer streamCancel()

	start := time.Now()
	stream, err := h.router.RouteStream(streamCtx, req, userID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "api_error", "message": err.Error()}})
		return
	}
	defer stream.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	id := "chatcmpl-" + uuid.New().String()[:8]
	model := req.Model
	created := time.Now().Unix()

	var tokensIn, tokensOut int

	ctx := streamCtx
	w := c.Writer
	encoder := json.NewEncoder(w) // reuse encoder across chunks — eliminates per-chunk allocation

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			stream.Close()
			return
		default:
		}

		event, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if event.Type == "done" {
			fmt.Fprint(w, "data: [DONE]\n\n")
			w.Flush()
			if event.Response != nil {
				tokensIn = event.Response.TokensIn
				tokensOut = event.Response.TokensOut
			}
			break
		}
		if event.Type == "content_delta" {
			chunk := streamChunk{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []streamDelta{{
					Index: 0,
					Delta: map[string]string{"content": event.Content.Text},
				}},
			}
			fmt.Fprint(w, "data: ")
			encoder.Encode(chunk) // writes JSON + newline
			fmt.Fprint(w, "\n")
			w.Flush()
		}
	}

	if h.recorder != nil {
		if uid, exists := c.Get("user_id"); exists {
			if userID, ok := uid.(string); ok {
				latency := time.Since(start)
				cost := usage.CalculateCost(model, tokensIn, tokensOut)
				go h.recorder.RecordUsage(usage.UsageRecord{ //nolint:errcheck
					UserID:    userID,
					Model:     model,
					Provider:  "",
					TokensIn:  tokensIn,
					TokensOut: tokensOut,
					Cost:      cost,
					LatencyMs: int(latency.Milliseconds()),
				})
			}
		}
	}
}

func (h *APIHandler) checkQuota(userID string) error {
	if h.db == nil {
		return nil
	}
	var dailyTokenLimit int
	var dailyCostLimit, monthlyCostLimit float64
	err := h.db.QueryRow(
		"SELECT COALESCE(daily_token_limit,0), COALESCE(daily_cost_limit,0), COALESCE(monthly_cost_limit,0) FROM users WHERE id = ?", userID,
	).Scan(&dailyTokenLimit, &dailyCostLimit, &monthlyCostLimit)
	if err != nil {
		return nil // no user = no limit
	}

	today := time.Now().Format("2006-01-02")
	monthStart := time.Now().Format("2006-01") + "-01"

	if dailyTokenLimit > 0 {
		var dailyTokens int
		h.db.QueryRow("SELECT COALESCE(SUM(tokens_in + tokens_out), 0) FROM usage_daily WHERE user_id = ? AND date = ?", userID, today).Scan(&dailyTokens) //nolint:errcheck
		if dailyTokens >= dailyTokenLimit {
			return fmt.Errorf("daily token limit exceeded (%d/%d)", dailyTokens, dailyTokenLimit)
		}
	}
	if dailyCostLimit > 0 {
		var dailyCost float64
		h.db.QueryRow("SELECT COALESCE(SUM(cost), 0) FROM usage_daily WHERE user_id = ? AND date = ?", userID, today).Scan(&dailyCost) //nolint:errcheck
		if dailyCost >= dailyCostLimit {
			return fmt.Errorf("daily cost limit exceeded ($%.2f/$%.2f)", dailyCost, dailyCostLimit)
		}
	}
	if monthlyCostLimit > 0 {
		var monthlyCost float64
		h.db.QueryRow("SELECT COALESCE(SUM(cost), 0) FROM usage_daily WHERE user_id = ? AND date >= ?", userID, monthStart).Scan(&monthlyCost) //nolint:errcheck
		if monthlyCost >= monthlyCostLimit {
			return fmt.Errorf("monthly cost limit exceeded ($%.2f/$%.2f)", monthlyCost, monthlyCostLimit)
		}
	}
	return nil
}

// ResponseCache wraps a MemCache for caching chat completion responses.
type ResponseCache struct {
	cache   *cache.MemCache
	ttl     time.Duration
	enabled bool
}

func NewResponseCache(c *cache.MemCache, ttl time.Duration, enabled bool) *ResponseCache {
	return &ResponseCache{cache: c, ttl: ttl, enabled: enabled}
}

// CachedResponse holds the cached fields of a chat completion response.
type CachedResponse struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
}

func (rc *ResponseCache) Key(model string, messages interface{}) string {
	data, _ := json.Marshal(map[string]interface{}{"model": model, "messages": messages})
	hash := sha256.Sum256(data)
	return "resp:" + hex.EncodeToString(hash[:])
}

func (rc *ResponseCache) Get(key string) (*CachedResponse, bool) {
	if !rc.enabled {
		return nil, false
	}
	val, ok := rc.cache.Get(key)
	if !ok {
		return nil, false
	}
	resp, ok := val.(*CachedResponse)
	return resp, ok
}

func (rc *ResponseCache) Set(key string, resp *CachedResponse) {
	if !rc.enabled {
		return
	}
	rc.cache.Set(key, resp, rc.ttl)
}
