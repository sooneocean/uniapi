package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/audit"
	"github.com/sooneocean/uniapi/internal/db"
	"github.com/sooneocean/uniapi/internal/usage"
)

// UsageHandler handles usage analytics API routes.
type UsageHandler struct {
	database *db.Database
	recorder *usage.Recorder
	audit    *audit.Logger
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler(database *db.Database, recorder *usage.Recorder, auditLogger *audit.Logger) *UsageHandler {
	return &UsageHandler{
		database: database,
		recorder: recorder,
		audit:    auditLogger,
	}
}

func dateRangeFromQuery(c *gin.Context) (time.Time, time.Time) {
	rangeParam := c.Query("range")
	now := time.Now()
	var from time.Time
	switch rangeParam {
	case "weekly":
		from = now.AddDate(0, 0, -7)
	case "monthly":
		from = now.AddDate(0, -1, 0)
	default: // daily
		from = now.AddDate(0, 0, -1)
	}
	return from, now
}

// GET /api/usage
func (h *UsageHandler) GetUsage(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		return
	}
	from, to := dateRangeFromQuery(c)
	results, err := h.recorder.GetUserUsage(userID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []usage.DailyUsage{}
	}
	c.JSON(http.StatusOK, results)
}

// GET /api/usage/all
func (h *UsageHandler) GetAllUsage(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	from, to := dateRangeFromQuery(c)
	results, err := h.recorder.GetAllUsage(from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []usage.UserUsageSummary{}
	}
	c.JSON(http.StatusOK, results)
}

// GET /api/usage/analytics?days=30
func (h *UsageHandler) UsageAnalytics(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	if days <= 0 || days > 365 {
		days = 30
	}

	uid, _ := c.Get("user_id")
	userID := uid.(string)
	role, _ := c.Get("role")

	from := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	// Daily cost/token series
	type DailyEntry struct {
		Date         string  `json:"date"`
		Cost         float64 `json:"cost"`
		TokensIn     int64   `json:"tokens_in"`
		TokensOut    int64   `json:"tokens_out"`
		RequestCount int64   `json:"request_count"`
	}
	var dailySeries []DailyEntry
	rows, err := h.database.DB.Query(
		`SELECT date, SUM(cost), SUM(tokens_in), SUM(tokens_out), SUM(request_count)
		 FROM usage_daily WHERE user_id = ? AND date >= ? GROUP BY date ORDER BY date`,
		userID, from)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e DailyEntry
			rows.Scan(&e.Date, &e.Cost, &e.TokensIn, &e.TokensOut, &e.RequestCount)
			dailySeries = append(dailySeries, e)
		}
	}
	if dailySeries == nil {
		dailySeries = []DailyEntry{}
	}

	// Cost by model
	type ModelEntry struct {
		Model string  `json:"model"`
		Cost  float64 `json:"cost"`
	}
	var modelSeries []ModelEntry
	modelRows, err := h.database.DB.Query(
		`SELECT model, SUM(cost) FROM usage_daily WHERE user_id = ? AND date >= ? GROUP BY model ORDER BY SUM(cost) DESC`,
		userID, from)
	if err == nil {
		defer modelRows.Close()
		for modelRows.Next() {
			var e ModelEntry
			modelRows.Scan(&e.Model, &e.Cost)
			modelSeries = append(modelSeries, e)
		}
	}
	if modelSeries == nil {
		modelSeries = []ModelEntry{}
	}

	// Top users (admin only)
	var topUsers interface{} = nil
	if r, ok := role.(string); ok && r == "admin" {
		type UserEntry struct {
			Username     string  `json:"username"`
			Cost         float64 `json:"cost"`
			RequestCount int64   `json:"request_count"`
		}
		var topUsersList []UserEntry
		userRows, err := h.database.DB.Query(
			`SELECT u.username, SUM(ud.cost), SUM(ud.request_count)
			 FROM usage_daily ud JOIN users u ON u.id = ud.user_id
			 WHERE ud.date >= ? GROUP BY ud.user_id ORDER BY SUM(ud.cost) DESC LIMIT 10`,
			from)
		if err == nil {
			defer userRows.Close()
			for userRows.Next() {
				var e UserEntry
				userRows.Scan(&e.Username, &e.Cost, &e.RequestCount)
				topUsersList = append(topUsersList, e)
			}
		}
		if topUsersList == nil {
			topUsersList = []UserEntry{}
		}
		topUsers = topUsersList
	}

	c.JSON(http.StatusOK, gin.H{"daily": dailySeries, "by_model": modelSeries, "top_users": topUsers})
}

// GET /api/dashboard
func (h *UsageHandler) Dashboard(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var totalUsers int
	h.database.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers) //nolint:errcheck

	var totalConversations int
	h.database.DB.QueryRow("SELECT COUNT(*) FROM conversations").Scan(&totalConversations) //nolint:errcheck

	var totalMessages int
	h.database.DB.QueryRow("SELECT COUNT(*) FROM messages").Scan(&totalMessages) //nolint:errcheck

	var totalAccounts int
	h.database.DB.QueryRow("SELECT COUNT(*) FROM accounts WHERE enabled = 1").Scan(&totalAccounts) //nolint:errcheck

	today := time.Now().Format("2006-01-02")
	var todayRequests int
	var todayCost float64
	var todayTokensIn, todayTokensOut int
	h.database.DB.QueryRow( //nolint:errcheck
		"SELECT COALESCE(SUM(request_count),0), COALESCE(SUM(cost),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0) FROM usage_daily WHERE date = ?",
		today,
	).Scan(&todayRequests, &todayCost, &todayTokensIn, &todayTokensOut)

	rows, _ := h.database.DB.Query(
		"SELECT model, SUM(request_count) as reqs, SUM(cost) as cost FROM usage_daily WHERE date = ? GROUP BY model ORDER BY reqs DESC LIMIT 5",
		today,
	)
	var topModels []gin.H
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var model string
			var reqs int
			var cost float64
			rows.Scan(&model, &reqs, &cost) //nolint:errcheck
			topModels = append(topModels, gin.H{"model": model, "requests": reqs, "cost": cost})
		}
	}
	if topModels == nil {
		topModels = []gin.H{}
	}

	var recentAudit []audit.Entry
	if h.audit != nil {
		recentAudit, _, _ = h.audit.List(10, 0)
	}
	if recentAudit == nil {
		recentAudit = []audit.Entry{}
	}

	c.JSON(http.StatusOK, gin.H{
		"users":            totalUsers,
		"conversations":    totalConversations,
		"messages":         totalMessages,
		"active_providers": totalAccounts,
		"today": gin.H{
			"requests":   todayRequests,
			"cost":       todayCost,
			"tokens_in":  todayTokensIn,
			"tokens_out": todayTokensOut,
		},
		"top_models":   topModels,
		"recent_audit": recentAudit,
	})
}
