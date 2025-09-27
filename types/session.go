// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"encoding/base64"
	"time"

	"github.com/go-json-experiment/json"
	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/internal/pool"
)

// Session represents a user session with events that can be stored in memory.
type Session interface {
	// ID returns the session ID.
	ID() string

	// AppName returns the application name.
	AppName() string

	// UserID returns the user ID.
	UserID() string

	// State is the state of the session.
	State() map[string]any

	// Events returns the events in the session.
	Events() []*Event

	// LastUpdateTime is the last update time of the session.
	LastUpdateTime() time.Time

	// AddEvent adds an event to this session.
	AddEvent(events ...*Event)

	// GetRecentEvents returns the most recent n events.
	SetLastUpdateTime(time.Time)
}

// EncodeContent encodes a Content object to a JSON dictionary.
func EncodeContent(content *genai.Content) (map[string]any, error) {
	if content == nil {
		return nil, nil
	}

	buf := pool.Buffer.Get()

	// First, convert to JSON
	if err := json.MarshalWrite(buf, content, json.DefaultOptionsV2()); err != nil {
		return nil, err
	}

	// Then unmarshal into a map
	var result map[string]any
	if err := json.UnmarshalRead(buf, &result, json.DefaultOptionsV2()); err != nil {
		return nil, err
	}

	pool.Buffer.Put(buf)

	// Handle base64 encoding for inline data
	if parts, ok := result["parts"].([]any); ok {
		for _, part := range parts {
			if p, ok := part.(map[string]any); ok {
				if inlineData, ok := p["inlineData"].(map[string]any); ok {
					if data, ok := inlineData["data"].([]byte); ok {
						inlineData["data"] = base64.StdEncoding.EncodeToString(data)
					}
				}
			}
		}
	}

	return result, nil
}

// DecodeContent decodes a Content object from a JSON dictionary.
func DecodeContent(content map[string]any) (*genai.Content, error) {
	if content == nil {
		return nil, nil
	}

	// Handle base64 decoding for inline data
	if parts, ok := content["parts"].([]any); ok {
		for _, part := range parts {
			if p, ok := part.(map[string]any); ok {
				if inlineData, ok := p["inlineData"].(map[string]any); ok {
					if data, ok := inlineData["data"].(string); ok {
						decoded, err := base64.StdEncoding.DecodeString(data)
						if err != nil {
							return nil, err
						}
						inlineData["data"] = decoded
					}
				}
			}
		}
	}

	// Convert map back to JSON
	buf := pool.Buffer.Get()
	if err := json.MarshalWrite(buf, content, json.DefaultOptionsV2()); err != nil {
		return nil, err
	}

	// Then unmarshal into a Content object
	var result genai.Content
	if err := json.UnmarshalRead(buf, &result, json.DefaultOptionsV2()); err != nil {
		return nil, err
	}
	pool.Buffer.Put(buf)

	return &result, nil
}
