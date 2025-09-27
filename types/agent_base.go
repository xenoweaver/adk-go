// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/xenoweaver/adk-go/internal/xiter"
)

// BaseAgent represents the base agent.
type BaseAgent struct {
	*Config
}

var _ Agent = (*BaseAgent)(nil)

// AsLLMAgent implements [types.Agent].
func (a *BaseAgent) AsLLMAgent() (LLMAgent, bool) {
	return nil, false
}

// NewBaseAgent creates a new agent configuration with the given name.
//
// TODO(zchee): implements validate logic same as belows in adk-python.
//
//	agents.BaseAgent.__validate_name
//	agents.BaseAgent.__set_parent_agent_for_sub_agents
func NewBaseAgent(name string, opts ...Option) *BaseAgent {
	base := &BaseAgent{
		Config: NewConfig(name),
	}
	for _, opt := range opts {
		opt.apply(base.Config)
	}

	for _, subAgent := range base.subAgents {
		if subAgent.ParentAgent() != nil {
			panic(fmt.Errorf("agent %s already has a parent agent, current parent: %s, trying to add: %s", subAgent.Name(), subAgent.ParentAgent().Name(), base.Name()))
		}
	}

	return base
}

// Name implements [Agent].
func (a *BaseAgent) Name() string {
	return a.Config.Name
}

// Description implements [Agent].
func (a *BaseAgent) Description() string {
	return a.Config.Description
}

// ParentAgent implements [types.Agent].
func (a *BaseAgent) ParentAgent() Agent {
	return a.parentAgent
}

// SubAgents implements [types.Agent].
func (a *BaseAgent) SubAgents() []Agent {
	return a.subAgents
}

// BeforeAgentCallbacks implements [Agent].
func (a *BaseAgent) BeforeAgentCallbacks() []AgentCallback {
	return a.beforeAgentCallbacks
}

// AfterAgentCallbacks implements [Agent].
func (a *BaseAgent) AfterAgentCallbacks() []AgentCallback {
	return a.afterAgentCallbacks
}

// Run implements [Agent].
func (a *BaseAgent) Run(ctx context.Context, parentContext *InvocationContext) iter.Seq2[*Event, error] {
	return func(yield func(*Event, error) bool) {
		parentContext = a.createInvocationContext(parentContext)
		beforeEvent, err := a.handleBeforeAgentCallbacks(ctx, parentContext)
		if err != nil {
			xiter.Error[Event](err)
			return
		}
		if beforeEvent != nil {
			if !yield(beforeEvent, nil) {
				return
			}
			if parentContext.EndInvocation {
				return
			}
		}

		for event, err := range a.Execute(ctx, parentContext) {
			if err != nil {
				xiter.Error[Event](err)
				return
			}
			if !yield(event, nil) {
				return
			}
		}

		if parentContext.EndInvocation {
			return
		}

		afterEvent, err := a.handleAfterAgentCallback(ctx, parentContext)
		if err != nil {
			xiter.Error[Event](err)
			return
		}
		if beforeEvent != nil {
			if !yield(afterEvent, nil) {
				return
			}
		}
	}
}

// RunLive implements [Agent].
func (a *BaseAgent) RunLive(ctx context.Context, parentContext *InvocationContext) iter.Seq2[*Event, error] {
	return func(yield func(*Event, error) bool) {
		parentContext = a.createInvocationContext(parentContext)
		// TODO(adk-python): support before/after_agent_callback

		for event, err := range a.ExecuteLive(ctx, parentContext) {
			if err != nil {
				xiter.Error[Event](err)
				return
			}
			if !yield(event, nil) {
				return
			}
		}
	}
}

// Execute implements [Agent].
func (a *BaseAgent) Execute(ctx context.Context, ictx *InvocationContext) iter.Seq2[*Event, error] {
	return func(yield func(*Event, error) bool) {
		xiter.Error[Event](NotImplementedError("Execute for Base is not implemented"))
		return
	}
}

