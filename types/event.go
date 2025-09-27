// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	rand "math/rand/v2"
	"time"
	"unsafe"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/pkg/py"
)

// Event represents an event in a conversation between agents and users.
//
// It is used to store the content of the conversation, as well as the actions
// taken by the agents like function calls, etc.
type Event struct {
	*LLMResponse

	// InvocationID is The invocation ID of the event.
	// TODO(adk-python): revert to be required after spark migration
	InvocationID string

	// Author is the 'user' or the name of the agent, indicating who appended the event to the session.
	Author string

	// Actions is the Actions taken by the agent
	Actions *EventActions

	// LongRunningToolIDs set of ids of the long running function calls.
	//
	// Agent client will know from this field about which function call is long running.
	// Only valid for function call event.
	LongRunningToolIDs py.Set[string]

	// Branch is The Branch of the event.
	//
	// The format is like agent_1.agent_2.agent_3, where agent_1 is the parent of
	// agent_2, and agent_2 is the parent of agent_3.
	//
	// Branch is used when multiple sub-agent shouldn't see their peer agents'
	// conversation history.
	Branch string

	// Do not assign the ID. It will be assigned by the session.

	// ID is the unique identifier of the event.
	//
	// ReadOnly. It will be assigned by the session layer.
	ID string

	// Timestamp is The Timestamp of the event.
	//
	// ReadOnly. It will be assigned by the session layer.
	Timestamp time.Time
}

// WithLLMResponse sets the LLMResponse for the event.
func (e *Event) WithLLMResponse(response *LLMResponse) *Event {
	e.LLMResponse = response
	return e
}

// WithContent sets the content of the event's LLMResponse.
func (e *Event) WithContent(content *genai.Content) *Event {
	if e.LLMResponse == nil {
		e.LLMResponse = new(LLMResponse)
	}
	e.LLMResponse.Content = content
	return e
}

// WithInvocationID sets the invocation ID of the event.
func (e *Event) WithInvocationID(id string) *Event {
	e.InvocationID = id
	return e
}

// WithAuthor sets the author of the event.
func (e *Event) WithAuthor(author string) *Event {
	e.Author = author
	return e
}

// WithActions sets the actions of the event.
func (e *Event) WithActions(actions *EventActions) *Event {
	e.Actions = actions
	return e
}

// WithLongRunningToolIDs sets the long running tool IDs of the event.
func (e *Event) WithLongRunningToolIDs(ids ...string) *Event {
	e.LongRunningToolIDs.Insert(ids...)
	return e
}

// WithBranch sets the branch of the event.
func (e *Event) WithBranch(branch string) *Event {
	e.Branch = branch
	return e
}

// NewEvent creates a new event with a unique ID and timestamp.
func NewEvent() *Event {
	ev := &Event{
		ID:        NewEventID(),
		Timestamp: time.Now(),
	}
	return ev
}

// IsFinalResponse returns whether the event is the final response of the agent.
func (e *Event) IsFinalResponse() bool {
	if e.Actions.SkipSummarization || len(e.LongRunningToolIDs) > 0 {
		return true
	}

	return len(e.GetFunctionCalls()) == 0 && len(e.GetFunctionResponses()) == 0 && !e.Partial && !e.HasTrailingCodeExecutionResult()
}

// GetFunctionCalls returns the function calls in the event.
func (e *Event) GetFunctionCalls() []*genai.FunctionCall {
	var funcCalls []*genai.FunctionCall

	if e.Content != nil && len(e.Content.Parts) > 0 {
		for _, part := range e.Content.Parts {
			if part.FunctionCall != nil {
				funcCalls = append(funcCalls, part.FunctionCall)
			}
		}
	}

	return funcCalls
}

// GetFunctionResponses returns the function responses in the event.
func (e *Event) GetFunctionResponses() []*genai.FunctionResponse {
	var funcResponse []*genai.FunctionResponse

	if e.Content != nil && len(e.Content.Parts) > 0 {
		for _, part := range e.Content.Parts {
			if part.FunctionResponse != nil {
				funcResponse = append(funcResponse, part.FunctionResponse)
			}
		}
	}

	return funcResponse
}

// HasTrailingCodeExecutionResult returns whether the event has a trailing code execution result.
func (e *Event) HasTrailingCodeExecutionResult() bool {
	if e.Content != nil && len(e.Content.Parts) > 0 {
		return e.Content.Parts[len(e.Content.Parts)-1].CodeExecutionResult != nil
	}
	return false
}

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func NewEventID() string {
	b := make([]byte, 8)
	for i, cache, remain := 8-1, rand.Int64(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache = rand.Int64()
			remain = letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}
