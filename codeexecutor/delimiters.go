// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package codeexecutor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xenoweaver/adk-go/types"
)

// CodeBlock represents a parsed code block with language and content.
type CodeBlock struct {
	Language string
	Code     string
	Start    int // Starting position in original text
	End      int // Ending position in original text
}

// CodeBlockParser extracts code blocks from text using configurable delimiters.
type CodeBlockParser struct {
	delimiters []types.DelimiterPair
}

// NewCodeBlockParser creates a new parser with the given delimiters.
func NewCodeBlockParser(delimiters []types.DelimiterPair) *CodeBlockParser {
	return &CodeBlockParser{
		delimiters: delimiters,
	}
}

// NewDefaultCodeBlockParser creates a parser with default delimiters.
func NewDefaultCodeBlockParser() *CodeBlockParser {
	return NewCodeBlockParser(types.DefaultConfig().CodeBlockDelimiters)
}

// ExtractCodeBlocks extracts all code blocks from the given text.
func (p *CodeBlockParser) ExtractCodeBlocks(text string) ([]*CodeBlock, error) {
	var blocks []*CodeBlock

	// First try markdown-style code blocks
	markdownBlocks, err := p.extractMarkdownCodeBlocks(text)
	if err != nil {
		return nil, fmt.Errorf("failed to extract markdown code blocks: %w", err)
	}
	blocks = append(blocks, markdownBlocks...)

	// Then try delimiter-based code blocks
	delimiterBlocks, err := p.extractDelimiterCodeBlocks(text)
	if err != nil {
		return nil, fmt.Errorf("failed to extract delimiter code blocks: %w", err)
	}
	blocks = append(blocks, delimiterBlocks...)

	return p.deduplicateBlocks(blocks), nil
}

// extractMarkdownCodeBlocks extracts standard markdown code blocks (```language\ncode\n```).
func (p *CodeBlockParser) extractMarkdownCodeBlocks(text string) ([]*CodeBlock, error) {
	// Regex to match markdown code blocks: ```language\ncode\n```
	re := regexp.MustCompile("```([a-zA-Z0-9_+-]*)\n([\\s\\S]*?)\n```")
	matches := re.FindAllStringSubmatchIndex(text, -1)

	var blocks []*CodeBlock
	for _, match := range matches {
		if len(match) >= 6 {
			language := text[match[2]:match[3]]
			code := text[match[4]:match[5]]

			blocks = append(blocks, &CodeBlock{
				Language: language,
				Code:     code,
				Start:    match[0],
				End:      match[1],
			})
		}
	}

	return blocks, nil
}

// extractDelimiterCodeBlocks extracts code blocks using configured delimiters.
func (p *CodeBlockParser) extractDelimiterCodeBlocks(text string) ([]*CodeBlock, error) {
	var blocks []*CodeBlock

	for _, delimiter := range p.delimiters {
		startPattern := regexp.QuoteMeta(delimiter.Start)
		endPattern := regexp.QuoteMeta(delimiter.End)

		// Create regex pattern for this delimiter pair
		pattern := startPattern + "([\\s\\S]*?)" + endPattern
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid delimiter pattern: %w", err)
		}

		matches := re.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			if len(match) >= 4 {
				code := text[match[2]:match[3]]
				language := p.inferLanguageFromDelimiter(delimiter.Start)

				blocks = append(blocks, &CodeBlock{
					Language: language,
					Code:     code,
					Start:    match[0],
					End:      match[1],
				})
			}
		}
	}

	return blocks, nil
}

// inferLanguageFromDelimiter attempts to infer the language from delimiter patterns.
func (p *CodeBlockParser) inferLanguageFromDelimiter(start string) string {
	start = strings.ToLower(start)
	if strings.Contains(start, "python") {
		return "python"
	}
	if strings.Contains(start, "go") {
		return "go"
	}
	if strings.Contains(start, "javascript") || strings.Contains(start, "js") {
		return "javascript"
	}
	if strings.Contains(start, "bash") || strings.Contains(start, "shell") {
		return "bash"
	}
	if strings.Contains(start, "tool_code") {
		return "python" // Default for tool_code
	}
	return "" // Unknown language
}

