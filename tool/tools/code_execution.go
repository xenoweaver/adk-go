// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/codeexecutor"
	"github.com/xenoweaver/adk-go/tool"
	"github.com/xenoweaver/adk-go/types"
)

// CodeExecutionTool provides code execution capabilities as a tool.
// It wraps a CodeExecutor and provides it as a tool that can be called by LLMs.
type CodeExecutionTool struct {
	*tool.Tool

	executor types.CodeExecutor
	parser   *codeexecutor.CodeBlockParser
}

var _ types.Tool = (*CodeExecutionTool)(nil)

// NewCodeExecutionTool creates a new code execution tool with the given executor.
func NewCodeExecutionTool(executor types.CodeExecutor) *CodeExecutionTool {
	return &CodeExecutionTool{
		Tool:     tool.NewTool("execute_code", "Execute code in a secure environment and return the results", executor.IsLongRunning()),
		executor: executor,
		parser:   codeexecutor.NewDefaultCodeBlockParser(),
	}
}

// Name implements [types.Tool].
func (t *CodeExecutionTool) Name() string {
	return t.Tool.Name()
}

// Description implements [types.Tool].
func (t *CodeExecutionTool) Description() string {
	return t.Tool.Description()
}

// IsLongRunning implements [types.Tool].
func (t *CodeExecutionTool) IsLongRunning() bool {
	return t.Tool.IsLongRunning()
}

// GetDeclaration implements [types.Tool].
func (t *CodeExecutionTool) GetDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"code": {
					Type:        genai.TypeString,
					Description: "The source code to execute",
				},
				"language": {
					Type:        genai.TypeString,
					Description: "The programming language (e.g., 'python', 'go', 'javascript', 'bash')",
					Enum:        []string{"python", "go", "javascript", "bash", "shell"},
				},
				"execution_id": {
					Type:        genai.TypeString,
					Description: "Optional execution session ID for stateful execution",
				},
				"timeout_seconds": {
					Type:        genai.TypeNumber,
					Description: "Optional timeout in seconds (default: 30)",
				},
				"working_directory": {
					Type:        genai.TypeString,
					Description: "Optional working directory for code execution",
				},
				"environment": {
					Type:        genai.TypeObject,
					Description: "Optional environment variables as key-value pairs",
				},
				"input_files": {
					Type:        genai.TypeArray,
					Description: "Optional input files to make available during execution",
					Items: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"name": {
								Type:        genai.TypeString,
								Description: "File name",
							},
							"content": {
								Type:        genai.TypeString,
								Description: "File content (base64 encoded for binary files)",
							},
							"mime_type": {
								Type:        genai.TypeString,
								Description: "MIME type of the file",
							},
						},
						Required: []string{"name", "content"},
					},
				},
			},
			Required: []string{"code"},
		},
	}
}

// Run implements [types.Tool].
func (t *CodeExecutionTool) Run(ctx context.Context, args map[string]any, toolCtx *types.ToolContext) (any, error) {
	// Extract and validate arguments
	input, err := t.parseArguments(args)
	if err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Execute code using the configured executor
	result, err := t.executor.ExecuteCode(ctx, toolCtx.InvocationContext(), input)
	if err != nil {
		return nil, fmt.Errorf("code execution failed: %w", err)
	}

	// Format and return result
	return t.formatResult(result), nil
}

// parseArguments extracts and validates the execution input from tool arguments.
func (t *CodeExecutionTool) parseArguments(args map[string]any) (*types.CodeExecutionInput, error) {
	input := &types.CodeExecutionInput{}

	// Required: code
	code, ok := args["code"].(string)
	if !ok || code == "" {
		return nil, fmt.Errorf("code parameter is required and must be a non-empty string")
	}
	input.Code = code

	// Optional: language
	if lang, ok := args["language"].(string); ok {
		input.Language = lang
	} else {
		// Try to infer language from code content
		input.Language = t.inferLanguage(code)
	}

	// Optional: execution_id
	if execID, ok := args["execution_id"].(string); ok {
		input.ExecutionID = execID
	}

	// Optional: timeout_seconds
	if timeoutSecs, ok := args["timeout_seconds"].(float64); ok && timeoutSecs > 0 {
		input.Timeout = time.Duration(timeoutSecs) * time.Second
	}

	// Optional: working_directory
	if workDir, ok := args["working_directory"].(string); ok {
		input.WorkingDirectory = workDir
	}

	// Optional: environment
	if env, ok := args["environment"].(map[string]any); ok {
		input.Environment = make(map[string]string)
		for key, value := range env {
			if strValue, ok := value.(string); ok {
				input.Environment[key] = strValue
			}
		}
	}

	// Optional: input_files
	if files, ok := args["input_files"].([]any); ok {
		for _, fileData := range files {
			if fileMap, ok := fileData.(map[string]any); ok {
				file, err := t.parseExecutionFile(fileMap)
				if err != nil {
					return nil, fmt.Errorf("invalid input file: %w", err)
				}
				input.InputFiles = append(input.InputFiles, file)
			}
		}
	}

	return input, nil
}

