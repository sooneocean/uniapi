package usage

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type UsageRecord struct {
	UserID    string
	Model     string
	Provider  string
	TokensIn  int
	TokensOut int
	Cost      float64
	LatencyMs int
}

type Recorder struct {
	db *sql.DB
}

func NewRecorder(db *sql.DB) *Recorder {
	return &Recorder{db: db}
}

// RecordUsage inserts a usage entry into usage_daily (upsert).
func (r *Recorder) RecordUsage(rec UsageRecord) error {
	id := uuid.New().String()
	date := time.Now().Format("2006-01-02")
	_, err := r.db.Exec(`
		INSERT INTO usage_daily (id, user_id, provider, model, date, tokens_in, tokens_out, cost, request_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(user_id, provider, model, date) DO UPDATE SET
			tokens_in = tokens_in + excluded.tokens_in,
			tokens_out = tokens_out + excluded.tokens_out,
			cost = cost + excluded.cost,
			request_count = request_count + 1
	`, id, rec.UserID, rec.Provider, rec.Model, date, rec.TokensIn, rec.TokensOut, rec.Cost)
	return err
}

// GetUserUsage returns usage stats for a user within a date range.
func (r *Recorder) GetUserUsage(userID string, from, to time.Time) ([]DailyUsage, error) {
	rows, err := r.db.Query(`
		SELECT provider, model, date, tokens_in, tokens_out, cost, request_count
		FROM usage_daily WHERE user_id = ? AND date >= ? AND date <= ?
		ORDER BY date DESC
	`, userID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []DailyUsage
	for rows.Next() {
		var u DailyUsage
		if err := rows.Scan(&u.Provider, &u.Model, &u.Date, &u.TokensIn, &u.TokensOut, &u.Cost, &u.RequestCount); err != nil {
			return nil, err
		}
		results = append(results, u)
	}
	return results, rows.Err()
}

// GetAllUsage returns usage for all users (admin).
func (r *Recorder) GetAllUsage(from, to time.Time) ([]UserUsageSummary, error) {
	rows, err := r.db.Query(`
		SELECT u.username, ud.user_id, SUM(ud.tokens_in), SUM(ud.tokens_out), SUM(ud.cost), SUM(ud.request_count)
		FROM usage_daily ud JOIN users u ON u.id = ud.user_id
		WHERE ud.date >= ? AND ud.date <= ?
		GROUP BY ud.user_id
		ORDER BY SUM(ud.cost) DESC
	`, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []UserUsageSummary
	for rows.Next() {
		var u UserUsageSummary
		if err := rows.Scan(&u.Username, &u.UserID, &u.TokensIn, &u.TokensOut, &u.Cost, &u.RequestCount); err != nil {
			return nil, err
		}
		results = append(results, u)
	}
	return results, rows.Err()
}

type DailyUsage struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Date         string  `json:"date"`
	TokensIn     int     `json:"tokens_in"`
	TokensOut    int     `json:"tokens_out"`
	Cost         float64 `json:"cost"`
	RequestCount int     `json:"request_count"`
}

type UserUsageSummary struct {
	Username     string  `json:"username"`
	UserID       string  `json:"user_id"`
	TokensIn     int     `json:"tokens_in"`
	TokensOut    int     `json:"tokens_out"`
	Cost         float64 `json:"cost"`
	RequestCount int     `json:"request_count"`
}
