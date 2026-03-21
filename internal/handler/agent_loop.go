package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sooneocean/uniapi/internal/plugin"
	"github.com/sooneocean/uniapi/internal/provider"
)

const maxToolLoops = 5

// runAgentLoop executes a multi-turn tool-calling loop.
// If the model returns tool_calls, it executes them via plugins and feeds results back.
// Returns the final response (with text content, not tool_calls).
func runAgentLoop(ctx context.Context, req *provider.ChatRequest, routeFn func(context.Context, *provider.ChatRequest, ...string) (*provider.ChatResponse, error), pluginMgr *plugin.Manager, userID string) (*provider.ChatResponse, error) {
	messages := make([]provider.Message, len(req.Messages))
	copy(messages, req.Messages)

	for i := 0; i < maxToolLoops; i++ {
		currentReq := &provider.ChatRequest{
			Model: req.Model, Messages: messages, Tools: req.Tools,
			MaxTokens: req.MaxTokens, Temperature: req.Temperature,
		}

		resp, err := routeFn(ctx, currentReq, userID)
		if err != nil {
			return nil, err
		}

		// No tool calls — return final response
		if len(resp.ToolCalls) == 0 || resp.StopReason != "tool_use" {
			return resp, nil
		}

		// Append assistant message with tool calls
		assistantMsg := provider.Message{Role: "assistant", Content: resp.Content}
		for _, tc := range resp.ToolCalls {
			tc := tc // capture loop variable
			assistantMsg.Content = append(assistantMsg.Content, provider.ContentBlock{
				Type:    "tool_use",
				ToolUse: &tc,
			})
		}
		messages = append(messages, assistantMsg)

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			result, err := executeToolCall(pluginMgr, userID, tc)
			if err != nil {
				result = fmt.Sprintf("Error: %s", err.Error())
			}
			messages = append(messages, provider.Message{
				Role: "tool",
				Content: []provider.ContentBlock{{
					Type: "tool_result",
					ToolResult: &struct {
						ToolUseID string `json:"tool_use_id"`
						Content   string `json:"content"`
					}{ToolUseID: tc.ID, Content: result},
				}},
			})
		}
		slog.Info("agent loop", "iteration", i+1, "tool_calls", len(resp.ToolCalls))
	}
	return nil, fmt.Errorf("agent loop exceeded max iterations (%d)", maxToolLoops)
}

func executeToolCall(pluginMgr *plugin.Manager, userID string, tc provider.ToolCall) (string, error) {
	if pluginMgr == nil {
		return "", fmt.Errorf("no plugin manager")
	}
	plugins, _ := pluginMgr.List(userID)
	for _, p := range plugins {
		if p.Name == tc.Function.Name {
			return pluginMgr.Execute(p, tc.Function.Arguments)
		}
	}
	return "", fmt.Errorf("plugin not found: %s", tc.Function.Name)
}
