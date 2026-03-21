package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/audit"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/repo"
	"github.com/sooneocean/uniapi/internal/router"
)

func (h *ConversationHandler) requireOwner(c *gin.Context, convoID, userID string) (*repo.Conversation, bool) {
	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		notFound(c, errNotFound)
		return nil, false
	}
	if conv.UserID != userID {
		forbidden(c, "forbidden")
		return nil, false
	}
	return conv, true
}

// ConversationHandler handles conversation-related API routes.
type ConversationHandler struct {
	convoRepo *repo.ConversationRepo
	router    *router.Router
	audit     *audit.Logger
}

// NewConversationHandler creates a new ConversationHandler.
func NewConversationHandler(convoRepo *repo.ConversationRepo, rtr *router.Router, auditLogger *audit.Logger) *ConversationHandler {
	return &ConversationHandler{
		convoRepo: convoRepo,
		router:    rtr,
		audit:     auditLogger,
	}
}

// GET /api/conversations
func (h *ConversationHandler) ListConversations(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	convs, err := h.convoRepo.ListByUserWithPreview(userID)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	if convs == nil {
		convs = []repo.ConversationWithPreview{}
	}
	c.JSON(http.StatusOK, convs)
}

type createConversationRequest struct {
	Title string `json:"title" binding:"required"`
}

// POST /api/conversations
func (h *ConversationHandler) CreateConversation(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	var req createConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if len(req.Title) > 500 {
		badRequest(c, errTitleTooLong)
		return
	}
	conv, err := h.convoRepo.Create(userID, req.Title)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.JSON(http.StatusCreated, conv)
}

// GET /api/conversations/:id
func (h *ConversationHandler) GetConversation(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	conv, ok := h.requireOwner(c, id, userID)
	if !ok {
		return
	}
	msgs, err := h.convoRepo.GetMessages(id)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	if msgs == nil {
		msgs = []repo.MessageRecord{}
	}
	c.JSON(http.StatusOK, gin.H{
		"id":         conv.ID,
		"user_id":    conv.UserID,
		"title":      conv.Title,
		"created_at": conv.CreatedAt,
		"updated_at": conv.UpdatedAt,
		"messages":   msgs,
	})
}

type updateConversationRequest struct {
	Title string `json:"title" binding:"required"`
}

// PUT /api/conversations/:id
func (h *ConversationHandler) UpdateConversation(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if _, ok := h.requireOwner(c, id, userID); !ok {
		return
	}
	var req updateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if len(req.Title) > 500 {
		badRequest(c, errTitleTooLong)
		return
	}
	if err := h.convoRepo.UpdateTitle(id, req.Title); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	success(c, gin.H{"ok": true})
}

// DELETE /api/conversations/:id
func (h *ConversationHandler) DeleteConversation(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if _, ok := h.requireOwner(c, id, userID); !ok {
		return
	}
	if err := h.convoRepo.Delete(id); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	success(c, gin.H{"ok": true})
}

// POST /api/conversations/:id/share
func (h *ConversationHandler) ShareConversation(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	convoID := c.Param("id")
	if _, ok := h.requireOwner(c, convoID, userID); !ok {
		return
	}
	token := uuid.New().String()[:12]
	if err := h.convoRepo.SetShareToken(convoID, token); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.JSON(http.StatusOK, gin.H{"share_url": "/shared/" + token})
}

// DELETE /api/conversations/:id/share
func (h *ConversationHandler) UnshareConversation(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	convoID := c.Param("id")
	if _, ok := h.requireOwner(c, convoID, userID); !ok {
		return
	}
	if err := h.convoRepo.SetShareToken(convoID, ""); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	success(c, gin.H{"ok": true})
}

// PUT /api/conversations/:id/folder
func (h *ConversationHandler) UpdateConversationFolder(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if _, ok := h.requireOwner(c, id, userID); !ok {
		return
	}
	var req struct {
		Folder string `json:"folder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := h.convoRepo.UpdateFolder(id, req.Folder); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	success(c, gin.H{"ok": true})
}

// PUT /api/conversations/:id/pin
func (h *ConversationHandler) ToggleConversationPin(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if _, ok := h.requireOwner(c, id, userID); !ok {
		return
	}
	if err := h.convoRepo.TogglePin(id); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	success(c, gin.H{"ok": true})
}

// POST /api/conversations/:id/auto-title
func (h *ConversationHandler) AutoTitle(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	convoID := c.Param("id")
	if _, ok := h.requireOwner(c, convoID, userID); !ok {
		return
	}
	messages, _ := h.convoRepo.GetMessages(convoID)
	if len(messages) < 2 {
		badRequest(c, "need at least one exchange")
		return
	}

	summary := messages[0].Content
	if len(summary) > 200 {
		summary = summary[:200]
	}

	titleReq := &provider.ChatRequest{
		Model: "",
		Messages: []provider.Message{
			{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: "Generate a short title (max 6 words, no quotes) for a conversation that starts with: " + summary}}},
		},
		MaxTokens: 20,
	}

	var title string
	if h.router != nil {
		resp, routeErr := h.router.Route(c.Request.Context(), titleReq, userID)
		if routeErr == nil && len(resp.Content) > 0 {
			title = resp.Content[0].Text
			if len(title) > 100 {
				title = title[:100]
			}
		}
	}

	if title == "" {
		title = summary
		if len(title) > 50 {
			title = title[:50] + "..."
		}
	}

	h.convoRepo.UpdateTitle(convoID, title) //nolint:errcheck
	c.JSON(http.StatusOK, gin.H{"title": title})
}
