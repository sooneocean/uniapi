package background

import (
    "database/sql"
    "log"
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
        log.Printf("background: cleanup messages error: %v", err)
        return
    }
    msgCount, _ := result.RowsAffected()

    result, err = b.db.Exec("DELETE FROM conversations WHERE updated_at < ?", cutoff)
    if err != nil {
        log.Printf("background: cleanup conversations error: %v", err)
        return
    }
    convoCount, _ := result.RowsAffected()

    if convoCount > 0 || msgCount > 0 {
        log.Printf("background: cleaned up %d conversations and %d messages older than %d days",
            convoCount, msgCount, b.retentionDays)
    }
}
