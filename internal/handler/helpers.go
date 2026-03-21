package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// jsonError sends a consistent error response.
func jsonError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

// userIDFromContext extracts user_id from gin context, returns 401 if missing or invalid.
// Returns the userID and true on success, or ("", false) after writing the error response.
func userIDFromContext(c *gin.Context) (string, bool) {
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return "", false
	}
	userID, ok := uid.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return "", false
	}
	return userID, true
}
