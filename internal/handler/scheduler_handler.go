package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/scheduler"
)

// SchedulerHandler handles scheduled task API routes.
type SchedulerHandler struct {
	db    *sql.DB
	sched *scheduler.Scheduler
}

// NewSchedulerHandler creates a SchedulerHandler.
func NewSchedulerHandler(db *sql.DB, sched *scheduler.Scheduler) *SchedulerHandler {
	return &SchedulerHandler{db: db, sched: sched}
}

// GET /api/scheduled
func (h *SchedulerHandler) List(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	tasks, err := h.sched.List(userID)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

type createScheduledTaskRequest struct {
	Model        string `json:"model" binding:"required"`
	Prompt       string `json:"prompt" binding:"required"`
	SystemPrompt string `json:"system_prompt"`
	RunAt        string `json:"run_at" binding:"required"`
}

// POST /api/scheduled
func (h *SchedulerHandler) Create(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	var req createScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	runAt, err := time.Parse(time.RFC3339, req.RunAt)
	if err != nil {
		badRequest(c, "invalid run_at format, use RFC3339 (e.g. 2026-03-22T09:00:00Z)")
		return
	}

	id, err := h.sched.Create(userID, req.Model, req.Prompt, req.SystemPrompt, runAt)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "status": "pending"})
}

// DELETE /api/scheduled/:id
func (h *SchedulerHandler) Delete(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := h.sched.Delete(id, userID); err != nil {
		serverError(c, errOperationFailed)
		return
	}
	success(c, gin.H{"ok": true})
}

// GET /api/scheduled/:id/result
func (h *SchedulerHandler) GetResult(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	task, err := h.sched.GetResult(id, userID)
	if err != nil {
		notFound(c, errNotFound)
		return
	}
	success(c, task)
}