// parseExecutionFile creates an ExecutionFile from a file argument map.
func (t *CodeExecutionTool) parseExecutionFile(fileMap map[string]any) (*types.CodeExecutionFile, error) {
	name, ok := fileMap["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("file name is required")
	}

	contentStr, ok := fileMap["content"].(string)
	if !ok {
		return nil, fmt.Errorf("file content is required")
	}

	mimeType, _ := fileMap["mime_type"].(string)

	// Try to decode as base64 first, if that fails, treat as plain text
	var content []byte
	if decoded, err := base64.StdEncoding.DecodeString(contentStr); err == nil {
		content = decoded
	} else {
		content = []byte(contentStr)
	}

	return types.NewExecutionFile(name, content, mimeType), nil
}

// inferLanguage attempts to infer the programming language from code content.
func (t *CodeExecutionTool) inferLanguage(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))

	// Check for language-specific patterns
	if strings.Contains(code, "print(") || strings.Contains(code, "import ") ||
		strings.HasPrefix(code, "def ") || strings.Contains(code, "if __name__") {
		return "python"
	}

	if strings.Contains(code, "package main") || strings.Contains(code, "func main()") ||
		strings.Contains(code, "fmt.Print") {
		return "go"
	}

	if strings.Contains(code, "console.log") || strings.Contains(code, "function ") ||
		strings.Contains(code, "const ") || strings.Contains(code, "let ") {
		return "javascript"
	}

	if strings.Contains(code, "echo ") || strings.Contains(code, "#!/bin/bash") ||
		strings.HasPrefix(code, "ls ") || strings.HasPrefix(code, "cd ") {
		return "bash"
	}

	// Default to Python for ambiguous cases
	return "python"
}

// formatResult converts ExecutionResult to a tool response format.
func (t *CodeExecutionTool) formatResult(result *types.CodeExecutionResult) map[string]any {
	response := map[string]any{
		"exit_code":      result.ExitCode,
		"execution_time": result.ExecutionTime.Seconds(),
		"execution_id":   result.ExecutionID,
	}

	// Add stdout if present
	if result.Stdout != "" {
		response["stdout"] = result.Stdout
	}

	// Add stderr if present
	if result.Stderr != "" {
		response["stderr"] = result.Stderr
	}

	// Add error if present
	if result.Error != nil {
		response["error"] = result.Error.Error()
	}

	// Add output files if any
	if len(result.OutputFiles) > 0 {
		files := make([]map[string]any, len(result.OutputFiles))
		for i, file := range result.OutputFiles {
			files[i] = map[string]any{
				"name":      file.Name,
				"size":      file.Size,
				"mime_type": file.MIMEType,
				"content":   base64.StdEncoding.EncodeToString(file.Content),
			}
		}
		response["output_files"] = files
	}

	// Add success indicator
	response["success"] = result.ExitCode == 0 && result.Error == nil

	return response
}

// ProcessLLMRequest implements [types.Tool].
//
// This method can be used to automatically add code execution capabilities to LLM requests.
func (t *CodeExecutionTool) ProcessLLMRequest(ctx context.Context, toolCtx *types.ToolContext, llmRequest *types.LLMRequest) error {
	// Check if the model supports built-in code execution
	if t.shouldUseBuiltInExecution(llmRequest.Model) {
		// Add built-in code execution tool
		if llmRequest.Config == nil {
			llmRequest.Config = &genai.GenerateContentConfig{}
		}

		// Add code execution tool to the request
		llmRequest.Config.Tools = append(llmRequest.Config.Tools, &genai.Tool{
			CodeExecution: &genai.ToolCodeExecution{},
		})
		return nil
	}

	// Otherwise, use the standard tool processing
	return t.Tool.ProcessLLMRequest(ctx, toolCtx, llmRequest)
}

// shouldUseBuiltInExecution checks if the model supports built-in code execution.
func (t *CodeExecutionTool) shouldUseBuiltInExecution(modelName string) bool {
	modelName = strings.ToLower(modelName)
	return strings.Contains(modelName, "gemini-2") ||
		strings.Contains(modelName, "gemini-exp") ||
		strings.Contains(modelName, "code-execution")
}

// ExecutorFromTool extracts the underlying CodeExecutor from a CodeExecutionTool.
// This can be useful for advanced use cases that need direct access to the executor.
func ExecutorFromTool(tool types.Tool) (types.CodeExecutor, bool) {
	if cet, ok := tool.(*CodeExecutionTool); ok {
		return cet.executor, true
	}
	return nil, false
}
