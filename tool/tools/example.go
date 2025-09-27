// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/example"
	"github.com/xenoweaver/adk-go/tool"
	"github.com/xenoweaver/adk-go/types"
)

// ExampleTool represents a tool that adds (few-shot) examples to the [types.LLMRequest].
type ExampleTool[T any] struct {
	*tool.Tool

	examples T
}

var _ types.Tool = (*ExampleTool[[]*example.Example])(nil)

// NewExampleTool creates a new ExampleTool with the given examples.
func NewExampleTool[T any](examples T) *ExampleTool[T] {
	switch any(examples).(type) {
	case []*example.Example, example.Provider:
		// nothing to do
	default:
		panic(fmt.Errorf("unknown examples type: %T", examples))
	}

	t := &ExampleTool[T]{
		Tool:     tool.NewTool("example_tool", "example tool", false),
		examples: examples,
	}

	return t
}

// Name implements [types.Tool].
func (t *ExampleTool[T]) Name() string {
	return t.Tool.Name()
}

// Description implements [types.Tool].
func (t *ExampleTool[T]) Description() string {
	return t.Tool.Description()
}

// IsLongRunning implements [types.Tool].
func (t *ExampleTool[T]) IsLongRunning() bool {
	return t.Tool.IsLongRunning()
}

// GetDeclaration implements [types.Tool].
func (t *ExampleTool[T]) GetDeclaration() *genai.FunctionDeclaration {
	return t.Tool.GetDeclaration()
}

// Run implements [types.Tool].
func (t *ExampleTool[T]) Run(context.Context, map[string]any, *types.ToolContext) (any, error) {
	return nil, errors.New("not implemented")
}

// ProcessLLMRequest implements [types.Tool].
func (t *ExampleTool[T]) ProcessLLMRequest(ctx context.Context, toolCtx *types.ToolContext, request *types.LLMRequest) error {
	parts := toolCtx.UserContent().Parts
	if len(parts) == 0 || parts[0].Text == "" {
		return nil
	}

	instructions, err := example.BuildExampleSI(ctx, t.examples, parts[0].Text, request.Model)
	if err != nil {
		panic(err)
	}
	request.AppendInstructions(instructions)

	return nil
}
