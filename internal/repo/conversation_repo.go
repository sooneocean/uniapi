package repo

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/db"
)

type Conversation struct {
	ID        string
	UserID    string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type MessageRecord struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	Model          string
	Provider       string
	TokensIn       int
	TokensOut      int
	Cost           float64
	LatencyMs      int
	CreatedAt      time.Time
}

type ConversationRepo struct {
	db *db.Database
}

func NewConversationRepo(database *db.Database) *ConversationRepo {
	return &ConversationRepo{db: database}
}

func (r *ConversationRepo) Create(userID, title string) (*Conversation, error) {
	now := time.Now()
	c := &Conversation{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := r.db.DB.Exec(
		"INSERT INTO conversations (id, user_id, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		c.ID, c.UserID, c.Title, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return c, nil
}

func (r *ConversationRepo) GetByID(id string) (*Conversation, error) {
	c := &Conversation{}
	err := r.db.DB.QueryRow(
		"SELECT id, user_id, title, created_at, updated_at FROM conversations WHERE id = ?",
		id,
	).Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// ConversationWithPreview wraps Conversation with an optional preview snippet.
type ConversationWithPreview struct {
	Conversation
	Preview string `json:"preview"`
}

// ListByUserWithPreview returns conversations for a user, each with a short preview
// of the first message (up to 80 chars).
func (r *ConversationRepo) ListByUserWithPreview(userID string) ([]ConversationWithPreview, error) {
	rows, err := r.db.DB.Query(`
		SELECT c.id, c.user_id, c.title, c.created_at, c.updated_at,
			COALESCE((SELECT SUBSTR(m.content, 1, 80) FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at LIMIT 1), '')
		FROM conversations c WHERE c.user_id = ? ORDER BY c.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var convs []ConversationWithPreview
	for rows.Next() {
		var c ConversationWithPreview
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt, &c.Preview); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, rows.Err()
}

func (r *ConversationRepo) ListByUser(userID string) ([]Conversation, error) {
	rows, err := r.db.DB.Query(
		"SELECT id, user_id, title, created_at, updated_at FROM conversations WHERE user_id = ? ORDER BY updated_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var convs []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, rows.Err()
}

func (r *ConversationRepo) UpdateTitle(id, title string) error {
	result, err := r.db.DB.Exec(
		"UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?",
		title, time.Now(), id,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("conversation not found: %s", id)
	}
	return nil
}

func (r *ConversationRepo) Delete(id string) error {
	// Explicitly delete messages first since foreign key cascade may not be
	// enforced depending on driver configuration.
	if _, err := r.db.DB.Exec("DELETE FROM messages WHERE conversation_id = ?", id); err != nil {
		return fmt.Errorf("delete messages for conversation %s: %w", id, err)
	}
	result, err := r.db.DB.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("conversation not found: %s", id)
	}
	return nil
}

func (r *ConversationRepo) AddMessage(msg *MessageRecord) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	_, err := r.db.DB.Exec(
		`INSERT INTO messages (id, conversation_id, role, content, model, provider, tokens_in, tokens_out, cost, latency_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ConversationID, msg.Role, msg.Content,
		msg.Model, msg.Provider, msg.TokensIn, msg.TokensOut,
		msg.Cost, msg.LatencyMs, msg.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("add message: %w", err)
	}
	// Update conversation updated_at
	_, _ = r.db.DB.Exec(
		"UPDATE conversations SET updated_at = ? WHERE id = ?",
		time.Now(), msg.ConversationID,
	)
	return nil
}

func (r *ConversationRepo) DeleteMessageAndAfter(conversationID, messageID string) error {
	var createdAt time.Time
	err := r.db.DB.QueryRow(
		"SELECT created_at FROM messages WHERE id = ? AND conversation_id = ?",
		messageID, conversationID,
	).Scan(&createdAt)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}
	_, err = r.db.DB.Exec(
		"DELETE FROM messages WHERE conversation_id = ? AND created_at >= ?",
		conversationID, createdAt,
	)
	return err
}

func (r *ConversationRepo) GetMessages(conversationID string) ([]MessageRecord, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, conversation_id, role, content, model, provider, tokens_in, tokens_out, cost, latency_ms, created_at
		 FROM messages WHERE conversation_id = ? ORDER BY created_at ASC`,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []MessageRecord
	for rows.Next() {
		var m MessageRecord
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.Role, &m.Content,
			&m.Model, &m.Provider, &m.TokensIn, &m.TokensOut,
			&m.Cost, &m.LatencyMs, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
