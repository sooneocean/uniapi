package handler

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/audit"
	"github.com/sooneocean/uniapi/internal/auth"
	"github.com/sooneocean/uniapi/internal/db"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/repo"
	"github.com/sooneocean/uniapi/internal/router"
	"github.com/sooneocean/uniapi/internal/usage"
)

type SettingsHandler struct {
	accountRepo      *repo.AccountRepo
	userRepo         *repo.UserRepo
	convoRepo        *repo.ConversationRepo
	systemPromptRepo *repo.SystemPromptRepo
	recorder         *usage.Recorder
	database         *db.Database
	audit            *audit.Logger
	registerAccount  func(acc *repo.Account)
	router           *router.Router
}

func NewSettingsHandler(
	accountRepo *repo.AccountRepo,
	userRepo *repo.UserRepo,
	convoRepo *repo.ConversationRepo,
	recorder *usage.Recorder,
	database *db.Database,
	auditLogger *audit.Logger,
	registerAccount func(acc *repo.Account),
	rtr *router.Router,
) *SettingsHandler {
	return &SettingsHandler{
		accountRepo:      accountRepo,
		userRepo:         userRepo,
		convoRepo:        convoRepo,
		systemPromptRepo: repo.NewSystemPromptRepo(database),
		recorder:         recorder,
		database:         database,
		audit:            auditLogger,
		registerAccount:  registerAccount,
		router:           rtr,
	}
}

func requireAdmin(c *gin.Context) bool {
	roleVal, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return false
	}
	role, ok := roleVal.(string)
	if !ok || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return false
	}
	return true
}

// ─── Provider management ─────────────────────────────────────────────────────

func (h *SettingsHandler) ListProviders(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	accounts, err := h.accountRepo.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, len(accounts))
	for i, a := range accounts {
		out[i] = gin.H{
			"id":             a.ID,
			"provider":       a.Provider,
			"label":          a.Label,
			"models":         a.Models,
			"max_concurrent": a.MaxConcurrent,
			"enabled":        a.Enabled,
			"config_managed": a.ConfigManaged,
			"created_at":     a.CreatedAt,
		}
	}
	c.JSON(http.StatusOK, out)
}

type addProviderRequest struct {
	Provider      string   `json:"provider" binding:"required"`
	Label         string   `json:"label" binding:"required"`
	APIKey        string   `json:"api_key" binding:"required"`
	Models        []string `json:"models"`
	MaxConcurrent int      `json:"max_concurrent"`
}

func (h *SettingsHandler) AddProvider(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	var req addProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Label) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label too long"})
		return
	}
	maxConc := req.MaxConcurrent
	if maxConc == 0 {
		maxConc = 5
	}
	account, err := h.accountRepo.Create(req.Provider, req.Label, req.APIKey, req.Models, maxConc, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.registerAccount != nil {
		h.registerAccount(account)
	}

	if h.audit != nil {
		uid, _ := c.Get("user_id")
		userID, _ := uid.(string)
		h.audit.Log(userID, "", "create_provider", "provider", account.ID, account.Label, c.ClientIP())
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":             account.ID,
		"provider":       account.Provider,
		"label":          account.Label,
		"models":         account.Models,
		"max_concurrent": account.MaxConcurrent,
		"enabled":        account.Enabled,
		"config_managed": account.ConfigManaged,
		"created_at":     account.CreatedAt,
	})
}

