// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package llmflow

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"iter"
	"path/filepath"
	"regexp"
	"unicode"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/codeexecutor"
	"github.com/xenoweaver/adk-go/internal/xiter"
	"github.com/xenoweaver/adk-go/internal/xmaps"
	"github.com/xenoweaver/adk-go/model"
	"github.com/xenoweaver/adk-go/pkg/py"
	"github.com/xenoweaver/adk-go/types"
)

// DataFileUtil represents a structure that contains a data file name and its content.
type DataFileUtil struct {
	// The file extension (e.g., ".csv").
	Extension string

	// The code template to load the data file.
	LoaderCodeTemplate string
}

var DataFileUtilMap = map[string]*DataFileUtil{
	"text/csv": {
		Extension:          ".csv",
		LoaderCodeTemplate: "pd.read_csv('%s')",
	},
}

const DataFileHelperLib = `
import pandas as pd

def explore_df(df: pd.DataFrame) -> None:
  """Prints some information about a pandas DataFrame."""

  with pd.option_context(
      'display.max_columns', None, 'display.expand_frame_repr', False
  ):
    # Print the column names to never encounter KeyError when selecting one.
    df_dtypes = df.dtypes

    # Obtain information about data types and missing values.
    df_nulls = (len(df) - df.isnull().sum()).apply(
        lambda x: f'{x} / {df.shape[0]} non-null'
    )

    # Explore unique total values in columns using .unique().
    df_unique_count = df.apply(lambda x: len(x.unique()))

    # Explore unique values in columns using .unique().
    df_unique = df.apply(lambda x: crop(str(list(x.unique()))))

    df_info = pd.concat(
        (
            df_dtypes.rename('Dtype'),
            df_nulls.rename('Non-Null Count'),
            df_unique_count.rename('Unique Values Count'),
            df_unique.rename('Unique Values'),
        ),
        axis=1,
    )
    df_info.index.name = 'Columns'
    print(f"""Total rows: {df.shape[0]}
Total columns: {df.shape[1]}

{df_info}""")
`

// CodeExecutionRequestProcessor represents a processes code execution requests.
type CodeExecutionRequestProcessor struct{}

var _ types.LLMRequestProcessor = (*CodeExecutionRequestProcessor)(nil)

// Run implements [types.LLMRequestProcessor].
func (p *CodeExecutionRequestProcessor) Run(ctx context.Context, ictx *types.InvocationContext, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		llmAgent, ok := ictx.Agent.AsLLMAgent()
		if !ok {
			return
		}
		if llmAgent.CodeExecutor() == nil {
			return
		}
	}
}

// runPreProcessor pre-process the user message by adding the user message to the Colab notebook.
func (p *CodeExecutionRequestProcessor) runPreProcessor(ctx context.Context, ictx *types.InvocationContext, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		llmAgent, ok := ictx.Agent.AsLLMAgent()
		if !ok {
			return
		}
		codeExecutor := llmAgent.CodeExecutor()
		if codeExecutor == nil {
			return
		}

		type builtInCodeExecutor interface {
			ProcessLLMRequest(context.Context, *types.LLMRequest)
		}
		if builtin, ok := codeExecutor.(builtInCodeExecutor); ok {
			builtin.ProcessLLMRequest(ctx, request)
			return
		}

		if codeExecutor.OptimizeDataFile() {
			return
		}

		codeExecutorContent := codeexecutor.NewExecutionContext(types.NewState(ictx.Session.State(), nil))

		// Skip if the error count exceeds the max retry attempts.
		if codeExecutorContent.GetErrorCount(ictx.InvocationID) >= codeExecutor.ErrorRetryAttempts() {
			return
		}

		// [Step 1] Extract data files from the session_history and store them in
		// memory. Meanwhile, mutate the inline data file to text part in session
		// history from all turns.
		allInputFiles := p.extractAndReplaceInlineFiles(codeExecutorContent, request)

		// [Step 2] Run Explore_Df code on the data files from the current turn. We
		// only need to explore the new data files because the previous data files
		// should already be explored and cached in the code execution runtime.
		processedFileNames := py.NewSet(codeExecutorContent.GetProcessedFileNames()...)
		filesToProcess := make([]*types.CodeExecutionFile, 0, len(allInputFiles))
		for _, file := range allInputFiles {
			if !processedFileNames.Has(file.Name) {
				filesToProcess = append(filesToProcess, file)
			}
		}

		for _, file := range filesToProcess {
			codeStr := p.getDataFilePreprocessingCode(file)
			// Skip for unsupported file or executor types.
			if codeStr == "" {
				return
			}

			// Emit the code to execute, and add it to the LLM request.
			parts := []*genai.Part{
				genai.NewPartFromText(fmt.Sprintf("Processing input file: `%s`", file.Name)),
				codeexecutor.NewCodeExecutionUtils().BuildExecutableCodePart(codeStr),
			}
			codeContent := genai.NewContentFromParts(parts, genai.Role(model.RoleModel))
			request.Contents = append(request.Contents, codeContent)

			event := types.NewEvent().
				WithInvocationID(ictx.InvocationID).
				WithAuthor(llmAgent.Name()).
				WithBranch(ictx.Branch).
				WithContent(codeContent)
			if !yield(event, nil) {
				return
			}

			input := &types.CodeExecutionInput{
				Code:        codeStr,
				InputFiles:  []*types.CodeExecutionFile{file},
				ExecutionID: getOrSetExecutionID(ictx, codeExecutorContent),
			}
			codeExecutionResult, err := codeExecutor.ExecuteCode(ctx, ictx, input)
			if err != nil {
				xiter.Error[types.Event](err)
				return
			}

			// Update the processing results to code executor context.
			codeExecutorContent.UpdateExecutionResult(ictx.InvocationID, codeStr, codeExecutionResult.Stdout, codeExecutionResult.Stderr)
			codeExecutorContent.AddProcessedFileNames(file.Name)

			// Emit the execution result, and add it to the LLM request.
			executionResultEvent, err := postProcessCodeExecutionResult(ctx, ictx, codeExecutorContent, codeExecutionResult)
			if err != nil {
				xiter.Error[types.Event](err)
				return
			}

			if !yield(executionResultEvent, nil) {
				return
			}
			request.Contents = append(request.Contents, executionResultEvent.Content)
		}
	}
}

