package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/user/uniapi/internal/provider"
)

const defaultBaseURL = "https://api.openai.com"

// openaiMessage is the wire format for OpenAI chat messages.
type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiRequest is the wire format sent to OpenAI.
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// openaiChoice represents a single choice in the OpenAI response.
type openaiChoice struct {
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
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
	apiKey   string
	baseURL  string
	client   *http.Client
}

// NewOpenAI constructs an OpenAI adapter.
func NewOpenAI(cfg provider.ProviderConfig, modelIDs []string, apiKey string) *OpenAI {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &OpenAI{
		cfg:      cfg,
		modelIDs: modelIDs,
		apiKey:   apiKey,
		baseURL:  baseURL,
		client:   &http.Client{},
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

// convertRequest converts an internal ChatRequest to the OpenAI wire format.
func convertRequest(req *provider.ChatRequest) *openaiRequest {
	msgs := make([]openaiMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		text := ""
		for _, block := range m.Content {
			text += block.Text
		}
		msgs = append(msgs, openaiMessage{
			Role:    m.Role,
			Content: text,
		})
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
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

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
	return nil, fmt.Errorf("streaming not yet implemented")
}

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
