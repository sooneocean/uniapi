package router

import (
    "context"
    "fmt"
    "testing"

    "github.com/user/uniapi/internal/cache"
    "github.com/user/uniapi/internal/provider"
)

type fakeProvider struct {
    name   string
    models []provider.Model
    fail   bool
}
func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Models() []provider.Model { return f.models }
func (f *fakeProvider) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
    if f.fail { return nil, fmt.Errorf("provider error") }
    return &provider.ChatResponse{
        Content: []provider.ContentBlock{{Type: "text", Text: "response from " + f.name}},
        Model: req.Model, TokensIn: 10, TokensOut: 5,
    }, nil
}
func (f *fakeProvider) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (provider.Stream, error) { return nil, nil }
func (f *fakeProvider) ValidateCredential(ctx context.Context, cred provider.Credential) error { return nil }
func (f *fakeProvider) GetUsage(ctx context.Context, cred provider.Credential) (*provider.Usage, error) { return &provider.Usage{}, nil }

func TestRouteToCorrectProvider(t *testing.T) {
    c := cache.New(); defer c.Stop()
    r := New(c, Config{Strategy: "round_robin", MaxRetries: 1, FailoverAttempts: 1})
    p1 := &fakeProvider{name: "openai", models: []provider.Model{{ID: "gpt-4o", Provider: "openai"}}}
    p2 := &fakeProvider{name: "claude", models: []provider.Model{{ID: "claude-sonnet-4-20250514", Provider: "claude"}}}
    r.AddAccount("acc1", p1, 5); r.AddAccount("acc2", p2, 5)
    resp, err := r.Route(context.Background(), &provider.ChatRequest{Model: "gpt-4o"})
    if err != nil { t.Fatal(err) }
    if resp.Content[0].Text != "response from openai" { t.Errorf("unexpected: %s", resp.Content[0].Text) }
}

func TestRouteNoProvider(t *testing.T) {
    c := cache.New(); defer c.Stop()
    r := New(c, Config{Strategy: "round_robin", MaxRetries: 1, FailoverAttempts: 1})
    _, err := r.Route(context.Background(), &provider.ChatRequest{Model: "nonexistent"})
    if err == nil { t.Error("expected error for unknown model") }
}

func TestRouteUserOwnership(t *testing.T) {
    c := cache.New()
    defer c.Stop()
    r := New(c, Config{Strategy: "round_robin", MaxRetries: 1, FailoverAttempts: 1})

    shared := &fakeProvider{name: "shared", models: []provider.Model{{ID: "gpt-4o", Provider: "shared"}}}
    private1 := &fakeProvider{name: "private1", models: []provider.Model{{ID: "gpt-4o", Provider: "private1"}}}

    r.AddAccountWithOwner("shared-acc", shared, 5, "")
    r.AddAccountWithOwner("priv-acc", private1, 5, "user-1")

    // user-1 sees both
    resp, err := r.Route(context.Background(), &provider.ChatRequest{Model: "gpt-4o"}, "user-1")
    if err != nil { t.Fatal(err) }
    if resp == nil { t.Fatal("expected response") }

    // user-2 sees only shared
    resp, err = r.Route(context.Background(), &provider.ChatRequest{Model: "gpt-4o"}, "user-2")
    if err != nil { t.Fatal(err) }
    if resp == nil { t.Fatal("expected response") }
}

func TestFailoverToNextAccount(t *testing.T) {
    c := cache.New(); defer c.Stop()
    r := New(c, Config{Strategy: "round_robin", MaxRetries: 1, FailoverAttempts: 2})
    failing := &fakeProvider{name: "p1", fail: true, models: []provider.Model{{ID: "model-a", Provider: "p1"}}}
    working := &fakeProvider{name: "p2", models: []provider.Model{{ID: "model-a", Provider: "p2"}}}
    r.AddAccount("acc1", failing, 5); r.AddAccount("acc2", working, 5)
    resp, err := r.Route(context.Background(), &provider.ChatRequest{Model: "model-a"})
    if err != nil { t.Fatalf("expected failover to succeed: %v", err) }
    if resp.Content[0].Text != "response from p2" { t.Errorf("expected p2, got: %s", resp.Content[0].Text) }
}
