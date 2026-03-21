package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/router"
	"github.com/sooneocean/uniapi/internal/workflow"
)

type WorkflowsHandler struct {
	db     *sql.DB
	router *router.Router
}

func NewWorkflowsHandler(db *sql.DB, rtr *router.Router) *WorkflowsHandler {
	return &WorkflowsHandler{db: db, router: rtr}
}

// GET /api/workflows — list user's + shared
func (h *WorkflowsHandler) List(c *gin.Context) {
	userID := mustUserID(c)
	rows, err := h.db.Query(
		`SELECT id, user_id, name, description, steps, shared, run_count
		 FROM workflows WHERE user_id = ? OR shared = 1 ORDER BY created_at DESC`, userID)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	defer rows.Close()

	var workflows []workflow.Workflow
	for rows.Next() {
		var wf workflow.Workflow
		var stepsJSON string
		var sharedInt int
		if err := rows.Scan(&wf.ID, &wf.UserID, &wf.Name, &wf.Description, &stepsJSON, &sharedInt, &wf.RunCount); err != nil {
			continue
		}
		wf.Shared = sharedInt == 1
		json.Unmarshal([]byte(stepsJSON), &wf.Steps)
		workflows = append(workflows, wf)
	}
	if workflows == nil {
		workflows = []workflow.Workflow{}
	}
	c.JSON(http.StatusOK, workflows)
}

// POST /api/workflows — create
func (h *WorkflowsHandler) Create(c *gin.Context) {
	userID := mustUserID(c)
	var req struct {
		Name        string          `json:"name" binding:"required"`
		Description string          `json:"description"`
		Steps       []workflow.Step `json:"steps"`
		Shared      bool            `json:"shared"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	stepsJSON, _ := json.Marshal(req.Steps)
	id := uuid.New().String()
	_, err := h.db.Exec(
		`INSERT INTO workflows (id, user_id, name, description, steps, shared) VALUES (?,?,?,?,?,?)`,
		id, userID, req.Name, req.Description, string(stepsJSON), req.Shared)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.JSON(http.StatusCreated, workflow.Workflow{
		ID: id, UserID: userID, Name: req.Name, Description: req.Description,
		Steps: req.Steps, Shared: req.Shared,
	})
}

// PUT /api/workflows/:id — update
func (h *WorkflowsHandler) Update(c *gin.Context) {
	userID := mustUserID(c)
	id := c.Param("id")

	var ownerID string
	if err := h.db.QueryRow("SELECT user_id FROM workflows WHERE id = ?", id).Scan(&ownerID); err != nil {
		if err == sql.ErrNoRows {
			notFound(c, "workflow not found")
			return
		}
		serverError(c, "database error")
		return
	}
	if ownerID != userID {
		forbidden(c, "forbidden")
		return
	}

	var req struct {
		Name        string          `json:"name" binding:"required"`
		Description string          `json:"description"`
		Steps       []workflow.Step `json:"steps"`
		Shared      bool            `json:"shared"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	stepsJSON, _ := json.Marshal(req.Steps)
	_, err := h.db.Exec(
		`UPDATE workflows SET name=?, description=?, steps=?, shared=? WHERE id=?`,
		req.Name, req.Description, string(stepsJSON), req.Shared, id)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	success(c, gin.H{"ok": true})
}

// DELETE /api/workflows/:id — delete
func (h *WorkflowsHandler) Delete(c *gin.Context) {
	userID := mustUserID(c)
	id := c.Param("id")

	var ownerID string
	if err := h.db.QueryRow("SELECT user_id FROM workflows WHERE id = ?", id).Scan(&ownerID); err != nil {
		if err == sql.ErrNoRows {
			notFound(c, "workflow not found")
			return
		}
		serverError(c, "database error")
		return
	}
	if ownerID != userID {
		forbidden(c, "forbidden")
		return
	}

	h.db.Exec("DELETE FROM workflows WHERE id = ?", id)
	c.Status(http.StatusNoContent)
}

// POST /api/workflows/:id/run — execute
func (h *WorkflowsHandler) Run(c *gin.Context) {
	userID := mustUserID(c)
	id := c.Param("id")

	var wf workflow.Workflow
	var stepsJSON string
	var sharedInt int
	err := h.db.QueryRow(
		`SELECT id, user_id, name, description, steps, shared, run_count FROM workflows WHERE id = ? AND (user_id = ? OR shared = 1)`,
		id, userID).Scan(&wf.ID, &wf.UserID, &wf.Name, &wf.Description, &stepsJSON, &sharedInt, &wf.RunCount)
	if err == sql.ErrNoRows {
		notFound(c, "workflow not found")
		return
	}
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	wf.Shared = sharedInt == 1
	json.Unmarshal([]byte(stepsJSON), &wf.Steps)

	var req struct {
		Input string `json:"input" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	rtr := h.router
	ctx := c.Request.Context()
	routeFn := workflow.RouteFn(func(fnCtx context.Context, chatReq *provider.ChatRequest, uid string) (*provider.ChatResponse, error) {
		return rtr.Route(fnCtx, chatReq, uid)
	})

	result, err := workflow.Execute(ctx, wf, req.Input, routeFn, userID)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}

	// Increment run_count
	h.db.Exec("UPDATE workflows SET run_count = run_count + 1 WHERE id = ?", id)

	c.JSON(http.StatusOK, result)
}
