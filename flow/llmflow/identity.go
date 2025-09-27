// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package llmflow

import (
	"context"
	"iter"

	"github.com/xenoweaver/adk-go/types"
)

// IdentityLlmRequestProcessor represents a gives the agent identity from the framework.
type IdentityLlmRequestProcessor struct{}

var _ types.LLMRequestProcessor = (*IdentityLlmRequestProcessor)(nil)

// Run implements [LLMRequestProcessor].
func (p *IdentityLlmRequestProcessor) Run(ctx context.Context, ictx *types.InvocationContext, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		llmAgent := ictx.Agent
		si := []string{`You are an agent. Your internal name is "` + llmAgent.Name() + `".`}
		if llmAgent.Description() != "" {
			si = append(si, ` The description about you is "`+llmAgent.Description()+`"`)
		}
		request.AppendInstructions(si...)

		return
	}
}
