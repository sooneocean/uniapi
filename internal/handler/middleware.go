package handler

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/auth"
	"github.com/sooneocean/uniapi/internal/cache"
	"github.com/sooneocean/uniapi/internal/metrics"
)

// RateLimitMiddleware limits requests per IP using in-memory counters.
func RateLimitMiddleware(mc *cache.MemCache, maxRequests int, window time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        key := "ratelimit:ip:" + ip

        val, exists := mc.Get(key)
        if !exists {
            mc.Set(key, 1, window)
            c.Next()
            return
        }

        count, _ := val.(int)
        if count >= maxRequests {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": gin.H{"type": "rate_limit_error", "message": "too many requests, try again later"},
            })
            return
        }

        mc.Increment(key) // preserves existing TTL
        c.Next()
    }
}

func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	originSet := make(map[string]bool)
	for _, o := range allowedOrigins {
		originSet[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/v1/") {
			// API routes: permissive (uses Authorization header, not cookies)
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", "*")
			}
		} else if origin != "" {
			// /api/* routes: check allowlist or allow same-origin
			if len(originSet) > 0 {
				if originSet[origin] {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Access-Control-Allow-Credentials", "true")
				}
				// else: no CORS headers = browser blocks
			} else {
				// No allowlist configured: reflect origin (same behavior as before for backward compat)
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
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

func APIKeyAuthMiddleware(db *sql.DB, jwtMgr *auth.JWTManager, mc *cache.MemCache) gin.HandlerFunc {
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
			cacheKey := "apikey:" + hash

			// Check cache first
			if cached, ok := mc.Get(cacheKey); ok {
				if uid, ok := cached.(string); ok {
					c.Set("user_id", uid)
					c.Next()
					return
				}
			}

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

			// Cache for 10 minutes
			mc.Set(cacheKey, userID, 10*time.Minute)
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
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		metrics.ActiveConnections.Inc()
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())
		metrics.RequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		metrics.RequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
		metrics.ActiveConnections.Dec()
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
