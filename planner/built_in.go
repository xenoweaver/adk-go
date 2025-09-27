// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package planner

import (
	"context"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/types"
)

// BuiltInPlanner represents a built-in planner that uses model's built-in thinking features.
type BuiltInPlanner struct {
	// Config for model built-in thinking features. An error will be returned if this
	// field is set for models that don't support thinking.
	thinkingConfig *genai.ThinkingConfig
}

var _ types.Planner = (*BuiltInPlanner)(nil)

// NewBuiltInPlanner returns a new BuiltInPlanner with the provided thinking configuration.
func NewBuiltInPlanner(thinkingConfig *genai.ThinkingConfig) *BuiltInPlanner {
	return &BuiltInPlanner{
		thinkingConfig: thinkingConfig,
	}
}

// ApplyThinkingConfig applies the thinking config to the LLM request.
func (p *BuiltInPlanner) ApplyThinkingConfig(request *types.LLMRequest) {
	if p.thinkingConfig != nil {
		if request.Config == nil {
			request.Config = new(genai.GenerateContentConfig)
		}

		request.Config.ThinkingConfig = p.thinkingConfig
	}
}

// BuildPlanningInstruction implements [types.Planner].
func (p *BuiltInPlanner) BuildPlanningInstruction(context.Context, *types.ReadOnlyContext, *types.LLMRequest) string {
	return ""
}

// ProcessPlanningResponse implements [types.Planner].
func (p *BuiltInPlanner) ProcessPlanningResponse(context.Context, *types.CallbackContext, []*genai.Part) []*genai.Part {
	return nil
}
