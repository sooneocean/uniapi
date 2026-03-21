package repo

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/db"
)

type SystemPrompt struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

type SystemPromptRepo struct {
	db *db.Database
}

func NewSystemPromptRepo(database *db.Database) *SystemPromptRepo {
	return &SystemPromptRepo{db: database}
}

func (r *SystemPromptRepo) Create(userID, name, content string, isDefault bool) (*SystemPrompt, error) {
	sp := &SystemPrompt{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      name,
		Content:   content,
		IsDefault: isDefault,
		CreatedAt: time.Now(),
	}
	_, err := r.db.DB.Exec(
		"INSERT INTO system_prompts (id, user_id, name, content, is_default, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		sp.ID, sp.UserID, sp.Name, sp.Content, sp.IsDefault, sp.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create system prompt: %w", err)
	}
	return sp, nil
}

func (r *SystemPromptRepo) ListByUser(userID string) ([]SystemPrompt, error) {
	rows, err := r.db.DB.Query(
		"SELECT id, user_id, name, content, is_default, created_at FROM system_prompts WHERE user_id = ? ORDER BY created_at ASC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prompts []SystemPrompt
	for rows.Next() {
		var sp SystemPrompt
		if err := rows.Scan(&sp.ID, &sp.UserID, &sp.Name, &sp.Content, &sp.IsDefault, &sp.CreatedAt); err != nil {
			return nil, err
		}
		prompts = append(prompts, sp)
	}
	return prompts, rows.Err()
}

func (r *SystemPromptRepo) GetByID(id string) (*SystemPrompt, error) {
	sp := &SystemPrompt{}
	err := r.db.DB.QueryRow(
		"SELECT id, user_id, name, content, is_default, created_at FROM system_prompts WHERE id = ?",
		id,
	).Scan(&sp.ID, &sp.UserID, &sp.Name, &sp.Content, &sp.IsDefault, &sp.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("system prompt not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	return sp, nil
}

func (r *SystemPromptRepo) Update(id, name, content string, isDefault bool) error {
	result, err := r.db.DB.Exec(
		"UPDATE system_prompts SET name = ?, content = ?, is_default = ? WHERE id = ?",
		name, content, isDefault, id,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("system prompt not found: %s", id)
	}
	return nil
}

func (r *SystemPromptRepo) Delete(id string) error {
	result, err := r.db.DB.Exec("DELETE FROM system_prompts WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("system prompt not found: %s", id)
	}
	return nil
}
