package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sooneocean/uniapi/internal/provider"
)

func TestConvertRequest(t *testing.T) {
	req := &provider.ChatRequest{
		Model: "gemini-1.5-flash",
		Messages: []provider.Message{
			{
				Role:    "system",
				Content: []provider.ContentBlock{{Type: "text", Text: "You are a helpful assistant."}},
			},
			{
				Role:    "user",
				Content: []provider.ContentBlock{{Type: "text", Text: "Hello Gemini!"}},
			},
			{
				Role:    "assistant",
				Content: []provider.ContentBlock{{Type: "text", Text: "Hello! How can I help?"}},
			},
		},
	}

	wireReq := convertRequest(req)

	// System message must NOT be in contents.
	if len(wireReq.Contents) != 2 {
		t.Fatalf("expected 2 contents (no system), got %d", len(wireReq.Contents))
	}

	// System instruction must be set.
	if wireReq.SystemInstruction == nil {
		t.Fatal("expected system_instruction to be set")
	}
	if len(wireReq.SystemInstruction.Parts) == 0 || wireReq.SystemInstruction.Parts[0].Text != "You are a helpful assistant." {
		t.Errorf("unexpected system_instruction text")
	}

	// user role stays "user".
	if wireReq.Contents[0].Role != "user" {
		t.Errorf("expected role user, got %s", wireReq.Contents[0].Role)
	}
	if wireReq.Contents[0].Parts[0].Text != "Hello Gemini!" {
		t.Errorf("unexpected user text: %s", wireReq.Contents[0].Parts[0].Text)
	}

	// assistant role maps to "model".
	if wireReq.Contents[1].Role != "model" {
		t.Errorf("expected role model for assistant, got %s", wireReq.Contents[1].Role)
	}
	if wireReq.Contents[1].Parts[0].Text != "Hello! How can I help?" {
		t.Errorf("unexpected assistant text: %s", wireReq.Contents[1].Parts[0].Text)
	}
}

func TestConvertResponse(t *testing.T) {
	resp := &geminiResponse{
		Candidates: []geminiCandidate{
			{
				Content: geminiContent{
					Role:  "model",
					Parts: []geminiPart{{Text: "Hello from Gemini!"}},
				},
				FinishReason: "STOP",
			},
		},
		UsageMetadata: geminiUsageMetadata{
			PromptTokenCount:     15,
			CandidatesTokenCount: 7,
		},
	}

	chatResp := convertResponse(resp, "gemini-1.5-flash")

	if chatResp.Model != "gemini-1.5-flash" {
		t.Errorf("unexpected model: %s", chatResp.Model)
	}
	if len(chatResp.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(chatResp.Content))
	}
	if chatResp.Content[0].Text != "Hello from Gemini!" {
		t.Errorf("unexpected text: %s", chatResp.Content[0].Text)
	}
	if chatResp.TokensIn != 15 {
		t.Errorf("expected tokens_in 15, got %d", chatResp.TokensIn)
	}
	if chatResp.TokensOut != 7 {
		t.Errorf("expected tokens_out 7, got %d", chatResp.TokensOut)
	}
	if chatResp.StopReason != "STOP" {
		t.Errorf("expected stop_reason STOP, got %s", chatResp.StopReason)
	}
}

func TestChatCompletionIntegration(t *testing.T) {
	mockResponse := geminiResponse{
		Candidates: []geminiCandidate{
			{
				Content: geminiContent{
					Role:  "model",
					Parts: []geminiPart{{Text: "Mock Gemini response."}},
				},
				FinishReason: "STOP",
			},
		},
		UsageMetadata: geminiUsageMetadata{
			PromptTokenCount:     9,
			CandidatesTokenCount: 4,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path prefix.
		if !strings.HasPrefix(r.URL.Path, "/v1beta/models/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Verify model and key in URL.
		if !strings.Contains(r.URL.Path, "gemini-1.5-flash") {
			t.Errorf("model not in path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "test-gemini-key" {
			t.Errorf("unexpected key param: %s", r.URL.Query().Get("key"))
		}
		// Verify method.
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Verify system message is NOT in contents.
		var body geminiRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		for _, c := range body.Contents {
			if c.Role == "system" {
				t.Errorf("system role should not appear in contents")
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := provider.ProviderConfig{
		Name:    "gemini-test",
		BaseURL: server.URL,
	}
	adapter := NewGemini(cfg, []string{"gemini-1.5-flash"}, func() (string, string) { return "test-gemini-key", "api_key" })

	req := &provider.ChatRequest{
		Model: "gemini-1.5-flash",
		Messages: []provider.Message{
			{
				Role:    "system",
				Content: []provider.ContentBlock{{Type: "text", Text: "Be concise."}},
			},
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
	if resp.Content[0].Text != "Mock Gemini response." {
		t.Errorf("unexpected content: %s", resp.Content[0].Text)
	}
	if resp.TokensIn != 9 {
		t.Errorf("unexpected tokens_in: %d", resp.TokensIn)
	}
	if resp.TokensOut != 4 {
		t.Errorf("unexpected tokens_out: %d", resp.TokensOut)
	}
}
