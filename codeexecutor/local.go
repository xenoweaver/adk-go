// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package codeexecutor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xenoweaver/adk-go/types"
)

// LocalExecutor executes code in the local environment.
//
// WARNING: This executor is inherently unsafe as it runs arbitrary code
// with the same privileges as the calling process. Use only in trusted environments.
type LocalExecutor struct {
	config *types.ExecutionConfig

	// allowUnsafe must be explicitly set to true to enable this executor
	allowUnsafe bool

	// workDir is the working directory for code execution
	workDir string

	// tempDir is used for temporary files when workDir is not specified
	tempDir string

	// stateful indicates if this executor maintains state between calls
	stateful bool
}

var _ types.CodeExecutor = (*LocalExecutor)(nil)

// LocalExecutorOption is a functional option for configuring LocalExecutor.
type LocalExecutorOption func(*LocalExecutor)

// WithAllowUnsafe explicitly enables unsafe local execution.
// This is required to use the LocalExecutor due to security implications.
func WithAllowUnsafe(allow bool) LocalExecutorOption {
	return func(e *LocalExecutor) {
		e.allowUnsafe = allow
	}
}

// WithWorkDir sets a specific working directory for code execution.
func WithWorkDir(dir string) LocalExecutorOption {
	return func(e *LocalExecutor) {
		e.workDir = dir
	}
}

// WithStateful enables stateful execution mode where variables persist between calls.
func WithStateful(stateful bool) LocalExecutorOption {
	return func(e *LocalExecutor) {
		e.stateful = stateful
	}
}

// NewLocalExecutor creates a new local code executor.
//
// NOTE(adk-go): This executor requires explicit opt-in to unsafe execution.
func NewLocalExecutor(opts ...any) (*LocalExecutor, error) {
	// Separate execution options from local executor options
	var execOpts []types.ExecutionOption
	var localOpts []LocalExecutorOption

	for _, opt := range opts {
		switch o := opt.(type) {
		case types.ExecutionOption:
			execOpts = append(execOpts, o)
		case LocalExecutorOption:
			localOpts = append(localOpts, o)
		default:
			return nil, fmt.Errorf("unsupported option type: %T", opt)
		}
	}

	config := types.DefaultConfig()
	for _, opt := range execOpts {
		opt(config)
	}

	executor := &LocalExecutor{
		config:      config,
		allowUnsafe: false, // Must be explicitly enabled
		stateful:    false,
	}

	for _, opt := range localOpts {
		opt(executor)
	}

	if !executor.allowUnsafe {
		return nil, fmt.Errorf("local executor requires explicit opt-in to unsafe execution via WithAllowUnsafe(true)")
	}

	// Create temporary directory if no working directory specified
	if executor.workDir == "" {
		tempDir, err := os.MkdirTemp("", "adk-local-executor-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary directory: %w", err)
		}
		executor.tempDir = tempDir
		executor.workDir = tempDir
	}

	return executor, nil
}

// OptimizeDataFile implements [types.CodeExecutor].
func (e *LocalExecutor) OptimizeDataFile() bool {
	return e.config.OptimizeDataFiles
}

// IsLongRunning implements [types.CodeExecutor].
func (e *LocalExecutor) IsLongRunning() bool {
	return e.config.LongRunning
}

// IsStateful implements [types.CodeExecutor].
func (e *LocalExecutor) IsStateful() bool {
	return e.config.Stateful
}

// ErrorRetryAttempts implements [types.CodeExecutor].
func (e *LocalExecutor) ErrorRetryAttempts() int {
	return e.config.MaxRetries
}

// CodeBlockDelimiters implements [types.CodeExecutor].
func (e *LocalExecutor) CodeBlockDelimiters() []types.DelimiterPair {
	return e.config.CodeBlockDelimiters
}

// ExecutionResultDelimiters implements [types.CodeExecutor].
func (e *LocalExecutor) ExecutionResultDelimiters() types.DelimiterPair {
	return e.config.ExecutionResultDelimiters
}

// ExecuteCode implements [types.CodeExecutor].
func (e *LocalExecutor) ExecuteCode(ctx context.Context, ictx *types.InvocationContext, input *types.CodeExecutionInput) (*types.CodeExecutionResult, error) {
	if !e.allowUnsafe {
		return nil, fmt.Errorf("unsafe execution not allowed - must be explicitly enabled")
	}

	startTime := time.Now()

	// Get or create execution context
	execCtx := GetContextFromInvocation(ictx, input.ExecutionID)

	// Prepare execution environment
	workDir := e.workDir
	if input.WorkingDirectory != "" {
		workDir = input.WorkingDirectory
	}

	// Execute with retry logic
	var result *types.CodeExecutionResult
	var lastErr error

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(e.config.RetryDelay):
				// Continue with retry
			}
		}

		result, lastErr = e.executeCode(ctx, input, workDir, execCtx)
		if lastErr == nil {
			break
		}

		// Increment error count
		if ictx != nil {
			execCtx.IncrementErrorCount(ictx.InvocationID)
		}
	}

	if lastErr != nil {
		return &types.CodeExecutionResult{
			Error:         lastErr,
			ExecutionTime: time.Since(startTime),
			ExecutionID:   input.ExecutionID,
		}, lastErr
	}

	result.ExecutionTime = time.Since(startTime)
	result.ExecutionID = input.ExecutionID

	// Store result in context
	execCtx.AddExecutionResult(result)

	return result, nil
}

