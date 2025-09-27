// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"

	"github.com/xenoweaver/adk-go/types"
)

// LoadMemoryResponse represents a response from the LoadMemory tool.
type LoadMemoryResponse struct {
	memories []*types.MemoryEntry
}

// LoadMemory loads the memory for the current user.
func LoadMemory(ctx context.Context, query string, toolCtx *types.ToolContext) (*LoadMemoryResponse, error) {
	searchMemoryResponse, err := toolCtx.SearchMemory(ctx, query)
	if err != nil {
		return nil, err
	}

	return &LoadMemoryResponse{
		memories: searchMemoryResponse.Memories,
	}, nil
}

// LoadMemoryTool represents a tool that loads the memory for the current user.
//
// NOTE(adk-python): Currently this tool only uses text part from the memory.
//
// TODO(zchee): depends on [FunctionTool].
type LoadMemoryTool struct{}
