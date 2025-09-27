// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package codeexecutor

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/session"
	"github.com/xenoweaver/adk-go/types"
)

func TestGetContextFromInvocation(t *testing.T) {
	tests := []struct {
		name           string
		invocationCtx  *types.InvocationContext
		executionID    string
		expectedResult bool // true if we expect a non-nil result
		expectedID     string
	}{
		{
			name:           "nil invocation context",
			invocationCtx:  nil,
			executionID:    "test-exec-id",
			expectedResult: false,
		},
		{
			name: "invocation context with nil session",
			invocationCtx: &types.InvocationContext{
				InvocationID: "test-id",
				Session:      nil,
			},
			executionID:    "test-exec-id",
			expectedResult: false,
		},
		{
			name: "valid invocation context with execution ID",
			invocationCtx: &types.InvocationContext{
				InvocationID: "test-invocation-id",
				Session: session.NewSession(
					"test-app",
					"test-user",
					"test-session",
					map[string]any{"existing": "data"},
					time.Now(),
				),
				UserContent: &genai.Content{
					Role:  "user",
					Parts: []*genai.Part{{Text: "test message"}},
				},
			},
			executionID:    "test-exec-id",
			expectedResult: true,
			expectedID:     "test-exec-id",
		},
		{
			name: "valid invocation context without execution ID",
			invocationCtx: &types.InvocationContext{
				InvocationID: "test-invocation-id",
				Session: session.NewSession(
					"test-app",
					"test-user",
					"test-session",
					map[string]any{"existing": "data"},
					time.Now(),
				),
			},
			executionID:    "",
			expectedResult: true,
			expectedID:     "test-invocation-id",
		},
		{
			name: "invocation context with empty invocation ID",
			invocationCtx: &types.InvocationContext{
				InvocationID: "",
				Session: session.NewSession(
					"test-app",
					"test-user",
					"test-session",
					map[string]any{"existing": "data"},
					time.Now(),
				),
			},
			executionID:    "",
			expectedResult: true,
			expectedID:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetContextFromInvocation(tt.invocationCtx, tt.executionID)

			if tt.expectedResult {
				if result == nil {
					t.Errorf("Expected non-nil result, got nil")
					return
				}

				// Check that the execution ID was set correctly
				if diff := cmp.Diff(tt.expectedID, result.GetExecutionID()); diff != "" {
					t.Errorf("Execution ID mismatch (-want +got):\n%s", diff)
				}

				// Check that the context has access to session state
				if result.sessionState == nil {
					t.Errorf("Expected sessionState to be set, got nil")
				}

				// If we have a session, verify it contains the expected data
				if tt.invocationCtx.Session != nil {
					sessionData := tt.invocationCtx.Session.State()
					if existingData, ok := sessionData["existing"]; ok {
						// The context should have access to this data through its state
						contextMap := result.sessionState.ToMap()
						if diff := cmp.Diff(existingData, contextMap["existing"]); diff != "" {
							t.Errorf("Session data mismatch (-want +got):\n%s", diff)
						}
					}
				}
			} else {
				if result != nil {
					t.Errorf("Expected nil result, got %v", result)
				}
			}
		})
	}
}

func TestGetContextFromInvocation_ContextIsolation(t *testing.T) {
	// Create a session with some initial state
	testSession := session.NewSession(
		"test-app",
		"test-user",
		"test-session",
		map[string]any{"shared": "data"},
		time.Now(),
	)

	// Create two different invocation contexts using the same session
	invocation1 := &types.InvocationContext{
		InvocationID: "invocation-1",
		Session:      testSession,
	}

	invocation2 := &types.InvocationContext{
		InvocationID: "invocation-2",
		Session:      testSession,
	}

	// Get contexts for both invocations
	ctx1 := GetContextFromInvocation(invocation1, "exec-1")
	ctx2 := GetContextFromInvocation(invocation2, "exec-2")

	if ctx1 == nil || ctx2 == nil {
		t.Fatal("Expected both contexts to be non-nil")
	}

	// Verify they have different execution IDs
	if ctx1.GetExecutionID() == ctx2.GetExecutionID() {
		t.Errorf("Expected different execution IDs, got same: %s", ctx1.GetExecutionID())
	}

	// Verify both contexts have access to the shared session data
	ctx1Map := ctx1.sessionState.ToMap()
	ctx2Map := ctx2.sessionState.ToMap()

	if diff := cmp.Diff(ctx1Map["shared"], ctx2Map["shared"]); diff != "" {
		t.Errorf("Shared session data should be the same (-ctx1 +ctx2):\n%s", diff)
	}

	// Verify they can maintain separate execution state
	ctx1.AddProcessedFileNames("file1.py")
	ctx2.AddProcessedFileNames("file2.py")

	ctx1Files := ctx1.GetProcessedFileNames()
	ctx2Files := ctx2.GetProcessedFileNames()

	if len(ctx1Files) != 1 || ctx1Files[0] != "file1.py" {
		t.Errorf("Expected ctx1 to have only file1.py, got %v", ctx1Files)
	}

	if len(ctx2Files) != 1 || ctx2Files[0] != "file2.py" {
		t.Errorf("Expected ctx2 to have only file2.py, got %v", ctx2Files)
	}
}
