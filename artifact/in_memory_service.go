// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package artifact

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/types"
)

// InMemoryService represents an in-memory implementation of the artifact service.
type InMemoryService struct {
	artifacts map[string][]*genai.Part
	mu        sync.Mutex
}

var _ types.ArtifactService = (*InMemoryService)(nil)

// NewInMemoryService creates a new instance of [InMemoryService].
func NewInMemoryService() *InMemoryService {
	return &InMemoryService{
		artifacts: make(map[string][]*genai.Part),
	}
}

// fileHasUserNamespace checks if the filename has a user namespace.
func (a *InMemoryService) fileHasUserNamespace(filename string) bool {
	return strings.HasPrefix(filename, "user:")
}

// artifactPath constructs the artifact path.
func (a *InMemoryService) artifactPath(appName, userID, sessionID, filename string) string {
	if a.fileHasUserNamespace(filename) {
		return fmt.Sprintf("%s/%s/user/%s", appName, userID, filename)
	}
	return fmt.Sprintf("%s/%s/%s/%s", appName, userID, sessionID, filename)
}

// SaveArtifact implements [types.ArtifactService].
func (a *InMemoryService) SaveArtifact(ctx context.Context, appName, userID, sessionID, filename string, artifact *genai.Part) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	path := a.artifactPath(appName, userID, sessionID, filename)
	version := len(a.artifacts[path])
	a.artifacts[path] = append(a.artifacts[path], artifact)

	return version, nil
}

// LoadArtifact implements [types.ArtifactService].
func (a *InMemoryService) LoadArtifact(ctx context.Context, appName, userID, sessionID, filename string, version int) (*genai.Part, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	path := a.artifactPath(appName, userID, sessionID, filename)
	versions, ok := a.artifacts[path]
	if !ok {
		return nil, nil
	}
	if version >= 0 {
		version = len(versions) - 1
	}

	return versions[version], nil
}

// ListArtifactKey implements [types.ArtifactService].
func (a *InMemoryService) ListArtifactKey(ctx context.Context, appName, userID, sessionID string) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	sessionPrefix := fmt.Sprintf("%s/%s/%s/", appName, userID, sessionID)
	usernamespacePrefix := fmt.Sprintf("%s/%s/user/", appName, userID)

	filenames := []string{}
	for path := range a.artifacts {
		switch {
		case strings.HasPrefix(path, sessionPrefix):
			filename := strings.TrimPrefix(path, sessionPrefix)
			filenames = append(filenames, filename)

		case strings.HasPrefix(path, usernamespacePrefix):
			filename := strings.TrimPrefix(path, usernamespacePrefix)
			filenames = append(filenames, filename)
		}
	}
	slices.Sort(filenames)

	return filenames, nil
}

// DeleteArtifact implements [types.ArtifactService].
func (a *InMemoryService) DeleteArtifact(ctx context.Context, appName, userID, sessionID, filename string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	path := a.artifactPath(appName, userID, sessionID, filename)
	if _, ok := a.artifacts[path]; !ok {
		return nil
	}
	delete(a.artifacts, path)

	return nil
}

// ListVersions implements [types.ArtifactService].
func (a *InMemoryService) ListVersions(ctx context.Context, appName, userID, sessionID, filename string) ([]int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	path := a.artifactPath(appName, userID, sessionID, filename)
	versions, ok := a.artifacts[path]
	if !ok {
		return nil, nil
	}

	verList := make([]int, len(versions))
	for i := range versions {
		verList[i] = i
	}

	return verList, nil
}

// Close implements [types.ArtifactService].
func (a *InMemoryService) Close() error {
	// nothing to do
	return nil
}