func (h *SettingsHandler) DeleteProvider(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	id := c.Param("id")
	account, err := h.accountRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}
	if account.ConfigManaged {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete config-managed provider"})
		return
	}
	if err := h.accountRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.audit != nil {
		uid, _ := c.Get("user_id")
		userID, _ := uid.(string)
		h.audit.Log(userID, "", "delete_provider", "provider", id, "", c.ClientIP())
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /api/provider-templates
func (h *SettingsHandler) ListTemplates(c *gin.Context) {
	c.JSON(http.StatusOK, provider.Templates)
}

// ─── User management ──────────────────────────────────────────────────────────

func (h *SettingsHandler) ListUsers(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	users, err := h.userRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, len(users))
	for i, u := range users {
		out[i] = gin.H{
			"id":         u.ID,
			"username":   u.Username,
			"role":       u.Role,
			"created_at": u.CreatedAt,
		}
	}
	c.JSON(http.StatusOK, out)
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`
}

func (h *SettingsHandler) CreateUser(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Username) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username too long"})
		return
	}
	if err := validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	role := req.Role
	if role == "" {
		role = "member"
	}
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	user, err := h.userRepo.Create(req.Username, passwordHash, role)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.audit != nil {
		uid, _ := c.Get("user_id")
		userID, _ := uid.(string)
		h.audit.Log(userID, "", "create_user", "user", user.ID, user.Username, c.ClientIP())
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"role":       user.Role,
		"created_at": user.CreatedAt,
	})
}

func (h *SettingsHandler) DeleteUser(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	id := c.Param("id")

	// Prevent self-deletion
	uid, _ := c.Get("user_id")
	if currentUID, ok := uid.(string); ok && currentUID == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete your own account"})
		return
	}

	if err := h.userRepo.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if h.audit != nil {
		uid, _ := c.Get("user_id")
		userID, _ := uid.(string)
		h.audit.Log(userID, "", "delete_user", "user", id, "", c.ClientIP())
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── API Key management ───────────────────────────────────────────────────────

func (h *SettingsHandler) ListAPIKeys(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	rows, err := h.database.DB.Query(
		"SELECT id, label, created_at, expires_at FROM api_keys WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	type keyItem struct {
		ID        string     `json:"id"`
		Label     string     `json:"label"`
		CreatedAt time.Time  `json:"created_at"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	var keys []keyItem
	for rows.Next() {
		var k keyItem
		var expiresAt *time.Time
		if err := rows.Scan(&k.ID, &k.Label, &k.CreatedAt, &expiresAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		k.ExpiresAt = expiresAt
		keys = append(keys, k)
	}
	if keys == nil {
		keys = []keyItem{}
	}
	c.JSON(http.StatusOK, keys)
}

type createAPIKeyRequest struct {
	Label string `json:"label" binding:"required"`
}

func (h *SettingsHandler) CreateAPIKey(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	key, err := auth.GenerateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
		return
	}
	hash := auth.HashAPIKey(key)
	id := uuid.New().String()
	now := time.Now()
	_, err = h.database.DB.Exec(
		"INSERT INTO api_keys (id, user_id, key_hash, label, created_at) VALUES (?, ?, ?, ?, ?)",
		id, userID, hash, req.Label, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.audit != nil {
		h.audit.Log(userID, "", "create_api_key", "api_key", id, req.Label, c.ClientIP())
	}

	c.JSON(http.StatusCreated, gin.H{"key": key, "id": id})
}

func (h *SettingsHandler) DeleteAPIKey(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	id := c.Param("id")
	result, err := h.database.DB.Exec(
		"DELETE FROM api_keys WHERE id = ? AND user_id = ?",
		id, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "api key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── Conversation management ──────────────────────────────────────────────────

func (h *SettingsHandler) ListConversations(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	convs, err := h.convoRepo.ListByUserWithPreview(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

func (h *SettingsHandler) CreateConversation(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	var req createConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Title) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title too long"})
		return
	}
	conv, err := h.convoRepo.Create(userID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, conv)
}

func (h *SettingsHandler) GetConversation(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	id := c.Param("id")
	conv, err := h.convoRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	msgs, err := h.convoRepo.GetMessages(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

func (h *SettingsHandler) UpdateConversation(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	id := c.Param("id")
	conv, err := h.convoRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req updateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Title) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title too long"})
		return
	}
	if err := h.convoRepo.UpdateTitle(id, req.Title); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *SettingsHandler) AddMessage(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	convoID := c.Param("id")
	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": msg.ID})
}

func (h *SettingsHandler) DeleteConversation(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	id := c.Param("id")
	conv, err := h.convoRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.convoRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── Conversation folders & pins ──────────────────────────────────────────────

// PUT /api/conversations/:id/folder
func (h *SettingsHandler) UpdateConversationFolder(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, _ := userIDVal.(string)
	id := c.Param("id")
	conv, err := h.convoRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		Folder string `json:"folder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.convoRepo.UpdateFolder(id, req.Folder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// PUT /api/conversations/:id/pin
func (h *SettingsHandler) ToggleConversationPin(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, _ := userIDVal.(string)
	id := c.Param("id")
	conv, err := h.convoRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.convoRepo.TogglePin(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── Conversation sharing ─────────────────────────────────────────────────────

// POST /api/conversations/:id/share
func (h *SettingsHandler) ShareConversation(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, _ := userIDVal.(string)
	convoID := c.Param("id")
	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	token := uuid.New().String()[:12]
	if err := h.convoRepo.SetShareToken(convoID, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"share_url": "/shared/" + token})
}

// DELETE /api/conversations/:id/share
func (h *SettingsHandler) UnshareConversation(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, _ := userIDVal.(string)
	convoID := c.Param("id")
	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.convoRepo.SetShareToken(convoID, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /api/shared/:token (public — no auth)
func (h *SettingsHandler) GetSharedConversation(c *gin.Context) {
	token := c.Param("token")
	convo, err := h.convoRepo.GetByShareToken(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	messages, _ := h.convoRepo.GetMessages(convo.ID)
	if messages == nil {
		messages = []repo.MessageRecord{}
	}
	c.JSON(http.StatusOK, gin.H{"conversation": convo, "messages": messages})
}

// ─── Usage ────────────────────────────────────────────────────────────────────

func dateRangeFromQuery(c *gin.Context) (time.Time, time.Time) {
	rangeParam := c.Query("range")
	now := time.Now()
	var from time.Time
	switch rangeParam {
	case "weekly":
		from = now.AddDate(0, 0, -7)
	case "monthly":
		from = now.AddDate(0, -1, 0)
	default: // daily
		from = now.AddDate(0, 0, -1)
	}
	return from, now
}

func (h *SettingsHandler) GetUsage(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	from, to := dateRangeFromQuery(c)
	results, err := h.recorder.GetUserUsage(userID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []usage.DailyUsage{}
	}
	c.JSON(http.StatusOK, results)
}

// ─── Audit log ────────────────────────────────────────────────────────────────

func (h *SettingsHandler) GetAuditLog(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries, "total": total})
}

func (h *SettingsHandler) GetAllUsage(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	from, to := dateRangeFromQuery(c)
	results, err := h.recorder.GetAllUsage(from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []usage.UserUsageSummary{}
	}
	c.JSON(http.StatusOK, results)
}

// ─── Message edit / regenerate ────────────────────────────────────────────────

// DELETE /api/conversations/:id/messages/:msgId
func (h *SettingsHandler) DeleteMessageAndAfter(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	convoID := c.Param("id")
	msgID := c.Param("msgId")

	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if err := h.convoRepo.DeleteMessageAndAfter(convoID, msgID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── Conversation export ──────────────────────────────────────────────────────

// GET /api/conversations/:id/export?format=markdown|json
func (h *SettingsHandler) ExportConversation(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	convoID := c.Param("id")
	format := c.DefaultQuery("format", "markdown")

	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	messages, err := h.convoRepo.GetMessages(convoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported format, use markdown or json"})
	}
}

// ─── System prompts ───────────────────────────────────────────────────────────

// GET /api/system-prompts
func (h *SettingsHandler) ListSystemPrompts(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	prompts, err := h.systemPromptRepo.ListByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if prompts == nil {
		prompts = []repo.SystemPrompt{}
	}
	c.JSON(http.StatusOK, prompts)
}

type systemPromptRequest struct {
	Name      string `json:"name" binding:"required"`
	Content   string `json:"content" binding:"required"`
	IsDefault bool   `json:"is_default"`
}

// POST /api/system-prompts
func (h *SettingsHandler) CreateSystemPrompt(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	var req systemPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sp, err := h.systemPromptRepo.Create(userID, req.Name, req.Content, req.IsDefault)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sp)
}

// PUT /api/system-prompts/:id
func (h *SettingsHandler) UpdateSystemPrompt(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	id := c.Param("id")
	sp, err := h.systemPromptRepo.GetByID(id)
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
	if err := h.systemPromptRepo.Update(id, req.Name, req.Content, req.IsDefault); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /api/system-prompts/:id
func (h *SettingsHandler) DeleteSystemPrompt(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	id := c.Param("id")
	sp, err := h.systemPromptRepo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "system prompt not found"})
		return
	}
	if sp.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := h.systemPromptRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── Auto-title ───────────────────────────────────────────────────────────────

// POST /api/conversations/:id/auto-title
func (h *SettingsHandler) AutoTitle(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	convoID := c.Param("id")
	conv, err := h.convoRepo.GetByID(convoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if conv.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	messages, _ := h.convoRepo.GetMessages(convoID)
	if len(messages) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "need at least one exchange"})
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

// ─── Admin dashboard ──────────────────────────────────────────────────────────

// GET /api/dashboard
func (h *SettingsHandler) Dashboard(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var totalUsers int
	h.database.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers) //nolint:errcheck

	var totalConversations int
	h.database.DB.QueryRow("SELECT COUNT(*) FROM conversations").Scan(&totalConversations) //nolint:errcheck

	var totalMessages int
	h.database.DB.QueryRow("SELECT COUNT(*) FROM messages").Scan(&totalMessages) //nolint:errcheck

	var totalAccounts int
	h.database.DB.QueryRow("SELECT COUNT(*) FROM accounts WHERE enabled = 1").Scan(&totalAccounts) //nolint:errcheck

	today := time.Now().Format("2006-01-02")
	var todayRequests int
	var todayCost float64
	var todayTokensIn, todayTokensOut int
	h.database.DB.QueryRow( //nolint:errcheck
		"SELECT COALESCE(SUM(request_count),0), COALESCE(SUM(cost),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0) FROM usage_daily WHERE date = ?",
		today,
	).Scan(&todayRequests, &todayCost, &todayTokensIn, &todayTokensOut)

	rows, _ := h.database.DB.Query(
		"SELECT model, SUM(request_count) as reqs, SUM(cost) as cost FROM usage_daily WHERE date = ? GROUP BY model ORDER BY reqs DESC LIMIT 5",
		today,
	)
	var topModels []gin.H
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var model string
			var reqs int
			var cost float64
			rows.Scan(&model, &reqs, &cost) //nolint:errcheck
			topModels = append(topModels, gin.H{"model": model, "requests": reqs, "cost": cost})
		}
	}
	if topModels == nil {
		topModels = []gin.H{}
	}

	var recentAudit []audit.Entry
	if h.audit != nil {
		recentAudit, _, _ = h.audit.List(10, 0)
	}
	if recentAudit == nil {
		recentAudit = []audit.Entry{}
	}

	c.JSON(http.StatusOK, gin.H{
		"users":            totalUsers,
		"conversations":    totalConversations,
		"messages":         totalMessages,
		"active_providers": totalAccounts,
		"today": gin.H{
			"requests":   todayRequests,
			"cost":       todayCost,
			"tokens_in":  todayTokensIn,
			"tokens_out": todayTokensOut,
		},
		"top_models":    topModels,
		"recent_audit":  recentAudit,
	})
}

// ─── Database backup ──────────────────────────────────────────────────────────

// GET /api/backup
func (h *SettingsHandler) BackupDB(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	dbPath := h.database.Path()
	if dbPath == "" || dbPath == ":memory:" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backup not supported for in-memory database"})
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
