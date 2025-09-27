// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/tool"
	"github.com/xenoweaver/adk-go/types"
)

// GoogleSearchTool represents a built-in tool that is automatically invoked by Gemini 2 models to retrieve search results from Google Search.
//
// This tool operates internally within the model and does not require or perform
// local code execution.
type GoogleSearchTool struct {
	*tool.Tool
}

var _ types.Tool = (*GoogleSearchTool)(nil)

// NewGoogleSearchTool returns the new [GoogleSearchTool].
func NewGoogleSearchTool() *GoogleSearchTool {
	return &GoogleSearchTool{
		Tool: tool.NewTool("google_search", "google_search", false),
	}
}

// Name implements [types.Tool].
func (t *GoogleSearchTool) Name() string {
	return t.Tool.Name()
}

// Description implements [types.Tool].
func (t *GoogleSearchTool) Description() string {
	return t.Tool.Description()
}

// IsLongRunning implements [types.Tool].
func (t *GoogleSearchTool) IsLongRunning() bool {
	return t.Tool.IsLongRunning()
}

// GetDeclaration implements [types.Tool].
func (t *GoogleSearchTool) GetDeclaration() *genai.FunctionDeclaration {
	return nil
}

// Run implements [types.Tool].
func (t *GoogleSearchTool) Run(ctx context.Context, args map[string]any, toolCtx *types.ToolContext) (any, error) {
	return nil, nil
}

// ProcessLLMRequest implements [types.Tool].
func (t *GoogleSearchTool) ProcessLLMRequest(ctx context.Context, toolCtx *types.ToolContext, request *types.LLMRequest) error {
	if request.Config == nil {
		request.Config = new(genai.GenerateContentConfig)
	}

	switch request.Model {
	case "gemini-1":
		if request.Model == "gemini-1" {
			if len(request.Config.Tools) > 0 {
				return errors.New("Google search tool can not be used with other tools in Gemini 1.x.")
			}
			request.Config.Tools = append(request.Config.Tools, &genai.Tool{
				GoogleSearchRetrieval: &genai.GoogleSearchRetrieval{},
			})
		}

	case "gemini-2":
		request.Config.Tools = append(request.Config.Tools, &genai.Tool{
			GoogleSearch: &genai.GoogleSearch{},
		})
	}

	return fmt.Errorf("Google search tool is not supported for model %s", request.Model)
}
