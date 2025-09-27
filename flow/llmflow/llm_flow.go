// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package llmflow

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"runtime"
	"time"

	"github.com/xenoweaver/adk-go/internal/xiter"
	"github.com/xenoweaver/adk-go/model"
	"github.com/xenoweaver/adk-go/pkg/py"
	"github.com/xenoweaver/adk-go/pkg/py/pyasyncio"
	"github.com/xenoweaver/adk-go/types"
)

// LLMFlow represents a base flow that calls the LLM in a loop until a final response is generated.
//
// This flow ends when it transfer to another agent.
type LLMFlow struct {
	RequestProcessors  []types.LLMRequestProcessor
	ResponseProcessors []types.LLMResponseProcessor
	Logger             *slog.Logger
}

var _ types.Flow = (*LLMFlow)(nil)

// WithLogger returns an option that sets the logger for a flow.
func (f *LLMFlow) WithLogger(logger *slog.Logger) *LLMFlow {
	f.Logger = logger.With("flow", "LLMFlow")
	return f
}

// WithRequestProcessors adds a request processor to the [LLMFlow].
func (f *LLMFlow) WithRequestProcessors(processors ...types.LLMRequestProcessor) *LLMFlow {
	f.RequestProcessors = append(f.RequestProcessors, processors...)
	return f
}

// WithResponseProcessors adds a response processor to the [LLMFlow].
func (f *LLMFlow) WithResponseProcessors(processors ...types.LLMResponseProcessor) *LLMFlow {
	f.ResponseProcessors = append(f.ResponseProcessors, processors...)
	return f
}

// NewLLMFlow creates a new [LLMFlow] with the given model and options.
func NewLLMFlow() *LLMFlow {
	return &LLMFlow{
		Logger: slog.Default().With("flow", "LLMFlow"),
	}
}

// RunLive implements [Flow].
//
// TODO(zchee): support OTel tracing.
func (f *LLMFlow) RunLive(ctx context.Context, ictx *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		request := &types.LLMRequest{}
		eventSeq := f.preprocess(ctx, ictx, request)
		for event, err := range eventSeq {
			if err != nil {
				xiter.Error[types.Event](err)
			}

			if !yield(event, nil) {
				return
			}
			if ictx.EndInvocation {
				return
			}
		}

		llm := f.getLLM(ctx, ictx)
		conn, err := llm.Connect(ctx, request)
		if err != nil {
			xiter.Error[types.Event](err)
			return
		}
		if len(request.Contents) > 0 {
			switch {
			case len(ictx.TranscriptionCache) > 0:
				// from . import audio_transcriber
				//
				// audio_transcriber = audio_transcriber.AudioTranscriber()
				// contents = audio_transcriber.transcribe_file(invocation_context)
				// logger.debug('Sending history to model: %s', contents)
				// await llm_connection.send_history(contents)
				// invocation_context.transcription_cache = None
				// trace_send_data(invocation_context, event_id, contents)
			default:
				if err := conn.SendHistory(ctx, request.Contents); err != nil {
					xiter.Error[types.Event](err)
					return
				}
			}
		}

		fn := func(ctx context.Context) (any, error) {
			if err := f.sendToModel(ctx, conn, ictx); err != nil {
				return nil, err
			}
			return nil, nil
		}
		sendTask := pyasyncio.CreateTask[any](ctx, fn)

		for event, err := range f.receiveFromModel(ctx, conn, ictx, request) {
			if err != nil {
				xiter.Error[types.Event](err)
				return
			}
			// Empty event means the queue is closed.
			if event == nil {
				break
			}

			f.Logger.DebugContext(ctx, "receive new event", slog.Any("event", event))
			if !yield(event, nil) {
				return
			}

			// send back the function response
			if len(event.GetFunctionResponses()) > 0 {
				f.Logger.DebugContext(ctx, "Sending back last function response event", slog.Any("event", event))
				ictx.LiveRequestQueue.SendContent(event.Content)
			}

			if event.Content != nil && len(event.Content.Parts) > 0 && event.Content.Parts[0].FunctionResponse != nil {
				switch {
				case event.Content.Parts[0].FunctionResponse.Name == "transfer_to_agent":
					// mimic Python `await asyncio.sleep(1)`
					select {
					case <-ctx.Done():
						xiter.Error[types.Event](ctx.Err())
						return
					case <-time.After(time.Second):
						xiter.Error[types.Event](pyasyncio.NewTaskCancelledError("timeout"))
						return
					default:
						runtime.Gosched()
					}

					// cancel the tasks that belongs to the closed connection.
					sendTask.Cancel()
					if err := conn.Close(); err != nil {
						xiter.Error[types.Event](err)
						return
					}

				case event.Content.Parts[0].FunctionResponse.Name == "task_completed":
					// this is used for sequential agent to signal the end of the agent.
					// mimic Python `await asyncio.sleep(1)`
					select {
					case <-ctx.Done():
						xiter.Error[types.Event](ctx.Err())
						return
					case <-time.After(time.Second):
						xiter.Error[types.Event](pyasyncio.NewTaskCancelledError("timeout"))
						return
					default:
						runtime.Gosched()
					}

					// cancel the tasks that belongs to the closed connection.
					sendTask.Cancel()
					return
				}
			}
		}

		if !sendTask.Done() {
			sendTask.Cancel()
		}
		_, err = sendTask.Wait(ctx)
		if err != nil {
			return
		}
	}
}

