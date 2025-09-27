// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package llmflow

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/xenoweaver/adk-go/types"
)

// InstructionsLlmRequestProcessor represents a handles instructions and global instructions for LLM flow.
type InstructionsLlmRequestProcessor struct{}

var _ types.LLMRequestProcessor = (*InstructionsLlmRequestProcessor)(nil)

// Run implements [LLMRequestProcessor].
func (p *InstructionsLlmRequestProcessor) Run(ctx context.Context, ictx *types.InvocationContext, request *types.LLMRequest) iter.Seq2[*types.Event, error] {
	return func(yield func(*types.Event, error) bool) {
		llmAgent, ok := ictx.Agent.AsLLMAgent()
		if !ok {
			return
		}

		rootAgent := llmAgent.RootAgent()

		// Appends global instructions if set.
		if rootAgent, ok := rootAgent.AsLLMAgent(); ok {
			rawSI, bypassStateInjection := rootAgent.CanonicalGlobalInstruction(types.NewReadOnlyContext(ictx))
			si := rawSI
			_ = si
			if !bypassStateInjection {
				// si = pop
			}
		}
	}
}

// Match represents a regular expression match
type Match struct {
	Text   string
	Groups []string
	Start  int
	End    int
}

// FindIterChannel returns a channel that yields match objects one by one
func FindIterChannel(pattern *regexp.Regexp, text string) <-chan Match {
	ch := make(chan Match)

	go func() {
		defer close(ch)

		// Get all matches with position information
		allMatches := pattern.FindAllStringSubmatch(text, -1)
		allIndices := pattern.FindAllStringSubmatchIndex(text, -1)

		for i, m := range allMatches {
			indices := allIndices[i]

			match := Match{
				Text:   m[0],
				Groups: m,
				Start:  indices[0],
				End:    indices[1],
			}

			ch <- match
		}
	}()

	return ch
}

// populateValues populates values in the instruction template, e.g. state, artifact, etc.
func (p *InstructionsLlmRequestProcessor) populateValues(ctx context.Context, instructionTemplate string, ictx *types.InvocationContext) string {
	sub := func(pattern *regexp.Regexp, fn func(match Match) (string, error), src string) string {
		results := []string{}
		lastEnd := 0
		for match := range FindIterChannel(pattern, src) {
			results = append(results, src[lastEnd:match.Start])
			replacement, err := fn(match)
			if err != nil {
				continue
			}
			results = append(results, replacement)
			lastEnd = match.End
		}
		results = append(results, src[lastEnd:])
		return strings.Join(results, "")
	}

	replaceMatch := func(m Match) (string, error) {
		varName := strings.TrimPrefix(strings.TrimSuffix(strings.Join(m.Groups, ""), "}"), "{")
		varName = strings.TrimSpace(varName)
		optional := false
		if strings.HasSuffix(varName, "?") {
			optional = true
			varName = strings.TrimSuffix(varName, "?")
		}
		if after, ok := strings.CutPrefix(varName, "artifact."); ok {
			varName = after
			if ictx.ArtifactService == nil {
				return "", errors.New("artifact service is not initialized")
			}
			artifact, err := ictx.ArtifactService.LoadArtifact(ctx, ictx.Session.AppName(), ictx.Session.UserID(), ictx.Session.ID(), varName, 0)
			if err != nil {
				return "", err
			}
			if varName == "" {
				// TODO(zchee): can't str(artifact) cast in Go
				return fmt.Sprintf("%v", artifact), nil
			}
		} else {
			if !p.isValidStateName(varName) {
				return strings.Join(m.Groups, ""), nil
			}
			if val, ok := ictx.Session.State()[varName]; ok {
				// TODO(zchee): can't str(artifact) cast in Go
				return fmt.Sprintf("%v", val), nil
			} else {
				if optional {
					return "", nil
				}
			}
		}
		return "", fmt.Errorf("Context variable not found: %s", varName)
	}

	return sub(regexp.MustCompile(`{+[^{}]*}+`), replaceMatch, instructionTemplate)
}

func isIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := rune(s[0])
	if !unicode.IsLetter(first) && first != '_' {
		return false
	}
	for _, r := range s[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

// isValidStateName checks if the variable name is a valid state name.
//
// Valid state is either:
//   - Valid identifier
//   - <Valid prefix>:<Valid identifier>
//
// All the others will just return as it is.
func (p *InstructionsLlmRequestProcessor) isValidStateName(varName string) bool {
	parts := strings.Split(varName, ":")

	switch len(parts) {
	case 1:
		return isIdentifier(varName)
	case 2:
		prefixes := []string{
			types.AppPrefix,
			types.UserPrefix,
			types.TempPrefix,
		}
		if slices.Contains(prefixes, parts[0]+":") {
			return isIdentifier(parts[1])
		}
	}

	return false
}
