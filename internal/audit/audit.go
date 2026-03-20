package audit

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Logger struct {
	db *sql.DB
}

type Entry struct {
	ID         string
	UserID     string
	Username   string
	Action     string // "create_user", "delete_user", "create_provider", "delete_provider", "bind_account", "unbind_account", "login", "setup"
	Resource   string // "user", "provider", "account", "api_key"
	ResourceID string
	Details    string
	IP         string
	CreatedAt  time.Time
}

func NewLogger(db *sql.DB) *Logger {
	return &Logger{db: db}
}

func (l *Logger) Log(userID, username, action, resource, resourceID, details, ip string) {
	l.db.Exec( //nolint:errcheck
		"INSERT INTO audit_log (id, user_id, username, action, resource, resource_id, details, ip, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		uuid.New().String(), userID, username, action, resource, resourceID, details, ip, time.Now(),
	)
}

func (l *Logger) List(limit, offset int) ([]Entry, int, error) {
	var total int
	l.db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&total) //nolint:errcheck

	rows, err := l.db.Query(
		"SELECT id, user_id, username, action, resource, resource_id, details, ip, created_at FROM audit_log ORDER BY created_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var resourceID, details, ip sql.NullString
		if err := rows.Scan(&e.ID, &e.UserID, &e.Username, &e.Action, &e.Resource, &resourceID, &details, &ip, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		if resourceID.Valid {
			e.ResourceID = resourceID.String
		}
		if details.Valid {
			e.Details = details.String
		}
		if ip.Valid {
			e.IP = ip.String
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}
