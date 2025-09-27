// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/go-json-experiment/json"
	deepcopy "github.com/tiendc/go-deepcopy"
	"golang.org/x/oauth2"

	"github.com/xenoweaver/adk-go/internal/pool"
)

type AuthHandler struct {
	authConfig *AuthConfig
}

// NewAuthHandler creates a new AuthHandler with the given authConfig.
func NewAuthHandler(authConfig *AuthConfig) *AuthHandler {
	return &AuthHandler{
		authConfig: authConfig,
	}
}

// ExchangeAuthToken Generates an auth token from the authorization response.
func (h *AuthHandler) ExchangeAuthToken(ctx context.Context) (*AuthCredential, error) {
	authScheme := h.authConfig.AuthScheme
	authCredential := h.authConfig.ExchangedAuthCredential

	var tokenEndpoint string
	var scopes []string
	switch authScheme := authScheme.(type) {
	case *OpenIDConnectWithConfig:
		if authScheme.TokenEndpoint == "" {
			return authCredential, nil
		}
		tokenEndpoint = authScheme.TokenEndpoint
		scopes = authScheme.Scopes

	case *OAuth2SecurityScheme:
		if authScheme.Flows.AuthorizationCode == nil || authScheme.Flows.AuthorizationCode.TokenURL == "" {
			return authCredential, nil
		}

	default:
		return authCredential, nil
	}

	if authCredential == nil || authCredential.OAuth2 == nil || authCredential.OAuth2.ClientID == "" || authCredential.OAuth2.ClientSecret == "" || authCredential.OAuth2.AccessToken != "" || authCredential.OAuth2.RefreshToken != "" {
		return h.authConfig.ExchangedAuthCredential, nil
	}

	conf := &oauth2.Config{
		ClientID:     authCredential.OAuth2.ClientID,
		ClientSecret: authCredential.OAuth2.ClientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenEndpoint,
		},
		Scopes:      scopes,
		RedirectURL: authCredential.OAuth2.RedirectURI,
	}

	tok, err := conf.Exchange(ctx, authCredential.OAuth2.AccessToken, oauth2.SetAuthURLParam("grant_type", string(AuthorizationCodeGrant)))
	if err != nil {
		return nil, err
	}

	updatedCredential := &AuthCredential{
		AuthType: OAuth2CredentialTypes,
		OAuth2: &OAuth2Auth{
			AccessToken:  tok.AccessToken,
			RefreshToken: tok.RefreshToken,
		},
	}

	return updatedCredential, nil
}

func (h *AuthHandler) ParseAndStoreAuthSesponse(ctx context.Context, state *State) error {
	credentialKey := h.GetCredentialKey()
	state.Set(credentialKey, h.authConfig.ExchangedAuthCredential)

	authScheme := h.authConfig.AuthScheme
	switch authScheme.(type) {
	case *APIKeySecurityScheme, *HTTPBaseSecurityScheme, *OAuth2SecurityScheme, *OpenIdConnectSecurityScheme:
		// no-op
	default:
		return nil
	}
	authSchemeType := GetAuthSchemeType(h.authConfig.AuthScheme)
	switch authSchemeType {
	case OAuth2CredentialTypes, OpenIDConnectCredentialTypes:
		return nil
	}

	creds, err := h.ExchangeAuthToken(ctx)
	if err != nil {
		return err
	}
	state.Set(credentialKey, creds)
	return nil
}

func (h *AuthHandler) GetAuthResponse(state *State) *AuthCredential {
	credentialKey := h.GetCredentialKey()
	creds, ok := state.Get(credentialKey)
	if !ok {
		return nil
	}
	return creds.(*AuthCredential)
}

func (h *AuthHandler) GenerateAuthRequest() (*AuthConfig, error) {
	isCopied := false
	authScheme := h.authConfig.AuthScheme
	switch authScheme.(type) {
	case *APIKeySecurityScheme, *HTTPBaseSecurityScheme, *OAuth2SecurityScheme, *OpenIdConnectSecurityScheme:
		// no-op
	case *OpenIDConnectWithConfig:
		isCopied = true
	}
	authSchemeType := GetAuthSchemeType(h.authConfig.AuthScheme)
	switch authSchemeType {
	case OAuth2CredentialTypes, OpenIDConnectCredentialTypes:
		isCopied = true
	}

	// auth_uri already in exchanged credential
	if exchangedAuthCreds := h.authConfig.ExchangedAuthCredential; exchangedAuthCreds != nil && exchangedAuthCreds.OAuth2 != nil && exchangedAuthCreds.OAuth2.AuthURI != "" {
		isCopied = true
	}

	if isCopied {
		var authConfig AuthConfig
		if err := deepcopy.Copy(&authConfig, h.authConfig); err != nil {
			panic(err)
		}
		return &authConfig, nil
	}

	// Check if raw_auth_credential exists
	if h.authConfig.RawAuthCredential == nil {
		return nil, fmt.Errorf("auth Scheme %s requires auth_credential", h.authConfig.AuthScheme)
	}

	// Check if oauth2 exists in raw_auth_credential
	if h.authConfig.RawAuthCredential.OAuth2 == nil {
		return nil, fmt.Errorf("auth Scheme %s requires oauth2 in auth_credential", h.authConfig.AuthScheme)
	}

	// auth_uri in raw credential
	if h.authConfig.RawAuthCredential.OAuth2.AuthURI != "" {
		var exchangedAuthCredential AuthCredential
		if err := deepcopy.Copy(&exchangedAuthCredential, h.authConfig.ExchangedAuthCredential); err != nil {
			return nil, err
		}
		return &AuthConfig{
			AuthScheme:              h.authConfig.AuthScheme,
			RawAuthCredential:       h.authConfig.RawAuthCredential,
			ExchangedAuthCredential: &exchangedAuthCredential,
		}, nil
	}

	// Check for client_id and client_secret
	if h.authConfig.RawAuthCredential.OAuth2.ClientID == "" || h.authConfig.RawAuthCredential.OAuth2.ClientSecret == "" {
		return nil, fmt.Errorf("auth Scheme %s requires both client_id and client_secret in auth_credential.oauth2", h.authConfig.AuthScheme)
	}

	// Generate new auth URI
	exchangedCredential, err := h.GenerateAuthURI()
	if err != nil {
		return nil, err
	}
	return &AuthConfig{
		AuthScheme:              h.authConfig.AuthScheme,
		RawAuthCredential:       h.authConfig.RawAuthCredential,
		ExchangedAuthCredential: exchangedCredential,
	}, nil
}

