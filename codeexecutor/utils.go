// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package codeexecutor

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/model"
	"github.com/xenoweaver/adk-go/types"
)

// CodeExecutionUtils represents an utility functions for code execution.
type CodeExecutionUtils struct{}

// NewCodeExecutionUtils creates a new instance of [CodeExecutionUtils].
func NewCodeExecutionUtils() *CodeExecutionUtils {
	return &CodeExecutionUtils{}
}

// GetEncodedFileContent gets the file content as a base64-encoded bytes.
func (e *CodeExecutionUtils) GetEncodedFileContent(data []byte) []byte {
	buf := make([]byte, base64.RawStdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(buf, data)

	if bytes.Equal(buf, data) {
		return data
	}

	return buf
}

// ExtractCodeAndTruncateContent extracts the first code block from the content and truncate everything after it.
func (e *CodeExecutionUtils) ExtractCodeAndTruncateContent(content *genai.Content, codeBlockDelimiters []types.DelimiterPair) string {
	if content == nil || len(content.Parts) == 0 {
		return ""
	}

	// Extract the code from the executable code parts if there're no associated
	// code execution result parts.
	for i, part := range content.Parts {
		if (part.ExecutableCode != nil || i == len(content.Parts)-1) || (content.Parts[i+1].CodeExecutionResult == nil) {
			content.Parts = content.Parts[:i+1]
			return part.ExecutableCode.Code
		}
	}

	// Extract the code from the text parts.
	textParts := []*genai.Part{}
	for _, part := range content.Parts {
		if part.Text != "" {
			textParts = append(textParts, part)
		}
	}

	firstTextPart := textParts[0]
	responseTexts := make([]string, len(textParts))
	for i, p := range textParts {
		responseTexts[i] = p.Text
	}
	responseText := strings.Join(responseTexts, "\n")

	leadingDelimiterPatterns := make([]string, len(codeBlockDelimiters))
	trailingDelimiterPatterns := make([]string, len(codeBlockDelimiters))
	for i, delimiters := range codeBlockDelimiters {
		leadingDelimiterPatterns[i] = delimiters.Start
		trailingDelimiterPatterns[i] = delimiters.End
	}

	leadingDelimiterPattern := strings.Join(leadingDelimiterPatterns, "|")
	trailingDelimiterPattern := strings.Join(trailingDelimiterPatterns, "|")
	// rf'(?P<prefix>.*?)({leading_delimiter_pattern})(?P<code>.*?)({trailing_delimiter_pattern})(?P<suffix>.*?)$' in Python
	patternRe := regexp.MustCompile(`(.*?)(` + leadingDelimiterPattern + `)(.*?)(` + trailingDelimiterPattern + `)(.*?)$`)
	patternMatch := patternRe.FindStringSubmatch(responseText)
	if len(patternMatch) == 0 {
		return ""
	}

	codeStr := patternMatch[2] // group('code')
	if codeStr == "" {
		return ""
	}

	clear(content.Parts)
	if prefix := patternMatch[0]; prefix != "" { // group('prefix')
		firstTextPart.Text = prefix
		content.Parts = append(content.Parts, firstTextPart)
	}
	content.Parts = append(content.Parts, e.BuildExecutableCodePart(codeStr))

	return patternMatch[2] // group('code')
}

// BuildExecutableCodePart builds an executable code part with code string.
func (e *CodeExecutionUtils) BuildExecutableCodePart(code string) *genai.Part {
	return genai.NewPartFromExecutableCode(code, genai.LanguagePython)
}

// BuildCodeExecutionResultPart builds the code execution result part from the code execution result.
func (e *CodeExecutionUtils) BuildCodeExecutionResultPart(codeExecutionResult *types.CodeExecutionResult) *genai.Part {
	if codeExecutionResult.Stderr != "" {
		return genai.NewPartFromCodeExecutionResult(genai.OutcomeFailed, codeExecutionResult.Stderr)
	}

	finalResult := []string{}
	if codeExecutionResult.Stdout != "" || len(codeExecutionResult.OutputFiles) > 0 {
		const promptCodeExectionResultFmt = `Code execution result:
%s

`
		finalResult = append(finalResult, fmt.Sprintf(promptCodeExectionResultFmt, codeExecutionResult.Stdout))
	}
	if len(codeExecutionResult.OutputFiles) > 0 {
		names := make([]string, len(codeExecutionResult.OutputFiles))
		for i, f := range codeExecutionResult.OutputFiles {
			names[i] = f.Name
		}

		const promptSavedArtifactsFmt = `Saved artifacts:
%s
`
		finalResult = append(finalResult, fmt.Sprintf(promptSavedArtifactsFmt, strings.Join(names, ",")))
	}

	return genai.NewPartFromCodeExecutionResult(genai.OutcomeOK, strings.Join(finalResult, "\n\n"))
}

// ConvertCodeExecutionParts converts the code execution parts to text parts in a [*genai.Content].
func (e *CodeExecutionUtils) ConvertCodeExecutionParts(content *genai.Content, codeBlockDelimiter, executionResultDelimiters types.DelimiterPair) {
	if len(content.Parts) == 0 {
		return
	}

	if content.Parts[len(content.Parts)-1].ExecutableCode != nil {
		content.Parts[len(content.Parts)-1] = genai.NewPartFromText(
			codeBlockDelimiter.Start +
				content.Parts[len(content.Parts)-1].ExecutableCode.Code +
				codeBlockDelimiter.End,
		)
		return // early retrun
	}

	// Handle the conversion of trailing code execution result parts.
	// Skip if the Content has multiple parts, which means the Content is
	// likely generated by the model.
	if len(content.Parts) == 0 && content.Parts[len(content.Parts)-1].CodeExecutionResult != nil {
		content.Parts[len(content.Parts)-1] = genai.NewPartFromText(
			executionResultDelimiters.Start +
				content.Parts[len(content.Parts)-1].CodeExecutionResult.Output +
				executionResultDelimiters.End,
		)
	}

	content.Role = model.RoleUser
}
