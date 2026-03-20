package handler

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/user/uniapi/internal/auth"
	"github.com/user/uniapi/internal/cache"
)

// RateLimitMiddleware limits requests per IP using in-memory counters.
func RateLimitMiddleware(c *cache.MemCache, maxRequests int, window time.Duration) gin.HandlerFunc {
    return func(ctx *gin.Context) {
        ip := ctx.ClientIP()
        key := "ratelimit:ip:" + ip

        val, exists := c.Get(key)
        if !exists {
            c.Set(key, 1, window)
            ctx.Next()
            return
        }

        count, ok := val.(int)
        if !ok {
            c.Set(key, 1, window)
            ctx.Next()
            return
        }

        if count >= maxRequests {
            ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": gin.H{"type": "rate_limit_error", "message": "too many requests, try again later"},
            })
            return
        }

        c.Set(key, count+1, window)
        ctx.Next()
    }
}

func CORSMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        if origin != "" {
            c.Header("Access-Control-Allow-Origin", origin)
            c.Header("Access-Control-Allow-Credentials", "true")
        }
        c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}

func ExtractBearerToken(c *gin.Context) string {
    a := c.GetHeader("Authorization")
    if strings.HasPrefix(a, "Bearer ") {
        return strings.TrimPrefix(a, "Bearer ")
    }
    return ""
}

func JWTAuthMiddleware(jwtMgr *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ExtractBearerToken(c)
		if token == "" {
			token, _ = c.Cookie("token")
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := jwtMgr.ParseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func APIKeyAuthMiddleware(db *sql.DB, jwtMgr *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ExtractBearerToken(c)
		if token == "" {
			token, _ = c.Cookie("token")
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "authentication_error", "message": "missing API key or session"}})
			return
		}
		if strings.HasPrefix(token, "uniapi-sk-") {
			hash := auth.HashAPIKey(token)
			var userID string
			var expiresAt sql.NullTime
			err := db.QueryRow("SELECT user_id, expires_at FROM api_keys WHERE key_hash = ?", hash).Scan(&userID, &expiresAt)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "authentication_error", "message": "invalid API key"}})
				return
			}
			if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "authentication_error", "message": "API key expired"}})
				return
			}
			c.Set("user_id", userID)
			c.Next()
			return
		}
		claims, err := jwtMgr.ParseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "authentication_error", "message": "invalid token"}})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()[:8]
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func RequestLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		reqID, _ := c.Get("request_id")
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
			"request_id", reqID,
		)
	}
}
