package plugin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/provider"
)

type Plugin struct {
	ID          string            `json:"id"`
	UserID      string            `json:"user_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	InputSchema json.RawMessage   `json:"input_schema"`
	Enabled     bool              `json:"enabled"`
	Shared      bool              `json:"shared"`
}

type Manager struct {
	db     *sql.DB
	client *http.Client
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db, client: &http.Client{Timeout: 30 * time.Second}}
}

func (m *Manager) Register(userID, name, description, endpoint, method string, headers map[string]string, inputSchema json.RawMessage, shared bool) (*Plugin, error) {
	id := uuid.New().String()
	headersJSON, _ := json.Marshal(headers)
	_, err := m.db.Exec(
		"INSERT INTO plugins (id, user_id, name, description, endpoint, method, headers, input_schema, shared) VALUES (?,?,?,?,?,?,?,?,?)",
		id, userID, name, description, endpoint, method, string(headersJSON), string(inputSchema), shared,
	)
	if err != nil {
		return nil, err
	}
	return &Plugin{ID: id, UserID: userID, Name: name, Description: description, Endpoint: endpoint, Method: method, Headers: headers, InputSchema: inputSchema, Shared: shared, Enabled: true}, nil
}

func (m *Manager) List(userID string) ([]Plugin, error) {
	rows, err := m.db.Query("SELECT id, user_id, name, description, endpoint, method, headers, input_schema, enabled, shared FROM plugins WHERE user_id = ? OR shared = 1 ORDER BY name", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var plugins []Plugin
	for rows.Next() {
		var p Plugin
		var headersStr, schemaStr string
		rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Endpoint, &p.Method, &headersStr, &schemaStr, &p.Enabled, &p.Shared)
		json.Unmarshal([]byte(headersStr), &p.Headers)
		p.InputSchema = json.RawMessage(schemaStr)
		plugins = append(plugins, p)
	}
	return plugins, nil
}

func (m *Manager) GetByID(id, userID string) (*Plugin, error) {
	var p Plugin
	var headersStr, schemaStr string
	err := m.db.QueryRow("SELECT id, user_id, name, description, endpoint, method, headers, input_schema, enabled, shared FROM plugins WHERE id = ? AND (user_id = ? OR shared = 1)", id, userID).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.Endpoint, &p.Method, &headersStr, &schemaStr, &p.Enabled, &p.Shared)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(headersStr), &p.Headers)
	p.InputSchema = json.RawMessage(schemaStr)
	return &p, nil
}

func (m *Manager) Delete(id, userID string) error {
	_, err := m.db.Exec("DELETE FROM plugins WHERE id = ? AND user_id = ?", id, userID)
	return err
}

// Execute calls the plugin's endpoint with the given arguments
func (m *Manager) Execute(p Plugin, arguments string) (string, error) {
	var body io.Reader
	if arguments != "" {
		body = bytes.NewBufferString(arguments)
	}
	req, err := http.NewRequest(p.Method, p.Endpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("plugin call failed: %w", err)
	}
	defer resp.Body.Close()
	result, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("plugin error (%d): %s", resp.StatusCode, string(result))
	}
	return string(result), nil
}

// ToTools converts plugins to provider Tool definitions for injection into requests
func (m *Manager) ToTools(userID string) ([]provider.Tool, error) {
	plugins, err := m.List(userID)
	if err != nil {
		return nil, err
	}
	var tools []provider.Tool
	for _, p := range plugins {
		if !p.Enabled {
			continue
		}
		tools = append(tools, provider.Tool{
			Name: p.Name, Description: p.Description, InputSchema: p.InputSchema,
		})
	}
	return tools, nil
}
