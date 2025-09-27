// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"log/slog"

	"github.com/xenoweaver/adk-go/tool"
	"github.com/xenoweaver/adk-go/types"
)

// AuthenticatedTool is a handles authentication before the actual tool logic
// gets called. Functions can accept a special `credential` argument which is the
// credential ready for use.
//
// # Experimental
//
// This feature is experimental and may change or be removed in future versions without notice. It may
// introduce breaking changes at any time.
type AuthenticatedTool struct {
	*tool.Tool

	Logger                  *slog.Logger
	CredentialsManager      *types.CredentialManager
	responseForAuthRequired map[string]any
}

var _ types.AuthenticatedTool = (*AuthenticatedTool)(nil)

// AuthenticatedToolOption configures an [AuthenticatedTool].
type AuthenticatedToolOption func(*AuthenticatedTool)

// WithResponseForAuthRequired sets the authRequired response for the [AuthenticatedTool].
func WithResponseForAuthRequired(authRequired map[string]any) AuthenticatedToolOption {
	return func(t *AuthenticatedTool) {
		t.responseForAuthRequired = authRequired
	}
}

// NewAuthenticatedTool creates a new authenticated tool with the given name, description, authConfig, and responseForAuthRequired.
func NewAuthenticatedTool(name, description string, authConfig *types.AuthConfig, opts ...AuthenticatedToolOption) *AuthenticatedTool {
	at := &AuthenticatedTool{
		Tool:                    tool.NewTool(name, description, false),
		Logger:                  slog.Default(),
		responseForAuthRequired: make(map[string]any),
	}
	for _, opt := range opts {
		opt(at)
	}

	if authConfig != nil && authConfig.AuthScheme != nil {
		at.CredentialsManager = types.NewCredentialManager(authConfig)
	} else {
		at.Logger.Warn("authConfig or authConfig.AuthScheme is missing. Will skip authentication. Using FunctionTool instead if authentication is not required")
	}

	return at
}

// Name implements [types.AuthenticatedTool].
func (t *AuthenticatedTool) Name() string {
	return t.Tool.Name()
}

// Description implements [types.AuthenticatedTool].
func (t *AuthenticatedTool) Description() string {
	return t.Tool.Description()
}

// IsLongRunning implements [types.AuthenticatedTool].
func (t *AuthenticatedTool) IsLongRunning() bool {
	return t.Tool.IsLongRunning()
}

// Run implements [types.AuthenticatedTool].
func (t *AuthenticatedTool) Run(ctx context.Context, args map[string]any, toolCtx *types.ToolContext) (any, error) {
	var credential *types.AuthCredential
	if t.CredentialsManager != nil {
		var err error
		credential, err = t.CredentialsManager.GetAuthCredential(ctx, toolCtx)
		if err != nil {
			return nil, err
		}
		if credential == nil {
			t.CredentialsManager.RequestCredential(ctx, toolCtx)
			return t.responseForAuthRequired, nil
		}
	}

	return t.Execute(ctx, args, toolCtx, credential)
}

// Execute implements [types.AuthenticatedTool].
func (t *AuthenticatedTool) Execute(ctx context.Context, args map[string]any, toolCtx *types.ToolContext, credential *types.AuthCredential) (any, error) {
	return nil, nil
}
