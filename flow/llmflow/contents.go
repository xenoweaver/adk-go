// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package llmflow

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	deepcopy "github.com/tiendc/go-deepcopy"
	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/internal/xiter"
	"github.com/xenoweaver/adk-go/model"
	"github.com/xenoweaver/adk-go/pkg/py"
	"github.com/xenoweaver/adk-go/types"
)

// ContentLLMRequestProcessor builds the contents for the LLM request.
type ContentLLMRequestProcessor struct{}

var _ types.LLMRequestProcessor = (*ContentLLMRequestProcessor)(nil)

// Run implements [LLMRequestProcessor].
func (cp *ContentLLMRequestProcessor) Run(ctx context.Context, ictx *types.InvocationContext, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		llmAgent, ok := ictx.Agent.AsLLMAgent()
		if !ok {
			return
		}

		if llmAgent.IncludeContents() != types.IncludeContentsNone {
			contents, err := cp.getContents(ictx.Branch, ictx.Session.Events(), llmAgent.Name())
			if err != nil {
				xiter.Error[*types.Event](err)
				return
			}
			request.Contents = contents
		}
	}
}

// getContents get the contents for the LLM request.
func (cp *ContentLLMRequestProcessor) getContents(currentBranch string, events []*types.Event, agentName string) ([]*genai.Content, error) {
	var filteredEvents []*types.Event

	for _, event := range events {
		if event.Content == nil || event.Content.Role == "" || len(event.Content.Parts) == 0 || event.Content.Parts[0].Text == "" {
			// Skip events without content, or generated neither by user nor by model
			// or has empty text.
			// E.g. events purely for mutating session states.
			continue
		}

		if !cp.isEventBelongsToBranch(currentBranch, event) {
			// Skip events not belong to current branch.
			continue
		}

		if cp.isAuthEvent(event) {
			// Skip events not belong to current branch.
			continue
		}

		ev := event
		if cp.isOtherAgentReply(currentBranch, event) {
			ev = cp.convertForeignEvent(event)
		}
		filteredEvents = append(filteredEvents, ev)
	}

	resultEvents, err := cp.rearrangeEventsForLatestFunctionResponse(filteredEvents)
	if err != nil {
		return nil, err
	}
	resultEvents, err = cp.rearrangeEventsForAsyncFunctionResponsesInHistory(resultEvents)
	if err != nil {
		return nil, err
	}

	contents := []*genai.Content{}
	for _, event := range resultEvents {
		content := &genai.Content{}
		if err := deepcopy.Copy(content, event.Content); err != nil {
			return nil, err
		}
		content = RemoveClientFunctionCallID(content)
		contents = append(contents, content)
	}

	return contents, nil
}

// rearrangeEventsForAsyncFunctionResponsesInHistory rearrange the async function_response events in the history.
func (cp *ContentLLMRequestProcessor) rearrangeEventsForAsyncFunctionResponsesInHistory(events []*types.Event) ([]*types.Event, error) {
	funcCallIDToResponseEventsIndex := make(map[string][]*types.Event)
	for i, event := range events {
		funcResponses := event.GetFunctionResponses()
		if len(funcResponses) > 0 {
			for _, funcResponse := range funcResponses {
				funcCallID := funcResponse.ID
				funcCallIDToResponseEventsIndex[funcCallID] = append(funcCallIDToResponseEventsIndex[funcCallID], events[i])
			}
		}
	}

	resultEvent := []*types.Event{}
	for _, event := range events {
		if len(event.GetFunctionResponses()) > 0 {
			// function_response should be handled together with function_call below.
			continue
		}

		if funcCalls := event.GetFunctionCalls(); len(funcCalls) > 0 {
			funcResponseEventsIndices := py.NewSet[*types.Event]()
			for _, funcCall := range funcCalls {
				funcCallID := funcCall.ID
				if evs, ok := funcCallIDToResponseEventsIndex[funcCallID]; ok {
					for _, ev := range evs {
						funcResponseEventsIndices.Insert(ev)
					}
				}
			}

			resultEvent = append(resultEvent, event)
			switch {
			case funcResponseEventsIndices.Len() == 0:
				continue

			case funcResponseEventsIndices.Len() == 1:
				resultEvent = append(resultEvent, funcResponseEventsIndices.UnsortedList()...)

			default:
				resEvent, err := cp.mergeFunctionResponseEvents(funcResponseEventsIndices.UnsortedList())
				if err != nil {
					return nil, err
				}
				resultEvent = append(resultEvent, resEvent)
			}
			continue
		}

		resultEvent = append(resultEvent, event)
	}

	return resultEvent, nil
}

