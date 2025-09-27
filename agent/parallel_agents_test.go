// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"context"
	"iter"
	"testing"

	"github.com/xenoweaver/adk-go/agent"
	"github.com/xenoweaver/adk-go/types"
)

func Test_mergeAgentRun(t *testing.T) {
	ctx := context.Background()

	// Create test iterators
	iter1 := func(yield func(*types.Event, error) bool) {
		if !yield(&types.Event{ /* fields */ }, nil) {
			return
		}
		if !yield(&types.Event{ /* fields */ }, nil) {
			return
		}
	}

	iter2 := func(yield func(*types.Event, error) bool) {
		yield(&types.Event{ /* fields */ }, nil)
		// No more events
	}

	// Merge the iterators
	merged := agent.MergeAgentRun(ctx, []iter.Seq2[*types.Event, error]{iter1, iter2})

	// Collect events from the merged iterator
	events := []*types.Event{}
	merged(func(event *types.Event, err error) bool {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return false
		}
		events = append(events, event)
		return true
	})

	// Verify the events
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}