// sendToModel sends data to model.
func (f *LLMFlow) sendToModel(ctx context.Context, connection types.ModelConnection, ic *types.InvocationContext) error {
	for {
		liveRequestQueue := ic.LiveRequestQueue

		// Streamlit's execution model doesn't preemptively yield to the event
		// loop. Therefore, we must explicitly introduce timeouts to allow the
		// event loop to process events.
		// TODO(adk-python): revert back(remove timeout) once we move off streamlit.
		liveRequest, err := liveRequestQueue.Get(ctx)               // TODO(zchee): support 250*time.Millisecond)
		if err != nil && errors.Is(err, context.DeadlineExceeded) { // NOTE(zchee): mimic Python `asyncio.TimeoutError`
			continue
		}

		// duplicate the live_request to all the active streams
		f.Logger.DebugContext(ctx,
			"sending live request %s to active streams",
			slog.Any("live_request", liveRequest),
			slog.Any("invocation_context.active_streaming_tools", ic.ActiveStreamingTools),
		)

		if len(ic.ActiveStreamingTools) > 0 {
			for v := range maps.Values(ic.ActiveStreamingTools) {
				if v.Stream != nil {
					v.Stream.Send(liveRequest)
				}
			}
		}

		// mimic Python `await asyncio.sleep(0)`
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			runtime.Gosched()
		}

		if liveRequest.Close {
			if err := connection.Close(); err != nil {
				return fmt.Errorf("close llm connection: %w", err)
			}
			break
		}
		if liveRequest.Blob != nil {
			if ic.RunConfig.InputAudioTranscription == nil {
				ic.TranscriptionCache = append(ic.TranscriptionCache, types.NewTranscriptionEntry(model.RoleUser, liveRequest.Blob))
			}

			if err := connection.SendRealtime(ctx, liveRequest.Blob.Data, liveRequest.Blob.MIMEType); err != nil {
				return fmt.Errorf("send realtime data: %w", err)
			}
		}

		if err := connection.SendContent(ctx, liveRequest.Content); err != nil {
			return fmt.Errorf("send content data: %w", err)
		}
	}

	return nil
}

// receiveFromModel receive data from model and process events using [types.ModelConnection].
func (f *LLMFlow) receiveFromModel(ctx context.Context, connection types.ModelConnection, ic *types.InvocationContext, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	// getAuthorForEvent gets the author of the event.
	getAuthorForEvent := func(response *types.LLMResponse) string {
		// When the model returns transcription, the author is "user". Otherwise, the
		// author is the agent.
		if response != nil && response.Content != nil && response.Content.Role == model.RoleUser {
			return model.RoleUser
		}

		return ic.Agent.Name()
	}

	return func(yield func(*types.Event, error) bool) {
		if ic.LiveRequestQueue == nil {
			xiter.Error[types.Event](errors.New("must be LiveRequestQueue field is non-nil"))
			return
		}

		for {
			for resp, err := range connection.Receive(ctx) {
				if err != nil {
					xiter.Error[types.Event](errors.New("must be LiveRequestQueue field is non-nil"))
				}

				modelRespEvent := types.NewEvent().
					WithInvocationID(ic.InvocationID).
					WithAuthor(getAuthorForEvent(resp))

				for event, err := range f.postProcessLive(ctx, ic, request, resp, modelRespEvent) {
					if err != nil {
						xiter.EndError[types.Event](err)
					}

					if event.Content != nil && len(event.Content.Parts) > 0 && event.Content.Parts[0].InlineData == nil && !event.Partial {
						ic.TranscriptionCache = append(ic.TranscriptionCache, types.NewTranscriptionEntry(event.Content.Role, event.Content))
					}

					if !yield(event, nil) {
						return
					}
				}
			}
			// Give opportunity for other tasks to run.
			// mimic Python `await asyncio.sleep(0)`
			select {
			case <-ctx.Done():
				xiter.Error[types.Event](ctx.Err())
				return
			default:
				runtime.Gosched()
			}
		}
	}
}

