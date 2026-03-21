package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sooneocean/uniapi/internal/db"
)

func setupPlugin(t *testing.T) *Manager {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.DB.Exec("INSERT INTO users (id, username, password, role) VALUES ('u1', 'alice', 'h', 'admin')")
	return NewManager(database.DB)
}

func TestRegisterAndList(t *testing.T) {
	m := setupPlugin(t)
	schema := json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)
	p, err := m.Register("u1", "weather", "Get weather", "https://api.weather.com", "POST", nil, schema, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "weather" {
		t.Errorf("expected weather, got %s", p.Name)
	}

	plugins, err := m.List("u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 {
		t.Errorf("expected 1, got %d", len(plugins))
	}
}

func TestExecutePlugin(t *testing.T) {
	m := setupPlugin(t)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"temperature": 25}`))
	}))
	defer server.Close()

	schema := json.RawMessage(`{}`)
	p, _ := m.Register("u1", "test", "Test", server.URL, "POST", nil, schema, false)

	result, err := m.Execute(*p, `{"city":"Taipei"}`)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected result")
	}
}

func TestExecutePluginError(t *testing.T) {
	m := setupPlugin(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	p, _ := m.Register("u1", "bad", "Bad", server.URL, "POST", nil, json.RawMessage(`{}`), false)
	_, err := m.Execute(*p, `{}`)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestDeletePlugin(t *testing.T) {
	m := setupPlugin(t)
	p, _ := m.Register("u1", "temp", "Temp", "http://x", "POST", nil, json.RawMessage(`{}`), false)
	m.Delete(p.ID, "u1")
	plugins, _ := m.List("u1")
	if len(plugins) != 0 {
		t.Error("expected 0 after delete")
	}
}

func TestToTools(t *testing.T) {
	m := setupPlugin(t)
	m.Register("u1", "weather", "Get weather", "http://x", "POST", nil, json.RawMessage(`{"type":"object"}`), false)
	tools, err := m.ToTools("u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "weather" {
		t.Error("wrong tool name")
	}
}
