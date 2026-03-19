package provider

import (
	"context"
	"testing"
)

type mockProvider struct {
	name   string
	models []Model
}

func (m *mockProvider) Name() string    { return m.name }
func (m *mockProvider) Models() []Model { return m.models }
func (m *mockProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{Content: []ContentBlock{{Type: "text", Text: "mock response"}}}, nil
}
func (m *mockProvider) ChatCompletionStream(ctx context.Context, req *ChatRequest) (Stream, error) {
	return nil, nil
}
func (m *mockProvider) ValidateCredential(ctx context.Context, cred Credential) error { return nil }
func (m *mockProvider) GetUsage(ctx context.Context, cred Credential) (*Usage, error) {
	return &Usage{}, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	mock := &mockProvider{name: "test", models: []Model{{ID: "test-model", Name: "Test Model"}}}
	reg.RegisterFactory("test", func(cfg ProviderConfig) (Provider, error) { return mock, nil })
	p, err := reg.Create("test", ProviderConfig{Name: "test", Type: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "test" {
		t.Errorf("expected test, got %s", p.Name())
	}
}

func TestRegistryUnknownType(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Create("unknown", ProviderConfig{})
	if err == nil {
		t.Error("unknown type should fail")
	}
}

func TestRegistryListModels(t *testing.T) {
	reg := NewRegistry()
	mock1 := &mockProvider{name: "p1", models: []Model{{ID: "model-a", Name: "A"}}}
	mock2 := &mockProvider{name: "p2", models: []Model{{ID: "model-b", Name: "B"}}}
	reg.RegisterFactory("type1", func(cfg ProviderConfig) (Provider, error) { return mock1, nil })
	reg.RegisterFactory("type2", func(cfg ProviderConfig) (Provider, error) { return mock2, nil })
	reg.Create("type1", ProviderConfig{Name: "p1", Type: "type1"})
	reg.Create("type2", ProviderConfig{Name: "p2", Type: "type2"})
	models := reg.AllModels()
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}
