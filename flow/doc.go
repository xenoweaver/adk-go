// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

// Package flow provides pipeline architecture and interfaces for processing LLM interactions in sophisticated agent workflows.
//
// The flow package defines the core abstractions and provides implementations for processing
// Large Language Model (LLM) requests and responses through configurable pipelines. It serves
// as the foundation for building sophisticated agent workflows with support for function calling,
// authentication, code execution, agent transfers, and real-time interactions.
//
// # Architecture Overview
//
// The flow package follows a modular architecture with clear separation of concerns:
//
//	┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
//	│                 │────▶│                  │────▶│                 │
//	│  Flow Package   │     │    llmflow/      │     │  llmprocessor/  │
//	│  (Interfaces)   │     │ (Implementation) │     │   (Public API)  │
//	│                 │     │                  │     │                 │
//	└─────────────────┘     └──────────────────┘     └─────────────────┘
//
// # Package Structure
//
// The flow package is organized into three main components:
//
//   - Core interfaces defined in types.Flow
//   - Internal implementation in flow/llmflow subpackage
//   - Public API facade in flow/llmprocessor subpackage
//
// ## Core Interface
//
// The fundamental Flow interface defined in types package:
//
//	type Flow interface {
//		Run(ctx context.Context, ictx *InvocationContext) iter.Seq2[*Event, error]
//		RunLive(ctx context.Context, ictx *InvocationContext) iter.Seq2[*Event, error]
//	}
//
// This interface provides the contract for all flow implementations, supporting both
// standard and live (real-time) execution modes.
//
// ## Implementation Architecture
//
// The flow/llmflow subpackage provides the complete implementation:
//
//   - Pipeline-based request/response processing
//   - Configurable processor chains
//   - Function calling and tool integration
//   - Authentication and credential management
//   - Code execution and result handling
//   - Agent transfer capabilities
//   - Natural language planning
//
// ## Public API Facade
//
// The flow/llmprocessor subpackage provides simplified, high-level interfaces:
//
//   - SingleFlow: For straightforward LLM interactions
//   - AutoFlow: For complex hierarchical agent workflows
//   - Factory methods for common configurations
//   - Clean API hiding implementation complexity
//
// # Usage Patterns
//
// ## Direct Flow Usage
//
// For simple LLM interactions without agent transfers:
//
//	import "github.com/xenoweaver/adk-go/flow/llmprocessor"
//
//	// Create a single flow for basic interactions
//	flow := llmprocessor.NewSingleFlow()
//
//	// Use with an agent
//	agent := agent.NewLLMAgent(ctx, "assistant",
//		agent.WithFlow(flow),
//		agent.WithModel("gemini-1.5-pro"),
//		agent.WithTools(tools...),
//	)
//
// ## Hierarchical Agent Workflows
//
// For complex agent systems with transfers and delegation:
//
//	// Create flows for different agent types
//	coordinatorFlow := llmprocessor.NewAutoFlow()
//	specialistFlow := llmprocessor.NewAutoFlow()
//	workerFlow := llmprocessor.NewSingleFlow()
//
//	// Build agent hierarchy
//	coordinator := agent.NewLLMAgent(ctx, "coordinator",
//		agent.WithFlow(coordinatorFlow))
//	specialist := agent.NewLLMAgent(ctx, "specialist",
//		agent.WithFlow(specialistFlow))
//	worker := agent.NewLLMAgent(ctx, "worker",
//		agent.WithFlow(workerFlow))
//
//	coordinator.WithAgents(specialist, worker)
//
// ## Custom Flow Implementation
//
// For advanced customization, implement the Flow interface directly:
//
//	type CustomFlow struct {
//		// Custom fields
//	}
//
//	func (f *CustomFlow) Run(ctx context.Context, ictx *types.InvocationContext) iter.Seq2[*types.Event, error] {
//		return func(yield func(*types.Event, error) bool) {
//			// Custom flow logic
//		}
//	}
//
//	func (f *CustomFlow) RunLive(ctx context.Context, ictx *types.InvocationContext) iter.Seq2[*types.Event, error] {
//		// Custom live flow logic
//	}
//
// # Key Capabilities
//
// ## Request/Response Processing
//
// Flows process LLM interactions through configurable pipelines:
//
//   - Request processors: Modify requests before sending to LLM
//   - Response processors: Handle responses and execute actions
//   - Streaming support: Real-time event processing
//   - Error handling: Comprehensive error management
//
// ## Function Calling Integration
//
// Sophisticated tool and function calling support:
//
//   - Automatic function declaration generation
//   - Parallel function execution
//   - Result integration into conversation
//   - Error handling and retry logic
//
// ## Authentication Management
//
// Seamless credential and authentication handling:
//
//   - OAuth2, API keys, and custom auth schemes
//   - Secure credential storage
//   - Automatic authentication flows
//   - Tool-specific credential management
//
// ## Code Execution
//
// Secure code execution with multiple backends:
//
//   - Built-in model execution (Gemini 2.0+)
//   - Container-based sandboxing
//   - Local execution (opt-in)
//   - Multiple language support
//   - Result capture and integration
//
// ## Agent Transfer Support
//
// Advanced agent delegation and transfer:
//
//   - Parent/child transfers
//   - Peer agent transfers
//   - Context preservation
//   - Automatic reversal logic
//   - Hierarchy navigation
//
// # Flow Types Comparison
//
// ## SingleFlow (llmprocessor.NewSingleFlow)
//
// Optimized for straightforward interactions:
//
//	✓ Function calling and tool integration
//	✓ Authentication handling
//	✓ Code execution
//	✓ Content processing
//	✓ Natural language planning
//	✗ Agent transfers
//	✗ Hierarchical delegation
//
//	Best for: Standalone agents, direct user interaction, simple workflows
//
// ## AutoFlow (llmprocessor.NewAutoFlow)
//
// Full-featured for complex workflows:
//
//	✓ All SingleFlow capabilities
//	✓ Parent/child agent transfers
//	✓ Peer agent transfers
//	✓ Hierarchical delegation
//	✓ Sophisticated workflow routing
//	✓ Context-aware agent selection
//
//	Best for: Multi-agent systems, complex workflows, specialist coordination
//
// # Performance Characteristics
//
// The flow architecture is designed for high performance:
//
//   - Streaming processing: Real-time event handling
//   - Parallel execution: Concurrent function calls
//   - Connection pooling: Efficient HTTP client management
//   - Memory optimization: Object pooling and reuse
//   - Pipeline efficiency: Optimized processor ordering
//
// # Integration with Agent Framework
//
// Flows integrate seamlessly with the agent system:
//
//	// Automatic flow selection based on agent configuration
//	agent := agent.NewLLMAgent(ctx, "name",
//		agent.WithModel("model"),
//		// Flow is automatically chosen based on agent features
//	)
//
//	// Explicit flow specification
//	agent := agent.NewLLMAgent(ctx, "name",
//		agent.WithFlow(llmprocessor.NewAutoFlow()),
//		agent.WithModel("model"),
//	)
//
// # Real-time and Live Connections
//
// Support for real-time interactions with capable models:
//
//	// Live mode for real-time audio/video interactions
//	for event, err := range flow.RunLive(ctx, ictx) {
//		if err != nil {
//			continue
//		}
//
//		switch event.Type {
//		case types.EventTypeAudioDelta:
//			// Handle real-time audio
//		case types.EventTypeTextDelta:
//			// Handle streaming text
//		}
//	}
//
// # Error Handling Strategy
//
// Comprehensive error handling at all levels:
//
//	for event, err := range flow.Run(ctx, ictx) {
//		if err != nil {
//			// Flow-level error handling
//			switch e := err.(type) {
//			case *types.RateLimitError:
//				// Rate limiting
//			case *types.AuthenticationError:
//				// Authentication failure
//			case *types.ExecutionError:
//				// Code execution failure
//			}
//			continue
//		}
//
//		// Process successful events
//	}
//
// # Thread Safety and Concurrency
//
// All flow implementations are designed for concurrent use:
//
//   - Thread-safe execution across multiple goroutines
//   - Proper context propagation and cancellation
//   - Isolated execution contexts for concurrent requests
//   - Safe state management with proper synchronization
//
// # Best Practices
//
//  1. Use llmprocessor.NewSingleFlow() for simple, direct interactions
//  2. Use llmprocessor.NewAutoFlow() for hierarchical agent systems
//  3. Implement custom flows only when standard flows are insufficient
//  4. Handle streaming events promptly to avoid blocking
//  5. Use appropriate error handling for different error types
//  6. Consider performance implications of flow choice
//  7. Test agent transfer scenarios thoroughly
//  8. Monitor resource usage in high-throughput scenarios
//
// # Subpackage Guide
//
// ## flow/llmflow
//
// Internal implementation package providing:
//   - Complete processor pipeline implementation
//   - Individual processor components
//   - Advanced customization capabilities
//   - Low-level flow control
//
// Use when: Building custom flows, deep customization needed
//
// ## flow/llmprocessor
//
// Public API facade providing:
//   - Simplified high-level interfaces
//   - Common configuration patterns
//   - Easy-to-use factory methods
//   - Stable public API
//
// Use when: Standard use cases, simple integration, stable API needed
//
// # Migration and Evolution
//
// The flow package is designed for API stability and easy migration:
//
//	// Legacy approach (still supported)
//	flow := &llmflow.LLMFlow{
//		RequestProcessors: processors,
//	}
//
//	// Modern approach (recommended)
//	flow := llmprocessor.NewSingleFlow()
//
// # Integration Examples
//
// ## Research Assistant
//
//	researchFlow := llmprocessor.NewSingleFlow()
//	researcher := agent.NewLLMAgent(ctx, "researcher",
//		agent.WithFlow(researchFlow),
//		agent.WithTools(searchTool, analysisTool),
//	)
//
// ## Customer Service System
//
//	// Multi-tier support with escalation
//	tier1Flow := llmprocessor.NewSingleFlow()
//	escalationFlow := llmprocessor.NewAutoFlow()
//
//	tier1 := agent.NewLLMAgent(ctx, "tier1", agent.WithFlow(tier1Flow))
//	supervisor := agent.NewLLMAgent(ctx, "supervisor", agent.WithFlow(escalationFlow))
//	supervisor.WithAgents(tier1)
//
// ## Data Processing Pipeline
//
//	// Coordinator delegates to specialists
//	coordinatorFlow := llmprocessor.NewAutoFlow()
//	processorFlow := llmprocessor.NewSingleFlow()
//
//	coordinator := agent.NewLLMAgent(ctx, "coordinator", agent.WithFlow(coordinatorFlow))
//	dataProcessor := agent.NewLLMAgent(ctx, "processor", agent.WithFlow(processorFlow))
//	coordinator.WithAgents(dataProcessor)
//
// The flow package provides the essential infrastructure for building sophisticated
// AI agent workflows with comprehensive LLM integration capabilities.
package flow
