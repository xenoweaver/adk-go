// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-json-experiment/json"

	"github.com/xenoweaver/adk-go/internal/pool"
)

// CodeExecutor defines the interface for executing code in various environments.
// It supports both stateful and stateless execution modes with configurable
// retry logic and file handling.
type CodeExecutor interface {
	// OptimizeDataFile reports whether the extract and process data files from the model request
	// and attach them to the code executor.
	// Supported data file MimeTypes are [text/csv].
	//
	// Default to false.
	OptimizeDataFile() bool

	// IsLongRunning reports whether the code executor supports long-running operations.
	IsLongRunning() bool

	// IsStateful reports whether the code executor is stateful.
	//
	// Stateful executors can reuse variables and imports across multiple Execute calls.
	IsStateful() bool

	// ErrorRetryAttempts returns the number of attempts to retry on consecutive code execution errors.
	//
	// Default to 2.
	ErrorRetryAttempts() int

	// CodeBlockDelimiters returns the list of the enclosing delimiters to identify the code blocks.
	//
	// For example, the delimiter ('```python\n', '\n```') can be
	// used to identify code blocks with the following format:
	//
	//  ```python
	//  print("hello")
	//  ```
	CodeBlockDelimiters() []DelimiterPair

	// ExecutionResultDelimiters returns the delimiters to format the code execution result.
	ExecutionResultDelimiters() DelimiterPair

	// Execute runs the provided code and returns the execution result.
	// The context can be used for cancellation and timeout control.
	ExecuteCode(ctx context.Context, ictx *InvocationContext, input *CodeExecutionInput) (*CodeExecutionResult, error)

	// Close cleans up any resources used by the executor.
	Close() error
}

// ExecutionConfig holds configuration options for code executors.
type ExecutionConfig struct {
	// OptimizeDataFiles enables optimization for large data files (e.g., CSV processing).
	OptimizeDataFiles bool

	// LongRunning indicates whether the code executor supports long-running operations.
	LongRunning bool

	// Stateful whether the code executor is stateful.
	Stateful bool

	// MaxRetries specifies the maximum number of retry attempts for failed executions.
	MaxRetries int

	// RetryDelay specifies the delay between retry attempts.
	RetryDelay time.Duration

	// DefaultTimeout specifies the default execution timeout.
	DefaultTimeout time.Duration

	// CodeBlockDelimiters defines the patterns used to extract code blocks from text.
	CodeBlockDelimiters []DelimiterPair

	// ExecutionResultDelimiters defines the patterns used to format execution results.
	ExecutionResultDelimiters DelimiterPair
}

// DelimiterPair represents a pair of start and end delimiters for text parsing.
type DelimiterPair struct {
	Start string
	End   string
}

// DefaultConfig returns a default ExecutionConfig with sensible defaults.
func DefaultConfig() *ExecutionConfig {
	return &ExecutionConfig{
		OptimizeDataFiles: false,
		LongRunning:       false,
		Stateful:          false,
		MaxRetries:        2,
		RetryDelay:        1 * time.Second,
		DefaultTimeout:    30 * time.Second,
		CodeBlockDelimiters: []DelimiterPair{
			{Start: "```tool_code\n", End: "\n```"},
			{Start: "```python\n", End: "\n```"},
			{Start: "```go\n", End: "\n```"},
			{Start: "```javascript\n", End: "\n```"},
			{Start: "```bash\n", End: "\n```"},
		},
		ExecutionResultDelimiters: DelimiterPair{
			Start: "```tool_output\n",
			End:   "\n```",
		},
	}
}

// ExecutionOption is a functional option for configuring code executors.
type ExecutionOption func(*ExecutionConfig)

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(retries int) ExecutionOption {
	return func(c *ExecutionConfig) {
		c.MaxRetries = retries
	}
}

// WithRetryDelay sets the delay between retry attempts.
func WithRetryDelay(delay time.Duration) ExecutionOption {
	return func(c *ExecutionConfig) {
		c.RetryDelay = delay
	}
}

// WithDefaultTimeout sets the default execution timeout.
func WithDefaultTimeout(timeout time.Duration) ExecutionOption {
	return func(c *ExecutionConfig) {
		c.DefaultTimeout = timeout
	}
}

// WithOptimizeDataFiles enables or disables data file optimization.
func WithOptimizeDataFiles(optimize bool) ExecutionOption {
	return func(c *ExecutionConfig) {
		c.OptimizeDataFiles = optimize
	}
}

// WithCodeBlockDelimiters sets custom code block delimiters.
func WithCodeBlockDelimiters(delimiters ...DelimiterPair) ExecutionOption {
	return func(c *ExecutionConfig) {
		c.CodeBlockDelimiters = delimiters
	}
}

// WithExecutionResultDelimiters sets custom execution result delimiters.
func WithExecutionResultDelimiters(delimiters DelimiterPair) ExecutionOption {
	return func(c *ExecutionConfig) {
		c.ExecutionResultDelimiters = delimiters
	}
}

// CodeExecutionInput represents a structure that contains the input of code execution.
type CodeExecutionInput struct {
	// Code is the code to execute.
	Code string `json:"code"`

	// Language specifies the programming language (e.g., "python", "go", "javascript").
	// If empty, the executor may attempt to auto-detect or use a default.
	Language string `json:"language,omitempty"`

	// InputFiles are files that should be available to the code during execution.
	InputFiles []*CodeExecutionFile `json:"input_files,omitempty"`

	// ExecutionID is an optional identifier for stateful execution sessions.
	// When provided, stateful executors will maintain session state across calls.
	ExecutionID string `json:"execution_id,omitempty"`

	// WorkingDirectory specifies the directory where code should be executed.
	// If empty, a temporary directory will be used.
	WorkingDirectory string `json:"working_directory,omitempty"`

	// Environment contains environment variables to set during execution.
	Environment map[string]string `json:"environment,omitempty"`

	// Timeout specifies the maximum execution time.
	// If zero, the executor's default timeout will be used.
	Timeout time.Duration `json:"timeout,omitempty"`
}