// Run implements [Flow].
func (f *LLMFlow) Run(ctx context.Context, ic *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		for {
			var lastEvent *types.Event
			for event, err := range f.runOneStep(ctx, ic) {
				if err != nil {
					xiter.EndError[types.Event](err)
					return
				}
				lastEvent = event
				if !yield(event, nil) {
					return
				}
			}
			if lastEvent == nil || lastEvent.IsFinalResponse() {
				break
			}
		}
	}
}

// runOneStepAsync one step means one LLM call.
func (f *LLMFlow) runOneStep(ctx context.Context, ic *types.InvocationContext) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		request := &types.LLMRequest{}

		// Preprocess before calling the LLM.
		eventSeq := f.preprocess(ctx, ic, request)
		for event, err := range eventSeq {
			if !yield(event, err) {
				return
			}
		}
		if ic.EndInvocation {
			return
		}

		// Calls the LLM.
		modelResponseEvent := types.NewEvent()
		modelResponseEvent.InvocationID = types.NewEventID()
		modelResponseEvent.Author = ic.Agent.Name()
		modelResponseEvent.Branch = ic.Branch

		// TODO(zchee): implements
		// async for llm_response in self._call_llm_async(
		//     invocation_context, llm_request, model_response_event
		// ):
		//   # Postprocess after calling the LLM.
		//   async for event in self._postprocess_async(
		//       invocation_context, llm_request, llm_response, model_response_event
		//   ):
		//     # Update the mutable event id to avoid conflict
		//     model_response_event.id = Event.new_id()
		//     yield event
	}
}

func (f *LLMFlow) preprocess(ctx context.Context, ic *types.InvocationContext, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		llmAgent, ok := ic.Agent.AsLLMAgent()
		if !ok {
			return
		}

		// Runs processors.
		for _, processor := range f.RequestProcessors {
			eventSeq := processor.Run(ctx, ic, request)
			for event, err := range eventSeq {
				if err != nil {
					yield(nil, err)
					return
				}
				if !yield(event, nil) {
					return
				}
			}
		}

		// Run processors for tools.
		for _, tool := range llmAgent.CanonicalTool(types.NewReadOnlyContext(ic)) {
			toolCtx := types.NewToolContext(ic)
			tool.ProcessLLMRequest(ctx, toolCtx, request)
		}
	}
}

// postprocess after calling the LLM.
func (f *LLMFlow) postProcess(ctx context.Context, ic *types.InvocationContext, request *types.LLMRequest, response *types.LLMResponse, modelRespEvent *types.Event) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		// Runs processors.
		for event, err := range f.postProcessRunProcessors(ctx, ic, response) {
			if err != nil {
				xiter.Error[types.Event](err)
				return
			}

			if !yield(event, nil) {
				return
			}

			if response != nil && response.ErrorCode == "" && !response.Interrupted {
				return
			}

			// Builds the event.
			modelResponseEvent := f.finalizeModelResponseEvent(ctx, request, response, modelRespEvent)
			if !yield(modelResponseEvent, nil) {
				return
			}

			// Handles function calls.
			if len(modelResponseEvent.GetFunctionCalls()) > 0 {
				for event, err := range f.postprocessHandleFunctionCalls(ctx, ic, modelResponseEvent, request) {
					if err != nil {
						xiter.Error[types.Event](err)
						return
					}
					if !yield(event, nil) {
						return
					}
				}
			}
		}
	}
}