// GetCredentialKey generates an unique key for the given auth scheme and credential.
func (h *AuthHandler) GetCredentialKey() string {
	authScheme := h.authConfig.AuthScheme
	authCredential := h.authConfig.RawAuthCredential

	var schemaName string
	buf := pool.Buffer.Get()
	if authScheme != nil {
		schemaType := GetAuthSchemeType(authScheme)
		if err := json.MarshalWrite(buf, authScheme, json.DefaultOptionsV2()); err != nil {
			panic(fmt.Errorf("marshal authScheme: %w", err))
		}
		hash := sha256.Sum256(buf.Bytes())
		schemaName = fmt.Sprintf("%s_%s", schemaType, hash[:4])
	}

	var credentialName string
	if authCredential != nil {
		buf.Reset()
		if err := json.MarshalWrite(buf, authCredential, json.DefaultOptionsV2()); err != nil {
			panic(fmt.Errorf("marshal authCredential: %w", err))
		}
		hash := sha256.Sum256(buf.Bytes())
		credentialName = fmt.Sprintf("%s_%s", authCredential.AuthType, hash[:4])
	}
	pool.Buffer.Put(buf)

	return fmt.Sprintf("temp:adk_%s_%s", schemaName, credentialName)
}

// GenerateAuthURI generates an response containing the auth uri for user to sign in.
func (h *AuthHandler) GenerateAuthURI() (*AuthCredential, error) {
	authScheme := h.authConfig.AuthScheme
	authCredential := h.authConfig.RawAuthCredential

	var authorizationEndpoint string
	var scopes []string
	switch authScheme := authScheme.(type) {
	case *OpenIDConnectWithConfig:
		authorizationEndpoint = authScheme.AuthorizationEndpoint
		scopes = authScheme.Scopes

	case *OAuth2SecurityScheme:
		if authScheme.Flows == nil {
			return nil, errors.New("oauth flows not defined in security scheme")
		}

		switch {
		case authScheme.Flows.Implicit != nil && authScheme.Flows.Implicit.AuthorizationURL != "":
			authorizationEndpoint = authScheme.Flows.Implicit.AuthorizationURL
			if authScheme.Flows.Implicit.Scopes != nil {
				scopes = make([]string, 0, len(authScheme.Flows.Implicit.Scopes))
				for scope := range authScheme.Flows.Implicit.Scopes {
					scopes = append(scopes, scope)
				}
			}
		case authScheme.Flows.AuthorizationCode != nil && authScheme.Flows.AuthorizationCode.AuthorizationURL != "":
			authorizationEndpoint = authScheme.Flows.AuthorizationCode.AuthorizationURL
			if authScheme.Flows.AuthorizationCode.Scopes != nil {
				scopes = make([]string, 0, len(authScheme.Flows.AuthorizationCode.Scopes))
				for scope := range authScheme.Flows.AuthorizationCode.Scopes {
					scopes = append(scopes, scope)
				}
			}
		case authScheme.Flows.ClientCredentials != nil && authScheme.Flows.ClientCredentials.TokenURL != "":
			authorizationEndpoint = authScheme.Flows.ClientCredentials.TokenURL
			if authScheme.Flows.ClientCredentials.Scopes != nil {
				scopes = make([]string, 0, len(authScheme.Flows.ClientCredentials.Scopes))
				for scope := range authScheme.Flows.ClientCredentials.Scopes {
					scopes = append(scopes, scope)
				}
			}
		case authScheme.Flows.Password != nil && authScheme.Flows.Password.TokenURL != "":
			authorizationEndpoint = authScheme.Flows.Password.TokenURL
			if authScheme.Flows.Password.Scopes != nil {
				scopes = make([]string, 0, len(authScheme.Flows.Password.Scopes))
				for scope := range authScheme.Flows.Password.Scopes {
					scopes = append(scopes, scope)
				}
			}
		default:
			return nil, errors.New("no valid authorization URL found in security scheme")
		}

	default:
		return nil, errors.New("unsupported auth scheme type")
	}

	conf := &oauth2.Config{
		ClientID:     authCredential.OAuth2.ClientID,
		ClientSecret: authCredential.OAuth2.ClientSecret,
		Scopes:       scopes,
		RedirectURL:  authCredential.OAuth2.RedirectURI,
		Endpoint: oauth2.Endpoint{
			AuthURL: authorizationEndpoint,
		},
	}
	state := generateState()
	uri := conf.AuthCodeURL(state, oauth2.ApprovalForce)

	var exchangedAuthCredential AuthCredential
	if err := deepcopy.Copy(&exchangedAuthCredential, h.authConfig.ExchangedAuthCredential); err != nil {
		return nil, err
	}
	exchangedAuthCredential.OAuth2.AuthURI = uri
	exchangedAuthCredential.OAuth2.State = state

	return &exchangedAuthCredential, nil
}

func generateState() string {
	data := make([]byte, 30)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
