// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package credentialservice

import (
	"context"

	"github.com/xenoweaver/adk-go/types"
)

// SessionState represents a credential service using session state as the
// store.
//
// # Experimental
//
// This feature is experimental and may change or be removed in future versions without notice. It may
// introduce breaking changes at any time.
type SessionState struct{}

var _ types.CredentialService = (*SessionState)(nil)

// LoadCredential implements [types.CredentialService].
func (c *SessionState) LoadCredential(ctx context.Context, authConfig *types.AuthConfig, toolCtx *types.ToolContext) (*types.AuthCredential, error) {
	creds, ok := toolCtx.State().Get(authConfig.CredentialKey())
	if !ok {
		return nil, nil
	}

	return creds.(*types.AuthCredential), nil
}

// SaveCredential implements [types.CredentialService].
func (c *SessionState) SaveCredential(ctx context.Context, authConfig *types.AuthConfig, toolCtx *types.ToolContext) error {
	toolCtx.State().Set(authConfig.CredentialKey(), authConfig.ExchangedAuthCredential)
	return nil
}
