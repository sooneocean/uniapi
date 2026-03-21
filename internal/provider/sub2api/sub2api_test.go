package sub2api

import (
	"context"
	"strings"
	"testing"

	"github.com/sooneocean/uniapi/internal/provider"
)

// credFunc helpers
func validCredFunc() (string, string) { return "tok_valid", "session_token" }
func emptyCredFunc() (string, string)  { return "", "session_token" }

// ---------------------------------------------------------------------------
// NewChatGPT
// ---------------------------------------------------------------------------

func TestNewChatGPT_Name(t *testing.T) {
	c := NewChatGPT([]string{"gpt-4o", "gpt-4"}, validCredFunc)
	if c.Name() != "chatgpt-web" {
		t.Fatalf("expected chatgpt-web, got %q", c.Name())
	}
}

func TestNewChatGPT_Models(t *testing.T) {
	want := []string{"gpt-4o", "gpt-4"}
	c := NewChatGPT(want, validCredFunc)
	models := c.Models()
	if len(models) != len(want) {
		t.Fatalf("expected %d models, got %d", len(want), len(models))
	}
	for i, m := range models {
		if m.ID != want[i] {
			t.Errorf("model[%d]: expected ID %q, got %q", i, want[i], m.ID)
		}
		if m.Provider != "chatgpt-web" {
			t.Errorf("model[%d]: expected Provider chatgpt-web, got %q", i, m.Provider)
		}
	}
}

// ---------------------------------------------------------------------------
// NewClaudeWeb
// ---------------------------------------------------------------------------

func TestNewClaudeWeb_Name(t *testing.T) {
	c := NewClaudeWeb([]string{"claude-3-opus"}, validCredFunc)
	if c.Name() != "claude-web" {
		t.Fatalf("expected claude-web, got %q", c.Name())
	}
}

// ---------------------------------------------------------------------------
// NewGeminiWeb
// ---------------------------------------------------------------------------

func TestNewGeminiWeb_Name(t *testing.T) {
	g := NewGeminiWeb([]string{"gemini-pro"}, validCredFunc)
	if g.Name() != "gemini-web" {
		t.Fatalf("expected gemini-web, got %q", g.Name())
	}
}

// ---------------------------------------------------------------------------
// GeminiWeb.ChatCompletion returns error (not implemented)
// ---------------------------------------------------------------------------

func TestGeminiWeb_ChatCompletion_NotImplemented(t *testing.T) {
	g := NewGeminiWeb([]string{"gemini-pro"}, validCredFunc)
	_, err := g.ChatCompletion(context.Background(), &provider.ChatRequest{Model: "gemini-pro"})
	if err == nil {
		t.Fatal("expected error for unimplemented ChatCompletion, got nil")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidateCredential
// ---------------------------------------------------------------------------

func TestValidateCredential_Valid(t *testing.T) {
	b := NewBase("test", "http://example.com", "bearer", "", nil, validCredFunc)
	p := &ChatGPT{Base: b}
	err := p.ValidateCredential(context.Background(), provider.Credential{APIKey: "tok_valid"})
	if err != nil {
		t.Fatalf("expected no error for valid token, got: %v", err)
	}
}

func TestValidateCredential_Empty(t *testing.T) {
	b := NewBase("test", "http://example.com", "bearer", "", nil, emptyCredFunc)
	p := &ChatGPT{Base: b}
	err := p.ValidateCredential(context.Background(), provider.Credential{APIKey: ""})
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("unexpected error text: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetUsage returns nil
// ---------------------------------------------------------------------------

func TestGetUsage_ReturnsNil(t *testing.T) {
	c := NewChatGPT([]string{"gpt-4o"}, validCredFunc)
	usage, err := c.GetUsage(context.Background(), provider.Credential{APIKey: "tok"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if usage != nil {
		t.Fatalf("expected nil usage, got: %+v", usage)
	}
}

// ---------------------------------------------------------------------------
// toChatGPTRequest conversion
// ---------------------------------------------------------------------------

func TestToChatGPTRequest(t *testing.T) {
	req := &provider.ChatRequest{
		Model: "gpt-4o",
		Messages: []provider.Message{
			{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}
	cgReq := toChatGPTRequest(req)
	if cgReq.Action != "next" {
		t.Errorf("expected action=next, got %q", cgReq.Action)
	}
	if cgReq.Model != "gpt-4o" {
		t.Errorf("expected model=gpt-4o, got %q", cgReq.Model)
	}
	if len(cgReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cgReq.Messages))
	}
	msg := cgReq.Messages[0]
	if msg.Author.Role != "user" {
		t.Errorf("expected role=user, got %q", msg.Author.Role)
	}
	if len(msg.Content.Parts) != 1 || msg.Content.Parts[0] != "Hello" {
		t.Errorf("unexpected content parts: %v", msg.Content.Parts)
	}
	if cgReq.ParentMessageID == "" {
		t.Error("expected non-empty ParentMessageID")
	}
}

// ---------------------------------------------------------------------------
// toClaudeWebRequest conversion
// ---------------------------------------------------------------------------

func TestToClaudeWebRequest(t *testing.T) {
	req := &provider.ChatRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []provider.Message{
			{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: "Hi Claude"}}},
		},
	}
	cwReq := toClaudeWebRequest(req)
	if cwReq.Model != "claude-3-opus" {
		t.Errorf("expected model=claude-3-opus, got %q", cwReq.Model)
	}
	if cwReq.MaxTokens != 1024 {
		t.Errorf("expected max_tokens=1024, got %d", cwReq.MaxTokens)
	}
	if !cwReq.Stream {
		t.Error("expected stream=true")
	}
	if len(cwReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cwReq.Messages))
	}
	msg := cwReq.Messages[0]
	if msg.Role != "user" {
		t.Errorf("expected role=user, got %q", msg.Role)
	}
	if len(msg.Content) != 1 || msg.Content[0].Text != "Hi Claude" {
		t.Errorf("unexpected content: %v", msg.Content)
	}
}

func TestToClaudeWebRequest_DefaultMaxTokens(t *testing.T) {
	req := &provider.ChatRequest{
		Model:    "claude-3-opus",
		Messages: []provider.Message{{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}}},
	}
	cwReq := toClaudeWebRequest(req)
	if cwReq.MaxTokens != 4096 {
		t.Errorf("expected default max_tokens=4096, got %d", cwReq.MaxTokens)
	}
}
