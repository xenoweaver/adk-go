// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/tool"
	"github.com/xenoweaver/adk-go/types"
)

// Agent is a [tool.Tool] that wraps an agent.
//
// This tool allows an agent to be called as a tool within a larger application.
// The agent's input schema is used to define the tool's input parameters, and
// the agent's output is returned as the tool's result.
type Agent struct {
	*tool.Tool

	skipSummarization bool
}

var _ types.Tool = (*Agent)(nil)

// NewAgent creates a new [Agent] tool with the given options.
func NewAgent(name, description string) *Agent {
	tool := &Agent{
		Tool: tool.NewTool(name, description, false),
	}

	return tool
}

// Name implements [types.Tool].
func (t *Agent) Name() string {
	return t.Tool.Name()
}

// Description implements [types.Tool].
func (t *Agent) Description() string {
	return t.Tool.Description()
}

// IsLongRunning implements [types.Tool].
func (t *Agent) IsLongRunning() bool {
	return t.Tool.IsLongRunning()
}

// GetDeclaration implements [types.Tool].
//
// TODO(zchee): implements correctly.
func (t *Agent) GetDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
	}
}

// Run implements [types.Tool].
//
// TODO(zchee): implements.
func (t *Agent) Run(ctx context.Context, args map[string]any, toolCtx *types.ToolContext) (any, error) {
	return nil, nil
}

// ProcessLLMRequest implements [types.Tool].
func (t *Agent) ProcessLLMRequest(ctx context.Context, toolCtx *types.ToolContext, request *types.LLMRequest) error {
	return t.Tool.ProcessLLMRequest(ctx, toolCtx, request)
}
