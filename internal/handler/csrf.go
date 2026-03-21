package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CSRFMiddleware implements double-submit cookie CSRF protection.
// Sets a csrf_token cookie on GET, validates X-CSRF-Token header on POST/PUT/DELETE.
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for /v1/* (API key auth, not cookie-based)
		if len(c.Request.URL.Path) >= 3 && c.Request.URL.Path[:3] == "/v1" {
			c.Next()
			return
		}

		method := c.Request.Method

		// On safe methods, set CSRF cookie if not present
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			if _, err := c.Cookie("csrf_token"); err != nil {
				token := generateCSRFToken()
				c.SetCookie("csrf_token", token, 86400, "/", "", false, false) // readable by JS
			}
			c.Next()
			return
		}

		// On unsafe methods (POST, PUT, DELETE), validate
		cookie, err := c.Cookie("csrf_token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF token missing"})
			return
		}
		header := c.GetHeader("X-CSRF-Token")
		if header == "" || header != cookie {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF token mismatch"})
			return
		}
		c.Next()
	}
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
