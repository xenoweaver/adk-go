// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"context"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/pkg/py/pyasyncio"
)

// LiveRequest represents a request send to live agents.
type LiveRequest struct {
	// Send the content to the model in turn-by-turn mode if any.
	Content *genai.Content `json:"content,omitempty"`

	// Send the blob to the model in realtime mode if any.
	Blob *genai.Blob `json:"blob,omitempty"`

	// Close the queue.
	Close bool `json:"close"`
}

// LiveRequestQueue is used to send [LiveRequest] in a bidirectional streaming way.
type LiveRequestQueue struct {
	queue pyasyncio.Queue[*LiveRequest]
}

// NewLiveRequestQueue creates a new LiveRequestQueue with a buffered channel.
func NewLiveRequestQueue() *LiveRequestQueue {
	return &LiveRequestQueue{
		queue: pyasyncio.NewQueue[*LiveRequest](100),
	}
}

// Close signals that the queue should be closed by sending a [LiveRequest] with Close set to true.
func (q *LiveRequestQueue) Close() {
	q.queue.PutNowait(&LiveRequest{
		Close: true,
	})
}

// SendContent sends content to the queue.
func (q *LiveRequestQueue) SendContent(content *genai.Content) {
	q.queue.PutNowait(&LiveRequest{
		Content: content,
	})
}

// SendRealtime sends a blob to the queue in realtime mode.
func (q *LiveRequestQueue) SendRealtime(blob *genai.Blob) {
	q.queue.PutNowait(&LiveRequest{
		Blob: blob,
	})
}

// Send sends a LiveRequest to the queue.
func (q *LiveRequestQueue) Send(req *LiveRequest) {
	q.queue.PutNowait(req)
}

// Get gets a [LiveRequest] from the queue.
//
// It accepts a context for cancellation, which is a Go idiomatic pattern
// different from the Python async/await approach.
func (q *LiveRequestQueue) Get(ctx context.Context) (*LiveRequest, error) {
	return q.queue.Get(ctx)
}
