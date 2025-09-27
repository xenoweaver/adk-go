// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package credentialservice

import (
	"context"

	"github.com/xenoweaver/adk-go/types"
)

type (
	// Credentials represents a map of application names to their respective user credentials.
	Credentials map[string]AppCredentials // appName -> appCredentials

	// AppCredentials represents a map of user IDs to their respective user credentials.
	AppCredentials map[string]UserCredentials // userID -> userCredentials

	// UserCredentials represents a map of credential keys to their respective authentication credentials.
	UserCredentials map[string]*types.AuthCredential // credential key -> *types.AuthCredential
)

// InMemory represents an in memory implementation of [types.CredentialService].
//
// # Experimental
//
// This feature is experimental and may change or be removed in future versions without notice. It may
// introduce breaking changes at any time.
type InMemory struct {
	credentials Credentials
}

var _ types.CredentialService = (*InMemory)(nil)

// NewInMemory returns the new [InMemory].
func NewInMemory() *InMemory {
	return &InMemory{
		credentials: make(Credentials),
	}
}

// LoadCredential implements [types.CredentialService].
func (c *InMemory) LoadCredential(ctx context.Context, authConfig *types.AuthConfig, toolCtx *types.ToolContext) (*types.AuthCredential, error) {
	credentialBucket := c.getBucketForCurrentContext(toolCtx)
	return credentialBucket[authConfig.CredentialKey()], nil
}

// SaveCredential implements [types.CredentialService].
func (c *InMemory) SaveCredential(ctx context.Context, authConfig *types.AuthConfig, toolCtx *types.ToolContext) error {
	credentialBucket := c.getBucketForCurrentContext(toolCtx)
	credentialBucket[authConfig.CredentialKey()] = authConfig.ExchangedAuthCredential
	return nil
}

func (c *InMemory) getBucketForCurrentContext(toolCtx *types.ToolContext) UserCredentials {
	appName := toolCtx.InvocationContext().AppName()
	// lazy initialize of appCredentials map
	if _, ok := c.credentials[appName]; !ok {
		c.credentials[appName] = make(AppCredentials)
	}

	userID := toolCtx.InvocationContext().UserID()
	// lazy initialize of userCredentials map
	if _, ok := c.credentials[appName][userID]; !ok {
		c.credentials[appName][userID] = make(UserCredentials)
	}

	return c.credentials[appName][userID]
}
