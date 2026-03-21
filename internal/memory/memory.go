package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/sooneocean/uniapi/internal/provider"
)

// Manager trims or summarises conversation history when it exceeds a token budget.
type Manager struct {
	maxTokens int // default 8000 — leave room for response
}

// NewManager creates a memory Manager with the given token budget (default 8000).
func NewManager(maxTokens int) *Manager {
	if maxTokens <= 0 {
		maxTokens = 8000
	}
	return &Manager{maxTokens: maxTokens}
}

// EstimateTokens roughly estimates token count (~4 chars per token)
func EstimateTokens(text string) int {
	return len(text) / 4
}

// CompactMessages summarizes older messages if total exceeds maxTokens.
// Keeps system prompt + last N messages intact, summarizes the rest.
func (m *Manager) CompactMessages(ctx context.Context, messages []provider.Message, routeFn func(context.Context, *provider.ChatRequest) (*provider.ChatResponse, error)) []provider.Message {
	total := 0
	for _, msg := range messages {
		for _, c := range msg.Content {
			total += EstimateTokens(c.Text)
		}
	}

	if total <= m.maxTokens {
		return messages // fits, no compaction needed
	}

	// Strategy: keep first message (system prompt if any) + last 6 messages
	// Summarize everything in between
	keepFirst := 0
	if len(messages) > 0 && messages[0].Role == "system" {
		keepFirst = 1
	}

	keepLast := 6
	if keepLast >= len(messages)-keepFirst {
		return messages // not enough to summarize
	}

	middleStart := keepFirst
	middleEnd := len(messages) - keepLast

	if middleEnd <= middleStart {
		return messages
	}

	// Build summary request
	var sb strings.Builder
	sb.WriteString("Summarize this conversation concisely, preserving key facts, decisions, and context:\n\n")
	for _, msg := range messages[middleStart:middleEnd] {
		sb.WriteString(fmt.Sprintf("[%s]: ", msg.Role))
		for _, c := range msg.Content {
			if c.Text != "" {
				sb.WriteString(c.Text)
			}
		}
		sb.WriteString("\n")
	}

	summaryReq := &provider.ChatRequest{
		Model:     "", // use whatever is available
		Messages:  []provider.Message{{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: sb.String()}}}},
		MaxTokens: 500,
	}

	resp, err := routeFn(ctx, summaryReq)
	if err != nil {
		// On failure, just truncate instead
		return append(messages[:keepFirst], messages[middleEnd:]...)
	}

	summaryText := "Previous conversation summary: "
	if len(resp.Content) > 0 {
		summaryText += resp.Content[0].Text
	}

	// Build compacted messages
	result := make([]provider.Message, 0, keepFirst+1+keepLast)
	result = append(result, messages[:keepFirst]...)
	result = append(result, provider.Message{
		Role:    "system",
		Content: []provider.ContentBlock{{Type: "text", Text: summaryText}},
	})
	result = append(result, messages[middleEnd:]...)

	return result
}
