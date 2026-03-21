package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/cache"
)

type ModelAliasHandler struct {
	db    *sql.DB
	cache *cache.MemCache
}

func NewModelAliasHandler(db *sql.DB) *ModelAliasHandler {
	return &ModelAliasHandler{db: db}
}

func NewModelAliasHandlerWithCache(db *sql.DB, mc *cache.MemCache) *ModelAliasHandler {
	return &ModelAliasHandler{db: db, cache: mc}
}

type modelAlias struct {
	Alias     string  `json:"alias"`
	ModelID   string  `json:"model_id"`
	UserID    *string `json:"user_id,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// ListModelAliases handles GET /api/model-aliases
// Returns user-specific aliases + global aliases (user_id IS NULL)
func (h *ModelAliasHandler) ListModelAliases(c *gin.Context) {
	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if u, ok := uid.(string); ok {
			userID = u
		}
	}

	rows, err := h.db.QueryContext(c.Request.Context(),
		"SELECT alias, model_id, user_id, created_at FROM model_aliases WHERE user_id IS NULL OR user_id = ? ORDER BY alias ASC",
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var aliases []modelAlias
	for rows.Next() {
		var a modelAlias
		if err := rows.Scan(&a.Alias, &a.ModelID, &a.UserID, &a.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		aliases = append(aliases, a)
	}
	if aliases == nil {
		aliases = []modelAlias{}
	}
	c.JSON(http.StatusOK, aliases)
}

// CreateModelAlias handles POST /api/model-aliases
func (h *ModelAliasHandler) CreateModelAlias(c *gin.Context) {
	var req struct {
		Alias   string `json:"alias" binding:"required"`
		ModelID string `json:"model_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if u, ok := uid.(string); ok {
			userID = u
		}
	}

	// Check if admin — admins can create global aliases (user_id = NULL)
	isGlobal := false
	if role, exists := c.Get("role"); exists {
		if r, ok := role.(string); ok && r == "admin" {
			isGlobal = c.Query("global") == "true"
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if isGlobal {
		_, err := h.db.ExecContext(c.Request.Context(),
			"INSERT OR REPLACE INTO model_aliases (alias, model_id, user_id, created_at) VALUES (?, ?, NULL, ?)",
			req.Alias, req.ModelID, now,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if h.cache != nil {
			h.cache.Delete("alias:" + req.Alias)
		}
		c.JSON(http.StatusOK, modelAlias{Alias: req.Alias, ModelID: req.ModelID, CreatedAt: now})
	} else {
		_, err := h.db.ExecContext(c.Request.Context(),
			"INSERT OR REPLACE INTO model_aliases (alias, model_id, user_id, created_at) VALUES (?, ?, ?, ?)",
			req.Alias, req.ModelID, userID, now,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if h.cache != nil {
			h.cache.Delete("alias:" + req.Alias)
		}
		c.JSON(http.StatusOK, modelAlias{Alias: req.Alias, ModelID: req.ModelID, UserID: &userID, CreatedAt: now})
	}
}

// DeleteModelAlias handles DELETE /api/model-aliases/:alias
func (h *ModelAliasHandler) DeleteModelAlias(c *gin.Context) {
	alias := c.Param("alias")

	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if u, ok := uid.(string); ok {
			userID = u
		}
	}

	role := ""
	if r, exists := c.Get("role"); exists {
		if rv, ok := r.(string); ok {
			role = rv
		}
	}

	var result sql.Result
	var err error
	if role == "admin" {
		// Admin can delete any alias
		result, err = h.db.ExecContext(c.Request.Context(),
			"DELETE FROM model_aliases WHERE alias = ? AND (user_id = ? OR user_id IS NULL)",
			alias, userID,
		)
	} else {
		// Regular users can only delete their own aliases
		result, err = h.db.ExecContext(c.Request.Context(),
			"DELETE FROM model_aliases WHERE alias = ? AND user_id = ?",
			alias, userID,
		)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "alias not found"})
		return
	}
	if h.cache != nil {
		h.cache.Delete("alias:" + alias)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
