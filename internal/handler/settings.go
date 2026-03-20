package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/user/uniapi/internal/audit"
	"github.com/user/uniapi/internal/auth"
	"github.com/user/uniapi/internal/db"
	"github.com/user/uniapi/internal/repo"
	"github.com/user/uniapi/internal/usage"
)

type SettingsHandler struct {
	accountRepo *repo.AccountRepo
	userRepo    *repo.UserRepo
	convoRepo   *repo.ConversationRepo
	recorder    *usage.Recorder
	database    *db.Database
	audit       *audit.Logger
}

func NewSettingsHandler(
	accountRepo *repo.AccountRepo,
	userRepo *repo.UserRepo,
	convoRepo *repo.ConversationRepo,
	recorder *usage.Recorder,
	database *db.Database,
	auditLogger *audit.Logger,
) *SettingsHandler {
	return &SettingsHandler{
		accountRepo: accountRepo,
		userRepo:    userRepo,
		convoRepo:   convoRepo,
		recorder:    recorder,
		database:    database,
		audit:       auditLogger,
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
	maxConc := req.MaxConcurrent
	if maxConc == 0 {
		maxConc = 5
	}
	account, err := h.accountRepo.Create(req.Provider, req.Label, req.APIKey, req.Models, maxConc, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
	convs, err := h.convoRepo.ListByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if convs == nil {
		convs = []repo.Conversation{}
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
