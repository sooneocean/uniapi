package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/metrics"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/router"
	"github.com/sooneocean/uniapi/internal/usage"
	"github.com/sooneocean/uniapi/internal/webhook"
)

type APIHandler struct {
	router     *router.Router
	recorder   *usage.Recorder
	webhookMgr *webhook.Manager
	respCache  *ResponseCache
	db         *sql.DB
}

func NewAPIHandler(r *router.Router, rec *usage.Recorder) *APIHandler {
	return &APIHandler{router: r, recorder: rec}
}

func NewAPIHandlerFull(r *router.Router, rec *usage.Recorder, webhookMgr *webhook.Manager, respCache *ResponseCache, db *sql.DB) *APIHandler {
	return &APIHandler{router: r, recorder: rec, webhookMgr: webhookMgr, respCache: respCache, db: db}
}

type chatCompletionRequest struct {
	Model       string     `json:"model" binding:"required"`
	Messages    []chatMsg  `json:"messages" binding:"required"`
	MaxTokens   int        `json:"max_tokens,omitempty"`
	Temperature *float64   `json:"temperature,omitempty"`
	Stream      bool       `json:"stream,omitempty"`
	Provider    string     `json:"provider,omitempty"`
}

type chatMsg struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentBlock
}

func (h *APIHandler) ChatCompletions(c *gin.Context) {
	var req chatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": err.Error()}})
		return
	}
	messages := make([]provider.Message, len(req.Messages))
	for i, m := range req.Messages {
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

	// Resolve model alias
	if h.db != nil {
		var resolvedModel string
		err := h.db.QueryRow(
			"SELECT model_id FROM model_aliases WHERE alias = ? AND (user_id IS NULL OR user_id = ?) ORDER BY user_id DESC LIMIT 1",
			chatReq.Model, userID,
		).Scan(&resolvedModel)
		if err == nil {
			chatReq.Model = resolvedModel
		}
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
	if len(resp.Content) > 0 {
		content = resp.Content[0].Text
	}

	// Store in response cache
	if h.respCache != nil {
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

	c.JSON(http.StatusOK, gin.H{
		"id": "chatcmpl-" + uuid.New().String()[:8], "object": "chat.completion",
		"created": time.Now().Unix(), "model": resp.Model,
		"choices": []gin.H{{"index": 0, "message": gin.H{"role": "assistant", "content": content}, "finish_reason": "stop"}},
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
			fmt.Fprintf(w, "data: [DONE]\n\n")
			w.Flush()
			if event.Response != nil {
				tokensIn = event.Response.TokensIn
				tokensOut = event.Response.TokensOut
			}
			break
		}
		if event.Type == "content_delta" {
			chunk := gin.H{
				"id":      id,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   model,
				"choices": []gin.H{
					{
						"index":         0,
						"delta":         gin.H{"content": event.Content.Text},
						"finish_reason": nil,
					},
				},
			}
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", b)
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
