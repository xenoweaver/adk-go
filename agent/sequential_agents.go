// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"iter"
	"reflect"
	"runtime"
	"strings"

	"github.com/xenoweaver/adk-go/types"
)

// SequentialAgent represents a shell agent that run its sub-agents in sequence.
type SequentialAgent struct {
	base *types.BaseAgent

	agents []types.Agent
}

var _ types.Agent = (*SequentialAgent)(nil)

// AsLLMAgent implements [types.Agent].
func (a *SequentialAgent) AsLLMAgent() (types.LLMAgent, bool) {
	return nil, false
}

// WithAgents sets the agents for the sequential agent.
func (a *SequentialAgent) WithAgents(agents ...types.Agent) *SequentialAgent {
	a.agents = agents
	return a
}

// NewSequentialAgent creates a new sequential agent with the given name and options.
func NewSequentialAgent(name string) *SequentialAgent {
	return &SequentialAgent{
		base: types.NewBaseAgent(name),
	}
}

// Name implements [types.Agent].
func (a *SequentialAgent) Name() string {
	return a.base.Name()
}

// Description implements [types.Agent].
func (a *SequentialAgent) Description() string {
	return a.base.Description()
}

// ParentAgent implements [types.Agent].
func (a *SequentialAgent) ParentAgent() types.Agent {
	return a.base.ParentAgent()
}

// SubAgents implements [types.Agent].
func (a *SequentialAgent) SubAgents() []types.Agent {
	return a.base.SubAgents()
}

// BeforeAgentCallbacks implements [types.Agent].
func (a *SequentialAgent) BeforeAgentCallbacks() []types.AgentCallback {
	return a.base.BeforeAgentCallbacks()
}

// AfterAgentCallbacks implements [types.Agent].
func (a *SequentialAgent) AfterAgentCallbacks() []types.AgentCallback {
	return a.base.AfterAgentCallbacks()
}

// Execute implements [types.Agent].
func (a *SequentialAgent) Execute(ctx context.Context, ictx *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		for _, subAgent := range a.base.SubAgents() {
			for event, err := range subAgent.Run(ctx, ictx) {
				if !yield(event, err) {
					return
				}
			}
		}
	}
}

// taskCompleted signals that the model has successfully completed the user's question
// or task.
func taskCompleted() string {
	return "Task completion signaled."
}

func getFunctionName(i any) string {
	funcName := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	if idx := strings.LastIndex(funcName, "."); idx > -1 {
		funcName = funcName[idx+1:]
	}
	return funcName
}

// ExecuteLive implements [types.Agent].
//
// ExecuteLive implementation for live SequentialAgent.
//
// Compared to non-live case, live agents process a continous streams of audio
// or video, so it doesn't have a way to tell if it's finished and should pass
// to next agent or not. So we introduce a task_compelted() function so the
// model can call this function to signal that it's finished the task and we
// can move on to next agent.
func (a *SequentialAgent) ExecuteLive(ctx context.Context, ictx *types.InvocationContext) iter.Seq2[*types.Event, error] {
	taskCompletedName := getFunctionName(taskCompleted)

	return func(yield func(*types.Event, error) bool) {
		for _, subAgent := range a.base.SubAgents() {
			llmAgent, ok := subAgent.(*LLMAgent)
			if ok {
				for _, t := range llmAgent.tools {
					if tt, ok := t.(func()); ok && getFunctionName(tt) != taskCompletedName {
						llmAgent.tools = append(llmAgent.tools, taskCompleted)
						llmAgent.instruction = `If you finished the user' request
          according to its description, call ` + taskCompletedName + `function
          to exit so the next agents can take over. When calling this function,
          do not generate any text other than the function call.`
					}
				}
			}
		}

		for _, subAgent := range a.base.SubAgents() {
			for event, err := range subAgent.RunLive(ctx, ictx) {
				if !yield(event, err) {
					return
				}
			}
		}
	}
}

// Run implements [types.Agent].
func (a *SequentialAgent) Run(ctx context.Context, parentContext *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return a.base.Run(ctx, parentContext)
}

// RunLive implements [types.Agent].
func (a *SequentialAgent) RunLive(ctx context.Context, parentContext *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return a.base.RunLive(ctx, parentContext)
}

// RootAgent implements [types.Agent].
func (a *SequentialAgent) RootAgent() types.Agent {
	return a.base.RootAgent()
}

// FindAgent implements [types.Agent].
func (a *SequentialAgent) FindAgent(name string) types.Agent {
	return a.base.FindAgent(name)
}

// FindSubAgent implements [types.Agent].
func (a *SequentialAgent) FindSubAgent(name string) types.Agent {
	return a.base.FindSubAgent(name)
}
