package background

import (
	"database/sql"
	"log/slog"
	"time"
)

type BackgroundTasks struct {
    db     *sql.DB
    stopCh chan struct{}
    retentionDays int
}

func New(db *sql.DB, retentionDays int) *BackgroundTasks {
    return &BackgroundTasks{
        db:            db,
        stopCh:        make(chan struct{}),
        retentionDays: retentionDays,
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
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            b.cleanup()
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

    // Delete old messages first (due to FK), then conversations
	result, err := b.db.Exec(`
        DELETE FROM messages WHERE conversation_id IN (
            SELECT id FROM conversations WHERE updated_at < ?
        )
    `, cutoff)
	if err != nil {
		slog.Error("background: cleanup messages error", "error", err)
		return
	}
	msgCount, _ := result.RowsAffected()

	result, err = b.db.Exec("DELETE FROM conversations WHERE updated_at < ?", cutoff)
	if err != nil {
		slog.Error("background: cleanup conversations error", "error", err)
		return
	}
	convoCount, _ := result.RowsAffected()

	if convoCount > 0 || msgCount > 0 {
		slog.Info("background: cleanup complete",
			"conversations", convoCount,
			"messages", msgCount,
			"retention_days", b.retentionDays,
		)
	}
}
