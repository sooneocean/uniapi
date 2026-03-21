package webhook

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// WebhookConfig defines a single webhook endpoint and its subscribed event types.
type WebhookConfig struct {
	URL    string   `mapstructure:"url"`
	Events []string `mapstructure:"events"` // "provider_error", "quota_warning", "user_login", "account_bound"
}

// Event is the JSON payload delivered to a webhook endpoint.
type Event struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Manager dispatches events to registered webhook endpoints.
type Manager struct {
	hooks  []Hook
	client *http.Client
}

type Hook struct {
	URL    string
	Events map[string]bool
}

// NewManager creates a webhook Manager from a list of endpoint configurations.
func NewManager(configs []WebhookConfig) *Manager {
	hooks := make([]Hook, len(configs))
	for i, c := range configs {
		events := make(map[string]bool)
		for _, e := range c.Events {
			events[e] = true
		}
		hooks[i] = Hook{URL: c.URL, Events: events}
	}
	return &Manager{hooks: hooks, client: &http.Client{Timeout: 10 * time.Second}}
}

// Fire sends event to all matching webhooks asynchronously
func (m *Manager) Fire(eventType string, data interface{}) {
	if len(m.hooks) == 0 {
		return
	}
	event := Event{
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
	for _, h := range m.hooks {
		if !h.Events[eventType] && !h.Events["*"] {
			continue
		}
		go m.send(h.URL, event)
	}
}

func (m *Manager) send(url string, event Event) {
	body, _ := json.Marshal(event)
	resp, err := m.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Warn("webhook failed", "url", url, "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		slog.Warn("webhook error", "url", url, "status", resp.StatusCode)
	}
}