// CodeExecutionResult represents the result of code execution.
type CodeExecutionResult struct {
	// Code is the code to execute.
	Code string `json:"code"`

	// Stdout contains the standard output from the executed code.
	Stdout string `json:"stdout"`

	// Stderr contains the standard error output from the executed code.
	Stderr string `json:"stderr"`

	// OutputFiles contains files generated during code execution.
	OutputFiles []*CodeExecutionFile `json:"output_files,omitempty"`

	// ExitCode is the exit code returned by the executed code.
	// A value of 0 typically indicates success.
	ExitCode int `json:"exit_code"`

	Timestamp time.Time `json:"timestamp"`

	// ExecutionTime is the duration the code took to execute.
	ExecutionTime time.Duration `json:"execution_time"`

	// ExecutionID is the session identifier used for this execution.
	ExecutionID string `json:"execution_id,omitempty"`

	// Error contains any execution error that occurred.
	// This is separate from stderr and represents infrastructure errors.
	Error error `json:"error,omitempty"`
}

// CodeExecutionFile represents a file with content for code execution.
// Content is stored as raw bytes internally but marshaled as base64 for JSON compatibility.
type CodeExecutionFile struct {
	// Name is the filename, including any relative path.
	Name string `json:"name"`

	// Content is the raw file content as bytes.
	Content []byte `json:"-"`

	// MIMEType specifies the MIME type of the file content.
	// If empty, it will be inferred from the file extension or content.
	MIMEType string `json:"mime_type,omitempty"`

	// Size is the size of the content in bytes.
	Size int64 `json:"size"`
}

// NewExecutionFile creates a new ExecutionFile with the given name and content.
func NewExecutionFile(name string, content []byte, mimeType string) *CodeExecutionFile {
	if mimeType == "" {
		mimeType = inferMIMEType(name, content)
	}
	return &CodeExecutionFile{
		Name:     name,
		Content:  content,
		MIMEType: mimeType,
		Size:     int64(len(content)),
	}
}

// NewExecutionFileFromPath creates a new ExecutionFile by reading from a file path.
func NewExecutionFileFromPath(path string) (*CodeExecutionFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	name := filepath.Base(path)
	return NewExecutionFile(name, content, ""), nil
}

// WriteToFile writes the ExecutionFile content to the specified path.
func (f *CodeExecutionFile) WriteToFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}

	if err := os.WriteFile(path, f.Content, 0o644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}

// String returns a string representation of the file.
func (f *CodeExecutionFile) String() string {
	return fmt.Sprintf("ExecutionFile{Name: %s, Size: %d, MIMEType: %s}", f.Name, f.Size, f.MIMEType)
}

// IsText returns true if the file appears to contain text content.
func (f *CodeExecutionFile) IsText() bool {
	return isTextMIMEType(f.MIMEType)
}

// IsBinary returns true if the file appears to contain binary content.
func (f *CodeExecutionFile) IsBinary() bool {
	return !f.IsText()
}

// MarshalJSON implements json.Marshaler to encode content as base64.
func (f *CodeExecutionFile) MarshalJSON() ([]byte, error) {
	buf := pool.Buffer.Get()

	type Alias CodeExecutionFile
	err := json.MarshalWrite(buf, &struct {
		Content string `json:"content"`
		*Alias
	}{
		Content: base64.StdEncoding.EncodeToString(f.Content),
		Alias:   (*Alias)(f),
	}, json.DefaultOptionsV2())

	out := buf.Bytes()
	pool.Buffer.Put(buf)

	return out, err
}

// UnmarshalJSON implements json.Unmarshaler to decode content from base64.
func (f *CodeExecutionFile) UnmarshalJSON(data []byte) error {
	type Alias CodeExecutionFile
	aux := &struct {
		Content string `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(f),
	}

	if err := json.Unmarshal(data, &aux, json.DefaultOptionsV2()); err != nil {
		return err
	}

	content, err := base64.StdEncoding.DecodeString(aux.Content)
	if err != nil {
		return fmt.Errorf("failed to decode base64 content: %w", err)
	}

	f.Content = content
	f.Size = int64(len(content))

	return nil
}

// inferMIMEType attempts to infer the MIME type from the filename and content.
func inferMIMEType(filename string, content []byte) string {
	// Simple MIME type inference based on file extension
	ext := filepath.Ext(filename)
	switch ext {
	case ".txt":
		return "text/plain"
	case ".py":
		return "text/x-python"
	case ".go":
		return "text/x-go"
	case ".js":
		return "text/javascript"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".html":
		return "text/html"
	case ".xml":
		return "application/xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	default:
		// Fallback to text/plain for most files
		if isLikelyText(content) {
			return "text/plain"
		}
		return "application/octet-stream"
	}
}

// isTextMIMEType returns true if the MIME type indicates text content.
func isTextMIMEType(mimeType string) bool {
	switch {
	case mimeType == "":
		return true // Default to text
	case mimeType[:5] == "text/":
		return true
	case mimeType == "application/json":
		return true
	case mimeType == "application/xml":
		return true
	case mimeType == "application/javascript":
		return true
	default:
		return false
	}
}

// isLikelyText performs a simple heuristic to determine if content is likely text.
func isLikelyText(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	// Check for null bytes (common in binary files)
	for i := 0; i < len(content) && i < 512; i++ {
		if content[i] == 0 {
			return false
		}
	}

	return true
}
