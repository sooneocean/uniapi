package workflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/sooneocean/uniapi/internal/provider"
)

func mockRoute(ctx context.Context, req *provider.ChatRequest, userID string) (*provider.ChatResponse, error) {
	// Echo back the user message content
	text := ""
	for _, m := range req.Messages {
		if m.Role == "user" {
			for _, c := range m.Content {
				text += c.Text
			}
		}
	}
	return &provider.ChatResponse{
		Content:   []provider.ContentBlock{{Type: "text", Text: "Processed: " + text}},
		TokensIn:  10,
		TokensOut: 20,
	}, nil
}

func TestSingleStep(t *testing.T) {
	wf := Workflow{
		Name:  "test",
		Steps: []Step{{Name: "step1", Model: "gpt-4o", UserPrompt: "{{input}}"}},
	}
	result, err := Execute(context.Background(), wf, "hello", mockRoute, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) != 1 {
		t.Errorf("expected 1 step result")
	}
	if result.FinalOutput == "" {
		t.Error("expected non-empty output")
	}
}

func TestMultiStep(t *testing.T) {
	wf := Workflow{
		Name: "chain",
		Steps: []Step{
			{Name: "translate", Model: "gpt-4o", UserPrompt: "Translate: {{input}}"},
			{Name: "summarize", Model: "gpt-4o", UserPrompt: "Summarize: {{step_1}}"},
		},
	}
	result, err := Execute(context.Background(), wf, "hello world", mockRoute, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(result.Steps))
	}
	if result.Steps[1].Output == "" {
		t.Error("step 2 should have output")
	}
}

func TestStepFailure(t *testing.T) {
	failRoute := func(ctx context.Context, req *provider.ChatRequest, userID string) (*provider.ChatResponse, error) {
		return nil, fmt.Errorf("provider down")
	}
	wf := Workflow{
		Name:  "fail",
		Steps: []Step{{Name: "step1", Model: "gpt-4o", UserPrompt: "{{input}}"}},
	}
	_, err := Execute(context.Background(), wf, "test", failRoute, "u1")
	if err == nil {
		t.Error("expected error on step failure")
	}
}

func TestSystemPromptInStep(t *testing.T) {
	wf := Workflow{
		Name:  "sys",
		Steps: []Step{{Name: "step1", Model: "gpt-4o", SystemPrompt: "You are helpful", UserPrompt: "{{input}}"}},
	}
	result, err := Execute(context.Background(), wf, "hello", mockRoute, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalOutput == "" {
		t.Error("expected output")
	}
}