// CodeExecutionResponseProcessor represents a processes code execution responses.
type CodeExecutionResponseProcessor struct{}

var _ types.LLMResponseProcessor = (*CodeExecutionResponseProcessor)(nil)

// Run implements [types.LLMResponseProcessor].
func (p *CodeExecutionResponseProcessor) Run(ctx context.Context, ictx *types.InvocationContext, response *types.LLMResponse) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		// Skip if the response is partial (streaming).
		if response.Partial {
			return
		}
	}
}

// runPostProcessor post-process the model response by extracting and executing the first code block.
func (p *CodeExecutionResponseProcessor) runPostProcessor(ctx context.Context, ictx *types.InvocationContext, response *types.LLMResponse) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		llmAgent, ok := ictx.Agent.AsLLMAgent()
		if !ok {
			return
		}
		codeExecutor := llmAgent.CodeExecutor()
		if codeExecutor == nil {
			return
		}
		if response == nil || response.Content == nil {
			return
		}

		if _, ok := codeExecutor.(*codeexecutor.BuiltInExecutor); ok {
			return
		}

		codeExecutorContent := codeexecutor.NewExecutionContext(types.NewState(ictx.Session.State(), nil))
		// Skip if the error count exceeds the max retry attempts.
		if codeExecutorContent.GetErrorCount(ictx.InvocationID) >= codeExecutor.ErrorRetryAttempts() {
			return
		}

		// [Step 1] Extract code from the model predict response and truncate the
		// content to the part with the first code block.
		responseContent := response.Content
		codeStr := codeexecutor.NewCodeExecutionUtils().ExtractCodeAndTruncateContent(responseContent, codeExecutor.CodeBlockDelimiters())
		if codeStr == "" {
			return
		}

		// [Step 2] Executes the code and emit 2 Events for code and execution result.
		event := types.NewEvent().
			WithInvocationID(ictx.InvocationID).
			WithAuthor(llmAgent.Name()).
			WithBranch(ictx.Branch).
			WithContent(responseContent).
			WithActions(types.NewEventActions())
		if !yield(event, nil) {
			return
		}

		codeExecutionResult, err := codeExecutor.ExecuteCode(ctx, ictx, &types.CodeExecutionInput{
			Code:        codeStr,
			InputFiles:  codeExecutorContent.GetInputFiles(),
			ExecutionID: getOrSetExecutionID(ictx, codeExecutorContent),
		})
		if err != nil {
			xiter.Error[types.Event](err)
			return
		}

		codeExecutorContent.UpdateExecutionResult(ictx.InvocationID, codeStr, codeExecutionResult.Stdout, codeExecutionResult.Stderr)
		if !yield(event, nil) {
			return
		}
	}
}

