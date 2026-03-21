package quota

import (
	"database/sql"
	"fmt"
	"time"
)

// Config holds default quota limits applied when a user has no per-user limits configured.
type Config struct {
	DailyLimitUSD   float64
	MonthlyLimitUSD float64
	WarnThreshold   float64 // e.g. 0.8 = warn at 80%
}

// CheckResult is returned by Engine.Check and describes the user's current quota state.
type CheckResult struct {
	Allowed      bool    `json:"allowed"`
	Warning      bool    `json:"warning"`
	Message      string  `json:"message,omitempty"`
	DailyUsed    float64 `json:"dailyUsed"`
	DailyLimit   float64 `json:"dailyLimit"`
	MonthlyUsed  float64 `json:"monthlyUsed"`
	MonthlyLimit float64 `json:"monthlyLimit"`
}

// Engine performs quota checks against the database.
type Engine struct {
	db            *sql.DB
	defaultConfig Config
}

// NewEngine creates a new quota Engine.
func NewEngine(db *sql.DB, defaultConfig Config) *Engine {
	return &Engine{db: db, defaultConfig: defaultConfig}
}

// Check evaluates whether the user is within their quota limits.
func (e *Engine) Check(userID string) CheckResult {
	cfg := e.getUserConfig(userID)
	dailyUsed := e.getDailyCost(userID)
	monthlyUsed := e.getMonthlyCost(userID)

	result := CheckResult{
		Allowed:      true,
		DailyUsed:    dailyUsed,
		DailyLimit:   cfg.DailyLimitUSD,
		MonthlyUsed:  monthlyUsed,
		MonthlyLimit: cfg.MonthlyLimitUSD,
	}

	// Check daily
	if cfg.DailyLimitUSD > 0 {
		if dailyUsed >= cfg.DailyLimitUSD {
			result.Allowed = false
			result.Message = fmt.Sprintf("Daily quota exceeded ($%.2f/$%.2f)", dailyUsed, cfg.DailyLimitUSD)
			return result
		}
		if cfg.WarnThreshold > 0 && dailyUsed >= cfg.DailyLimitUSD*cfg.WarnThreshold {
			result.Warning = true
			result.Message = fmt.Sprintf("Daily quota %.0f%% used ($%.2f/$%.2f)",
				dailyUsed/cfg.DailyLimitUSD*100, dailyUsed, cfg.DailyLimitUSD)
		}
	}

	// Check monthly
	if cfg.MonthlyLimitUSD > 0 {
		if monthlyUsed >= cfg.MonthlyLimitUSD {
			result.Allowed = false
			result.Message = fmt.Sprintf("Monthly quota exceeded ($%.2f/$%.2f)", monthlyUsed, cfg.MonthlyLimitUSD)
			return result
		}
		if !result.Warning && cfg.WarnThreshold > 0 && monthlyUsed >= cfg.MonthlyLimitUSD*cfg.WarnThreshold {
			result.Warning = true
			result.Message = fmt.Sprintf("Monthly quota %.0f%% used ($%.2f/$%.2f)",
				monthlyUsed/cfg.MonthlyLimitUSD*100, monthlyUsed, cfg.MonthlyLimitUSD)
		}
	}

	return result
}

// getUserConfig reads per-user limits from the DB; falls back to defaultConfig.
func (e *Engine) getUserConfig(userID string) Config {
	var daily, monthly float64
	err := e.db.QueryRow(
		"SELECT COALESCE(daily_cost_limit, 0), COALESCE(monthly_cost_limit, 0) FROM users WHERE id = ?",
		userID,
	).Scan(&daily, &monthly)
	if err != nil || (daily == 0 && monthly == 0) {
		return e.defaultConfig
	}
	return Config{
		DailyLimitUSD:   daily,
		MonthlyLimitUSD: monthly,
		WarnThreshold:   e.defaultConfig.WarnThreshold,
	}
}

func (e *Engine) getDailyCost(userID string) float64 {
	var cost float64
	today := time.Now().Format("2006-01-02")
	e.db.QueryRow( //nolint:errcheck
		"SELECT COALESCE(SUM(cost), 0) FROM usage_daily WHERE user_id = ? AND date = ?",
		userID, today,
	).Scan(&cost)
	return cost
}

func (e *Engine) getMonthlyCost(userID string) float64 {
	var cost float64
	monthStart := time.Now().Format("2006-01") + "-01"
	e.db.QueryRow( //nolint:errcheck
		"SELECT COALESCE(SUM(cost), 0) FROM usage_daily WHERE user_id = ? AND date >= ?",
		userID, monthStart,
	).Scan(&cost)
	return cost
}

// SetUserQuota updates a user's cost quota limits (admin operation).
func (e *Engine) SetUserQuota(userID string, daily, monthly float64) error {
	_, err := e.db.Exec(
		"UPDATE users SET daily_cost_limit = ?, monthly_cost_limit = ? WHERE id = ?",
		daily, monthly, userID,
	)
	return err
}
