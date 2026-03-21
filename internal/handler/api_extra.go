package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/usage"
)

// CompareModels runs the same prompt against multiple models in parallel and returns results.
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

	// Enforce maximum compare timeout
	cmpCtx, cmpCancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cmpCancel()

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
			resp, err := h.router.Route(cmpCtx, chatReq, userID)
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

// ListModels returns the list of all available models across registered providers.
func (h *APIHandler) ListModels(c *gin.Context) {
	models := h.router.AllModels()
	data := make([]gin.H, len(models))
	for i, m := range models {
		data[i] = gin.H{"id": m.ID, "object": "model", "created": time.Now().Unix(), "owned_by": m.Provider}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}