// deduplicateBlocks removes duplicate code blocks based on position.
func (p *CodeBlockParser) deduplicateBlocks(blocks []*CodeBlock) []*CodeBlock {
	seen := make(map[string]bool)
	var result []*CodeBlock

	for _, block := range blocks {
		key := fmt.Sprintf("%d-%d", block.Start, block.End)
		if !seen[key] {
			seen[key] = true
			result = append(result, block)
		}
	}

	return result
}

// FilterByLanguage returns only code blocks matching the specified languages.
// If no languages are specified, all blocks are returned.
func (p *CodeBlockParser) FilterByLanguage(blocks []*CodeBlock, languages ...string) []*CodeBlock {
	if len(languages) == 0 {
		return blocks
	}

	languageSet := make(map[string]bool)
	for _, lang := range languages {
		languageSet[strings.ToLower(lang)] = true
	}

	var filtered []*CodeBlock
	for _, block := range blocks {
		if languageSet[strings.ToLower(block.Language)] {
			filtered = append(filtered, block)
		}
	}

	return filtered
}

// ExecutionResultFormatter formats execution results with configurable delimiters.
type ExecutionResultFormatter struct {
	delimiters types.DelimiterPair
}

// NewExecutionResultFormatter creates a new formatter with the given delimiters.
func NewExecutionResultFormatter(delimiters types.DelimiterPair) *ExecutionResultFormatter {
	return &ExecutionResultFormatter{
		delimiters: delimiters,
	}
}

// NewDefaultExecutionResultFormatter creates a formatter with default delimiters.
func NewDefaultExecutionResultFormatter() *ExecutionResultFormatter {
	return NewExecutionResultFormatter(types.DefaultConfig().ExecutionResultDelimiters)
}

// FormatResult formats an execution result with delimiters.
func (f *ExecutionResultFormatter) FormatResult(result *types.CodeExecutionResult) string {
	var output strings.Builder

	output.WriteString(f.delimiters.Start)

	if result.ExitCode == 0 {
		if result.Stdout != "" {
			output.WriteString(result.Stdout)
			if !strings.HasSuffix(result.Stdout, "\n") {
				output.WriteString("\n")
			}
		}
	} else {
		if result.Stderr != "" {
			output.WriteString("Error: ")
			output.WriteString(result.Stderr)
			if !strings.HasSuffix(result.Stderr, "\n") {
				output.WriteString("\n")
			}
		}
		if result.Error != nil {
			output.WriteString("Execution error: ")
			output.WriteString(result.Error.Error())
			output.WriteString("\n")
		}
	}

	// Add information about output files if any
	if len(result.OutputFiles) > 0 {
		output.WriteString(fmt.Sprintf("\n[Generated %d output file(s)]\n", len(result.OutputFiles)))
		for _, file := range result.OutputFiles {
			output.WriteString(fmt.Sprintf("- %s (%d bytes)\n", file.Name, file.Size))
		}
	}

	output.WriteString(f.delimiters.End)

	return output.String()
}

// FormatInlineResult formats a short inline result without delimiters.
func (f *ExecutionResultFormatter) FormatInlineResult(result *types.CodeExecutionResult) string {
	if result.ExitCode == 0 {
		if result.Stdout != "" {
			return strings.TrimSpace(result.Stdout)
		}
		return "Execution completed successfully"
	}

	if result.Stderr != "" {
		return fmt.Sprintf("error: %s", strings.TrimSpace(result.Stderr))
	}

	if result.Error != nil {
		return fmt.Sprintf("execution failed: %s", result.Error.Error())
	}

	return "Execution failed"
}

// ExtractExecutionResults extracts execution results from formatted text.
func (f *ExecutionResultFormatter) ExtractExecutionResults(text string) ([]string, error) {
	startPattern := regexp.QuoteMeta(f.delimiters.Start)
	endPattern := regexp.QuoteMeta(f.delimiters.End)

	pattern := startPattern + "([\\s\\S]*?)" + endPattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid delimiter pattern: %w", err)
	}

	matches := re.FindAllStringSubmatch(text, -1)
	var results []string
	for _, match := range matches {
		if len(match) >= 2 {
			results = append(results, match[1])
		}
	}

	return results, nil
}
