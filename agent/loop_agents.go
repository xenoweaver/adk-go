// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"iter"

	"github.com/xenoweaver/adk-go/internal/xiter"
	"github.com/xenoweaver/adk-go/types"
)

// LoopAgent runs an agent repeatedly until a condition is met.
type LoopAgent struct {
	base *types.BaseAgent

	// The maximum number of iterations to run the loop agent.
	//
	// If not set, the loop agent will run indefinitely until a sub-agent
	// escalates.
	maxIterations int
}

var _ types.Agent = (*LoopAgent)(nil)

// AsLLMAgent implements [types.Agent].
func (a *LoopAgent) AsLLMAgent() (types.LLMAgent, bool) {
	return nil, false
}

// WithMaxIterations sets the maximum number of iterations.
func (a *LoopAgent) WithMaxIterations(maxIterations int) *LoopAgent {
	a.maxIterations = maxIterations
	return a
}

// NewLoopAgent creates a new loop agent with the given name and options.
func NewLoopAgent(name string) *LoopAgent {
	a := &LoopAgent{
		base:          types.NewBaseAgent(name),
		maxIterations: 10, // Default
	}

	return a
}

// Name implements [types.Agent].
func (a *LoopAgent) Name() string {
	return a.base.Name()
}

// Description implements [types.Agent].
func (a *LoopAgent) Description() string {
	return a.base.Description()
}

// ParentAgent implements [types.Agent].
func (a *LoopAgent) ParentAgent() types.Agent {
	return a.base.ParentAgent()
}

// SubAgents implements [types.Agent].
func (a *LoopAgent) SubAgents() []types.Agent {
	return a.base.SubAgents()
}

// BeforeAgentCallbacks implements [types.Agent].
func (a *LoopAgent) BeforeAgentCallbacks() []types.AgentCallback {
	return a.base.BeforeAgentCallbacks()
}

// AfterAgentCallbacks implements [types.Agent].
func (a *LoopAgent) AfterAgentCallbacks() []types.AgentCallback {
	return a.base.AfterAgentCallbacks()
}

// Execute implements [types.Agent].
func (a *LoopAgent) Execute(ctx context.Context, ictx *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		timesLooped := 0
		for a.maxIterations == 0 || timesLooped < a.maxIterations {
			for _, subAgent := range a.base.SubAgents() {
				for event, err := range subAgent.Run(ctx, ictx) {
					if err != nil {
						xiter.Error[types.Event](err)
						return
					}
					if !yield(event, nil) {
						return
					}

					if event.Actions.Escalate {
						return
					}
				}
				timesLooped++
			}
		}
	}
}

// ExecuteLive implements [types.Agent].
func (a *LoopAgent) ExecuteLive(ctx context.Context, ictx *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		xiter.Error[types.Event](types.NotImplementedError("ExecuteLive not supported yet for LoopAgent"))
	}
}

// Run implements [types.Agent].
func (a *LoopAgent) Run(ctx context.Context, parentContext *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return a.base.Run(ctx, parentContext)
}

// RunLive implements [types.Agent].
func (a *LoopAgent) RunLive(ctx context.Context, parentContext *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return a.base.RunLive(ctx, parentContext)
}

// RootAgent implements [types.Agent].
func (a *LoopAgent) RootAgent() types.Agent {
	return a.base.RootAgent()
}

// FindAgent implements [types.Agent].
func (a *LoopAgent) FindAgent(name string) types.Agent {
	return a.base.FindAgent(name)
}

// FindSubAgent implements [types.Agent].
func (a *LoopAgent) FindSubAgent(name string) types.Agent {
	return a.base.FindSubAgent(name)
}