// postProcessLive postprocess after calling the LLM asynchronously.
func (f *LLMFlow) postProcessLive(ctx context.Context, ic *types.InvocationContext, request *types.LLMRequest, response *types.LLMResponse, modelRespEvent *types.Event) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		// Runs processors
		for event, err := range f.postProcessRunProcessors(ctx, ic, response) {
			if err != nil {
				xiter.Error[types.Event](err)
				return
			}
			if !yield(event, nil) {
				return
			}
		}

		// Skip the model response event if there is no content and no error code.
		// This is needed for the code executor to trigger another loop.
		// But don't skip control events like turn_complete.
		if response.Content == nil && response.ErrorCode != "" && !response.Interrupted && !response.TurnComplete {
			return
		}

		// Builds the event.
		modelResponseEvent := f.finalizeModelResponseEvent(ctx, request, response, modelRespEvent)
		if !yield(modelResponseEvent, nil) {
			return
		}

		// Handles function calls.
		if len(modelResponseEvent.GetFunctionCalls()) > 0 {
			funcResponseEvent, err := HandleFunctionCallsLive(ctx, ic, modelResponseEvent, request.ToolMap)
			if err != nil {
				xiter.Error[types.Event](err)
				return
			}
			if !yield(funcResponseEvent, nil) {
				return
			}

			transferToAgent := funcResponseEvent.Actions.TransferToAgent
			if transferToAgent != "" {
				agentToRun, err := f.getAgentToRun(ctx, ic, transferToAgent)
				if err != nil {
					xiter.Error[types.Event](err)
					return
				}
				for event, err := range agentToRun.RunLive(ctx, ic) {
					if !yield(event, err) {
						return
					}
				}
			}
		}
	}
}

func (f *LLMFlow) postProcessRunProcessors(ctx context.Context, ic *types.InvocationContext, response *types.LLMResponse) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		for _, processor := range f.ResponseProcessors {
			for event, err := range processor.Run(ctx, ic, response) {
				if err != nil {
					xiter.EndError[types.Event](errors.New("must be LiveRequestQueue field is non-nil"))
				}
				if !yield(event, nil) {
					return
				}
			}
		}
	}
}

func (f *LLMFlow) postprocessHandleFunctionCalls(ctx context.Context, ic *types.InvocationContext, funcCallEvent *types.Event, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		funcResponseEvent, err := HandleFunctionCalls(ctx, ic, funcCallEvent, request.ToolMap, py.Set[string]{})
		if err != nil {
			xiter.Error[types.Event](err)
			return
		}

		authEvent, err := GenerateAuthEvent(ctx, ic, funcResponseEvent)
		if err != nil {
			xiter.Error[types.Event](err)
			return
		}
		if authEvent != nil {
			if !yield(authEvent, nil) {
				return
			}
		}

		if !yield(funcResponseEvent, nil) {
			return
		}

		transferToAgent := funcResponseEvent.Actions.TransferToAgent
		if transferToAgent != "" {
			agentToRun, err := f.getAgentToRun(ctx, ic, transferToAgent)
			if err != nil {
				xiter.Error[*types.ModelConnection](err)
				return
			}
			for event, err := range agentToRun.Run(ctx, ic) {
				if !yield(event, err) {
					return
				}
			}
		}
	}
}

func (f *LLMFlow) getAgentToRun(ctx context.Context, ic *types.InvocationContext, transferToAgent string) (types.Agent, error) {
	rootAgent := ic.Agent.RootAgent()
	agentToRun := rootAgent.FindAgent(transferToAgent)
	if agentToRun == nil {
		return nil, fmt.Errorf("agent %s not found in the agent tree", transferToAgent)
	}
	return agentToRun, nil
}

