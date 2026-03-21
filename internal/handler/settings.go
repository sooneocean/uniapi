package handler

import (
	"net/http"
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
	accountRepo     *repo.AccountRepo
	userRepo        *repo.UserRepo
	recorder        *usage.Recorder
	database        *db.Database
	audit           *audit.Logger
	registerAccount func(acc *repo.Account)
	router          *router.Router
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
		accountRepo:     accountRepo,
		userRepo:        userRepo,
		recorder:        recorder,
		database:        database,
		audit:           auditLogger,
		registerAccount: registerAccount,
		router:          rtr,
	}
}

// ─── Provider management ─────────────────────────────────────────────────────

func (h *SettingsHandler) ListProviders(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	accounts, err := h.accountRepo.ListAll()
	if err != nil {
		serverError(c, "operation failed")
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
		badRequest(c, err.Error())
		return
	}
	if len(req.Label) > 100 {
		badRequest(c, "label too long")
		return
	}
	maxConc := req.MaxConcurrent
	if maxConc == 0 {
		maxConc = 5
	}
	account, err := h.accountRepo.Create(req.Provider, req.Label, req.APIKey, req.Models, maxConc, false)
	if err != nil {
		serverError(c, "operation failed")
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
		notFound(c, "provider not found")
		return
	}
	if account.ConfigManaged {
		badRequest(c, "cannot delete config-managed provider")
		return
	}
	if err := h.accountRepo.Delete(id); err != nil {
		serverError(c, "operation failed")
		return
	}

	if h.audit != nil {
		uid, _ := c.Get("user_id")
		userID, _ := uid.(string)
		h.audit.Log(userID, "", "delete_provider", "provider", id, "", c.ClientIP())
	}

	success(c, gin.H{"ok": true})
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
		serverError(c, "operation failed")
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
		badRequest(c, err.Error())
		return
	}
	if len(req.Username) > 100 {
		badRequest(c, "username too long")
		return
	}
	if err := validatePassword(req.Password); err != nil {
		badRequest(c, err.Error())
		return
	}
	role := req.Role
	if role == "" {
		role = "member"
	}
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverError(c, "failed to hash password")
		return
	}
	user, err := h.userRepo.Create(req.Username, passwordHash, role)
	if err != nil {
		badRequest(c, err.Error())
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

// PUT /api/users/:id/quotas
func (h *SettingsHandler) UpdateUserQuotas(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	id := c.Param("id")
	var req struct {
		DailyTokenLimit  int     `json:"daily_token_limit"`
		DailyCostLimit   float64 `json:"daily_cost_limit"`
		MonthlyCostLimit float64 `json:"monthly_cost_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := h.userRepo.UpdateQuotas(id, req.DailyTokenLimit, req.DailyCostLimit, req.MonthlyCostLimit); err != nil {
		serverError(c, "operation failed")
		return
	}
	success(c, gin.H{"ok": true})
}

func (h *SettingsHandler) DeleteUser(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	id := c.Param("id")

	// Prevent self-deletion
	uid, _ := c.Get("user_id")
	if currentUID, ok := uid.(string); ok && currentUID == id {
		badRequest(c, "cannot delete your own account")
		return
	}

	if err := h.userRepo.Delete(id); err != nil {
		notFound(c, "user not found")
		return
	}

	if h.audit != nil {
		uid, _ := c.Get("user_id")
		userID, _ := uid.(string)
		h.audit.Log(userID, "", "delete_user", "user", id, "", c.ClientIP())
	}

	success(c, gin.H{"ok": true})
}

// ─── API Key management ───────────────────────────────────────────────────────

func (h *SettingsHandler) ListAPIKeys(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": "not authenticated"}})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		serverError(c, "invalid user context")
		return
	}
	rows, err := h.database.DB.Query(
		"SELECT id, label, created_at, expires_at FROM api_keys WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		serverError(c, "operation failed")
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
			serverError(c, "operation failed")
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": "not authenticated"}})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		serverError(c, "invalid user context")
		return
	}
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	key, err := auth.GenerateAPIKey()
	if err != nil {
		serverError(c, "failed to generate API key")
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
		serverError(c, "operation failed")
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": "not authenticated"}})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok {
		serverError(c, "invalid user context")
		return
	}
	id := c.Param("id")
	result, err := h.database.DB.Exec(
		"DELETE FROM api_keys WHERE id = ? AND user_id = ?",
		id, userID,
	)
	if err != nil {
		serverError(c, "operation failed")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		notFound(c, "api key not found")
		return
	}
	success(c, gin.H{"ok": true})
}
