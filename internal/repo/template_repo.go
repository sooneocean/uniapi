package repo

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/db"
)

type PromptTemplate struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"system_prompt"`
	UserPrompt   string    `json:"user_prompt"`
	Tags         string    `json:"tags"`
	Shared       bool      `json:"shared"`
	UseCount     int       `json:"use_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type TemplateRepo struct {
	db *db.Database
}

func NewTemplateRepo(database *db.Database) *TemplateRepo {
	return &TemplateRepo{db: database}
}

func (r *TemplateRepo) Create(userID, title, description, systemPrompt, userPrompt, tags string, shared bool) (*PromptTemplate, error) {
	t := &PromptTemplate{
		ID:           uuid.New().String(),
		UserID:       userID,
		Title:        title,
		Description:  description,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Tags:         tags,
		Shared:       shared,
		UseCount:     0,
		CreatedAt:    time.Now(),
	}
	_, err := r.db.DB.Exec(
		`INSERT INTO prompt_templates (id, user_id, title, description, system_prompt, user_prompt, tags, shared, use_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.Title, t.Description, t.SystemPrompt, t.UserPrompt, t.Tags, t.Shared, t.UseCount, t.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}
	return t, nil
}

func (r *TemplateRepo) List(userID string) ([]PromptTemplate, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, user_id, title, COALESCE(description,''), system_prompt, COALESCE(user_prompt,''), COALESCE(tags,''), shared, use_count, created_at
		 FROM prompt_templates
		 WHERE user_id = ? OR shared = 1
		 ORDER BY use_count DESC, created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var templates []PromptTemplate
	for rows.Next() {
		var t PromptTemplate
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.SystemPrompt, &t.UserPrompt, &t.Tags, &t.Shared, &t.UseCount, &t.CreatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func (r *TemplateRepo) GetByID(id string) (*PromptTemplate, error) {
	t := &PromptTemplate{}
	err := r.db.DB.QueryRow(
		`SELECT id, user_id, title, COALESCE(description,''), system_prompt, COALESCE(user_prompt,''), COALESCE(tags,''), shared, use_count, created_at
		 FROM prompt_templates WHERE id = ?`,
		id,
	).Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.SystemPrompt, &t.UserPrompt, &t.Tags, &t.Shared, &t.UseCount, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *TemplateRepo) Update(id, title, description, systemPrompt, userPrompt, tags string, shared bool) error {
	result, err := r.db.DB.Exec(
		`UPDATE prompt_templates SET title = ?, description = ?, system_prompt = ?, user_prompt = ?, tags = ?, shared = ? WHERE id = ?`,
		title, description, systemPrompt, userPrompt, tags, shared, id,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("template not found: %s", id)
	}
	return nil
}

func (r *TemplateRepo) Delete(id string) error {
	result, err := r.db.DB.Exec("DELETE FROM prompt_templates WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("template not found: %s", id)
	}
	return nil
}

func (r *TemplateRepo) IncrementUseCount(id string) (*PromptTemplate, error) {
	_, err := r.db.DB.Exec("UPDATE prompt_templates SET use_count = use_count + 1 WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}
