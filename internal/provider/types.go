// Package provider defines the shared types and interface for all AI provider adapters.
package provider

import "context"

// ContentBlock is a single typed element within a message (text, image, tool call, or tool result).
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

// ToolCall represents a function invocation requested by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// Message is a single turn in a conversation with a role and structured content.
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// Tool describes a callable function that the model may invoke.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

// ChatRequest is the canonical request payload passed to a provider adapter.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Provider    string    `json:"provider,omitempty"`
}

// ChatResponse is the canonical response returned by a provider adapter.
type ChatResponse struct {
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	TokensIn   int            `json:"tokens_in"`
	TokensOut  int            `json:"tokens_out"`
	StopReason string         `json:"stop_reason,omitempty"` // "stop", "tool_use", "end_turn"
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
}

// StreamEvent is a single event emitted by a streaming provider response.
type StreamEvent struct {
	Type     string        `json:"type"`
	Content  ContentBlock  `json:"content,omitempty"`
	Response *ChatResponse `json:"response,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// Stream is the interface for reading incremental events from a streaming provider response.
type Stream interface {
	Next() (*StreamEvent, error)
	Close() error
}

// Model represents a single AI model available from a provider.
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

// Provider is the interface that all AI provider adapters must implement.
type Provider interface {
	Name() string
	Models() []Model
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req *ChatRequest) (Stream, error)
	ValidateCredential(ctx context.Context, cred Credential) error
	GetUsage(ctx context.Context, cred Credential) (*Usage, error)
}

// ProviderFactory is a constructor function registered for each provider type.
type ProviderFactory func(config ProviderConfig) (Provider, error)
