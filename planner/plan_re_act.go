// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package planner

import (
	"context"
	"slices"
	"strings"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/types"
)

const (
	PlanningTag    = "/*PLANNING*/"
	ReplanningTag  = "/*REPLANNING*/"
	ReasoningTag   = "/*REASONING*/"
	ActionTag      = "/*ACTION*/"
	FinalAnswerTag = "/*FINAL_ANSWER*/"
)

const (
	HighLevelPreamble = `
When answering the question, try to leverage the available tools to gather the information instead of your memorized knowledge.

Follow this process when answering the question: (1) first come up with a plan in natural language text format; (2) Then use tools to execute the plan and provide reasoning between tool code snippets to make a summary of current state and next step. Tool code snippets and reasoning should be interleaved with each other. (3) In the end, return one final answer.

Follow this format when answering the question: (1) The planning part should be under ` + PlanningTag + `. (2) The tool code snippets should be under ` + ActionTag + `, and the reasoning parts should be under ` + ReasoningTag + `. (3) The final answer part should be under ` + FinalAnswerTag + `.
`

	PlanningRreamble = `
Below are the requirements for the planning:
The plan is made to answer the user query if following the plan. The plan is coherent and covers all aspects of information from user query, and only involves the tools that are accessible by the agent. The plan contains the decomposed steps as a numbered list where each step should use one or multiple available tools. By reading the plan, you can intuitively know which tools to trigger or what actions to take.
If the initial plan cannot be successfully executed, you should learn from previous execution results and revise your plan. The revised plan should be be under ` + ReasoningTag + `. Then use tools to follow the new plan.
`

	ReasoningPreamble = `
Below are the requirements for the reasoning:
The reasoning makes a summary of the current trajectory based on the user query and tool outputs. Based on the tool outputs and plan, the reasoning also comes up with instructions to the next steps, making the trajectory closer to the final answer.
`

	FinalAnswerPreamble = `
Below are the requirements for the final answer:
The final answer should be precise and follow query formatting requirements. Some queries may not be answerable with the available tools and information. In those cases, inform the user why you cannot process their query and ask for more information.
`

	// Only contains the requirements for custom tool/libraries.
	ToolCodeWithoutPythonLibrariesPreamble = `
Below are the requirements for the tool code:

**Custom Tools:** The available tools are described in the context and can be directly used.
- Code must be valid self-contained Python snippets with no imports and no references to tools or Python libraries that are not in the context.
- You cannot use any parameters or fields that are not explicitly defined in the APIs in the context.
- The code snippets should be readable, efficient, and directly relevant to the user query and reasoning steps.
- When using the tools, you should use the library name together with the function name, e.g., vertex_search.search().
- If Python libraries are not provided in the context, NEVER write your own code other than the function calls using the provided tools.
`

	UserInputPreamble = `
VERY IMPORTANT instruction that you MUST follow in addition to the above instructions:

You should ask for clarification if you need more information to answer the question.
You should prefer using the information available in the context instead of repeated tool use.
`
)

// PlanReActPlanner represents a plan-Re-Act planner that constrains the LLM response to generate a plan before any action/observation.
//
// NOTE(adk-go): this planner does not require the model to support built-in thinking
// features or setting the thinking config.
type PlanReActPlanner struct{}

var _ types.Planner = (*PlanReActPlanner)(nil)

// BuildPlanningInstruction implements [types.Planner].
func (p *PlanReActPlanner) BuildPlanningInstruction(ctx context.Context, rctx *types.ReadOnlyContext, request *types.LLMRequest) string {
	return p.buildNLPlannerInstruction()
}

// buildNLPlannerInstruction builds the NL planner instruction for the Plan-Re-Act planner.
func (p *PlanReActPlanner) buildNLPlannerInstruction() string {
	preambles := []string{
		HighLevelPreamble,
		PlanningRreamble,
		ReasoningPreamble,
		FinalAnswerPreamble,
		ToolCodeWithoutPythonLibrariesPreamble,
		UserInputPreamble,
	}

	return strings.Join(preambles, "\n\n")
}

// ProcessPlanningResponse implements [types.Planner].
func (p *PlanReActPlanner) ProcessPlanningResponse(ctx context.Context, cctx *types.CallbackContext, responseParts []*genai.Part) []*genai.Part {
	if len(responseParts) == 0 {
		return []*genai.Part{}
	}

	preservedParts := []*genai.Part{}
	firstFCPartIndex := -1
	for i := range responseParts {
		// Stop at the first (group of) function calls.
		if responseParts[i].FunctionCall != nil {
			// Ignore and filter out function calls with empty names.
			if responseParts[i].FunctionCall.Name == "" {
				continue
			}

			preservedParts = append(preservedParts, responseParts[i])
			firstFCPartIndex = i
			break
		}

		// Split the response into reasoning and final answer parts.
		preservedParts = p.handleNonFunctionCallParts(responseParts[i], preservedParts)
	}

	if firstFCPartIndex > 0 {
		for j := firstFCPartIndex + 1; j < len(responseParts); j++ {
			if responseParts[j].FunctionCall != nil {
				preservedParts = append(preservedParts, responseParts[j])
			}
		}
	}

	return preservedParts
}

// handleNonFunctionCallParts handles non-function-call parts of the response.
func (p *PlanReActPlanner) handleNonFunctionCallParts(responsePart *genai.Part, preservedParts []*genai.Part) []*genai.Part {
	preservedPartsCopy := slices.Clone(preservedParts)
	var (
		reasoningText   string
		finalAnswerText string
	)

	if responsePart.Text != "" && strings.Contains(responsePart.Text, FinalAnswerTag) {
		idx := strings.LastIndex(responsePart.Text, FinalAnswerTag)
		reasoningText, finalAnswerText = responsePart.Text[idx:], responsePart.Text[:idx]
		var reasoningPart *genai.Part
		if reasoningText != "" {
			reasoningPart = genai.NewPartFromText(reasoningText)
			reasoningPart.Thought = true
			preservedPartsCopy = append(preservedPartsCopy, reasoningPart)
		}

		if finalAnswerText != "" {
			preservedPartsCopy = append(preservedPartsCopy, genai.NewPartFromText(finalAnswerText))
		}

		return preservedPartsCopy // early return
	}

	responseText := responsePart.Text

	// If the part is a text part with a planning/reasoning/action tag, label it as reasoning.
	isReasoningTags := []string{
		PlanningTag,
		ReasoningTag,
		ActionTag,
		ReplanningTag,
	}
	containsFn := func(tag string) bool {
		if strings.HasPrefix(responseText, tag) {
			return true
		}
		return false
	}
	if responseText != "" && slices.ContainsFunc(isReasoningTags, containsFn) {
		responsePart.Thought = true
	}
	preservedPartsCopy = append(preservedPartsCopy, responsePart)

	return preservedPartsCopy
}
