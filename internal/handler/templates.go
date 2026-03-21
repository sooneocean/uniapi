package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/db"
	"github.com/sooneocean/uniapi/internal/repo"
)

type TemplatesHandler struct {
	templateRepo *repo.TemplateRepo
}

func NewTemplatesHandler(database *db.Database) *TemplatesHandler {
	return &TemplatesHandler{
		templateRepo: repo.NewTemplateRepo(database),
	}
}

// GET /api/templates
func (h *TemplatesHandler) List(c *gin.Context) {
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": "not authenticated"}})
		return
	}
	userID, _ := uid.(string)

	templates, err := h.templateRepo.List(userID)
	if err != nil {
		serverError(c, "operation failed")
		return
	}
	if templates == nil {
		templates = []repo.PromptTemplate{}
	}
	c.JSON(http.StatusOK, templates)
}

type templateRequest struct {
	Title        string `json:"title" binding:"required"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt" binding:"required"`
	UserPrompt   string `json:"user_prompt"`
	Tags         string `json:"tags"`
	Shared       bool   `json:"shared"`
}

// POST /api/templates
func (h *TemplatesHandler) Create(c *gin.Context) {
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": "not authenticated"}})
		return
	}
	userID, _ := uid.(string)

	var req templateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	tmpl, err := h.templateRepo.Create(userID, req.Title, req.Description, req.SystemPrompt, req.UserPrompt, req.Tags, req.Shared)
	if err != nil {
		serverError(c, "operation failed")
		return
	}
	c.JSON(http.StatusCreated, tmpl)
}

// PUT /api/templates/:id
func (h *TemplatesHandler) Update(c *gin.Context) {
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": "not authenticated"}})
		return
	}
	userID, _ := uid.(string)
	id := c.Param("id")

	existing, err := h.templateRepo.GetByID(id)
	if err != nil {
		notFound(c, "template not found")
		return
	}
	if existing.UserID != userID {
		forbidden(c, "forbidden")
		return
	}

	var req templateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	if err := h.templateRepo.Update(id, req.Title, req.Description, req.SystemPrompt, req.UserPrompt, req.Tags, req.Shared); err != nil {
		serverError(c, "operation failed")
		return
	}
	success(c, gin.H{"ok": true})
}

// DELETE /api/templates/:id
func (h *TemplatesHandler) Delete(c *gin.Context) {
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": "not authenticated"}})
		return
	}
	userID, _ := uid.(string)
	id := c.Param("id")

	existing, err := h.templateRepo.GetByID(id)
	if err != nil {
		notFound(c, "template not found")
		return
	}
	if existing.UserID != userID {
		forbidden(c, "forbidden")
		return
	}

	if err := h.templateRepo.Delete(id); err != nil {
		serverError(c, "operation failed")
		return
	}
	success(c, gin.H{"ok": true})
}

// POST /api/templates/:id/use
func (h *TemplatesHandler) Use(c *gin.Context) {
	id := c.Param("id")

	tmpl, err := h.templateRepo.IncrementUseCount(id)
	if err != nil {
		notFound(c, "template not found")
		return
	}
	c.JSON(http.StatusOK, tmpl)
}
