package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/repo"
)

// SystemPromptHandler handles system-prompt API routes.
type SystemPromptHandler struct {
	repo *repo.SystemPromptRepo
}

// NewSystemPromptHandler creates a new SystemPromptHandler.
func NewSystemPromptHandler(systemPromptRepo *repo.SystemPromptRepo) *SystemPromptHandler {
	return &SystemPromptHandler{repo: systemPromptRepo}
}

type systemPromptRequest struct {
	Name      string `json:"name" binding:"required"`
	Content   string `json:"content" binding:"required"`
	IsDefault bool   `json:"is_default"`
}

// GET /api/system-prompts
func (h *SystemPromptHandler) ListSystemPrompts(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	prompts, err := h.repo.ListByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if prompts == nil {
		prompts = []repo.SystemPrompt{}
	}
	c.JSON(http.StatusOK, prompts)
}

// POST /api/system-prompts
func (h *SystemPromptHandler) CreateSystemPrompt(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	var req systemPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sp, err := h.repo.Create(userID, req.Name, req.Content, req.IsDefault)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sp)
}

// PUT /api/system-prompts/:id
func (h *SystemPromptHandler) UpdateSystemPrompt(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	sp, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "system prompt not found"})
		return
	}
	if sp.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req systemPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Update(id, req.Name, req.Content, req.IsDefault); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /api/system-prompts/:id
func (h *SystemPromptHandler) DeleteSystemPrompt(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	sp, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "system prompt not found"})
		return
	}
	if sp.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
