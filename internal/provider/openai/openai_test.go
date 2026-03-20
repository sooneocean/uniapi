package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/uniapi/internal/provider"
)

func TestConvertRequest(t *testing.T) {
	temp := 0.7
	req := &provider.ChatRequest{
		Model: "gpt-4o",
		Messages: []provider.Message{
			{
				Role:    "system",
				Content: []provider.ContentBlock{{Type: "text", Text: "You are helpful."}},
			},
			{
				Role:    "user",
				Content: []provider.ContentBlock{{Type: "text", Text: "Hello!"}},
			},
		},
		MaxTokens:   256,
		Temperature: &temp,
	}

	wireReq := convertRequest(req)

	if wireReq.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", wireReq.Model)
	}
	if len(wireReq.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(wireReq.Messages))
	}
	if wireReq.Messages[0].Role != "system" {
		t.Errorf("expected first message role system, got %s", wireReq.Messages[0].Role)
	}
	if wireReq.Messages[0].Content != "You are helpful." {
		t.Errorf("unexpected content: %s", wireReq.Messages[0].Content)
	}
	if wireReq.Messages[1].Role != "user" {
		t.Errorf("expected second message role user, got %s", wireReq.Messages[1].Role)
	}
	if wireReq.MaxTokens != 256 {
		t.Errorf("expected max_tokens 256, got %d", wireReq.MaxTokens)
	}
	if wireReq.Temperature == nil || *wireReq.Temperature != 0.7 {
		t.Errorf("unexpected temperature")
	}
}

func TestConvertResponse(t *testing.T) {
	resp := &openaiResponse{
		ID:    "chatcmpl-abc",
		Model: "gpt-4o",
		Choices: []openaiChoice{
			{
				Message:      openaiMessage{Role: "assistant", Content: "Hello there!"},
				FinishReason: "stop",
			},
		},
		Usage: openaiUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
		},
	}

	chatResp := convertResponse(resp)

	if chatResp.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", chatResp.Model)
	}
	if len(chatResp.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(chatResp.Content))
	}
	if chatResp.Content[0].Text != "Hello there!" {
		t.Errorf("unexpected content text: %s", chatResp.Content[0].Text)
	}
	if chatResp.TokensIn != 10 {
		t.Errorf("expected tokens_in 10, got %d", chatResp.TokensIn)
	}
	if chatResp.TokensOut != 5 {
		t.Errorf("expected tokens_out 5, got %d", chatResp.TokensOut)
	}
	if chatResp.StopReason != "stop" {
		t.Errorf("expected stop_reason stop, got %s", chatResp.StopReason)
	}
}

func TestChatCompletionIntegration(t *testing.T) {
	mockResponse := openaiResponse{
		ID:    "chatcmpl-test",
		Model: "gpt-4o",
		Choices: []openaiChoice{
			{
				Message:      openaiMessage{Role: "assistant", Content: "Hi from mock!"},
				FinishReason: "stop",
			},
		},
		Usage: openaiUsage{
			PromptTokens:     8,
			CompletionTokens: 4,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path.
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Verify auth header.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("unexpected auth header: %s", auth)
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
		Name:    "openai-test",
		BaseURL: server.URL,
	}
	adapter := NewOpenAI(cfg, []string{"gpt-4o"}, func() (string, string) { return "test-api-key", "api_key" })

	req := &provider.ChatRequest{
		Model: "gpt-4o",
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
	if resp.Content[0].Text != "Hi from mock!" {
		t.Errorf("unexpected content: %s", resp.Content[0].Text)
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("unexpected model: %s", resp.Model)
	}
	if resp.TokensIn != 8 {
		t.Errorf("unexpected tokens_in: %d", resp.TokensIn)
	}
	if resp.TokensOut != 4 {
		t.Errorf("unexpected tokens_out: %d", resp.TokensOut)
	}
}
