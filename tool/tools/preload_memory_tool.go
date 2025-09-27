// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/xenoweaver/adk-go/tool"
	"github.com/xenoweaver/adk-go/types"
)

// PreloadMemoryTool represents a tool that preloads the memory for the current user.
//
// NOTE(adk-python): Currently this tool only uses text part from the memory.
type PreloadMemoryTool struct {
	*tool.Tool
}

var _ types.Tool = (*PreloadMemoryTool)(nil)

// NewPreloadMemoryTool returns the new [PreloadMemoryTool].
func NewPreloadMemoryTool() *PreloadMemoryTool {
	return &PreloadMemoryTool{
		Tool: tool.NewTool("preload_memory", "preload_memory", false),
	}
}

// ProcessLLMRequest implements [types.Tool].
func (t *PreloadMemoryTool) ProcessLLMRequest(ctx context.Context, toolCtx *types.ToolContext, request *types.LLMRequest) error {
	userContent := toolCtx.UserContent()
	if userContent == nil || len(userContent.Parts) == 0 || userContent.Parts[0].Text == "" {
		return nil
	}

	userQuery := userContent.Parts[0].Text
	response, err := toolCtx.SearchMemory(ctx, userQuery)
	if err != nil {
		return err
	}

	var memoryTextLines []string
	for _, memory := range response.Memories {
		if !memory.Timestamp.IsZero() {
			timeStr := fmt.Sprintf("Time: %s", memory.Timestamp)
			memoryTextLines = append(memoryTextLines, timeStr)
		}

		if memoryText := extractText(memory, " "); memoryText != "" {
			switch {
			case memory.Author != "":
				memoryTextLines = append(memoryTextLines, fmt.Sprintf("%s: %s", memory.Author, memoryText))
			default:
				memoryTextLines = append(memoryTextLines, memoryText)
			}
		}
	}
	if len(memoryTextLines) == 0 {
		return nil
	}

	fullMemoryText := strings.Join(memoryTextLines, "\n")
	si := `The following content is from your previous conversations with the user.
They may be useful for answering the user's current query.
<PAST_CONVERSATIONS>
` +
		fullMemoryText +
		`
</PAST_CONVERSATIONS>
`
	request.AppendInstructions(si)

	return nil
}

// extractText extracts the text from the memory entry.
func extractText(memory *types.MemoryEntry, splitter string) string {
	if len(memory.Content.Parts) == 0 {
		return ""
	}

	texts := make([]string, 0, len(memory.Content.Parts))
	for _, part := range memory.Content.Parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}

	return strings.Join(texts, splitter)
}
