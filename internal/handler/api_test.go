package handler

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/sooneocean/uniapi/internal/cache"
    "github.com/sooneocean/uniapi/internal/provider"
    "github.com/sooneocean/uniapi/internal/router"
)

type fakeProvider struct {
    name   string
    models []provider.Model
}
func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Models() []provider.Model { return f.models }
func (f *fakeProvider) ChatCompletion(_ context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
    return &provider.ChatResponse{Content: []provider.ContentBlock{{Type: "text", Text: "test response"}}, Model: req.Model, TokensIn: 10, TokensOut: 5}, nil
}
func (f *fakeProvider) ChatCompletionStream(_ context.Context, _ *provider.ChatRequest) (provider.Stream, error) { return nil, nil }
func (f *fakeProvider) ValidateCredential(_ context.Context, _ provider.Credential) error { return nil }
func (f *fakeProvider) GetUsage(_ context.Context, _ provider.Credential) (*provider.Usage, error) { return &provider.Usage{}, nil }

func setupTestRouter() *gin.Engine {
    gin.SetMode(gin.TestMode)
    c := cache.New()
    r := router.New(c, router.Config{Strategy: "round_robin", MaxRetries: 1, FailoverAttempts: 1})
    fake := &fakeProvider{name: "test", models: []provider.Model{{ID: "test-model", Name: "test-model", Provider: "test"}}}
    r.AddAccount("acc1", fake, 5)
    engine := gin.New()
    api := NewAPIHandler(r, nil)
    v1 := engine.Group("/v1")
    v1.POST("/chat/completions", api.ChatCompletions)
    v1.GET("/models", api.ListModels)
    return engine
}

func TestListModels(t *testing.T) {
    engine := setupTestRouter()
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/v1/models", nil)
    engine.ServeHTTP(w, req)
    if w.Code != 200 { t.Errorf("expected 200, got %d", w.Code) }
    var resp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &resp)
    data := resp["data"].([]interface{})
    if len(data) != 1 { t.Errorf("expected 1 model, got %d", len(data)) }
}

func TestChatCompletions(t *testing.T) {
    engine := setupTestRouter()
    body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
    req.Header.Set("Content-Type", "application/json")
    engine.ServeHTTP(w, req)
    if w.Code != 200 { t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String()) }
    var resp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &resp)
    choices := resp["choices"].([]interface{})
    if len(choices) == 0 { t.Error("expected at least 1 choice") }
}

func TestChatCompletionsInvalidModel(t *testing.T) {
    engine := setupTestRouter()
    body := `{"model":"nonexistent","messages":[{"role":"user","content":"hello"}]}`
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
    req.Header.Set("Content-Type", "application/json")
    engine.ServeHTTP(w, req)
    if w.Code == 200 { t.Error("expected error for nonexistent model") }
}
