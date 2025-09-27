// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/xenoweaver/adk-go/internal/xmaps"
	"github.com/xenoweaver/adk-go/pkg/py"
	"github.com/xenoweaver/adk-go/types"
)

// InMemoryService represents an in-memory memory service for prototyping purpose only.
//
// Uses keyword matching instead of semantic search.
type InMemoryService struct {
	// Keys are app_name/user_id, session_id. Values are session event lists.
	sessionEvents map[string]map[string][]*types.Event
	logger        *slog.Logger
	mu            sync.RWMutex
}

var _ types.MemoryService = (*InMemoryService)(nil)

// WithLogger sets the logger for the InMemoryService.
func (s *InMemoryService) WithLogger(logger *slog.Logger) *InMemoryService {
	s.logger = logger
	return s
}

// NewInMemoryService creates a new InMemoryService.
func NewInMemoryService() *InMemoryService {
	return &InMemoryService{
		sessionEvents: make(map[string]map[string][]*types.Event),
		logger:        slog.Default(),
	}
}

func (s *InMemoryService) userKey(appName, userID string) string {
	return fmt.Sprintf("%s/%s", appName, userID)
}

func (s *InMemoryService) extractWordsLower(text string) py.Set[string] {
	return py.NewSet(strings.ToLower(text))
}

// AddSessionToMemory implements [types.MemoryService].
func (s *InMemoryService) AddSessionToMemory(ctx context.Context, session types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userKey := s.userKey(session.AppName(), session.UserID())
	for _, event := range session.Events() {
		if event.Content != nil || len(event.Content.Parts) > 0 {
			s.sessionEvents[userKey][session.ID()] = append(s.sessionEvents[userKey][session.ID()], event)
		}
	}

	return nil
}

// SearchMemory implements [types.MemoryService].
func (s *InMemoryService) SearchMemory(ctx context.Context, appName, userID, query string) (*types.SearchMemoryResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userKey := s.userKey(appName, userID)
	if !xmaps.Contains(s.sessionEvents, userKey) {
		return &types.SearchMemoryResponse{}, nil
	}

	wordsInQuery := py.NewSet(strings.Split(query, " ")...)
	response := &types.SearchMemoryResponse{
		Memories: make([]*types.MemoryEntry, 0),
	}

	for _, sessionEvent := range s.sessionEvents[userKey] {
		for _, event := range sessionEvent {
			if event.Content == nil || len(event.Content.Parts) == 0 {
				continue
			}
			var partText []string
			for _, part := range event.Content.Parts {
				partText = append(partText, part.Text)
			}
			wordsInEvent := s.extractWordsLower(strings.Join(partText, ""))
			if wordsInEvent.Len() == 0 {
				continue
			}

			for _, queryWord := range wordsInQuery.UnsortedList() {
				if wordsInEvent.Has(queryWord) {
					response.Memories = append(response.Memories, &types.MemoryEntry{
						Content:   event.Content,
						Author:    event.Author,
						Timestamp: event.Timestamp,
					})
				}
			}
		}
	}

	return response, nil
}

// SearchMemory implements [types.MemoryService].
func (s *InMemoryService) Close() error {
	// nothing to do
	return nil
}
