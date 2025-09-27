// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"iter"
	"sync"

	"github.com/xenoweaver/adk-go/internal/xiter"
	"github.com/xenoweaver/adk-go/types"
)

// A shell agent that run its sub-agents in parallel in isolated manner.
//
// This approach is beneficial for scenarios requiring multiple perspectives or
// attempts on a single task, such as:
//
//   - Running different algorithms simultaneously.
//   - Generating multiple responses for review by a subsequent evaluation agent.
type ParallelAgent struct {
	base *types.BaseAgent
}

var _ types.Agent = (*ParallelAgent)(nil)

// AsLLMAgent implements [types.Agent].
func (a *ParallelAgent) AsLLMAgent() (types.LLMAgent, bool) {
	return nil, false
}

// NewParallelAgent creates a new parallel agent with the given name and options.
func NewParallelAgent(name string, agents ...types.Agent) *ParallelAgent {
	return &ParallelAgent{
		base: types.NewBaseAgent(name, types.WithSubAgents(agents...)),
	}
}

// Name implements [types.Agent].
func (a *ParallelAgent) Name() string {
	return a.base.Name()
}

// Description implements [types.Agent].
func (a *ParallelAgent) Description() string {
	return a.base.Description()
}

// ParentAgent implements [types.Agent].
func (a *ParallelAgent) ParentAgent() types.Agent {
	return a.base.ParentAgent()
}

// SubAgents implements [types.Agent].
func (a *ParallelAgent) SubAgents() []types.Agent {
	return a.base.SubAgents()
}

// BeforeAgentCallbacks implements [types.Agent].
func (a *ParallelAgent) BeforeAgentCallbacks() []types.AgentCallback {
	return a.base.BeforeAgentCallbacks()
}

// AfterAgentCallbacks implements [types.Agent].
func (a *ParallelAgent) AfterAgentCallbacks() []types.AgentCallback {
	return a.base.AfterAgentCallbacks()
}

// Execute implements [types.Agent].
func (a *ParallelAgent) Execute(ctx context.Context, ictx *types.InvocationContext) iter.Seq2[*types.Event, error] {
	ictx = a.setBranchForCurrentAgent(a, ictx)

	agentRuns := make([]iter.Seq2[*types.Event, error], len(a.base.SubAgents()))
	for i, subAgent := range a.base.SubAgents() {
		agentRuns[i] = subAgent.Run(ctx, ictx)
	}

	return func(yield func(*types.Event, error) bool) {
		for event, err := range MergeAgentRun(ctx, agentRuns) {
			if !yield(event, err) {
				return
			}
		}
	}
}

// ExecuteLive implements [types.Agent].
func (a *ParallelAgent) ExecuteLive(ctx context.Context, ictx *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return xiter.EndError[types.Event](types.NotImplementedError("this is not supported yet for ParallelAgent"))
}

// Run implements [types.Agent].
func (a *ParallelAgent) Run(ctx context.Context, parentContext *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return a.base.Run(ctx, parentContext)
}

// RunLive implements [types.Agent].
func (a *ParallelAgent) RunLive(ctx context.Context, parentContext *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return a.base.RunLive(ctx, parentContext)
}

// RootAgent implements [types.Agent].
func (a *ParallelAgent) RootAgent() types.Agent {
	return a.base.RootAgent()
}

// FindAgent implements [types.Agent].
func (a *ParallelAgent) FindAgent(name string) types.Agent {
	return a.base.FindAgent(name)
}

// FindSubAgent implements [types.Agent].
func (a *ParallelAgent) FindSubAgent(name string) types.Agent {
	return a.base.FindSubAgent(name)
}

func (a *ParallelAgent) setBranchForCurrentAgent(currentAgent types.Agent, ictx *types.InvocationContext) *types.InvocationContext {
	if ictx.Branch != "" {
		ictx.Branch = ictx.Branch + "." + currentAgent.Name()
		return ictx
	}

	ictx.Branch = currentAgent.Name()
	return ictx
}

// eventResult holds an event result from an agent with metadata.
type eventResult struct {
	event   *types.Event
	err     error
	agentID int
}

// MergeAgentRun merges the agent run event generator.
//
// This implementation guarantees for each agent, it won't move on until the
// generated event is processed by upstream runner.
func MergeAgentRun(ctx context.Context, agentRuns []iter.Seq2[*types.Event, error]) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		// Handle empty case
		if len(agentRuns) == 0 {
			return
		}

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		eventCh := make(chan eventResult)
		wg := new(sync.WaitGroup)

		// Start goroutine for each agent
		for i, agentRun := range agentRuns {
			wg.Add(1)
			go func(agentID int, run iter.Seq2[*types.Event, error]) {
				defer wg.Done()
				for event, err := range run {
					select {
					case eventCh <- eventResult{
						event:   event,
						err:     err,
						agentID: agentID,
					}:
					case <-ctx.Done():
						return
					}
				}
			}(i, agentRun)
		}

		// Close eventCh when all agents complete
		go func() {
			wg.Wait()
			close(eventCh)
		}()

		// Yield events as they arrive
		for result := range eventCh {
			if !yield(result.event, result.err) {
				return // Consumer stopped - context cancellation will stop agents
			}
		}
	}
}
