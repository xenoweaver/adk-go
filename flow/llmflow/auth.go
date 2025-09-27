// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package llmflow

import (
	"context"
	"iter"
	"log/slog"

	"github.com/go-json-experiment/json"

	"github.com/xenoweaver/adk-go/internal/pool"
	"github.com/xenoweaver/adk-go/internal/xiter"
	"github.com/xenoweaver/adk-go/pkg/py"
	"github.com/xenoweaver/adk-go/types"
)

// AuthLLMRequestProcessor represents a handles auth information to build the LLM request.
type AuthLLMRequestProcessor struct {
	logger *slog.Logger
}

var _ types.LLMRequestProcessor = (*AuthLLMRequestProcessor)(nil)

// WithLogger sets the logger for the Preprocessor.
func (p *AuthLLMRequestProcessor) WithLogger(logger *slog.Logger) *AuthLLMRequestProcessor {
	p.logger = logger
	return p
}

// NewAuthPreprocessor creates a new authentication [*AuthLLMRequestProcessor].
func NewAuthPreprocessor() *AuthLLMRequestProcessor {
	return &AuthLLMRequestProcessor{
		logger: slog.Default(),
	}
}

// Run implements [types.LLMRequestProcessor].
func (p *AuthLLMRequestProcessor) Run(ctx context.Context, ictx *types.InvocationContext, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		llmAgent, ok := ictx.Agent.AsLLMAgent()
		if !ok {
			return
		}

		events := ictx.Session.Events()
		if len(events) == 0 {
			return
		}

		requestEucFuncCallIDs := py.NewSet[string]()

		// Look for [flow.RequestEUCFunctionCallName] responses in user events
		for k := len(events) - 1; k >= 0; k-- {
			event := events[k]
			// look for first event authored by user
			if event.Author == "" || event.Author != "user" {
				continue
			}
			responses := event.GetFunctionResponses()
			if len(responses) == 0 {
				return
			}

			for _, funcCallResp := range responses {
				if funcCallResp.Name != RequestEUCFunctionCallName {
					continue
				}
				// found the function call response for the system long running request euc
				// function call
				requestEucFuncCallIDs.Insert(funcCallResp.ID)

				var (
					authConfig *types.AuthConfig
					err        error
				)
				authConfig, err = types.ConvertToAuthConfig(funcCallResp.Response, authConfig)
				if err != nil {
					xiter.Error[types.Event](err)
					return
				}
				authHandler := types.NewAuthHandler(authConfig)

				// Parse and store auth response
				if err := authHandler.ParseAndStoreAuthSesponse(ctx, types.NewState(ictx.Session.State(), nil)); err != nil {
					p.logger.WarnContext(ctx, "Failed to parse auth config", slog.String("function_call_id", funcCallResp.ID), slog.Any("error", err))
					continue
				}
			}
			break
		}

		if requestEucFuncCallIDs.Len() == 0 {
			return
		}

		// Look for the system long running request euc function call
		buf := pool.Buffer.Get()
		for i := len(events) - 2; i >= 0; i-- {
			event := events[i]
			functionCalls := event.GetFunctionCalls()
			if len(functionCalls) == 0 {
				continue
			}

			toolsToResume := py.NewSet[string]()

			for _, functionCall := range functionCalls {
				if !requestEucFuncCallIDs.Has(functionCall.ID) {
					continue
				}

				// Parse auth tool arguments
				var args types.AuthToolArguments
				buf.Reset()
				if err := json.MarshalWrite(buf, functionCall.Args, json.DefaultOptionsV2()); err != nil {
					p.logger.WarnContext(ctx, "Failed to marshal function call args", slog.String("function_call_id", functionCall.ID), slog.Any("error", err))
					continue
				}

				if err := json.UnmarshalRead(buf, &args, json.DefaultOptionsV2()); err != nil {
					p.logger.WarnContext(ctx, "Failed to unmarshal auth tool arguments", slog.String("function_call_id", functionCall.ID), slog.Any("error", err))
					continue
				}

				toolsToResume.Insert(args.FunctionCallID)
			}

			if toolsToResume.Len() == 0 {
				continue
			}

			// Found the system long running request euc function call
			// Look for original function call that requests euc
			for j := i - 1; j >= 0; j-- {
				event := events[j]
				functionCalls := event.GetFunctionCalls()
				if len(functionCalls) == 0 {
					continue
				}

				for _, functionCall := range functionCalls {
					if !toolsToResume.Has(functionCall.ID) {
						continue
					}

					// Get canonical tools for the agent
					rctx := types.NewReadOnlyContext(ictx)
					canonicalTools := llmAgent.CanonicalTool(rctx)

					// Build tools dictionary
					toolsDict := make(map[string]types.Tool)
					for _, tool := range canonicalTools {
						toolsDict[tool.Name()] = tool
					}

					// Handle function calls with auth
					functionResponseEvent, err := HandleFunctionCalls(ctx, ictx, event, toolsDict, toolsToResume)
					if err != nil {
						xiter.EndError[*types.Event](err)
						return
					}

					if functionResponseEvent != nil {
						if !yield(functionResponseEvent, nil) {
							return
						}
					}
					return
				}
			}
			return
		}
	}
}
