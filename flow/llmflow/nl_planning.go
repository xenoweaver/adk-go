// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package llmflow

import (
	"context"
	"iter"

	"github.com/xenoweaver/adk-go/planner"
	"github.com/xenoweaver/adk-go/types"
)

// NLPlanningRequestProcessor represents a processor for NL planning.
type NLPlanningRequestProcessor struct{}

var _ types.LLMRequestProcessor = (*NLPlanningRequestProcessor)(nil)

// Run implements [types.LLMRequestProcessor].
func (p *NLPlanningRequestProcessor) Run(ctx context.Context, ictx *types.InvocationContext, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		plnr := getPlanner(ictx)
		if plnr == nil {
			return
		}

		if plnr, ok := plnr.(interface {
			ApplyThinkingConfig(request *types.LLMRequest)
		}); ok {
			plnr.ApplyThinkingConfig(request)
		}

		if planningInstruction := plnr.BuildPlanningInstruction(ctx, types.NewReadOnlyContext(ictx), request); planningInstruction != "" {
			request.AppendInstructions(planningInstruction)
		}

		removeThoughtFromRequest(request)
	}
}

type NLPlanningResponseProcessor struct{}

var _ types.LLMResponseProcessor = (*NLPlanningResponseProcessor)(nil)

func (p *NLPlanningResponseProcessor) Run(ctx context.Context, ictx *types.InvocationContext, response *types.LLMResponse) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		if response == nil || response.Content == nil || len(response.Content.Parts) == 0 {
			return
		}

		plnr := getPlanner(ictx)
		if plnr == nil {
			return
		}

		// Postprocess the LLM response.
		cctx := types.NewCallbackContext(ictx)
		processedParts := plnr.ProcessPlanningResponse(ctx, cctx, response.Content.Parts)
		if len(processedParts) > 0 {
			response.Content.Parts = append(response.Content.Parts, processedParts...)
		}

		if cctx.State().HasDelta() {
			stateUpdateEvent := types.NewEvent().
				WithInvocationID(ictx.InvocationID).
				WithAuthor(ictx.Agent.Name()).
				WithBranch(ictx.Branch).
				WithActions(cctx.EventActions())

			if !yield(stateUpdateEvent, nil) {
				return
			}
		}
	}
}

func getPlanner(ictx *types.InvocationContext) types.Planner {
	llmAgent, ok := ictx.Agent.AsLLMAgent()
	if !ok {
		return nil
	}

	plnr := llmAgent.Planner()
	if plnr != nil {
		return plnr
	}

	return &planner.PlanReActPlanner{}
}

func removeThoughtFromRequest(request *types.LLMRequest) {
	if len(request.Contents) == 0 {
		return
	}

	for i, content := range request.Contents {
		if len(content.Parts) == 0 {
			continue
		}
		for j := range content.Parts {
			request.Contents[i].Parts[j].Thought = false
		}
	}
}
