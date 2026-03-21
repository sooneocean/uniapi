package provider

import "context"

type ContentBlock struct {
	Type     string    `json:"type"`               // "text", "image", "tool_use", "tool_result"
	Text     string    `json:"text,omitempty"`
	ImageURL string    `json:"image_url,omitempty"` // base64 data URL or http URL
	ToolUse  *ToolCall `json:"tool_use,omitempty"`
	ToolResult *struct {
		ToolUseID string `json:"tool_use_id"`
		Content   string `json:"content"`
	} `json:"tool_result,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Provider    string    `json:"provider,omitempty"`
}

type ChatResponse struct {
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	TokensIn   int            `json:"tokens_in"`
	TokensOut  int            `json:"tokens_out"`
	StopReason string         `json:"stop_reason,omitempty"` // "stop", "tool_use", "end_turn"
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
}

type StreamEvent struct {
	Type     string        `json:"type"`
	Content  ContentBlock  `json:"content,omitempty"`
	Response *ChatResponse `json:"response,omitempty"`
	Error    string        `json:"error,omitempty"`
}

type Stream interface {
	Next() (*StreamEvent, error)
	Close() error
}

type Model struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

type Credential struct {
	APIKey string
}

type Usage struct {
	TotalTokensIn  int
	TotalTokensOut int
	TotalCost      float64
}

type ProviderConfig struct {
	Name    string
	Type    string
	BaseURL string
	Options map[string]string
}

type Provider interface {
	Name() string
	Models() []Model
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req *ChatRequest) (Stream, error)
	ValidateCredential(ctx context.Context, cred Credential) error
	GetUsage(ctx context.Context, cred Credential) (*Usage, error)
}

type ProviderFactory func(config ProviderConfig) (Provider, error)
