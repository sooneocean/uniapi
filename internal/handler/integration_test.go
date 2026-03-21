package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sooneocean/uniapi/internal/auth"
	"github.com/sooneocean/uniapi/internal/cache"
	"github.com/sooneocean/uniapi/internal/crypto"
	"github.com/sooneocean/uniapi/internal/db"
	"github.com/sooneocean/uniapi/internal/provider"
	"github.com/sooneocean/uniapi/internal/repo"
	"github.com/sooneocean/uniapi/internal/router"
	"github.com/sooneocean/uniapi/internal/usage"
)

// setupIntegration creates a fully wired test server
func setupIntegration(t *testing.T) (*gin.Engine, *db.Database, *auth.JWTManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	encKey, _ := crypto.DeriveKey("test-secret")
	jwtKey, _ := crypto.DeriveKeyWithInfo("test-secret", "jwt")
	jwtMgr := auth.NewJWTManager(jwtKey, 1*time.Hour)

	userRepo := repo.NewUserRepo(database)
	_ = repo.NewAccountRepo(database, encKey)
	recorder := usage.NewRecorder(database.DB)

	// Create admin user
	hash, _ := auth.HashPassword("admin123")
	userRepo.Create("admin", hash, "admin")

	// Setup router with fake provider
	memCache := cache.New()
	t.Cleanup(func() { memCache.Stop() })
	rtr := router.New(memCache, router.Config{Strategy: "round_robin", MaxRetries: 1, FailoverAttempts: 1})

	fake := &fakeProvider{name: "test", models: []provider.Model{{ID: "test-model", Name: "test-model", Provider: "test"}}}
	rtr.AddAccount("test-acc", fake, 5)

	// Build engine
	engine := gin.New()
	engine.Use(CORSMiddleware(nil))

	// Auth routes
	authHandler := NewAuthHandler(userRepo, jwtMgr, database, nil) // nil audit for tests
	api := engine.Group("/api")
	api.GET("/status", authHandler.Status)
	api.POST("/setup", authHandler.Setup)
	api.POST("/login", authHandler.Login)

	apiAuth := api.Group("")
	apiAuth.Use(JWTAuthMiddleware(jwtMgr))
	apiAuth.GET("/me", authHandler.Me)

	// API routes
	apiHandler := NewAPIHandler(rtr, recorder)
	v1 := engine.Group("/v1")
	v1.Use(APIKeyAuthMiddleware(database.DB, jwtMgr, memCache))
	v1.POST("/chat/completions", apiHandler.ChatCompletions)
	v1.GET("/models", apiHandler.ListModels)

	return engine, database, jwtMgr
}

func TestIntegration_FullAuthFlow(t *testing.T) {
	engine, _, _ := setupIntegration(t)

	// 1. Check status
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/status", nil)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}

	// 2. Login
	w = httptest.NewRecorder()
	body := `{"username":"admin","password":"admin123"}`
	req, _ = http.NewRequest("POST", "/api/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}

	// Extract token from Set-Cookie
	cookies := w.Result().Cookies()
	var token string
	for _, c := range cookies {
		if c.Name == "token" {
			token = c.Value
			break
		}
	}
	if token == "" {
		t.Fatal("no token cookie")
	}

	// 3. Access /api/me with cookie
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("me: %d %s", w.Code, w.Body.String())
	}

	var me map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &me)
	if me["username"] != "admin" {
		t.Errorf("expected admin, got %v", me["username"])
	}
}

func TestIntegration_ChatWithJWT(t *testing.T) {
	engine, _, jwtMgr := setupIntegration(t)

	// Get a JWT for the admin user
	token, _ := jwtMgr.CreateToken("admin-id", "admin")

	// Use JWT as Bearer token for /v1/chat/completions
	w := httptest.NewRecorder()
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("chat: %d %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	choices := resp["choices"].([]interface{})
	if len(choices) == 0 {
		t.Error("no choices")
	}
}

func TestIntegration_UnauthorizedAccess(t *testing.T) {
	engine, _, _ := setupIntegration(t)

	// /api/me without auth
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/me", nil)
	engine.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}

	// /v1/chat/completions without auth
	w = httptest.NewRecorder()
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`
	req, _ = http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestIntegration_WrongPassword(t *testing.T) {
	engine, _, _ := setupIntegration(t)

	w := httptest.NewRecorder()
	body := `{"username":"admin","password":"wrongpass"}`
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestIntegration_InvalidModel(t *testing.T) {
	engine, _, jwtMgr := setupIntegration(t)
	token, _ := jwtMgr.CreateToken("admin-id", "admin")

	w := httptest.NewRecorder()
	body := `{"model":"nonexistent","messages":[{"role":"user","content":"hello"}]}`
	req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)
	if w.Code == 200 {
		t.Error("expected error for nonexistent model")
	}
}
