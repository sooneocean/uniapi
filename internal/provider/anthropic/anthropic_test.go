package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sooneocean/uniapi/internal/provider"
)

func TestConvertRequest(t *testing.T) {
	req := &provider.ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: []provider.ContentBlock{{Type: "text", Text: "Hello Claude!"}},
			},
		},
		MaxTokens: 512,
	}

	wireReq := convertRequest(req)

	if wireReq.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected model claude-3-5-sonnet-20241022, got %s", wireReq.Model)
	}
	if len(wireReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(wireReq.Messages))
	}
	if wireReq.Messages[0].Role != "user" {
		t.Errorf("expected role user, got %s", wireReq.Messages[0].Role)
	}
	if len(wireReq.Messages[0].Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(wireReq.Messages[0].Content))
	}
	if wireReq.Messages[0].Content[0].Type != "text" {
		t.Errorf("expected content type text, got %s", wireReq.Messages[0].Content[0].Type)
	}
	if wireReq.Messages[0].Content[0].Text != "Hello Claude!" {
		t.Errorf("unexpected text: %s", wireReq.Messages[0].Content[0].Text)
	}
	if wireReq.MaxTokens != 512 {
		t.Errorf("expected max_tokens 512, got %d", wireReq.MaxTokens)
	}
}

func TestConvertRequestDefaultMaxTokens(t *testing.T) {
	req := &provider.ChatRequest{
		Model: "claude-3-haiku-20240307",
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}},
			},
		},
		// MaxTokens intentionally left as 0 to trigger default.
	}

	wireReq := convertRequest(req)

	if wireReq.MaxTokens != defaultMaxTokens {
		t.Errorf("expected default max_tokens %d, got %d", defaultMaxTokens, wireReq.MaxTokens)
	}
}

func TestConvertResponse(t *testing.T) {
	resp := &anthropicResponse{
		ID:    "msg_abc",
		Model: "claude-3-5-sonnet-20241022",
		Content: []anthropicContentResp{
			{Type: "text", Text: "Hello from Claude!"},
		},
		StopReason: "end_turn",
		Usage: anthropicUsage{
			InputTokens:  12,
			OutputTokens: 6,
		},
	}

	chatResp := convertResponse(resp)

	if chatResp.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("unexpected model: %s", chatResp.Model)
	}
	if len(chatResp.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(chatResp.Content))
	}
	if chatResp.Content[0].Text != "Hello from Claude!" {
		t.Errorf("unexpected text: %s", chatResp.Content[0].Text)
	}
	if chatResp.TokensIn != 12 {
		t.Errorf("expected tokens_in 12, got %d", chatResp.TokensIn)
	}
	if chatResp.TokensOut != 6 {
		t.Errorf("expected tokens_out 6, got %d", chatResp.TokensOut)
	}
	if chatResp.StopReason != "end_turn" {
		t.Errorf("expected stop_reason end_turn, got %s", chatResp.StopReason)
	}
}

func TestChatCompletionIntegration(t *testing.T) {
	mockResponse := anthropicResponse{
		ID:    "msg_test",
		Model: "claude-3-5-sonnet-20241022",
		Content: []anthropicContentResp{
			{Type: "text", Text: "Mock response from Claude."},
		},
		StopReason: "end_turn",
		Usage: anthropicUsage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path.
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Verify API key header.
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "test-anthropic-key" {
			t.Errorf("unexpected x-api-key: %s", apiKey)
		}
		// Verify version header.
		version := r.Header.Get("anthropic-version")
		if version != anthropicVersion {
			t.Errorf("unexpected anthropic-version: %s", version)
		}
		// Verify method.
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := provider.ProviderConfig{
		Name:    "anthropic-test",
		BaseURL: server.URL,
	}
	adapter := NewAnthropic(cfg, []string{"claude-3-5-sonnet-20241022"}, func() (string, string) { return "test-anthropic-key", "api_key" })

	req := &provider.ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: []provider.ContentBlock{{Type: "text", Text: "Hello!"}},
			},
		},
	}

	resp, err := adapter.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	if resp.Content[0].Text != "Mock response from Claude." {
		t.Errorf("unexpected content: %s", resp.Content[0].Text)
	}
	if resp.TokensIn != 10 {
		t.Errorf("unexpected tokens_in: %d", resp.TokensIn)
	}
	if resp.TokensOut != 5 {
		t.Errorf("unexpected tokens_out: %d", resp.TokensOut)
	}
}