// rearrangeEventsForLatestFunctionResponse rearrange the events for the latest function_response.
//
// If the latest function_response is for an async function_call, all events
// between the initial function_call and the latest function_response will be
// removed.
func (cp *ContentLLMRequestProcessor) rearrangeEventsForLatestFunctionResponse(events []*types.Event) ([]*types.Event, error) {
	if len(events) == 0 {
		return events, nil
	}

	funcResponses := events[len(events)-1].GetFunctionResponses()
	if len(funcResponses) == 0 {
		// No need to process, since the latest event is not fuction_response.
		return events, nil
	}

	funcResponsesIDs := py.NewSet[string]()
	for _, funcResponse := range funcResponses {
		funcResponsesIDs.Insert(funcResponse.ID)
	}

	funcCalls := events[len(events)-2].GetFunctionCalls()
	if len(funcCalls) > 0 {
		for _, funcCall := range funcCalls {
			// The latest function_response is already matched
			if funcResponsesIDs.Has(funcCall.ID) {
				return events, nil
			}
		}
	}

	funcCallEventIdx := -1
	// look for corresponding function call event reversely
	for idx := len(events) - 2; idx >= -1; idx-- {
		event := events[idx]
		funcCalls := event.GetFunctionCalls()
		if len(funcCalls) > 0 {
			for _, funcCall := range funcCalls {
				if funcResponsesIDs.Has(funcCall.ID) {
					funcCallEventIdx = idx
					break
				}
			}
			for _, funcCall := range funcCalls {
				if funcCallEventIdx != -1 {
					// in case the last response event only have part of the responses
					// for the function calls in the function call event
					funcResponsesIDs.Insert(funcCall.ID)
					break
				}
			}
		}
	}

	if funcCallEventIdx == -1 {
		return nil, fmt.Errorf("no function call event found for function responses ids: %v", funcResponsesIDs.UnsortedList())
	}

	// collect all function response between last function response event
	// and function call event

	funcResponseEvents := []*types.Event{}
	for idx := funcCallEventIdx + 1; idx <= len(events)-1; idx++ {
		event := events[idx]
		funcResponses := event.GetFunctionResponses()
		if len(funcResponses) > 0 && funcResponsesIDs.Has(funcResponses[0].ID) {
			funcResponseEvents = append(funcResponseEvents, event)
		}
	}
	funcResponseEvents = append(funcResponseEvents, events[len(events)-1])

	resultEvents := events[:funcCallEventIdx+1]
	evs, err := cp.mergeFunctionResponseEvents(funcResponseEvents)
	if err != nil {
		return nil, err
	}
	resultEvents = append(resultEvents, evs)

	return resultEvents, nil
}

// isOtherAgentReply whether the event is a reply from another agent.
func (cp *ContentLLMRequestProcessor) isOtherAgentReply(currentAgentName string, event *types.Event) bool {
	return currentAgentName != "" && event.Author != currentAgentName && event.Author != "user"
}

