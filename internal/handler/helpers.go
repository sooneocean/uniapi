package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// userIDFromContext extracts user_id safely.
func userIDFromContext(c *gin.Context) (string, bool) {
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": "not authenticated"}})
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "auth_error", "message": "not authenticated"}})
		return false
	}
	role, ok := roleVal.(string)
	if !ok || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "forbidden", "message": "admin access required"}})
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
