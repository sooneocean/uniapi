package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/quota"
)

// QuotaHandler exposes quota status and admin management endpoints.
type QuotaHandler struct {
	engine *quota.Engine
}

// NewQuotaHandler creates a new QuotaHandler.
func NewQuotaHandler(engine *quota.Engine) *QuotaHandler {
	return &QuotaHandler{engine: engine}
}

// GET /api/quota — returns the calling user's current quota usage and limits.
func (h *QuotaHandler) GetQuota(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	result := h.engine.Check(userID)
	c.JSON(http.StatusOK, result)
}

// PUT /api/admin/users/:id/quota — admin sets per-user quota limits.
func (h *QuotaHandler) SetUserQuota(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	id := c.Param("id")
	var req struct {
		DailyCostLimit   float64 `json:"daily_cost_limit"`
		MonthlyCostLimit float64 `json:"monthly_cost_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := h.engine.SetUserQuota(id, req.DailyCostLimit, req.MonthlyCostLimit); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	success(c, gin.H{"ok": true})
}
