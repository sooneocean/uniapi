package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/plugin"
)

type PluginHandler struct {
	mgr *plugin.Manager
}

func NewPluginHandler(mgr *plugin.Manager) *PluginHandler {
	return &PluginHandler{mgr: mgr}
}

func (h *PluginHandler) List(c *gin.Context) {
	userID := mustUserID(c)
	plugins, err := h.mgr.List(userID)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	if plugins == nil {
		plugins = []plugin.Plugin{}
	}
	c.JSON(http.StatusOK, plugins)
}

func (h *PluginHandler) Register(c *gin.Context) {
	userID := mustUserID(c)
	var req struct {
		Name        string            `json:"name" binding:"required"`
		Description string            `json:"description" binding:"required"`
		Endpoint    string            `json:"endpoint" binding:"required"`
		Method      string            `json:"method"`
		Headers     map[string]string `json:"headers"`
		InputSchema json.RawMessage   `json:"input_schema" binding:"required"`
		Shared      bool              `json:"shared"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if req.Method == "" {
		req.Method = "POST"
	}
	if req.Headers == nil {
		req.Headers = map[string]string{}
	}
	p, err := h.mgr.Register(userID, req.Name, req.Description, req.Endpoint, req.Method, req.Headers, req.InputSchema, req.Shared)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *PluginHandler) Delete(c *gin.Context) {
	userID := mustUserID(c)
	id := c.Param("id")
	if err := h.mgr.Delete(id, userID); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PluginHandler) Test(c *gin.Context) {
	userID := mustUserID(c)
	id := c.Param("id")
	p, err := h.mgr.GetByID(id, userID)
	if err != nil {
		notFound(c, "plugin not found")
		return
	}
	var input json.RawMessage
	if err := c.ShouldBindJSON(&input); err != nil {
		input = json.RawMessage("{}")
	}
	result, err := h.mgr.Execute(*p, string(input))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result})
}
