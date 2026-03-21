package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIntegration_ConversationCRUD(t *testing.T) {
	engine, _, jwtMgr, adminID := setupIntegrationFull(t)
	token, _ := jwtMgr.CreateToken(adminID, "admin")

	// Create conversation
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/conversations", bytes.NewBufferString(`{"title":"Test Chat"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create convo: %d %s", w.Code, w.Body.String())
	}

	var createResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	convoID, _ := createResp["id"].(string)
	if convoID == "" {
		t.Fatal("no conversation ID returned")
	}

	// List conversations
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/conversations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list: %d", w.Code)
	}

	var listResp []interface{}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	if len(listResp) == 0 {
		t.Error("expected at least 1 conversation")
	}

	// Get conversation
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/conversations/"+convoID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("get: %d", w.Code)
	}

	// Update title
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/conversations/"+convoID, bytes.NewBufferString(`{"title":"Updated Title"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}

	// Delete conversation
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/conversations/"+convoID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("delete: %d", w.Code)
	}
}

func TestIntegration_APIKeyManagement(t *testing.T) {
	engine, _, jwtMgr, adminID := setupIntegrationFull(t)
	token, _ := jwtMgr.CreateToken(adminID, "admin")

	// Create API key
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/api-keys", bytes.NewBufferString(`{"label":"Test Key"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create key: %d %s", w.Code, w.Body.String())
	}

	var keyResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &keyResp)
	apiKey, _ := keyResp["key"].(string)
	if apiKey == "" {
		t.Fatal("no API key returned")
	}
	keyID, _ := keyResp["id"].(string)

	// Use API key to access /v1/models
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("models with api key: %d", w.Code)
	}

	// List API keys
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list keys: %d", w.Code)
	}

	// Delete API key
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/api-keys/"+keyID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("delete key: %d", w.Code)
	}
}

func TestIntegration_UserManagement(t *testing.T) {
	engine, _, jwtMgr, adminID := setupIntegrationFull(t)
	token, _ := jwtMgr.CreateToken(adminID, "admin")

	// Create user
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/users", bytes.NewBufferString(`{"username":"bob","password":"password123","role":"member"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create user: %d %s", w.Code, w.Body.String())
	}

	// List users
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list users: %d", w.Code)
	}

	var users []interface{}
	json.Unmarshal(w.Body.Bytes(), &users)
	if len(users) < 2 {
		t.Errorf("expected 2+ users, got %d", len(users))
	}
}

func TestIntegration_ProviderTemplates(t *testing.T) {
	engine, _, jwtMgr, adminID := setupIntegrationFull(t)
	token, _ := jwtMgr.CreateToken(adminID, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/provider-templates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("templates: %d", w.Code)
	}

	var templates []interface{}
	json.Unmarshal(w.Body.Bytes(), &templates)
	if len(templates) < 5 {
		t.Errorf("expected 5+ templates, got %d", len(templates))
	}
}
