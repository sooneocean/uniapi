package handler

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ThemesHandler struct {
	db *sql.DB
}

func NewThemesHandler(db *sql.DB) *ThemesHandler {
	return &ThemesHandler{db: db}
}

type Theme struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Colors    string `json:"colors"` // JSON string
	Shared    bool   `json:"shared"`
	CreatedAt string `json:"created_at"`
}

// GET /api/themes — list own + shared
func (h *ThemesHandler) List(c *gin.Context) {
	userID := mustUserID(c)
	rows, err := h.db.Query(
		`SELECT id, user_id, name, colors, shared, created_at FROM themes WHERE user_id = ? OR shared = 1 ORDER BY created_at DESC`,
		userID)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	defer rows.Close()

	var themes []Theme
	for rows.Next() {
		var t Theme
		var sharedInt int
		rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Colors, &sharedInt, &t.CreatedAt)
		t.Shared = sharedInt == 1
		themes = append(themes, t)
	}
	if themes == nil {
		themes = []Theme{}
	}
	c.JSON(http.StatusOK, themes)
}

// POST /api/themes — create
func (h *ThemesHandler) Create(c *gin.Context) {
	userID := mustUserID(c)
	var req struct {
		Name   string `json:"name" binding:"required"`
		Colors string `json:"colors" binding:"required"`
		Shared bool   `json:"shared"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	id := uuid.New().String()
	_, err := h.db.Exec(
		`INSERT INTO themes (id, user_id, name, colors, shared) VALUES (?,?,?,?,?)`,
		id, userID, req.Name, req.Colors, req.Shared)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.JSON(http.StatusCreated, Theme{ID: id, UserID: userID, Name: req.Name, Colors: req.Colors, Shared: req.Shared})
}

// DELETE /api/themes/:id — delete
func (h *ThemesHandler) Delete(c *gin.Context) {
	userID := mustUserID(c)
	id := c.Param("id")

	var ownerID string
	if err := h.db.QueryRow("SELECT user_id FROM themes WHERE id = ?", id).Scan(&ownerID); err != nil {
		if err == sql.ErrNoRows {
			notFound(c, "theme not found")
			return
		}
		serverError(c, "database error")
		return
	}
	if ownerID != userID {
		forbidden(c, "forbidden")
		return
	}
	h.db.Exec("DELETE FROM themes WHERE id = ?", id)
	c.Status(http.StatusNoContent)
}

// PUT /api/themes/:id/apply — set as active theme
func (h *ThemesHandler) Apply(c *gin.Context) {
	userID := mustUserID(c)
	id := c.Param("id")

	// Allow applying shared themes too
	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM themes WHERE id = ? AND (user_id = ? OR shared = 1)", id, userID).Scan(&count)
	if count == 0 {
		notFound(c, "theme not found")
		return
	}

	_, err := h.db.Exec("UPDATE users SET active_theme = ? WHERE id = ?", id, userID)
	if err != nil {
		serverError(c, errOperationFailed)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "active_theme": id})
}
