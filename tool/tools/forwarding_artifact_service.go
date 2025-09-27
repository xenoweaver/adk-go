// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"errors"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/types"
)

// ForwardingArtifactService represents an artifact service that forwards to the parent tool context.
type ForwardingArtifactService struct {
	toolCtx *types.ToolContext
	ictx    *types.InvocationContext
}

var _ types.ArtifactService = (*ForwardingArtifactService)(nil)

// NewForwardingArtifactService returns a new [ForwardingArtifactService] given a tool context.
func NewForwardingArtifactService(toolCtx *types.ToolContext) *ForwardingArtifactService {
	return &ForwardingArtifactService{
		toolCtx: toolCtx,
		ictx:    toolCtx.InvocationContext(),
	}
}

// SaveArtifact implements [types.ArtifactService].
func (a *ForwardingArtifactService) SaveArtifact(ctx context.Context, appName, userID, sessionID, filename string, artifact *genai.Part) (int, error) {
	return a.toolCtx.SaveArtifact(ctx, filename, artifact)
}

// LoadArtifact implements [types.ArtifactService].
func (a *ForwardingArtifactService) LoadArtifact(ctx context.Context, appName, userID, sessionID, filename string, version int) (*genai.Part, error) {
	return a.toolCtx.LoadArtifact(ctx, filename, version)
}

// ListArtifactKey implements [types.ArtifactService].
func (a *ForwardingArtifactService) ListArtifactKey(ctx context.Context, appName, userID, sessionID string) ([]string, error) {
	return a.toolCtx.ListArtifacts(ctx)
}

// DeleteArtifact implements [types.ArtifactService].
func (a *ForwardingArtifactService) DeleteArtifact(ctx context.Context, appName, userID, sessionID, filename string) error {
	if a.ictx.ArtifactService == nil {
		return errors.New("artifact service is not initialized")
	}

	return a.ictx.ArtifactService.DeleteArtifact(ctx, a.ictx.AppName(), a.ictx.UserID(), a.ictx.Session.ID(), filename)
}

// ListVersions implements [types.ArtifactService].
func (a *ForwardingArtifactService) ListVersions(ctx context.Context, appName, userID, sessionID, filename string) ([]int, error) {
	if a.ictx.ArtifactService == nil {
		return nil, errors.New("artifact service is not initialized")
	}

	return a.ictx.ArtifactService.ListVersions(ctx, a.ictx.AppName(), a.ictx.UserID(), a.ictx.Session.ID(), filename)
}

// Close implements [types.ArtifactService].
func (a *ForwardingArtifactService) Close() error {
	// nothing to do
	return nil
}
