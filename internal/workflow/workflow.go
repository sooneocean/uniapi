package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sooneocean/uniapi/internal/provider"
)

type Step struct {
	Name         string `json:"name"`
	Model        string `json:"model"`
	SystemPrompt string `json:"system_prompt"`
	UserPrompt   string `json:"user_prompt"` // can contain {{input}} and {{step_N}} placeholders
	MaxTokens    int    `json:"max_tokens"`
}

type Workflow struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       []Step `json:"steps"`
	Shared      bool   `json:"shared"`
	RunCount    int    `json:"run_count"`
}

type StepResult struct {
	StepName  string `json:"step_name"`
	Model     string `json:"model"`
	Output    string `json:"output"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	LatencyMs int64  `json:"latency_ms"`
}

type RunResult struct {
	WorkflowName string       `json:"workflow_name"`
	Steps        []StepResult `json:"steps"`
	FinalOutput  string       `json:"final_output"`
	TotalCost    float64      `json:"total_cost"`
}

type RouteFn func(ctx context.Context, req *provider.ChatRequest, userID string) (*provider.ChatResponse, error)

// Execute runs a workflow with the given input
func Execute(ctx context.Context, wf Workflow, input string, routeFn RouteFn, userID string) (*RunResult, error) {
	results := make([]StepResult, 0, len(wf.Steps))
	variables := map[string]string{"input": input}

	for i, step := range wf.Steps {
		// Replace placeholders in user prompt
		prompt := step.UserPrompt
		for k, v := range variables {
			prompt = strings.ReplaceAll(prompt, "{{"+k+"}}", v)
		}

		messages := []provider.Message{}
		if step.SystemPrompt != "" {
			messages = append(messages, provider.Message{
				Role: "system", Content: []provider.ContentBlock{{Type: "text", Text: step.SystemPrompt}},
			})
		}
		messages = append(messages, provider.Message{
			Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: prompt}},
		})

		maxTokens := step.MaxTokens
		if maxTokens == 0 {
			maxTokens = 2048
		}

		req := &provider.ChatRequest{Model: step.Model, Messages: messages, MaxTokens: maxTokens}
		start := time.Now()
		resp, err := routeFn(ctx, req, userID)
		latency := time.Since(start)

		if err != nil {
			return nil, fmt.Errorf("step %d (%s) failed: %w", i+1, step.Name, err)
		}

		output := ""
		if len(resp.Content) > 0 {
			output = resp.Content[0].Text
		}

		results = append(results, StepResult{
			StepName: step.Name, Model: step.Model, Output: output,
			TokensIn: resp.TokensIn, TokensOut: resp.TokensOut,
			LatencyMs: latency.Milliseconds(),
		})

		// Store for next step reference
		variables[fmt.Sprintf("step_%d", i+1)] = output
	}

	finalOutput := ""
	if len(results) > 0 {
		finalOutput = results[len(results)-1].Output
	}

	return &RunResult{WorkflowName: wf.Name, Steps: results, FinalOutput: finalOutput}, nil
}
