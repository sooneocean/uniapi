package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/metrics"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/router"
	"github.com/sooneocean/uniapi/internal/usage"
)

type APIHandler struct {
	router   *router.Router
	recorder *usage.Recorder
}

func NewAPIHandler(r *router.Router, rec *usage.Recorder) *APIHandler {
	return &APIHandler{router: r, recorder: rec}
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
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (h *APIHandler) ChatCompletions(c *gin.Context) {
	var req chatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": err.Error()}})
		return
	}
	for _, m := range req.Messages {
		if len(m.Content) > 1_000_000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "message content too large"}})
			return
		}
	}
	messages := make([]provider.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = provider.Message{Role: m.Role, Content: []provider.ContentBlock{{Type: "text", Text: m.Content}}}
	}
	chatReq := &provider.ChatRequest{
		Model: req.Model, Messages: messages, MaxTokens: req.MaxTokens,
		Temperature: req.Temperature, Stream: req.Stream, Provider: req.Provider,
	}

	if req.Stream {
		h.handleStream(c, chatReq)
		return
	}

	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if u, ok := uid.(string); ok {
			userID = u
		}
	}
	start := time.Now()
	resp, err := h.router.Route(c.Request.Context(), chatReq, userID)
	latency := time.Since(start)
	if err != nil {
		metrics.ProviderRequestsTotal.WithLabelValues(req.Model, req.Model, "error").Inc()
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "api_error", "message": err.Error()}})
		return
	}
	content := ""
	if len(resp.Content) > 0 {
		content = resp.Content[0].Text
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

	w := c.Writer
	for {
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

func (h *APIHandler) ListModels(c *gin.Context) {
	models := h.router.AllModels()
	data := make([]gin.H, len(models))
	for i, m := range models {
		data[i] = gin.H{"id": m.ID, "object": "model", "created": time.Now().Unix(), "owned_by": m.Provider}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}
