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

const chatgptBaseURL = "https://chatgpt.com/backend-api"

// ChatGPT is a provider.Provider adapter for the ChatGPT web API using session tokens.
type ChatGPT struct {
	Base
}

// NewChatGPT constructs a ChatGPT adapter.
func NewChatGPT(modelIDs []string, credFunc func() (string, string)) *ChatGPT {
	return &ChatGPT{
		Base: NewBase("chatgpt-web", chatgptBaseURL, "bearer", "", modelIDs, credFunc),
	}
}

// ChatCompletion sends a chat request and returns the full response.
// It consumes the SSE stream internally and returns the last assistant message.
func (c *ChatGPT) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	resp, err := c.doJSON(ctx, http.MethodPost, "/conversation", toChatGPTRequest(req))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var last *chatgptSSEData
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
		var data chatgptSSEData
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			continue
		}
		if data.Error != nil {
			return nil, fmt.Errorf("chatgpt-web: API error: %s", *data.Error)
		}
		if data.Message.Author.Role == "assistant" {
			last = &data
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("chatgpt-web: read SSE stream: %w", err)
	}
	if last == nil {
		return nil, fmt.Errorf("chatgpt-web: no assistant message received")
	}
	return chatgptDataToResponse(last, req.Model), nil
}

// ChatCompletionStream opens a streaming request and returns a provider.Stream.
func (c *ChatGPT) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (provider.Stream, error) {
	resp, err := c.doJSON(ctx, http.MethodPost, "/conversation", toChatGPTRequest(req))
	if err != nil {
		return nil, err
	}
	return &sseStream{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
		ctx:    ctx,
		model:  req.Model,
	}, nil
}

// sseStream implements provider.Stream over a ChatGPT SSE response body.
type sseStream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	ctx    context.Context
	model  string
	done   bool
}

// Next reads the next stream event. Returns io.EOF when the stream ends.
func (s *sseStream) Next() (*provider.StreamEvent, error) {
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
			return nil, fmt.Errorf("chatgpt-web: stream read: %w", err)
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

		var data chatgptSSEData
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			continue
		}
		if data.Error != nil {
			return nil, fmt.Errorf("chatgpt-web: API error: %s", *data.Error)
		}
		if data.Message.Author.Role != "assistant" {
			continue
		}
		return chatgptDataToStreamEvent(&data), nil
	}
}

// Close closes the underlying HTTP response body.
func (s *sseStream) Close() error { return s.body.Close() }
