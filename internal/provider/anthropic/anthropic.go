package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sooneocean/uniapi/internal/provider"
)

const (
	defaultBaseURL      = "https://api.anthropic.com"
	anthropicVersion    = "2023-06-01"
	defaultMaxTokens    = 4096
)

// anthropicContentBlock is the wire format for a single content block in a message.
type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicMessage is the wire format for an Anthropic chat message.
type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

// anthropicRequest is the wire format sent to Anthropic.
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

// anthropicContentResp is a content block in the Anthropic response.
type anthropicContentResp struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicUsage holds token counts from Anthropic.
type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// anthropicResponse is the wire format received from Anthropic.
type anthropicResponse struct {
	ID         string                 `json:"id"`
	Model      string                 `json:"model"`
	Content    []anthropicContentResp `json:"content"`
	StopReason string                 `json:"stop_reason"`
	Usage      anthropicUsage         `json:"usage"`
}

// Anthropic is a provider.Provider implementation for the Anthropic API.
type Anthropic struct {
	cfg      provider.ProviderConfig
	modelIDs []string
	credFunc func() (credential string, authType string)
	baseURL  string
	client   *http.Client
}

// NewAnthropic constructs an Anthropic adapter.
func NewAnthropic(cfg provider.ProviderConfig, modelIDs []string, credFunc func() (string, string)) *Anthropic {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Anthropic{
		cfg:      cfg,
		modelIDs: modelIDs,
		credFunc: credFunc,
		baseURL:  baseURL,
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

// Name implements provider.Provider.
func (a *Anthropic) Name() string {
	if a.cfg.Name != "" {
		return a.cfg.Name
	}
	return "anthropic"
}

// Models implements provider.Provider.
func (a *Anthropic) Models() []provider.Model {
	models := make([]provider.Model, 0, len(a.modelIDs))
	for _, id := range a.modelIDs {
		models = append(models, provider.Model{
			ID:       id,
			Name:     id,
			Provider: a.Name(),
		})
	}
	return models
}

// convertRequest converts an internal ChatRequest to the Anthropic wire format.
func convertRequest(req *provider.ChatRequest) *anthropicRequest {
	msgs := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		blocks := make([]anthropicContentBlock, 0, len(m.Content))
		for _, block := range m.Content {
			blocks = append(blocks, anthropicContentBlock{
				Type: block.Type,
				Text: block.Text,
			})
		}
		msgs = append(msgs, anthropicMessage{
			Role:    m.Role,
			Content: blocks,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}

	return &anthropicRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}
}

// convertResponse converts an Anthropic wire response to the internal ChatResponse.
func convertResponse(resp *anthropicResponse) *provider.ChatResponse {
	content := make([]provider.ContentBlock, 0, len(resp.Content))
	for _, block := range resp.Content {
		content = append(content, provider.ContentBlock{
			Type: block.Type,
			Text: block.Text,
		})
	}
	return &provider.ChatResponse{
		Content:    content,
		Model:      resp.Model,
		TokensIn:   resp.Usage.InputTokens,
		TokensOut:  resp.Usage.OutputTokens,
		StopReason: resp.StopReason,
	}
}

// ChatCompletion implements provider.Provider.
func (a *Anthropic) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	wireReq := convertRequest(req)
	body, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	url := a.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	cred, authType := a.credFunc()
	httpReq.Header.Set("Content-Type", "application/json")
	if authType == "api_key" {
		httpReq.Header.Set("x-api-key", cred)
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+cred)
	}
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic: unexpected status %d", resp.StatusCode)
	}

	var wireResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&wireResp); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}

	return convertResponse(&wireResp), nil
}

// ChatCompletionStream implements provider.Provider.
func (a *Anthropic) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (provider.Stream, error) {
	cReq := convertRequest(req)
	cReq.Stream = true
	body, _ := json.Marshal(cReq)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create stream request: %w", err)
	}
	cred, authType := a.credFunc()
	httpReq.Header.Set("Content-Type", "application/json")
	if authType == "api_key" {
		httpReq.Header.Set("x-api-key", cred)
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+cred)
	}
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: do stream request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic error (%d): %s", resp.StatusCode, string(b))
	}

	return &anthropicStream{reader: bufio.NewReader(resp.Body), body: resp.Body, model: req.Model}, nil
}

type anthropicStream struct {
	reader    *bufio.Reader
	body      io.ReadCloser
	model     string
	done      bool
	eventType string
	tokensIn  int
	tokensOut int
}

func (s *anthropicStream) Next() (*provider.StreamEvent, error) {
	for {
		if s.done {
			return nil, io.EOF
		}
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			s.eventType = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		switch s.eventType {
		case "message_start":
			var msg struct {
				Message struct {
					Usage struct {
						InputTokens int `json:"input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &msg); err == nil {
				s.tokensIn = msg.Message.Usage.InputTokens
			}
		case "message_delta":
			var delta struct {
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &delta); err == nil {
				s.tokensOut = delta.Usage.OutputTokens
			}
		case "message_stop":
			s.done = true
			return &provider.StreamEvent{
				Type: "done",
				Response: &provider.ChatResponse{
					Model:     s.model,
					TokensIn:  s.tokensIn,
					TokensOut: s.tokensOut,
				},
			}, nil
		case "content_block_delta":
			var chunk struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if chunk.Delta.Text != "" {
				return &provider.StreamEvent{
					Type:    "content_delta",
					Content: provider.ContentBlock{Type: "text", Text: chunk.Delta.Text},
				}, nil
			}
		}
	}
}

func (s *anthropicStream) Close() error { return s.body.Close() }

// ValidateCredential implements provider.Provider.
func (a *Anthropic) ValidateCredential(ctx context.Context, cred provider.Credential) error {
	// Anthropic does not have a simple GET /models endpoint on the same path;
	// we send a minimal messages request to verify the key.
	model := "claude-haiku-4-20250414"
	if len(a.modelIDs) > 0 {
		model = a.modelIDs[0]
	}
	minReq := &anthropicRequest{
		Model:     model,
		MaxTokens: 1,
		Messages: []anthropicMessage{
			{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: "hi"}}},
		},
	}
	body, err := json.Marshal(minReq)
	if err != nil {
		return fmt.Errorf("anthropic: validate credential: %w", err)
	}

	url := a.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("anthropic: validate credential: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", cred.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("anthropic: validate credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("anthropic: invalid credential, status %d", resp.StatusCode)
	}
	return nil
}

// GetUsage implements provider.Provider.
func (a *Anthropic) GetUsage(ctx context.Context, cred provider.Credential) (*provider.Usage, error) {
	return &provider.Usage{}, nil
}
