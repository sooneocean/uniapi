// Package sub2api provides adapters for web-session-based AI APIs (ChatGPT web,
// Claude.ai web, Gemini web) that use session tokens instead of official API keys.
package sub2api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sooneocean/uniapi/internal/provider"
)

// Base holds shared state and helpers for all sub2api adapters.
type Base struct {
	name       string
	baseURL    string
	credFunc   func() (string, string)
	modelIDs   []string
	client     *http.Client
	authStyle  string // "bearer" or "cookie"
	cookieKey  string // header/cookie name when authStyle == "cookie"
}

// NewBase constructs a Base with the given parameters.
func NewBase(name, baseURL string, authStyle, cookieKey string, modelIDs []string, credFunc func() (string, string)) Base {
	return Base{
		name:      name,
		baseURL:   baseURL,
		credFunc:  credFunc,
		modelIDs:  modelIDs,
		client:    provider.DefaultHTTPClient(),
		authStyle: authStyle,
		cookieKey: cookieKey,
	}
}

// Name returns the provider name.
func (b *Base) Name() string { return b.name }

// Models returns the list of models advertised by this provider.
func (b *Base) Models() []provider.Model {
	models := make([]provider.Model, 0, len(b.modelIDs))
	for _, id := range b.modelIDs {
		models = append(models, provider.Model{
			ID:       id,
			Name:     id,
			Provider: b.name,
		})
	}
	return models
}

// ValidateCredential returns an error if the credential token is empty.
func (b *Base) ValidateCredential(_ context.Context, cred provider.Credential) error {
	if cred.APIKey == "" {
		return fmt.Errorf("%s: credential token must not be empty", b.name)
	}
	return nil
}

// GetUsage is a no-op stub; web APIs do not expose usage statistics.
func (b *Base) GetUsage(_ context.Context, _ provider.Credential) (*provider.Usage, error) {
	return nil, nil
}

// doJSON performs an authenticated JSON request and returns the raw HTTP response.
// The caller is responsible for closing the response body.
// Returns structured errors for 401/403 (auth failure), 429 (rate limit), and 5xx (server error).
func (b *Base) doJSON(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%s: marshal request body: %w", b.name, err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	url := b.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", b.name, err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "text/event-stream, application/json")

	token, _ := b.credFunc()

	switch b.authStyle {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+token)
	case "cookie":
		req.AddCookie(&http.Cookie{Name: b.cookieKey, Value: token})
	default:
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: do request: %w", b.name, err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		defer resp.Body.Close()
		b2, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: authentication failed (%d): %s", b.name, resp.StatusCode, string(b2))
	case resp.StatusCode == http.StatusTooManyRequests:
		defer resp.Body.Close()
		return nil, fmt.Errorf("%s: rate limit exceeded (429)", b.name)
	case resp.StatusCode >= 500:
		defer resp.Body.Close()
		b2, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: server error (%d): %s", b.name, resp.StatusCode, string(b2))
	case resp.StatusCode != http.StatusOK:
		defer resp.Body.Close()
		b2, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: unexpected status %d: %s", b.name, resp.StatusCode, string(b2))
	}

	return resp, nil
}