// extractAndReplaceInlineFiles extracts and replaces inline files with file names in the LLM request.
func (p *CodeExecutionRequestProcessor) extractAndReplaceInlineFiles(codeExecutorContext *codeexecutor.CodeExecutorContext, request *types.LLMRequest) []*types.CodeExecutionFile {
	allInputFiles := codeExecutorContext.GetInputFiles()
	savedFileNames := py.NewSet[string]()
	for _, inputFile := range allInputFiles {
		savedFileNames.Insert(inputFile.Name)
	}

	// [Step 1] Process input files from LlmRequest and cache them in CodeExecutor.
	for i := range len(request.Contents) {
		content := request.Contents[i]
		// Only process the user message
		if content.Role != model.RoleUser && len(content.Parts) == 0 {
			continue
		}

		for j := range len(content.Parts) {
			part := content.Parts[j]
			// Skip if the inline data is not supported.
			if part.InlineData != nil || !xmaps.Contains(DataFileUtilMap, part.InlineData.MIMEType) {
				continue
			}

			// Replace the inline data file with a file name placeholder.
			mimeType := part.InlineData.MIMEType
			fileName := fmt.Sprintf("data_%d_%d%s", i+1, j+1, DataFileUtilMap[mimeType].Extension)
			request.Contents[i].Parts[j] = genai.NewPartFromText(fmt.Sprintf("\nAvailable file: `%s`\n", fileName))

			// Add the inlne data as input file to the code executor context.
			file := types.NewExecutionFile(fileName, codeexecutor.NewCodeExecutionUtils().GetEncodedFileContent(part.InlineData.Data), mimeType)
			if !savedFileNames.Has(fileName) {
				codeExecutorContext.AddInputFiles(file)
				allInputFiles = append(allInputFiles, file)
			}
		}
	}

	return allInputFiles
}

// getOrSetExecutionID returns the ID for stateful code execution or None if not stateful.
func getOrSetExecutionID(ictx *types.InvocationContext, codeExecutorContext *codeexecutor.CodeExecutorContext) string {
	llmAgent, ok := ictx.Agent.AsLLMAgent()
	if !ok {
		return ""
	}
	if llmAgent.CodeExecutor().IsStateful() {
		return ""
	}

	executionID := codeExecutorContext.GetExecutionID()
	if executionID == "" {
		executionID = ictx.Session.ID()
		codeExecutorContext.SetExecutionID(executionID)
	}

	return executionID
}

// postProcessCodeExecutionResult post-process the code execution result and emit an Event.
func postProcessCodeExecutionResult(ctx context.Context, ictx *types.InvocationContext, codeExecutorContext *codeexecutor.CodeExecutorContext, codeExecutionResult *types.CodeExecutionResult) (*types.Event, error) {
	if ictx.ArtifactService == nil {
		return nil, errors.New("artifact service is not initialized")
	}

	resultContent := &genai.Content{
		Role:  model.RoleModel,
		Parts: []*genai.Part{codeexecutor.NewCodeExecutionUtils().BuildCodeExecutionResultPart(codeExecutionResult)},
	}
	eventActions := types.NewEventActions().WithStateDelta(codeExecutorContext.GetStateDelta())

	// Handle code execution error retry.
	switch {
	case codeExecutionResult.Stderr != "":
		codeExecutorContext.IncrementErrorCount(ictx.InvocationID)
	default:
		codeExecutorContext.ResetErrorCount(ictx.InvocationID)
	}

	// Handle output files.
	for _, outputFile := range codeExecutionResult.OutputFiles {
		buf := make([]byte, base64.StdEncoding.EncodedLen(len(outputFile.Content)))
		base64.StdEncoding.Encode(buf, outputFile.Content)

		version, err := ictx.ArtifactService.SaveArtifact(
			ctx,
			ictx.AppName(),
			ictx.UserID(),
			ictx.Session.ID(),
			outputFile.Name,
			genai.NewPartFromBytes(buf, outputFile.MIMEType),
		)
		if err != nil {
			return nil, err
		}
		eventActions.ArtifactDelta[outputFile.Name] = version
	}

	event := types.NewEvent().
		WithInvocationID(ictx.InvocationID).
		WithAuthor(ictx.Agent.Name()).
		WithBranch(ictx.Branch).
		WithContent(resultContent).
		WithActions(eventActions)

	return event, nil
}

var varNameRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// getDataFilePreprocessingCode returns the code to explore the data file.
func (p *CodeExecutionRequestProcessor) getDataFilePreprocessingCode(file *types.CodeExecutionFile) string {
	getRormalizedFileName := func(filename string) string {
		varName := filename[:len(filepath.Ext(filename))+1]
		// Replace non-alphanumeric characters with underscores
		varName = varNameRe.ReplaceAllString(varName, "_")

		// If the filename starts with a digit, prepend an underscore
		if unicode.IsDigit(rune(varName[0])) {
			varName = "_" + varName
		}
		return varName
	}

	if !xmaps.Contains(DataFileUtilMap, file.MIMEType) {
		return ""
	}

	varName := getRormalizedFileName(file.Name)
	loaderCode := fmt.Sprintf(DataFileUtilMap[file.MIMEType].LoaderCodeTemplate, file.Name)

	return `
` + DataFileHelperLib + `

# Load the dataframe.
` +
		varName + `=` + loaderCode + `

# Use ` + "`explore_df`" + ` to guide my analysis.
explore_df(` + varName + `)
`
}
