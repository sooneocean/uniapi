package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/user/uniapi/internal/provider"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// geminiPart is a single text part in a Gemini content block.
type geminiPart struct {
	Text string `json:"text"`
}

// geminiContent is a content block (role + parts) used in both requests and responses.
type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

// geminiRequest is the wire format sent to Gemini generateContent.
type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
}

// geminiCandidate is a single candidate in the Gemini response.
type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

// geminiUsageMetadata holds token counts from Gemini.
type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// geminiResponse is the wire format received from Gemini.
type geminiResponse struct {
	Candidates    []geminiCandidate   `json:"candidates"`
	UsageMetadata geminiUsageMetadata `json:"usageMetadata"`
}

// Gemini is a provider.Provider implementation for the Google Gemini API.
type Gemini struct {
	cfg      provider.ProviderConfig
	modelIDs []string
	apiKey   string
	baseURL  string
	client   *http.Client
}

// NewGemini constructs a Gemini adapter.
func NewGemini(cfg provider.ProviderConfig, modelIDs []string, apiKey string) *Gemini {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Gemini{
		cfg:      cfg,
		modelIDs: modelIDs,
		apiKey:   apiKey,
		baseURL:  baseURL,
		client:   &http.Client{},
	}
}

// Name implements provider.Provider.
func (g *Gemini) Name() string {
	if g.cfg.Name != "" {
		return g.cfg.Name
	}
	return "gemini"
}

// Models implements provider.Provider.
func (g *Gemini) Models() []provider.Model {
	models := make([]provider.Model, 0, len(g.modelIDs))
	for _, id := range g.modelIDs {
		models = append(models, provider.Model{
			ID:       id,
			Name:     id,
			Provider: g.Name(),
		})
	}
	return models
}

// convertRequest converts an internal ChatRequest to the Gemini wire format.
// System role messages are extracted to system_instruction; other messages
// map user→user and assistant→model.
func convertRequest(req *provider.ChatRequest) *geminiRequest {
	wireReq := &geminiRequest{}

	for _, m := range req.Messages {
		// Collect text from all content blocks.
		text := ""
		for _, block := range m.Content {
			text += block.Text
		}

		switch m.Role {
		case "system":
			// Gemini uses a dedicated system_instruction field.
			if wireReq.SystemInstruction == nil {
				wireReq.SystemInstruction = &geminiContent{
					Parts: []geminiPart{{Text: text}},
				}
			} else {
				// Append additional system text to existing instruction.
				wireReq.SystemInstruction.Parts = append(
					wireReq.SystemInstruction.Parts,
					geminiPart{Text: text},
				)
			}
		case "assistant":
			wireReq.Contents = append(wireReq.Contents, geminiContent{
				Role:  "model",
				Parts: []geminiPart{{Text: text}},
			})
		default:
			// "user" and any unknown roles pass through.
			wireReq.Contents = append(wireReq.Contents, geminiContent{
				Role:  m.Role,
				Parts: []geminiPart{{Text: text}},
			})
		}
	}

	return wireReq
}

// convertResponse converts a Gemini wire response to the internal ChatResponse.
func convertResponse(resp *geminiResponse, model string) *provider.ChatResponse {
	var content []provider.ContentBlock
	var stopReason string

	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		stopReason = candidate.FinishReason
		for _, part := range candidate.Content.Parts {
			content = append(content, provider.ContentBlock{
				Type: "text",
				Text: part.Text,
			})
		}
	}

	return &provider.ChatResponse{
		Content:    content,
		Model:      model,
		TokensIn:   resp.UsageMetadata.PromptTokenCount,
		TokensOut:  resp.UsageMetadata.CandidatesTokenCount,
		StopReason: stopReason,
	}
}

// ChatCompletion implements provider.Provider.
func (g *Gemini) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	wireReq := convertRequest(req)
	body, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", g.baseURL, req.Model, g.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini: unexpected status %d", resp.StatusCode)
	}

	var wireResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&wireResp); err != nil {
		return nil, fmt.Errorf("gemini: decode response: %w", err)
	}

	return convertResponse(&wireResp, req.Model), nil
}

// ChatCompletionStream implements provider.Provider.
func (g *Gemini) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (provider.Stream, error) {
	return nil, fmt.Errorf("streaming not yet implemented")
}

// ValidateCredential implements provider.Provider.
func (g *Gemini) ValidateCredential(ctx context.Context, cred provider.Credential) error {
	model := "gemini-1.5-flash"
	if len(g.modelIDs) > 0 {
		model = g.modelIDs[0]
	}
	url := fmt.Sprintf("%s/v1beta/models/%s?key=%s", g.baseURL, model, cred.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("gemini: validate credential: %w", err)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("gemini: validate credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gemini: invalid credential, status %d", resp.StatusCode)
	}
	return nil
}

// GetUsage implements provider.Provider.
func (g *Gemini) GetUsage(ctx context.Context, cred provider.Credential) (*provider.Usage, error) {
	return &provider.Usage{}, nil
}
