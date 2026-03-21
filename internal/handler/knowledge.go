package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/rag"
)

type KnowledgeHandler struct {
	mgr *rag.Manager
}

func NewKnowledgeHandler(mgr *rag.Manager) *KnowledgeHandler {
	return &KnowledgeHandler{mgr: mgr}
}

func (h *KnowledgeHandler) Upload(c *gin.Context) {
	userID := mustUserID(c)
	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
		Shared  bool   `json:"shared"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	doc, err := h.mgr.Upload(userID, req.Title, req.Content, req.Shared)
	if err != nil {
		serverError(c, "operation failed")
		return
	}
	c.JSON(http.StatusCreated, doc)
}

func (h *KnowledgeHandler) List(c *gin.Context) {
	userID := mustUserID(c)
	docs, err := h.mgr.ListDocs(userID)
	if err != nil {
		serverError(c, "operation failed")
		return
	}
	if docs == nil {
		docs = []rag.Document{}
	}
	c.JSON(http.StatusOK, docs)
}

func (h *KnowledgeHandler) Delete(c *gin.Context) {
	userID := mustUserID(c)
	id := c.Param("id")
	if err := h.mgr.DeleteDoc(id, userID); err != nil {
		serverError(c, "operation failed")
		return
	}
	c.Status(http.StatusNoContent)
}

func mustUserID(c *gin.Context) string {
	if uid, exists := c.Get("user_id"); exists {
		if u, ok := uid.(string); ok {
			return u
		}
	}
	return ""
}