// convertForeignEvent converts an event authored by another agent as a user-content event.
//
// This is to provide another agent's output as context to the current agent, so
// that current agent can continue to respond, such as summarizing previous
// agent's reply, etc.
func (cp *ContentLLMRequestProcessor) convertForeignEvent(event *types.Event) *types.Event {
	if event.Content == nil || len(event.Content.Parts) == 0 {
		return event
	}

	content := &genai.Content{
		Role: model.RoleUser,
		Parts: []*genai.Part{
			genai.NewPartFromText("For context:"),
		},
	}

	for _, part := range event.Content.Parts {
		switch {
		case part.Text != "":
			content.Parts = append(content.Parts, genai.NewPartFromText(fmt.Sprintf("[%s] said: %s", event.Author, part.Text)))

		case part.FunctionCall != nil:
			content.Parts = append(content.Parts, genai.NewPartFromText(fmt.Sprintf("[%s] called tool `%s` with parameters: %v", event.Author, part.FunctionCall.Name, part.FunctionCall.Args)))

		case part.FunctionResponse != nil:
			// Otherwise, create a new text part.
			content.Parts = append(content.Parts, genai.NewPartFromText(fmt.Sprintf("[%s] `%s` returned result: %v", event.Author, part.FunctionResponse.Name, part.FunctionResponse.Response)))

		default:
			content.Parts = append(content.Parts, part)
		}
	}

	ev := types.NewEvent().
		WithAuthor("user").
		WithContent(content).
		WithBranch(event.Branch)
	ev.Timestamp = event.Timestamp

	return ev
}

// mergeFunctionResponseEvents merges a list of function_response events into one event.
//
// The key goal is to ensure:
// 1. function_call and function_response are always of the same number.
// 2. The function_call and function_response are consecutively in the content.
func (cp *ContentLLMRequestProcessor) mergeFunctionResponseEvents(funcResponseEvents []*types.Event) (*types.Event, error) {
	if len(funcResponseEvents) == 0 {
		return nil, errors.New("at least one function_response event is required")
	}

	mergedEvent := &types.Event{}
	if err := deepcopy.Copy(mergedEvent, funcResponseEvents[0]); err != nil {
		return nil, err
	}
	partsInMergedEvent := mergedEvent.Content.Parts

	if len(partsInMergedEvent) == 0 {
		return nil, errors.New("there should be at least one function_response part")
	}

	partIndicesInMergedEvent := make(map[string]int)
	for i, part := range partsInMergedEvent {
		if part.FunctionResponse != nil {
			funcCallID := part.FunctionResponse.ID
			partIndicesInMergedEvent[funcCallID] = i
		}
	}

	for _, event := range funcResponseEvents[1:] {
		if len(event.Content.Parts) == 0 {
			return nil, errors.New("there should be at least one function_response part")
		}

		for _, part := range event.Content.Parts {
			if part.FunctionResponse != nil {
				funcCallID := part.FunctionResponse.ID
				if _, ok := partIndicesInMergedEvent[funcCallID]; ok {
					idx := partIndicesInMergedEvent[funcCallID]
					partsInMergedEvent = slices.Insert(partsInMergedEvent, idx, part)
				} else {
					partsInMergedEvent = append(partsInMergedEvent, part)
					partIndicesInMergedEvent[funcCallID] = len(partsInMergedEvent) - 1
				}
			} else {
				partsInMergedEvent = append(partsInMergedEvent, part)
			}
		}
	}

	return mergedEvent, nil
}

// isEventBelongsToBranch Event belongs to a branch, when event.branch is prefix of the invocation branch.
func (cp *ContentLLMRequestProcessor) isEventBelongsToBranch(invocationBranch string, event *types.Event) bool {
	if invocationBranch == "" || event.Branch == "" {
		return true
	}
	return strings.HasPrefix(invocationBranch, event.Branch)
}

func (cp *ContentLLMRequestProcessor) isAuthEvent(event *types.Event) bool {
	if len(event.Content.Parts) == 0 {
		return false
	}

	for _, part := range event.Content.Parts {
		if part.FunctionCall != nil && part.FunctionCall.Name == RequestEUCFunctionCallName {
			return true
		}

		if part.FunctionResponse != nil && part.FunctionResponse.Name == RequestEUCFunctionCallName {
			return true
		}
	}

	return false
}
