package sub2api

import (
	"context"
	"fmt"

	"github.com/sooneocean/uniapi/internal/provider"
)

const geminiWebBaseURL = "https://gemini.google.com/api"

// GeminiWeb is a stub provider.Provider adapter for the Gemini web API.
// Neither chat method is implemented yet.
type GeminiWeb struct {
	Base
}

// NewGeminiWeb constructs a GeminiWeb stub adapter. credFunc should return
// the __Secure-1PSID cookie value as the first string.
func NewGeminiWeb(modelIDs []string, credFunc func() (string, string)) *GeminiWeb {
	return &GeminiWeb{
		Base: NewBase("gemini-web", geminiWebBaseURL, "cookie", "__Secure-1PSID", modelIDs, credFunc),
	}
}

// ChatCompletion is not yet implemented.
func (g *GeminiWeb) ChatCompletion(_ context.Context, _ *provider.ChatRequest) (*provider.ChatResponse, error) {
	return nil, fmt.Errorf("gemini-web: not yet implemented")
}

// ChatCompletionStream is not yet implemented.
func (g *GeminiWeb) ChatCompletionStream(_ context.Context, _ *provider.ChatRequest) (provider.Stream, error) {
	return nil, fmt.Errorf("gemini-web: not yet implemented")
}
