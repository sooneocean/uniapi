package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/repo"
)

// POST /api/conversations/:id/messages
func (h *ConversationHandler) AddMessage(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	convoID := c.Param("id")
	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		notFound(c, "conversation not found")
		return
	}
	if conv.UserID != userID {
		forbidden(c, "forbidden")
		return
	}
	var req struct {
		Role      string  `json:"role"`
		Content   string  `json:"content"`
		Model     string  `json:"model"`
		TokensIn  int     `json:"tokens_in"`
		TokensOut int     `json:"tokens_out"`
		Cost      float64 `json:"cost"`
		LatencyMs int     `json:"latency_ms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	msg := &repo.MessageRecord{
		ID:             uuid.New().String(),
		ConversationID: convoID,
		Role:           req.Role,
		Content:        req.Content,
		Model:          req.Model,
		TokensIn:       req.TokensIn,
		TokensOut:      req.TokensOut,
		Cost:           req.Cost,
		LatencyMs:      req.LatencyMs,
	}
	if err := h.convoRepo.AddMessage(msg); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": msg.ID})
}

// DELETE /api/conversations/:id/messages/:msgId
func (h *ConversationHandler) DeleteMessageAndAfter(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	convoID := c.Param("id")
	msgID := c.Param("msgId")

	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		notFound(c, "conversation not found")
		return
	}
	if conv.UserID != userID {
		forbidden(c, "forbidden")
		return
	}

	if err := h.convoRepo.DeleteMessageAndAfter(convoID, msgID); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	success(c, gin.H{"ok": true})
}

// GET /api/conversations/:id/export?format=markdown|json
func (h *ConversationHandler) ExportConversation(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	convoID := c.Param("id")
	format := c.DefaultQuery("format", "markdown")

	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		notFound(c, "conversation not found")
		return
	}
	if conv.UserID != userID {
		forbidden(c, "forbidden")
		return
	}

	messages, err := h.convoRepo.GetMessages(convoID)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	if messages == nil {
		messages = []repo.MessageRecord{}
	}

	switch format {
	case "json":
		c.JSON(http.StatusOK, gin.H{"conversation": conv, "messages": messages})
	case "markdown":
		var md strings.Builder
		md.WriteString("# " + conv.Title + "\n\n")
		for _, m := range messages {
			role := strings.ToUpper(m.Role[:1]) + m.Role[1:]
			md.WriteString("## " + role + "\n\n")
			md.WriteString(m.Content + "\n\n")
		}
		safeTitle := strings.ReplaceAll(conv.Title, " ", "_")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.md", safeTitle))
		c.Data(http.StatusOK, "text/markdown", []byte(md.String()))
	default:
		badRequest(c, "unsupported format, use markdown or json")
	}
}

// GET /api/shared/:token (public — no auth)
func (h *ConversationHandler) GetSharedConversation(c *gin.Context) {
	token := c.Param("token")
	convo, err := h.convoRepo.GetByShareToken(token)
	if err != nil {
		notFound(c, errNotFound)
		return
	}
	messages, _ := h.convoRepo.GetMessages(convo.ID)
	if messages == nil {
		messages = []repo.MessageRecord{}
	}
	c.JSON(http.StatusOK, gin.H{"conversation": convo, "messages": messages})
}
