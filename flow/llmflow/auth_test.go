// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package llmflow_test

import (
	"testing"
	"time"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/agent"
	"github.com/xenoweaver/adk-go/flow/llmflow"
	"github.com/xenoweaver/adk-go/session"
	"github.com/xenoweaver/adk-go/types"
)

func TestAuthLLMRequestProcessor_Run(t *testing.T) {
	ctx := t.Context()

	// Create a test agent
	agent, err := agent.NewLLMAgent(ctx, "test-agent")
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Create a test session
	sess := session.NewSession("test-app", "test-user", "test-session", nil, time.Now())

	// Create invocation context
	ictx := &types.InvocationContext{
		Agent:   agent,
		Session: sess,
	}

	// Create test events with auth function response
	// Create function response event with auth config
	funcResponse := &genai.Part{
		FunctionResponse: &genai.FunctionResponse{
			ID:   "test-func-response-id",
			Name: llmflow.RequestEUCFunctionCallName,
			Response: map[string]any{
				"scheme_type": "oauth2",
				"oauth2": map[string]any{
					"client_id":     "test-client-id",
					"client_secret": "test-client-secret",
					"auth_url":      "https://example.com/auth",
					"token_url":     "https://example.com/token",
					"redirect_url":  "https://example.com/callback",
					"scopes":        []string{"read", "write"},
				},
			},
		},
	}

	userEvent := types.NewEvent().
		WithAuthor("user").
		WithContent(&genai.Content{
			Role:  "user",
			Parts: []*genai.Part{funcResponse},
		})

	// Add event to session
	sess.AddEvent(userEvent)

	// Create the auth preprocessor
	processor := llmflow.NewAuthPreprocessor()

	// Create a dummy LLM request
	request := &types.LLMRequest{}

	// Run the processor
	events := make([]*types.Event, 0)
	for event, err := range processor.Run(ctx, ictx, request) {
		if err != nil {
			t.Errorf("Processor returned error: %v", err)
			continue
		}
		if event != nil {
			events = append(events, event)
		}
	}

	// Verify that auth config was stored in session state
	state := sess.State()
	found := false
	for key, value := range state {
		if key[:9] == "temp:adk_" {
			if config, ok := value.(*types.AuthConfig); ok {
				if config.AuthScheme.AuthType() == types.OAuth2CredentialTypes && config.RawAuthCredential.OAuth2.ClientID == "test-client-id" {
					found = true
					break
				}
			}
		}
	}

	if !found {
		t.Error("Auth config was not stored in session state")
	}
}

func TestAuthLLMRequestProcessor_EmptyEvents(t *testing.T) {
	ctx := t.Context()

	// Create a test agent
	agent, err := agent.NewLLMAgent(ctx, "test-agent")
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Create a test session with no events
	sess := session.NewSession("test-app", "test-user", "test-session", nil, time.Now())

	// Create invocation context
	ictx := &types.InvocationContext{
		Agent:   agent,
		Session: sess,
	}

	// Create the auth preprocessor
	processor := llmflow.NewAuthPreprocessor()

	// Create a dummy LLM request
	request := &types.LLMRequest{}

	// Run the processor
	events := make([]*types.Event, 0)
	for event, err := range processor.Run(ctx, ictx, request) {
		if err != nil {
			t.Errorf("Processor returned error: %v", err)
			continue
		}
		if event != nil {
			events = append(events, event)
		}
	}

	// Should return no events for empty session
	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}
