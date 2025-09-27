// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package codeexecutor

import (
	"maps"
	"sync"
	"time"

	"github.com/xenoweaver/adk-go/types"
)

const (
	ContextKey            = "_code_execution_context"
	SessionIDKey          = "execution_session_id"
	ProcessedFileNamesKey = "processed_input_files"
	InputFileKey          = "_code_executor_input_files"
	ErrorCountKey         = "_code_executor_error_counts"

	CodeExecutionResultsKey = "_code_execution_results"
)

// getCodeExecutorContext gets the code executor context from the session state.
func getCodeExecutorContext(sessionState *types.State) map[string]any {
	if !sessionState.Has(ContextKey) {
		sessionState.Set(ContextKey, struct{}{})
	}
	return sessionState.ToMap()
}

// CodeExecutorContext manages persistent state for code execution sessions.
// It tracks execution history, error counts, and file management across multiple execution calls.
type CodeExecutorContext struct {
	context map[string]any

	sessionState *types.State

	mu sync.RWMutex
}

// NewExecutionContext creates a new execution context with the given execution ID.
func NewExecutionContext(sessionState *types.State) *CodeExecutorContext {
	return &CodeExecutorContext{
		context:      getCodeExecutorContext(sessionState),
		sessionState: sessionState,
	}
}

// GetStateDelta gets the state delta to update in the persistent session state.
func (ec *CodeExecutorContext) GetStateDelta() map[string]any {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	m := maps.Clone(ec.context)
	return map[string]any{ContextKey: m}
}

// GetExecutionID returns the execution ID for this context.
func (ec *CodeExecutorContext) GetExecutionID() string {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	val, ok := ec.context[SessionIDKey]
	if !ok {
		return ""
	}
	return val.(string)
}

// SetExecutionID updates the execution ID for this context.
func (ec *CodeExecutorContext) SetExecutionID(executionID string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.context[SessionIDKey] = executionID
}

// GetProcessedFileNames returns a copy of the processed file names.
func (ec *CodeExecutorContext) GetProcessedFileNames() []string {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	val, ok := ec.context[ProcessedFileNamesKey]
	if !ok {
		return nil
	}
	return val.([]string)
}

// AddProcessedFileNames adds file names to the processed files list.
func (ec *CodeExecutorContext) AddProcessedFileNames(fileNames ...string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if _, ok := ec.context[ProcessedFileNamesKey]; !ok {
		ec.context[ProcessedFileNamesKey] = []string{}
	}
	ec.context[ProcessedFileNamesKey] = append(ec.context[ProcessedFileNamesKey].([]string), fileNames...)
}

// GetInputFiles returns a copy of the input files.
func (ec *CodeExecutorContext) GetInputFiles() []*types.CodeExecutionFile {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	val, ok := ec.context[InputFileKey]
	if !ok {
		return nil
	}
	return val.([]*types.CodeExecutionFile)
}

// AddInputFiles adds files to the input files list.
func (ec *CodeExecutorContext) AddInputFiles(files ...*types.CodeExecutionFile) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if _, ok := ec.context[InputFileKey]; !ok {
		ec.context[InputFileKey] = []string{}
	}
	ec.context[InputFileKey] = append(ec.context[InputFileKey].([]*types.CodeExecutionFile), files...)
}

// ClearInputFiles removes the input files and processed file names to the code executor context.
func (ec *CodeExecutorContext) ClearInputFiles(files ...*types.CodeExecutionFile) {
	if _, ok := ec.context[InputFileKey]; ok {
		ec.context[InputFileKey] = []*types.CodeExecutionFile{}
	}
	if _, ok := ec.context[ProcessedFileNamesKey]; ok {
		ec.context[ProcessedFileNamesKey] = []string{}
	}
}

// GetErrorCount returns the error count for the given invocation ID.
func (ec *CodeExecutorContext) GetErrorCount(invocationID string) int {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	val, ok := ec.context[ErrorCountKey]
	if !ok {
		return 0
	}
	return val.(map[string]int)[invocationID]
}

// IncrementErrorCount increments the error count for the given invocation ID.
func (ec *CodeExecutorContext) IncrementErrorCount(invocationID string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	val, ok := ec.context[ErrorCountKey]
	if !ok {
		ec.context[ErrorCountKey] = make(map[string]int)
	}
	val.(map[string]int)[invocationID] = ec.GetErrorCount(invocationID) + 1
}

// ResetErrorCount resets the error count from the session state.
func (ec *CodeExecutorContext) ResetErrorCount(invocationID string) {
	val, ok := ec.context[ErrorCountKey]
	if !ok {
		return
	}
	if _, ok := val.(map[string]int)[invocationID]; ok {
		delete(val.(map[string]int), invocationID)
	}
}

// UpdateExecutionResult updates the execution result for the given invocation ID.
// If the result doesn't exist, it will be added.
func (ec *CodeExecutorContext) UpdateExecutionResult(invocationID, code, stdout, stderr string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if _, ok := ec.context[CodeExecutionResultsKey]; !ok {
		ec.context[ErrorCountKey] = make(map[string][]*types.CodeExecutionResult)
	}
	if _, ok := ec.context[CodeExecutionResultsKey].(map[string][]*types.CodeExecutionResult)[invocationID]; !ok {
		ec.context[CodeExecutionResultsKey].(map[string][]*types.CodeExecutionResult)[invocationID] = []*types.CodeExecutionResult{}
	}

	ec.context[CodeExecutionResultsKey] = append(ec.context[CodeExecutionResultsKey].(map[string][]*types.CodeExecutionResult)[invocationID], &types.CodeExecutionResult{
		Code:      code,
		Stdout:    stdout,
		Stderr:    stderr,
		Timestamp: time.Now(),
	})
}

// AddExecutionResult adds an execution result to the context.
// This is a convenience method that extracts the execution details from the result.
func (ec *CodeExecutorContext) AddExecutionResult(result *types.CodeExecutionResult) {
	if result == nil {
		return
	}

	// Use the execution ID from the result, or fall back to the context's execution ID
	executionID := result.ExecutionID
	if executionID == "" {
		executionID = ec.GetExecutionID()
	}

	ec.UpdateExecutionResult(executionID, result.Code, result.Stdout, result.Stderr)
}

// GetContextFromInvocation extracts or creates a CodeExecutorContext from an InvocationContext.
// This enables isolated code execution state management per invocation while maintaining
// session-level persistence across multiple execution calls.
//
// The function creates a State object from the session's state map and uses the provided
// execution ID for context isolation. If executionID is empty, it falls back to the
// invocation ID from the context.
//
// Returns nil if the invocation context or its session is nil.
func GetContextFromInvocation(ictx *types.InvocationContext, executionID string) *CodeExecutorContext {
	if ictx == nil || ictx.Session == nil {
		return nil
	}

	// Create a State object from the session's state map
	sessionStateMap := ictx.Session.State()
	sessionState := types.NewState(sessionStateMap, nil)

	// Create the code executor context
	execContext := NewExecutionContext(sessionState)

	// Use provided execution ID, fallback to invocation ID
	if executionID != "" {
		execContext.SetExecutionID(executionID)
	} else if ictx.InvocationID != "" {
		execContext.SetExecutionID(ictx.InvocationID)
	}

	return execContext
}
