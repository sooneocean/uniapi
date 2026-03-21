package sub2api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sooneocean/uniapi/internal/provider"
)

const claudeWebBaseURL = "https://claude.ai/api"

// ---------------------------------------------------------------------------
// Claude web API wire types
// ---------------------------------------------------------------------------

type claudeWebContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type claudeWebMessage struct {
	Role    string                 `json:"role"`
	Content []claudeWebContentPart `json:"content"`
}

type claudeWebRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens,omitempty"`
	Messages  []claudeWebMessage `json:"messages"`
	Stream    bool               `json:"stream"`
}

// claudeWebSSEData represents the subset of SSE event data shapes returned by
// the Claude.ai web backend that we care about.
type claudeWebSSEData struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	ContentBlock *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content_block,omitempty"`
	Message *struct {
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message,omitempty"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// toClaudeWebRequest converts a provider.ChatRequest to the Claude web wire format.
func toClaudeWebRequest(req *provider.ChatRequest) claudeWebRequest {
	msgs := make([]claudeWebMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		parts := make([]claudeWebContentPart, 0, len(m.Content))
		for _, block := range m.Content {
			if block.Text != "" {
				parts = append(parts, claudeWebContentPart{Type: "text", Text: block.Text})
			}
		}
		if len(parts) == 0 {
			parts = []claudeWebContentPart{{Type: "text", Text: ""}}
		}
		msgs = append(msgs, claudeWebMessage{Role: m.Role, Content: parts})
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	return claudeWebRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  msgs,
		Stream:    true,
	}
}

// ---------------------------------------------------------------------------
// ClaudeWeb adapter
// ---------------------------------------------------------------------------

// ClaudeWeb is a provider.Provider adapter for the Claude.ai web API.
type ClaudeWeb struct {
	Base
}

// NewClaudeWeb constructs a ClaudeWeb adapter. credFunc should return the
// sessionKey cookie value as the first string.
func NewClaudeWeb(modelIDs []string, credFunc func() (string, string)) *ClaudeWeb {
	return &ClaudeWeb{
		Base: NewBase("claude-web", claudeWebBaseURL, "cookie", "sessionKey", modelIDs, credFunc),
	}
}

// ChatCompletion sends a streaming request internally and collects the full response.
func (c *ClaudeWeb) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	resp, err := c.doJSON(ctx, http.MethodPost, "/chat/completions", toClaudeWebRequest(req))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var (
		accumulated strings.Builder
		stopReason  string
		tokensIn    int
		tokensOut   int
		model       = req.Model
	)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var data claudeWebSSEData
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			continue
		}

		if data.Error != nil {
			return nil, fmt.Errorf("claude-web: API error (%s): %s", data.Error.Type, data.Error.Message)
		}

		switch data.Type {
		case "message_start":
			if data.Message != nil {
				if data.Message.Model != "" {
					model = data.Message.Model
				}
				tokensIn = data.Message.Usage.InputTokens
			}
		case "content_block_delta":
			if data.Delta != nil && data.Delta.Type == "text_delta" {
				accumulated.WriteString(data.Delta.Text)
			}
		case "message_delta":
			if data.Delta != nil {
				stopReason = data.Delta.Type
			}
			if data.Message != nil {
				tokensOut = data.Message.Usage.OutputTokens
				if data.Message.StopReason != "" {
					stopReason = data.Message.StopReason
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("claude-web: read SSE stream: %w", err)
	}

	return &provider.ChatResponse{
		Content:    []provider.ContentBlock{{Type: "text", Text: accumulated.String()}},
		Model:      model,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		StopReason: stopReason,
	}, nil
}

// ChatCompletionStream opens a streaming request and returns a provider.Stream.
func (c *ClaudeWeb) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (provider.Stream, error) {
	resp, err := c.doJSON(ctx, http.MethodPost, "/chat/completions", toClaudeWebRequest(req))
	if err != nil {
		return nil, err
	}
	return &claudeWebStream{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
		ctx:    ctx,
		model:  req.Model,
	}, nil
}

// ---------------------------------------------------------------------------
// claudeWebStream — implements provider.Stream
// ---------------------------------------------------------------------------

type claudeWebStream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	ctx    context.Context
	model  string
	done   bool
}

// Next returns the next stream event.
func (s *claudeWebStream) Next() (*provider.StreamEvent, error) {
	for {
		if s.done {
			return nil, io.EOF
		}
		select {
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		default:
		}

		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.done = true
				return nil, io.EOF
			}
			return nil, fmt.Errorf("claude-web: stream read: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			s.done = true
			return &provider.StreamEvent{
				Type:     "done",
				Response: &provider.ChatResponse{Model: s.model},
			}, nil
		}

		var data claudeWebSSEData
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			continue
		}
		if data.Error != nil {
			return nil, fmt.Errorf("claude-web: API error (%s): %s", data.Error.Type, data.Error.Message)
		}

		if data.Type == "content_block_delta" && data.Delta != nil && data.Delta.Type == "text_delta" {
			return &provider.StreamEvent{
				Type:    "content_delta",
				Content: provider.ContentBlock{Type: "text", Text: data.Delta.Text},
			}, nil
		}

		if data.Type == "message_stop" {
			s.done = true
			return &provider.StreamEvent{
				Type:     "done",
				Response: &provider.ChatResponse{Model: s.model},
			}, nil
		}
	}
}

// Close closes the underlying HTTP response body.
func (s *claudeWebStream) Close() error { return s.body.Close() }
