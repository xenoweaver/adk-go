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

// URLContextTool represents a built-in tool that is automatically invoked by Gemini 2 models to retrieve content from the URLs and use that content to inform and shape its response.
//
// This tool operates internally within the model and does not require or perform
// local code execution.
type URLContextTool struct {
	*tool.Tool
}

var _ types.Tool = (*URLContextTool)(nil)

// NewUrlContextTool returns the new [URLContextTool].
func NewUrlContextTool() *URLContextTool {
	return &URLContextTool{
		Tool: tool.NewTool("url_context", "url_context", false),
	}
}

// ProcessLLMRequest implements [types.Tool].
func (t *URLContextTool) ProcessLLMRequest(ctx context.Context, toolCtx *types.ToolContext, request *types.LLMRequest) error {
	if request.Model == "gemini-1" {
		return errors.New("ValueError('Url context tool can not be used in Gemini 1.x.')")
	}

	if request.Model == "gemini-2" {
		request.Config.Tools = append(request.Config.Tools, &genai.Tool{
			URLContext: &genai.URLContext{},
		})
		return nil
	}

	return fmt.Errorf("Url context tool is not supported for model %s'", request.Model)
}
