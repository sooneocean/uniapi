package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/cache"
	"github.com/sooneocean/uniapi/internal/memory"
	"github.com/sooneocean/uniapi/internal/metrics"
	"github.com/sooneocean/uniapi/internal/plugin"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/rag"
	"github.com/sooneocean/uniapi/internal/router"
	"github.com/sooneocean/uniapi/internal/usage"
	"github.com/sooneocean/uniapi/internal/webhook"
)

type APIHandler struct {
	router     *router.Router
	recorder   *usage.Recorder
	webhookMgr *webhook.Manager
	respCache  *ResponseCache
	aliasCache *cache.MemCache
	db         *sql.DB
	ragMgr     *rag.Manager
	pluginMgr  *plugin.Manager
	memMgr     *memory.Manager
}

func (h *APIHandler) SetRAGManager(m *rag.Manager) {
	h.ragMgr = m
}

func (h *APIHandler) SetPluginManager(m *plugin.Manager) {
	h.pluginMgr = m
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
						call := &provider.ToolCall{
							ID:   fmt.Sprintf("%v", tcMap["id"]),
							Type: fmt.Sprintf("%v", tcMap["type"]),
						}
						if fn, ok := tcMap["function"].(map[string]interface{}); ok {
							call.Function.Name = fmt.Sprintf("%v", fn["name"])
							call.Function.Arguments = fmt.Sprintf("%v", fn["arguments"])
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
		switch v := m.Content.(type) {
		case string:
			if len(v) > 1_000_000 {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "message content too large"}})
				return
			}
			messages[i] = provider.Message{Role: m.Role, Content: []provider.ContentBlock{{Type: "text", Text: v}}}
		case []interface{}:
			var blocks []provider.ContentBlock
			for _, item := range v {
				if block, ok := item.(map[string]interface{}); ok {
					cb := provider.ContentBlock{Type: fmt.Sprintf("%v", block["type"])}
					if t, ok := block["text"]; ok {
						cb.Text = fmt.Sprintf("%v", t)
					}
					if iu, ok := block["image_url"]; ok {
						if iuMap, ok := iu.(map[string]interface{}); ok {
							cb.ImageURL = fmt.Sprintf("%v", iuMap["url"])
						}
					}
					blocks = append(blocks, cb)
				}
			}
			messages[i] = provider.Message{Role: m.Role, Content: blocks}
		default:
			// Fallback: treat as empty text
			messages[i] = provider.Message{Role: m.Role, Content: []provider.ContentBlock{{Type: "text", Text: ""}}}
		}
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
	if h.db != nil {
		aliasCacheKey := "alias:" + chatReq.Model
		resolved := false
		if h.aliasCache != nil {
			if cached, ok := h.aliasCache.Get(aliasCacheKey); ok {
				if resolvedModel, ok := cached.(string); ok && resolvedModel != "" {
					chatReq.Model = resolvedModel
					resolved = true
				} else {
					// cached miss — skip DB lookup
					resolved = true
				}
			}
		}
		if !resolved {
			var resolvedModel string
			err := h.db.QueryRow(
				"SELECT model_id FROM model_aliases WHERE alias = ? AND (user_id IS NULL OR user_id = ?) ORDER BY user_id DESC LIMIT 1",
				chatReq.Model, userID,
			).Scan(&resolvedModel)
			if err == nil {
				chatReq.Model = resolvedModel
				if h.aliasCache != nil {
					h.aliasCache.Set(aliasCacheKey, resolvedModel, 5*time.Minute)
				}
			} else if h.aliasCache != nil {
				// Cache the miss to avoid repeated DB queries for real model names
				h.aliasCache.Set(aliasCacheKey, "", 5*time.Minute)
			}
		}
	}

	// Quota check
	if userID != "" {
		if err := h.checkQuota(userID); err != nil {
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

	start := time.Now()
	resp, err := h.router.Route(c.Request.Context(), chatReq, userID)
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

func (h *APIHandler) handleStream(c *gin.Context, req *provider.ChatRequest) {
	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if u, ok := uid.(string); ok {
			userID = u
		}
	}
	start := time.Now()
	stream, err := h.router.RouteStream(c.Request.Context(), req, userID)
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

	ctx := c.Request.Context()
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
			chunk := map[string]interface{}{
				"id":      id,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   model,
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{"content": event.Content.Text},
						"finish_reason": nil,
					},
				},
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

func (h *APIHandler) CompareModels(c *gin.Context) {
	var req struct {
		Prompt       string   `json:"prompt" binding:"required"`
		Models       []string `json:"models" binding:"required"`
		SystemPrompt string   `json:"system_prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if len(req.Models) < 2 || len(req.Models) > 4 {
		c.JSON(400, gin.H{"error": "2-4 models required"})
		return
	}

	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if u, ok := uid.(string); ok {
			userID = u
		}
	}

	type result struct {
		Model     string  `json:"model"`
		Content   string  `json:"content"`
		TokensIn  int     `json:"tokens_in"`
		TokensOut int     `json:"tokens_out"`
		LatencyMs int64   `json:"latency_ms"`
		Cost      float64 `json:"cost"`
		Error     string  `json:"error,omitempty"`
	}

	results := make([]result, len(req.Models))
	var wg sync.WaitGroup

	for i, model := range req.Models {
		wg.Add(1)
		go func(idx int, modelName string) {
			defer wg.Done()
			messages := []provider.Message{}
			if req.SystemPrompt != "" {
				messages = append(messages, provider.Message{Role: "system", Content: []provider.ContentBlock{{Type: "text", Text: req.SystemPrompt}}})
			}
			messages = append(messages, provider.Message{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: req.Prompt}}})

			chatReq := &provider.ChatRequest{Model: modelName, Messages: messages, MaxTokens: 4096}
			start := time.Now()
			resp, err := h.router.Route(c.Request.Context(), chatReq, userID)
			latency := time.Since(start)

			if err != nil {
				results[idx] = result{Model: modelName, Error: err.Error()}
				return
			}
			content := ""
			if len(resp.Content) > 0 {
				content = resp.Content[0].Text
			}
			cost := usage.CalculateCost(modelName, resp.TokensIn, resp.TokensOut)
			results[idx] = result{
				Model:     modelName,
				Content:   content,
				TokensIn:  resp.TokensIn,
				TokensOut: resp.TokensOut,
				LatencyMs: latency.Milliseconds(),
				Cost:      cost,
			}
		}(i, model)
	}
	wg.Wait()

	c.JSON(200, gin.H{"results": results})
}

func (h *APIHandler) ListModels(c *gin.Context) {
	models := h.router.AllModels()
	data := make([]gin.H, len(models))
	for i, m := range models {
		data[i] = gin.H{"id": m.ID, "object": "model", "created": time.Now().Unix(), "owned_by": m.Provider}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
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
