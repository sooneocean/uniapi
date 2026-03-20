package background

import (
	"database/sql"
	"log/slog"
	"time"
)

// OAuthRefresher is the interface for background OAuth token refresh operations.
type OAuthRefresher interface {
	RefreshExpiring() error
	CleanupStates() error
}

type BackgroundTasks struct {
	db            *sql.DB
	stopCh        chan struct{}
	retentionDays int
	oauthMgr      OAuthRefresher
}

func New(db *sql.DB, retentionDays int, oauthMgr OAuthRefresher) *BackgroundTasks {
	return &BackgroundTasks{
		db:            db,
		stopCh:        make(chan struct{}),
		retentionDays: retentionDays,
		oauthMgr:      oauthMgr,
	}
}

func (b *BackgroundTasks) Start() {
	go b.run()
}

func (b *BackgroundTasks) Stop() {
	close(b.stopCh)
}

func (b *BackgroundTasks) run() {
	// Run cleanup immediately on start, then daily
	b.cleanup()
	dailyTicker := time.NewTicker(24 * time.Hour)
	refreshTicker := time.NewTicker(5 * time.Minute)
	defer dailyTicker.Stop()
	defer refreshTicker.Stop()
	for {
		select {
		case <-dailyTicker.C:
			b.cleanup()
		case <-refreshTicker.C:
			b.refreshTokens()
		case <-b.stopCh:
			return
		}
	}
}

func (b *BackgroundTasks) cleanup() {
	if b.retentionDays <= 0 {
		return // retention disabled
	}
	cutoff := time.Now().AddDate(0, 0, -b.retentionDays).Format("2006-01-02T15:04:05")

	tx, err := b.db.Begin()
	if err != nil {
		slog.Error("cleanup tx begin", "error", err)
		return
	}

	result, err := tx.Exec(`DELETE FROM messages WHERE conversation_id IN (SELECT id FROM conversations WHERE updated_at < ?)`, cutoff)
	if err != nil {
		tx.Rollback()
		slog.Error("cleanup messages", "error", err)
		return
	}
	msgCount, _ := result.RowsAffected()

	result, err = tx.Exec("DELETE FROM conversations WHERE updated_at < ?", cutoff)
	if err != nil {
		tx.Rollback()
		slog.Error("cleanup conversations", "error", err)
		return
	}
	convoCount, _ := result.RowsAffected()

	if err := tx.Commit(); err != nil {
		slog.Error("cleanup commit", "error", err)
		return
	}

	if convoCount > 0 || msgCount > 0 {
		slog.Info("cleanup", "conversations", convoCount, "messages", msgCount, "retention_days", b.retentionDays)
	}
}

func (b *BackgroundTasks) refreshTokens() {
	if b.oauthMgr == nil {
		return
	}
	if err := b.oauthMgr.RefreshExpiring(); err != nil {
		slog.Error("background: token refresh failed", "error", err)
	}
	if err := b.oauthMgr.CleanupStates(); err != nil {
		slog.Error("background: state cleanup failed", "error", err)
	}
}
