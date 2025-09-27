// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"sync"

	"github.com/xenoweaver/adk-go/pkg/py/pyasyncio"
)

// ActiveStreamingTool manages streaming tool related resources during invocation.
type ActiveStreamingTool[T any] struct {
	// The active task of this streaming tool.
	Task *pyasyncio.Task[T]

	// Stream is the active LiveRequestQueue for this tool.
	Stream *LiveRequestQueue

	// mu protects concurrent access to fields.
	mu sync.Mutex
}

func (a *ActiveStreamingTool[T]) WithTask(task *pyasyncio.Task[T]) *ActiveStreamingTool[T] {
	a.Task = task
	return a
}

func (a *ActiveStreamingTool[T]) WithStream(stream *LiveRequestQueue) *ActiveStreamingTool[T] {
	a.Stream = stream
	return a
}

// NewActiveStreamingTool creates a new [ActiveStreamingTool] instance.
func NewActiveStreamingTool[T any]() *ActiveStreamingTool[T] {
	return &ActiveStreamingTool[T]{}
}

// SetStream sets the active stream for this tool.
func (a *ActiveStreamingTool[T]) SetStream(stream *LiveRequestQueue) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Stream = stream
}

// GetStream returns the current stream.
func (a *ActiveStreamingTool[T]) GetStream() *LiveRequestQueue {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.Stream
}

// ClearStream clears the current stream.
func (a *ActiveStreamingTool[T]) ClearStream() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Stream = nil
}

// IsActive returns true if there is an active task or stream.
func (a *ActiveStreamingTool[T]) IsActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.Task != nil || a.Stream != nil
}