func (f *LLMFlow) callLLM(ctx context.Context, ic *types.InvocationContext, request *types.LLMRequest, modelResponseEvent *types.Event) iter.Seq2[*types.LLMResponse, error] {
	return func(yield func(*types.LLMResponse, error) bool) {
		// Runs before_model_callback if it exists
		response, err := f.handleBeforeModelCallback(ctx, ic, request, modelResponseEvent)
		if err != nil {
			if !yield(nil, err) {
				return
			}
		}
		if response != nil {
			if !yield(response, nil) {
				return
			}
		}

		// Calls the LLM.
		if ic.RunConfig.SupportCFC {
			ic.LiveRequestQueue = types.NewLiveRequestQueue()
			eventSeq := f.RunLive(ctx, ic)
			// Runs after_model_callback if it exists.
			for llmRespEvent, err := range eventSeq {
				if err != nil {
					if !yield(nil, err) {
						return
					}
				}
				alterResponse, err := f.handleAfterModelCallback(ctx, ic, response, modelResponseEvent)
				if err == nil && alterResponse != nil {
					response = alterResponse
				}

				// only yield partial response in SSE streaming mode
				if ic.RunConfig.StreamingMode == types.StreamingModeSSE || !llmRespEvent.Partial {
					// TODO(zchee): return llmRespEvent?
					yield(llmRespEvent.LLMResponse, nil)
				}

				if llmRespEvent.TurnComplete {
					ic.LiveRequestQueue.Close()
				}
			}
		} else {
			// Check if we can make this llm call or not. If the current call pushes
			// the counter beyond the max set value, then the execution is stopped
			// right here, and exception is thrown.
			ic.IncrementLLMCallCount()

			llm := f.getLLM(ctx, ic)
			isStream := ic.RunConfig.StreamingMode == types.StreamingModeSSE
			if isStream {
				respSeq := llm.StreamGenerateContent(ctx, request)
				for response, err := range respSeq {
					if err != nil {
						if !yield(nil, err) {
							return
						}
					}

					// Runs after_model_callback if it exists.
					alterResponse, err := f.handleAfterModelCallback(ctx, ic, response, modelResponseEvent)
					if err == nil && alterResponse != nil {
						response = alterResponse
					}
					if !yield(response, nil) {
						return
					}
				}
			}
		}
	}
}

// handleBeforeModelCallback processes callbacks that should run before the model has generated a response.
func (f *LLMFlow) handleBeforeModelCallback(ctx context.Context, ic *types.InvocationContext, request *types.LLMRequest, modelResponseEvent *types.Event) (*types.LLMResponse, error) {
	llmAgent, ok := ic.Agent.AsLLMAgent()
	if !ok {
		return nil, nil
	}

	if len(llmAgent.BeforeModelCallbacks()) == 0 {
		return nil, nil
	}

	cc := types.NewCallbackContext(ic).WithEventActions(modelResponseEvent.Actions)
	for _, callback := range llmAgent.BeforeModelCallbacks() {
		beforeModelCallbackContent, err := callback(cc, request)
		if err != nil {
			return nil, err
		}
		if beforeModelCallbackContent != nil {
			return beforeModelCallbackContent, nil
		}
	}

	return nil, nil
}

// handleAfterModelCallback processes callbacks that should run after the model has generated a response.
func (f *LLMFlow) handleAfterModelCallback(ctx context.Context, ic *types.InvocationContext, response *types.LLMResponse, modelResponseEvent *types.Event) (*types.LLMResponse, error) {
	llmAgent, ok := ic.Agent.AsLLMAgent()
	if !ok {
		return nil, nil
	}
	if len(llmAgent.AfterModelCallbacks()) == 0 {
		return nil, nil
	}

	cc := types.NewCallbackContext(ic).WithEventActions(modelResponseEvent.Actions)
	for _, callback := range llmAgent.AfterModelCallbacks() {
		afterModelCallbackContent, err := callback(cc, response)
		if err != nil {
			return nil, err
		}
		if afterModelCallbackContent != nil {
			return afterModelCallbackContent, nil
		}
	}

	return nil, nil
}

func (f *LLMFlow) finalizeModelResponseEvent(ctx context.Context, request *types.LLMRequest, response *types.LLMResponse, modelResponseEvent *types.Event) *types.Event {
	if modelResponseEvent.Content != nil {
		funcCalls := modelResponseEvent.GetFunctionCalls()
		if len(funcCalls) > 0 {
			PopulateClientFunctionCallID(ctx, modelResponseEvent)
			modelResponseEvent.LongRunningToolIDs.Insert(GetLongRunningFunctionCalls(ctx, funcCalls, request.ToolMap).UnsortedList()...)
		}
	}
	return modelResponseEvent
}

// getLLM extracts the LLM model from the invocation context
func (f *LLMFlow) getLLM(ctx context.Context, ic *types.InvocationContext) types.Model {
	llmAgent, _ := ic.Agent.AsLLMAgent()
	model, err := llmAgent.CanonicalModel(ctx)
	if err != nil {
		panic(fmt.Errorf("LLMFlow.getLLM: %w", err))
	}
	return model
}
