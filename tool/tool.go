// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tool

import (
	"context"
	"errors"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/types"
)

// Tool represents a base class for all tools.
type Tool struct {
	// The name of the tool.
	name string

	// The description of the tool.
	description string

	// Whether the tool is a long running operation, which typically returns a
	// resource id first and finishes the operation later.
	isLongRunning bool
}

var _ types.Tool = (*Tool)(nil)

// NewTool returns the tool with the given name, description and isLongRunning.
func NewTool(name, description string, isLongRunning bool) *Tool {
	return &Tool{
		name:          name,
		description:   description,
		isLongRunning: isLongRunning,
	}
}

// Name implements [types.Tool].
func (t *Tool) Name() string {
	return t.name
}

// Description implements [types.Tool].
func (t *Tool) Description() string {
	return t.description
}

// IsLongRunning implements [types.Tool].
func (t *Tool) IsLongRunning() bool {
	return t.isLongRunning
}

// SetLongRunning sets thether the isLongRunning.
func (t *Tool) SetLongRunning(longRunning bool) {
	t.isLongRunning = longRunning
}

// GetDeclaration implements [types.Tool].
func (t *Tool) GetDeclaration() *genai.FunctionDeclaration {
	return nil
}

// Run implements [types.Tool].
func (t *Tool) Run(ctx context.Context, args map[string]any, toolCtx *types.ToolContext) (any, error) {
	return nil, errors.New("not implemented")
}

// ProcessLLMRequest implements [types.Tool].
func (t *Tool) ProcessLLMRequest(ctx context.Context, toolCtx *types.ToolContext, request *types.LLMRequest) error {
	funcDeclaration := t.GetDeclaration()
	if funcDeclaration == nil {
		return nil
	}

	request.ToolMap[t.Name()] = t
	toolWithFuncDeclarations := t.findToolWithFunctionDeclarations(request)
	if toolWithFuncDeclarations != nil {
		if len(toolWithFuncDeclarations.FunctionDeclarations) == 0 {
			toolWithFuncDeclarations.FunctionDeclarations = append(toolWithFuncDeclarations.FunctionDeclarations, funcDeclaration)
		}
		return nil
	}

	if request.Config == nil {
		request.Config = &genai.GenerateContentConfig{}
	}
	request.Config.Tools = append(request.Config.Tools, &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			funcDeclaration,
		},
	})

	return nil
}

func (t *Tool) findToolWithFunctionDeclarations(request *types.LLMRequest) *genai.Tool {
	if request.Config == nil || len(request.Config.Tools) == 0 {
		return nil
	}

	for _, tool := range request.Config.Tools {
		if len(tool.FunctionDeclarations) > 0 {
			return tool
		}
	}

	return nil
}
