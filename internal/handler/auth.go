package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/uniapi/internal/auth"
	"github.com/user/uniapi/internal/db"
	"github.com/user/uniapi/internal/repo"
)

type AuthHandler struct {
	userRepo *repo.UserRepo
	jwtMgr   *auth.JWTManager
	database *db.Database
}

func NewAuthHandler(userRepo *repo.UserRepo, jwtMgr *auth.JWTManager, database *db.Database) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, jwtMgr: jwtMgr, database: database}
}

func (h *AuthHandler) Status(c *gin.Context) {
	needsSetup, err := h.database.NeedsSetup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	authenticated := false
	token := ExtractBearerToken(c)
	if token == "" {
		token, _ = c.Cookie("token")
	}
	if token != "" {
		if _, err := h.jwtMgr.ParseToken(token); err == nil {
			authenticated = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"needs_setup":   needsSetup,
		"authenticated": authenticated,
	})
}

type setupRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Setup(c *gin.Context) {
	needsSetup, err := h.database.NeedsSetup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if !needsSetup {
		c.JSON(http.StatusBadRequest, gin.H{"error": "setup already completed"})
		return
	}

	var req setupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	_, err = h.userRepo.Create(req.Username, passwordHash, "admin")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userRepo.GetByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if !auth.VerifyPassword(user.Password, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.jwtMgr.CreateToken(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}

	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", token, 7*24*3600, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	uid, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	role, _ := c.Get("role")

	user, err := h.userRepo.GetByID(uid)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     role,
	})
}
