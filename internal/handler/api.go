package handler

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/user/uniapi/internal/provider"
    "github.com/user/uniapi/internal/router"
)

type APIHandler struct {
    router *router.Router
}

func NewAPIHandler(r *router.Router) *APIHandler {
    return &APIHandler{router: r}
}

type chatCompletionRequest struct {
    Model       string      `json:"model" binding:"required"`
    Messages    []chatMsg   `json:"messages" binding:"required"`
    MaxTokens   int         `json:"max_tokens,omitempty"`
    Temperature *float64    `json:"temperature,omitempty"`
    Stream      bool        `json:"stream,omitempty"`
    Provider    string      `json:"provider,omitempty"`
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
    messages := make([]provider.Message, len(req.Messages))
    for i, m := range req.Messages {
        messages[i] = provider.Message{Role: m.Role, Content: []provider.ContentBlock{{Type: "text", Text: m.Content}}}
    }
    chatReq := &provider.ChatRequest{
        Model: req.Model, Messages: messages, MaxTokens: req.MaxTokens,
        Temperature: req.Temperature, Stream: req.Stream, Provider: req.Provider,
    }
    start := time.Now()
    resp, err := h.router.Route(c.Request.Context(), chatReq)
    latency := time.Since(start)
    if err != nil {
        c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "api_error", "message": err.Error()}})
        return
    }
    content := ""
    if len(resp.Content) > 0 { content = resp.Content[0].Text }
    c.JSON(http.StatusOK, gin.H{
        "id": "chatcmpl-" + uuid.New().String()[:8], "object": "chat.completion",
        "created": time.Now().Unix(), "model": resp.Model,
        "choices": []gin.H{{"index": 0, "message": gin.H{"role": "assistant", "content": content}, "finish_reason": "stop"}},
        "usage": gin.H{"prompt_tokens": resp.TokensIn, "completion_tokens": resp.TokensOut, "total_tokens": resp.TokensIn + resp.TokensOut},
        "x_uniapi": gin.H{"latency_ms": latency.Milliseconds()},
    })
}

func (h *APIHandler) ListModels(c *gin.Context) {
    models := h.router.AllModels()
    data := make([]gin.H, len(models))
    for i, m := range models {
        data[i] = gin.H{"id": m.ID, "object": "model", "created": time.Now().Unix(), "owned_by": m.Provider}
    }
    c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}