// ExecuteLive implements [Agent].
func (a *BaseAgent) ExecuteLive(ctx context.Context, ictx *InvocationContext) iter.Seq2[*Event, error] {
	return func(yield func(*Event, error) bool) {
		xiter.Error[Event](NotImplementedError("ExecuteLive for Base is not implemented"))
		return
	}
}

// RootAgent implements [Agent].
func (a *BaseAgent) RootAgent() Agent {
	rootAgent := Agent(a)
	for {
		parentAgent := rootAgent.ParentAgent()
		if parentAgent == nil {
			break
		}
		rootAgent = parentAgent
	}

	return rootAgent
}

// FindAgent implements [Agent].
func (a *BaseAgent) FindAgent(name string) Agent {
	return a.findAgent(name)
}

// findAgent finds the agent with the given name in this agent and its descendants.
func (a *BaseAgent) findAgent(name string) Agent {
	if name == a.Config.Name {
		return a
	}
	return a.FindSubAgent(name)
}

// FindSubAgent finds the agent with the given name in this agent's descendants.
func (a *BaseAgent) FindSubAgent(name string) Agent {
	for _, subAgent := range a.subAgents {
		if result := subAgent.FindAgent(name); result != nil {
			return result
		}
	}
	return nil
}

// createInvocationContext creates a new invocation context for this agent.
func (a *BaseAgent) createInvocationContext(parentContext *InvocationContext) *InvocationContext {
	parentContext.Agent = a
	if parentContext.Branch != "" {
		parentContext.Branch += "." + a.Name()
	}
	return parentContext
}

// handleBeforeAgentCallback runs the beforeAgentCallbacks if it exists.
func (a *BaseAgent) handleBeforeAgentCallbacks(ctx context.Context, ictx *InvocationContext) (*Event, error) {
	var event *Event

	if len(a.beforeAgentCallbacks) == 0 {
		return event, nil
	}

	callbackCtx := NewCallbackContext(ictx)
	for _, callbacks := range a.beforeAgentCallbacks {
		beforeAgentCallbackContent, err := callbacks(callbackCtx)
		if err != nil {
			a.logger.ErrorContext(ctx, "before callback error", slog.Any("error", err))
			return nil, err
		}
		if beforeAgentCallbackContent != nil {
			event = NewEvent().
				WithInvocationID(ictx.InvocationID).
				WithAuthor(a.Config.Name).
				WithBranch(ictx.Branch).
				WithContent(beforeAgentCallbackContent).
				WithActions(callbackCtx.EventActions())
			ictx.EndInvocation = true
			return event, nil
		}
	}

	if callbackCtx.State().HasDelta() {
		event = NewEvent().
			WithInvocationID(ictx.InvocationID).
			WithAuthor(a.Config.Name).
			WithBranch(ictx.Branch).
			WithActions(callbackCtx.EventActions())
	}

	return event, nil
}

// handleAfterAgentCallback runs the afterAgentCallbacks if it exists.
func (a *BaseAgent) handleAfterAgentCallback(ctx context.Context, ictx *InvocationContext) (*Event, error) {
	var event *Event

	if len(a.afterAgentCallbacks) == 0 {
		return event, nil
	}

	callbackCtx := NewCallbackContext(ictx)
	for _, callbacks := range a.afterAgentCallbacks {
		afterAgentCallbackContent, err := callbacks(callbackCtx)
		if err != nil {
			a.logger.ErrorContext(ctx, "before callback error", slog.Any("error", err))
			return nil, err
		}
		if afterAgentCallbackContent != nil {
			event = NewEvent().
				WithAuthor(a.Config.Name).
				WithBranch(ictx.Branch).
				WithContent(afterAgentCallbackContent).
				WithActions(callbackCtx.EventActions())
			return event, nil
		}

		if callbackCtx.State().HasDelta() {
			event = NewEvent().
				WithInvocationID(ictx.InvocationID).
				WithAuthor(a.Config.Name).
				WithBranch(ictx.Branch).
				WithContent(afterAgentCallbackContent).
				WithActions(callbackCtx.EventActions())
		}
	}

	return event, nil
}
