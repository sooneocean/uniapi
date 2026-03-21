package memory

import (
	"context"
	"testing"

	"github.com/sooneocean/uniapi/internal/provider"
)

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Error("empty should be 0")
	}
	// ~4 chars per token
	est := EstimateTokens("hello world test") // 16 chars = 4 tokens
	if est < 3 || est > 5 {
		t.Errorf("expected ~4, got %d", est)
	}
}

func TestCompactNoAction(t *testing.T) {
	mgr := NewManager(8000)
	msgs := []provider.Message{
		{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: "short message"}}},
		{Role: "assistant", Content: []provider.ContentBlock{{Type: "text", Text: "short reply"}}},
	}
	result := mgr.CompactMessages(context.Background(), msgs, nil)
	if len(result) != 2 {
		t.Errorf("short messages should not be compacted, got %d", len(result))
	}
}

func TestCompactLongConversation(t *testing.T) {
	mgr := NewManager(100) // very low limit to force compaction

	msgs := make([]provider.Message, 20)
	for i := range msgs {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = provider.Message{
			Role:    role,
			Content: []provider.ContentBlock{{Type: "text", Text: "This is a long message with enough content to exceed token limits easily and force compaction behavior in the memory manager"}},
		}
	}

	mockRoute := func(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
		return &provider.ChatResponse{Content: []provider.ContentBlock{{Type: "text", Text: "Summary of conversation"}}}, nil
	}

	result := mgr.CompactMessages(context.Background(), msgs, mockRoute)
	if len(result) >= len(msgs) {
		t.Errorf("expected compaction, original=%d result=%d", len(msgs), len(result))
	}
}
