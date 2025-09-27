// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package codeexecutor

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/types"
)

// BuiltInExecutor uses the model's built-in code execution capabilities.
// This is supported by Gemini 2.0+ models with native code execution tools.
type BuiltInExecutor struct {
	config *types.ExecutionConfig
}

var _ types.CodeExecutor = (*BuiltInExecutor)(nil)

// NewBuiltInExecutor creates a new built-in executor using the model's native capabilities.
func NewBuiltInExecutor(opts ...types.ExecutionOption) *BuiltInExecutor {
	config := types.DefaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	return &BuiltInExecutor{
		config: config,
	}
}

// OptimizeDataFile implements [types.CodeExecutor].
func (e *BuiltInExecutor) OptimizeDataFile() bool {
	return e.config.OptimizeDataFiles
}

// IsLongRunning implements [types.CodeExecutor].
func (e *BuiltInExecutor) IsLongRunning() bool {
	return e.config.LongRunning
}

// IsStateful implements [types.CodeExecutor].
func (e *BuiltInExecutor) IsStateful() bool {
	return e.config.Stateful
}

// ErrorRetryAttempts implements [types.CodeExecutor].
func (e *BuiltInExecutor) ErrorRetryAttempts() int {
	return e.config.MaxRetries
}

// CodeBlockDelimiters implements [types.CodeExecutor].
func (e *BuiltInExecutor) CodeBlockDelimiters() []types.DelimiterPair {
	return e.config.CodeBlockDelimiters
}

// ExecutionResultDelimiters implements [types.CodeExecutor].
func (e *BuiltInExecutor) ExecutionResultDelimiters() types.DelimiterPair {
	return e.config.ExecutionResultDelimiters
}

// ExecuteCode implements [types.CodeExecutor].
func (e *BuiltInExecutor) ExecuteCode(ctx context.Context, ictx *types.InvocationContext, input *types.CodeExecutionInput) (*types.CodeExecutionResult, error) {
	return nil, nil
}

// ProcessLLMRequest processes the LLM request to ensure it has the necessary configuration for code execution.
func (a *BuiltInExecutor) ProcessLLMRequest(ctx context.Context, request *types.LLMRequest) {
	if request.Model == "" || !strings.HasPrefix(request.Model, "gemini-2") {
		panic(fmt.Errorf("Gemini code execution tool is not supported for model %s", request.Model))
	}

	if request.Config == nil {
		request.Config = new(genai.GenerateContentConfig)
	}
	request.Config.Tools = append(request.Config.Tools, &genai.Tool{
		CodeExecution: new(genai.ToolCodeExecution),
	})
}

// Close implements [types.CodeExecutor].
func (e *BuiltInExecutor) Close() error {
	// nothing to do
	return nil
}