// executeCode performs the actual code execution.
func (e *LocalExecutor) executeCode(ctx context.Context, input *types.CodeExecutionInput, workDir string, execCtx *CodeExecutorContext) (*types.CodeExecutionResult, error) {
	// Create working directory if it doesn't exist
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	// Write input files to working directory
	for _, file := range input.InputFiles {
		filePath := filepath.Join(workDir, file.Name)
		if err := file.WriteToFile(filePath); err != nil {
			return nil, fmt.Errorf("failed to write input file %s: %w", file.Name, err)
		}
		execCtx.AddProcessedFileNames(file.Name)
	}

	// Determine execution strategy based on language
	switch strings.ToLower(input.Language) {
	case "python", "py":
		return e.executePython(ctx, input, workDir)
	case "go":
		return e.executeGo(ctx, input, workDir)
	case "javascript", "js", "node":
		return e.executeJavaScript(ctx, input, workDir)
	case "bash", "shell", "sh":
		return e.executeBash(ctx, input, workDir)
	default:
		// Try to infer from code content or default to Python
		return e.executePython(ctx, input, workDir)
	}
}

// executePython executes Python code.
func (e *LocalExecutor) executePython(ctx context.Context, input *types.CodeExecutionInput, workDir string) (*types.CodeExecutionResult, error) {
	// Create a temporary Python file
	tmpFile := filepath.Join(workDir, "code.py")
	if err := os.WriteFile(tmpFile, []byte(input.Code), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write Python code: %w", err)
	}
	defer os.Remove(tmpFile)

	// Execute Python
	return e.executeCommand(ctx, "python3", []string{tmpFile}, workDir, input.Environment)
}

// executeGo executes Go code.
func (e *LocalExecutor) executeGo(ctx context.Context, input *types.CodeExecutionInput, workDir string) (*types.CodeExecutionResult, error) {
	// Create a temporary Go file
	tmpFile := filepath.Join(workDir, "main.go")

	// Wrap code in main function if needed
	code := input.Code
	if !strings.Contains(code, "package main") {
		code = fmt.Sprintf("package main\n\nimport \"fmt\"\n\nfunc main() {\n%s\n}", code)
	}

	if err := os.WriteFile(tmpFile, []byte(code), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write Go code: %w", err)
	}
	defer os.Remove(tmpFile)

	// Execute Go
	return e.executeCommand(ctx, "go", []string{"run", tmpFile}, workDir, input.Environment)
}

// executeJavaScript executes JavaScript/Node.js code.
func (e *LocalExecutor) executeJavaScript(ctx context.Context, input *types.CodeExecutionInput, workDir string) (*types.CodeExecutionResult, error) {
	// Create a temporary JavaScript file
	tmpFile := filepath.Join(workDir, "code.js")
	if err := os.WriteFile(tmpFile, []byte(input.Code), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write JavaScript code: %w", err)
	}
	defer os.Remove(tmpFile)

	// Execute Node.js
	return e.executeCommand(ctx, "node", []string{tmpFile}, workDir, input.Environment)
}

// executeBash executes bash/shell code.
func (e *LocalExecutor) executeBash(ctx context.Context, input *types.CodeExecutionInput, workDir string) (*types.CodeExecutionResult, error) {
	// Execute bash directly with code as stdin
	return e.executeCommandWithStdin(ctx, "bash", []string{}, input.Code, workDir, input.Environment)
}

// executeCommand executes a command with the given arguments.
func (e *LocalExecutor) executeCommand(ctx context.Context, command string, args []string, workDir string, env map[string]string) (*types.CodeExecutionResult, error) {
	// Set timeout
	if e.config.DefaultTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.config.DefaultTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &types.CodeExecutionResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	// Look for output files in working directory
	outputFiles, err := e.findOutputFiles(workDir)
	if err == nil {
		result.OutputFiles = outputFiles
	}

	return result, nil
}

// executeCommandWithStdin executes a command with stdin input.
func (e *LocalExecutor) executeCommandWithStdin(ctx context.Context, command string, args []string, stdin, workDir string, env map[string]string) (*types.CodeExecutionResult, error) {
	// Set timeout
	if e.config.DefaultTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.config.DefaultTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(stdin)

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &types.CodeExecutionResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	// Look for output files in working directory
	outputFiles, err := e.findOutputFiles(workDir)
	if err == nil {
		result.OutputFiles = outputFiles
	}

	return result, nil
}

// findOutputFiles looks for files created during execution.
func (e *LocalExecutor) findOutputFiles(workDir string) ([]*types.CodeExecutionFile, error) {
	var outputFiles []*types.CodeExecutionFile

	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip temporary files and common artifacts
		if strings.HasPrefix(name, ".") ||
			strings.HasSuffix(name, ".pyc") ||
			strings.HasSuffix(name, ".tmp") ||
			name == "code.py" || name == "main.go" || name == "code.js" {
			continue
		}

		filePath := filepath.Join(workDir, name)
		file, err := types.NewExecutionFileFromPath(filePath)
		if err != nil {
			continue // Skip files we can't read
		}

		outputFiles = append(outputFiles, file)
	}

	return outputFiles, nil
}

// Close implements [types.CodeExecutor].
func (e *LocalExecutor) Close() error {
	var err error

	// Clean up temporary directory if we created one
	if e.tempDir != "" {
		if rmErr := os.RemoveAll(e.tempDir); rmErr != nil {
			err = fmt.Errorf("failed to clean up temporary directory: %w", rmErr)
		}
	}

	return err
}
