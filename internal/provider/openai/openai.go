package openai

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

const defaultBaseURL = "https://api.openai.com"

// openaiMessage is the wire format for OpenAI chat messages.
// Content may be a plain string or a slice of content parts (for vision).
type openaiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []openaiContentPart
}

// openaiContentPart is a single element in a multi-part message (vision).
type openaiContentPart struct {
	Type     string              `json:"type"`
	Text     string              `json:"text,omitempty"`
	ImageURL *openaiImageURLPart `json:"image_url,omitempty"`
}

// openaiImageURLPart holds the URL for an image content part.
type openaiImageURLPart struct {
	URL string `json:"url"`
}

// openaiRequest is the wire format sent to OpenAI.
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// openaiResponseMessage is the message structure inside an OpenAI response choice.
// The response content is always a plain string.
type openaiResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiChoice represents a single choice in the OpenAI response.
type openaiChoice struct {
	Message      openaiResponseMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

// openaiUsage holds token counts from OpenAI.
type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openaiResponse is the wire format received from OpenAI.
type openaiResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

// OpenAI is a provider.Provider implementation for the OpenAI API.
type OpenAI struct {
	cfg      provider.ProviderConfig
	modelIDs []string
	credFunc func() (credential string, authType string)
	baseURL  string
	client   *http.Client
}

// NewOpenAI constructs an OpenAI adapter.
func NewOpenAI(cfg provider.ProviderConfig, modelIDs []string, credFunc func() (string, string)) *OpenAI {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &OpenAI{
		cfg:      cfg,
		modelIDs: modelIDs,
		credFunc: credFunc,
		baseURL:  baseURL,
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

// Name implements provider.Provider.
func (o *OpenAI) Name() string {
	if o.cfg.Name != "" {
		return o.cfg.Name
	}
	return "openai"
}

// Models implements provider.Provider.
func (o *OpenAI) Models() []provider.Model {
	models := make([]provider.Model, 0, len(o.modelIDs))
	for _, id := range o.modelIDs {
		models = append(models, provider.Model{
			ID:       id,
			Name:     id,
			Provider: o.Name(),
		})
	}
	return models
}

// hasImageBlocks returns true if any content block is an image.
func hasImageBlocks(blocks []provider.ContentBlock) bool {
	for _, b := range blocks {
		if b.Type == "image" || b.ImageURL != "" {
			return true
		}
	}
	return false
}

// convertRequest converts an internal ChatRequest to the OpenAI wire format.
func convertRequest(req *provider.ChatRequest) *openaiRequest {
	msgs := make([]openaiMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if hasImageBlocks(m.Content) {
			// Use multi-part content format for vision messages.
			parts := make([]openaiContentPart, 0, len(m.Content))
			for _, block := range m.Content {
				if block.ImageURL != "" {
					parts = append(parts, openaiContentPart{
						Type:     "image_url",
						ImageURL: &openaiImageURLPart{URL: block.ImageURL},
					})
				} else if block.Text != "" {
					parts = append(parts, openaiContentPart{
						Type: "text",
						Text: block.Text,
					})
				}
			}
			msgs = append(msgs, openaiMessage{Role: m.Role, Content: parts})
		} else {
			text := ""
			for _, block := range m.Content {
				text += block.Text
			}
			msgs = append(msgs, openaiMessage{Role: m.Role, Content: text})
		}
	}
	return &openaiRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}
}

// convertResponse converts an OpenAI wire response to the internal ChatResponse.
func convertResponse(resp *openaiResponse) *provider.ChatResponse {
	var content []provider.ContentBlock
	var stopReason string
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		content = []provider.ContentBlock{
			{Type: "text", Text: choice.Message.Content},
		}
		stopReason = choice.FinishReason
	}
	return &provider.ChatResponse{
		Content:    content,
		Model:      resp.Model,
		TokensIn:   resp.Usage.PromptTokens,
		TokensOut:  resp.Usage.CompletionTokens,
		StopReason: stopReason,
	}
}

// messageContent extracts the string content from an openaiMessage (for tests).
func messageContent(m openaiMessage) string {
	if s, ok := m.Content.(string); ok {
		return s
	}
	return ""
}

// ChatCompletion implements provider.Provider.
func (o *OpenAI) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	wireReq := convertRequest(req)
	body, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	url := o.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	cred, authType := o.credFunc()
	_ = authType // OpenAI and openai_compatible both use Bearer regardless of authType
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cred)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: unexpected status %d", resp.StatusCode)
	}

	var wireResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&wireResp); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	return convertResponse(&wireResp), nil
}

// ChatCompletionStream implements provider.Provider.
func (o *OpenAI) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (provider.Stream, error) {
	oaiReq := convertRequest(req)
	oaiReq.Stream = true
	body, _ := json.Marshal(oaiReq)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create stream request: %w", err)
	}
	cred, authType := o.credFunc()
	_ = authType // OpenAI and openai_compatible both use Bearer regardless of authType
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cred)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: do stream request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error (%d): %s", resp.StatusCode, string(b))
	}

	return &sseStream{reader: bufio.NewReader(resp.Body), body: resp.Body, ctx: ctx, model: req.Model}, nil
}

type sseStream struct {
	reader    *bufio.Reader
	body      io.ReadCloser
	ctx       context.Context
	model     string
	done      bool
	tokensIn  int
	tokensOut int
}

func (s *sseStream) Next() (*provider.StreamEvent, error) {
	for {
		if s.done {
			return nil, io.EOF
		}
		// Check context cancellation
		select {
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		default:
		}
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			s.done = true
			return &provider.StreamEvent{
				Type: "done",
				Response: &provider.ChatResponse{
					Model:     s.model,
					TokensIn:  s.tokensIn,
					TokensOut: s.tokensOut,
				},
			}, nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			s.tokensIn = chunk.Usage.PromptTokens
			s.tokensOut = chunk.Usage.CompletionTokens
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			return &provider.StreamEvent{
				Type:    "content_delta",
				Content: provider.ContentBlock{Type: "text", Text: chunk.Choices[0].Delta.Content},
			}, nil
		}
	}
}

func (s *sseStream) Close() error { return s.body.Close() }

// ValidateCredential implements provider.Provider.
func (o *OpenAI) ValidateCredential(ctx context.Context, cred provider.Credential) error {
	url := o.baseURL + "/v1/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("openai: validate credential: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+cred.APIKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("openai: validate credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openai: invalid credential, status %d", resp.StatusCode)
	}
	return nil
}

// GetUsage implements provider.Provider.
func (o *OpenAI) GetUsage(ctx context.Context, cred provider.Credential) (*provider.Usage, error) {
	return &provider.Usage{}, nil
}
