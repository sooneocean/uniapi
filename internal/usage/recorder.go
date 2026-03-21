package usage

import (
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// UsageRecord holds per-request token and cost data for a single API call.
type UsageRecord struct {
	UserID    string
	Model     string
	Provider  string
	TokensIn  int
	TokensOut int
	Cost      float64
	LatencyMs int
}

// Recorder batches usage records and persists them to the database asynchronously.
type Recorder struct {
	db     *sql.DB
	buffer []UsageRecord
	mu     sync.Mutex
	stopCh chan struct{}
	doneCh chan struct{}
}

// NewRecorder creates a Recorder and starts its background flush loop.
func NewRecorder(db *sql.DB) *Recorder {
	r := &Recorder{
		db:     db,
		buffer: make([]UsageRecord, 0, 32),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	go r.flushLoop()
	return r
}

// RecordUsage buffers a usage entry for batch writing.
func (r *Recorder) RecordUsage(rec UsageRecord) {
	r.mu.Lock()
	r.buffer = append(r.buffer, rec)
	shouldFlush := len(r.buffer) >= 20
	r.mu.Unlock()

	if shouldFlush {
		r.flush()
	}
}

func (r *Recorder) flushLoop() {
	defer close(r.doneCh)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.flush()
		case <-r.stopCh:
			r.flush() // final flush
			return
		}
	}
}

func (r *Recorder) flush() {
	r.mu.Lock()
	if len(r.buffer) == 0 {
		r.mu.Unlock()
		return
	}
	records := r.buffer
	r.buffer = make([]UsageRecord, 0, 32)
	r.mu.Unlock()

	// Write all records in a single transaction
	tx, err := r.db.Begin()
	if err != nil {
		slog.Error("usage flush tx begin", "error", err)
		return
	}

	stmt, err := tx.Prepare(`
		INSERT INTO usage_daily (id, user_id, provider, model, date, tokens_in, tokens_out, cost, request_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(user_id, provider, model, date) DO UPDATE SET
			tokens_in = tokens_in + excluded.tokens_in,
			tokens_out = tokens_out + excluded.tokens_out,
			cost = cost + excluded.cost,
			request_count = request_count + 1
	`)
	if err != nil {
		tx.Rollback()
		slog.Error("usage flush prepare", "error", err)
		return
	}
	defer stmt.Close()

	date := time.Now().Format("2006-01-02")
	for _, rec := range records {
		stmt.Exec(uuid.New().String(), rec.UserID, rec.Provider, rec.Model, date,
			rec.TokensIn, rec.TokensOut, rec.Cost)
	}

	if err := tx.Commit(); err != nil {
		slog.Error("usage flush commit", "error", err)
	}
}

// Stop flushes remaining records and shuts down the background flush loop.
func (r *Recorder) Stop() {
	close(r.stopCh)
	<-r.doneCh
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

// DailyUsage summarises token and cost usage for one model/provider/day combination.
type DailyUsage struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Date         string  `json:"date"`
	TokensIn     int     `json:"tokens_in"`
	TokensOut    int     `json:"tokens_out"`
	Cost         float64 `json:"cost"`
	RequestCount int     `json:"request_count"`
}

// UserUsageSummary aggregates total token and cost usage per user for admin reporting.
type UserUsageSummary struct {
	Username     string  `json:"username"`
	UserID       string  `json:"user_id"`
	TokensIn     int     `json:"tokens_in"`
	TokensOut    int     `json:"tokens_out"`
	Cost         float64 `json:"cost"`
	RequestCount int     `json:"request_count"`
}
