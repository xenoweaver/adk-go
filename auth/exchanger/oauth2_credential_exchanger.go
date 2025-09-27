// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package exchanger

import (
	"context"
	"fmt"
	"net/url"

	"github.com/xenoweaver/adk-go/types"
)

// OAuth2CredentialExchanger exchanges OAuth2 credentials from authorization responses.
//
// # Experimental
//
// This feature is experimental and may change or be removed in future versions without notice. It may
// introduce breaking changes at any time.
type OAuth2CredentialExchanger struct{}

var _ types.CredentialExchanger = (*OAuth2CredentialExchanger)(nil)

// Exchange implements [types.CredentialExchanger].
func (e *OAuth2CredentialExchanger) Exchange(ctx context.Context, authCredential *types.AuthCredential, authScheme types.AuthScheme) (*types.AuthCredential, error) {
	if authScheme == nil {
		return nil, types.CredentialExchangError("authScheme is required for OAuth2 credential exchange")
	}

	if authCredential.OAuth2 != nil && authCredential.OAuth2.AccessToken != "" {
		return authCredential, nil
	}

	client := types.CreateOAuth2Session(ctx, authScheme, authCredential)
	if client == nil {
		return authCredential, nil
	}

	authURL, err := url.Parse(authCredential.OAuth2.AuthResponseURI)
	if err != nil {
		return nil, types.CredentialExchangError(fmt.Errorf("invalid auth response URI: %w", err).Error())
	}
	// Validate state parameter (CSRF protection)
	receivedState := authURL.Query().Get("state")
	if receivedState != authCredential.OAuth2.State {
		return nil, fmt.Errorf("state mismatch: expected %s, got %s", authCredential.OAuth2.State, receivedState)
	}

	// Use the code from the credential if provided, otherwise extract from URL
	code := authCredential.OAuth2.AuthCode
	if code == "" {
		code = authURL.Query().Get("code")
		if code == "" {
			return nil, fmt.Errorf("authorization code not found")
		}
	}

	token, err := client.Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	newAuthCredential := types.UpdateCredentialWithTokens(authCredential, token)

	return newAuthCredential, nil
}
