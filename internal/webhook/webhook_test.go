package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestFireMatchingEvent(t *testing.T) {
	var received Event
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer server.Close()

	mgr := NewManager([]WebhookConfig{{URL: server.URL, Events: []string{"user_login"}}})
	mgr.Fire("user_login", map[string]string{"user": "alice"})

	time.Sleep(100 * time.Millisecond) // async
	mu.Lock()
	defer mu.Unlock()
	if received.Type != "user_login" {
		t.Errorf("expected user_login, got %s", received.Type)
	}
}

func TestFireNonMatchingEvent(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	defer server.Close()

	mgr := NewManager([]WebhookConfig{{URL: server.URL, Events: []string{"user_login"}}})
	mgr.Fire("provider_error", nil) // different event
	time.Sleep(100 * time.Millisecond)
	if called {
		t.Error("should not fire for non-matching event")
	}
}

func TestFireWildcard(t *testing.T) {
	var received Event
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer server.Close()

	mgr := NewManager([]WebhookConfig{{URL: server.URL, Events: []string{"*"}}})
	mgr.Fire("anything", nil)
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if received.Type != "anything" {
		t.Errorf("wildcard should match, got %s", received.Type)
	}
}

func TestFireNoHooks(t *testing.T) {
	mgr := NewManager(nil)
	mgr.Fire("test", nil) // should not panic
}
