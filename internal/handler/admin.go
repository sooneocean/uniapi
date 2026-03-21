package handler

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/audit"
	"github.com/sooneocean/uniapi/internal/db"
	"github.com/sooneocean/uniapi/internal/repo"
)

// AdminHandler handles admin-only API routes.
type AdminHandler struct {
	database         *db.Database
	convoRepo        *repo.ConversationRepo
	systemPromptRepo *repo.SystemPromptRepo
	audit            *audit.Logger
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(database *db.Database, convoRepo *repo.ConversationRepo, systemPromptRepo *repo.SystemPromptRepo, auditLogger *audit.Logger) *AdminHandler {
	return &AdminHandler{
		database:         database,
		convoRepo:        convoRepo,
		systemPromptRepo: systemPromptRepo,
		audit:            auditLogger,
	}
}

// GET /api/audit-log
func (h *AdminHandler) GetAuditLog(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	if h.audit == nil {
		c.JSON(http.StatusOK, gin.H{"entries": []struct{}{}, "total": 0})
		return
	}
	entries, total, err := h.audit.List(limit, offset)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries, "total": total})
}

// GET /api/backup
func (h *AdminHandler) BackupDB(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	dbPath := h.database.Path()
	if dbPath == "" || dbPath == ":memory:" {
		badRequest(c, "backup not supported for in-memory database")
		return
	}

	backupPath := dbPath + ".backup"
	_, err := h.database.DB.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backup failed: " + err.Error()})
		return
	}
	defer os.Remove(backupPath)

	c.Header("Content-Disposition", "attachment; filename=uniapi-backup.db")
	c.File(backupPath)
}

// GET /api/export
func (h *AdminHandler) ExportUserData(c *gin.Context) {
	uid, _ := c.Get("user_id")
	userID := uid.(string)

	conversations, _ := h.convoRepo.ListByUser(userID)
	var convosWithMessages []gin.H
	for _, conv := range conversations {
		messages, _ := h.convoRepo.GetMessages(conv.ID)
		convosWithMessages = append(convosWithMessages, gin.H{
			"conversation": conv, "messages": messages,
		})
	}

	systemPrompts, _ := h.systemPromptRepo.ListByUser(userID)

	data := gin.H{
		"version":        "1.0",
		"exported_at":    time.Now().UTC().Format(time.RFC3339),
		"conversations":  convosWithMessages,
		"system_prompts": systemPrompts,
	}

	c.Header("Content-Disposition", "attachment; filename=uniapi-export.json")
	c.JSON(200, data)
}

// POST /api/import
func (h *AdminHandler) ImportUserData(c *gin.Context) {
	uid, _ := c.Get("user_id")
	userID := uid.(string)

	var data struct {
		Conversations []struct {
			Conversation struct {
				Title string `json:"title"`
			} `json:"conversation"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
				Model   string `json:"model"`
			} `json:"messages"`
		} `json:"conversations"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		badRequest(c, err.Error())
		return
	}

	imported := 0
	for _, conv := range data.Conversations {
		newConv, err := h.convoRepo.Create(userID, conv.Conversation.Title)
		if err != nil {
			continue
		}
		for _, msg := range conv.Messages {
			h.convoRepo.AddMessage(&repo.MessageRecord{ //nolint:errcheck
				ID:             uuid.New().String(),
				ConversationID: newConv.ID,
				Role:           msg.Role,
				Content:        msg.Content,
				Model:          msg.Model,
			})
		}
		imported++
	}

	c.JSON(200, gin.H{"ok": true, "imported_conversations": imported})
}
