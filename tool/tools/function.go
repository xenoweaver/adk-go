// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"maps"
	"reflect"
	"runtime"
	"strings"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/tool"
	"github.com/xenoweaver/adk-go/types"
)

// Function represents a user-defined function that can be called with a context.
type Function func(ctx context.Context, args map[string]any) (any, error)

// FunctionTool represents a tool that wraps a user-defined function.
//
// TODO(zchee): implements correctly.
type FunctionTool struct {
	*tool.Tool

	fn          Function
	declaration *genai.FunctionDeclaration
}

var _ types.Tool = (*FunctionTool)(nil)

// NewFunctionTool returns the new FunctionTool with the given name, description and function.
func NewFunctionTool(fn Function) *FunctionTool {
	funcName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	if idx := strings.LastIndex(funcName, "."); idx > -1 {
		funcName = funcName[idx+1:]
	}

	return &FunctionTool{
		Tool: tool.NewTool(funcName, "", false),
		fn:   fn,
	}
}

// Name implements [types.Tool].
func (t *FunctionTool) Name() string {
	return t.Tool.Name()
}

// Description implements [types.Tool].
func (t *FunctionTool) Description() string {
	return t.Tool.Description()
}

// IsLongRunning implements [types.Tool].
func (t *FunctionTool) IsLongRunning() bool {
	return t.Tool.IsLongRunning()
}

// GetDeclaration implements [types.Tool].
func (t *FunctionTool) GetDeclaration() *genai.FunctionDeclaration {
	funcDecl, err := buildFunctionDeclaration(t.fn)
	if err != nil {
		panic(err)
	}
	return funcDecl
}

// Run implements [types.Tool].
func (t *FunctionTool) Run(ctx context.Context, args map[string]any, toolCtx *types.ToolContext) (any, error) {
	argsToCall := maps.Clone(args)

	return t.fn(ctx, argsToCall)
}

// ProcessLLMRequest implements [types.Tool].
func (t *FunctionTool) ProcessLLMRequest(ctx context.Context, toolCtx *types.ToolContext, request *types.LLMRequest) error {
	return t.Tool.ProcessLLMRequest(ctx, toolCtx, request)
}
