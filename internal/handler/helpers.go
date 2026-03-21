package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/provider"
	pAnthropic "github.com/sooneocean/uniapi/internal/provider/anthropic"
	pGemini "github.com/sooneocean/uniapi/internal/provider/gemini"
	pOpenai "github.com/sooneocean/uniapi/internal/provider/openai"
	pSub2api "github.com/sooneocean/uniapi/internal/provider/sub2api"
)

// userIDFromContext extracts user_id safely.
func userIDFromContext(c *gin.Context) (string, bool) {
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": errNotAuthenticated}})
		return "", false
	}
	userID, ok := uid.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"type": "internal_error", "message": "invalid session"}})
		return "", false
	}
	return userID, true
}

// requireAdmin checks admin role.
func requireAdmin(c *gin.Context) bool {
	roleVal, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": errNotAuthenticated}})
		return false
	}
	role, ok := roleVal.(string)
	if !ok || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "forbidden", "message": errAdminRequired}})
		return false
	}
	return true
}

// Standard error response helpers — use consistent format everywhere.

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "bad_request", "message": msg}})
}

func notFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found", "message": msg}})
}

func serverError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"type": "internal_error", "message": msg}})
}

func forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "forbidden", "message": msg}})
}

func tooManyRequests(c *gin.Context, msg string) {
	c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"type": "rate_limit", "message": msg}})
}

func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

// CreateProvider builds a provider instance from type name, config, models, and credFunc.
func CreateProvider(provType string, cfg provider.ProviderConfig, models []string, credFunc func() (string, string)) provider.Provider {
	// NEW: Check if session_token → use sub2api adapter
	_, authType := credFunc()
	if authType == "session_token" {
		switch provType {
		case "openai":
			return pSub2api.NewChatGPT(models, credFunc)
		case "anthropic":
			return pSub2api.NewClaudeWeb(models, credFunc)
		case "gemini":
			return pSub2api.NewGeminiWeb(models, credFunc)
		}
	}

	switch provType {
	case "anthropic":
		return pAnthropic.NewAnthropic(cfg, models, credFunc)
	case "gemini":
		return pGemini.NewGemini(cfg, models, credFunc)
	case "openai", "openai_compatible":
		return pOpenai.NewOpenAI(cfg, models, credFunc)
	default:
		return pOpenai.NewOpenAI(cfg, models, credFunc) // fallback
	}
}
