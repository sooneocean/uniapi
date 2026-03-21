package scheduler

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/provider"
)

// RouteFn is the signature of the router's Route function.
type RouteFn func(ctx context.Context, req *provider.ChatRequest, userID ...string) (*provider.ChatResponse, error)

// Scheduler polls the database for pending tasks and executes them.
type Scheduler struct {
	db      *sql.DB
	routeFn RouteFn
	stopCh  chan struct{}
}

// New creates a Scheduler backed by the given database and route function.
func New(db *sql.DB, routeFn RouteFn) *Scheduler {
	return &Scheduler{db: db, routeFn: routeFn, stopCh: make(chan struct{})}
}

// Start begins the background polling loop.
func (s *Scheduler) Start() {
	go s.loop()
}

// Stop shuts down the polling loop.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.runDue()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) runDue() {
	now := time.Now()
	rows, err := s.db.Query(
		"SELECT id, user_id, model, prompt, system_prompt FROM scheduled_tasks WHERE status = 'pending' AND run_at <= ? AND run_at IS NOT NULL",
		now)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, userID, model, prompt, sysPrompt string
		if err := rows.Scan(&id, &userID, &model, &prompt, &sysPrompt); err != nil {
			continue
		}
		go s.executeTask(id, userID, model, prompt, sysPrompt)
	}
}

func (s *Scheduler) executeTask(id, userID, model, prompt, sysPrompt string) {
	s.db.Exec("UPDATE scheduled_tasks SET status = 'running' WHERE id = ?", id) //nolint:errcheck

	messages := []provider.Message{}
	if sysPrompt != "" {
		messages = append(messages, provider.Message{Role: "system", Content: []provider.ContentBlock{{Type: "text", Text: sysPrompt}}})
	}
	messages = append(messages, provider.Message{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: prompt}}})

	resp, err := s.routeFn(context.Background(), &provider.ChatRequest{Model: model, Messages: messages, MaxTokens: 4096}, userID)

	result := ""
	status := "completed"
	if err != nil {
		result = "Error: " + err.Error()
		status = "failed"
	} else if len(resp.Content) > 0 {
		result = resp.Content[0].Text
	}

	s.db.Exec("UPDATE scheduled_tasks SET status = ?, result = ?, last_run = ? WHERE id = ?", //nolint:errcheck
		status, result, time.Now(), id)
	slog.Info("scheduled task completed", "id", id, "status", status)
}

// Create inserts a new scheduled task.
func (s *Scheduler) Create(userID, model, prompt, sysPrompt string, runAt time.Time) (string, error) {
	id := uuid.New().String()
	_, err := s.db.Exec(
		"INSERT INTO scheduled_tasks (id, user_id, model, prompt, system_prompt, run_at) VALUES (?,?,?,?,?,?)",
		id, userID, model, prompt, sysPrompt, runAt)
	return id, err
}

// List returns all scheduled tasks for a user.
func (s *Scheduler) List(userID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		"SELECT id, model, prompt, system_prompt, run_at, last_run, result, status, created_at FROM scheduled_tasks WHERE user_id = ? ORDER BY created_at DESC",
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []map[string]interface{}
	for rows.Next() {
		var id, model, prompt, sysPrompt, status, createdAt string
		var runAt, lastRun, result sql.NullString
		rows.Scan(&id, &model, &prompt, &sysPrompt, &runAt, &lastRun, &result, &status, &createdAt) //nolint:errcheck
		tasks = append(tasks, map[string]interface{}{
			"id": id, "model": model, "prompt": prompt, "system_prompt": sysPrompt,
			"run_at": runAt.String, "last_run": lastRun.String, "result": result.String,
			"status": status, "created_at": createdAt,
		})
	}
	if tasks == nil {
		tasks = []map[string]interface{}{}
	}
	return tasks, nil
}

// Delete removes a scheduled task owned by the user.
func (s *Scheduler) Delete(id, userID string) error {
	_, err := s.db.Exec("DELETE FROM scheduled_tasks WHERE id = ? AND user_id = ?", id, userID)
	return err
}

// GetResult returns the result of a specific task.
func (s *Scheduler) GetResult(id, userID string) (map[string]interface{}, error) {
	var taskID, model, prompt, status, result, createdAt string
	var runAt, lastRun sql.NullString
	err := s.db.QueryRow(
		"SELECT id, model, prompt, status, result, run_at, last_run, created_at FROM scheduled_tasks WHERE id = ? AND user_id = ?",
		id, userID).Scan(&taskID, &model, &prompt, &status, &result, &runAt, &lastRun, &createdAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": taskID, "model": model, "prompt": prompt, "status": status,
		"result": result, "run_at": runAt.String, "last_run": lastRun.String, "created_at": createdAt,
	}, nil
}
