// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package example

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"github.com/xenoweaver/adk-go/internal/pool"
	"github.com/xenoweaver/adk-go/model"
)

// Constant parts of the example string.
const (
	ExamplesIntro          = "<EXAMPLES>\nBegin few-shot\nThe following are examples of user queries and model responses using the available tools.\n\n"
	ExamplesEnd            = "End few-shot\n<EXAMPLES>"
	ExampleStart           = "EXAMPLE %d:\nBegin example\n"
	ExampleEnd             = "End example\n\n"
	UserPrefix             = "[user]\n"
	ModelPrefix            = "[model]\n"
	FunctionPrefix         = "```\n"
	FunctionCallPrefix     = "```tool_code\n"
	FunctionCallSuffix     = "\n```\n"
	FunctionResponsePrefix = "```tool_outputs\n"
	FunctionResponseSuffix = "\n```\n"
)

// ConvertExamplesToText converts a list of examples to a string that can be used in a system instruction.
//
// TODO(adk-python: yaojie): Add unit tests for this function.
func ConvertExamplesToText(examples []*Example, modelStr string) (string, error) {
	var (
		examplesStr strings.Builder
		otuput      strings.Builder
	)

	sb := pool.String.Get()
	for i, example := range examples {
		otuput.Reset() // reuse

		otuput.WriteString(fmt.Sprintf(ExampleStart, i+1) + UserPrefix)
		if example.Input != nil && len(example.Input.Parts) > 0 {
			partTexts := make([]string, 0, len(example.Input.Parts))
			for _, part := range example.Input.Parts {
				if part.Text != "" {
					partTexts = append(partTexts, part.Text)
				}
			}
			otuput.WriteString(strings.Join(partTexts, "\n") + "\n")
		}

		gemini2 := ""
		if modelStr == "gemini-2" {
			gemini2 = modelStr
		}
		previousRole := ""
		for _, content := range example.Output {
			role := UserPrefix
			if content.Role == model.RoleModel {
				role = ModelPrefix
			}
			if role != previousRole {
				otuput.WriteString(role)
			}

			for _, part := range content.Parts {
				switch {
				case part.FunctionCall != nil:
					args := []string{}
					// Convert function call part to python-like function call
					for k, v := range part.FunctionCall.Args {
						switch v := v.(type) {
						case string:
							args = append(args, fmt.Sprintf("%s='%s'", k, v))
						default:
							args = append(args, fmt.Sprintf("%s=%v", k, v))
						}
					}
					prefix := FunctionCallPrefix
					if gemini2 != "" {
						prefix = FunctionPrefix
					}
					otuput.WriteString(fmt.Sprintf("%s%s(%s)%s", prefix, part.FunctionCall.Name, strings.Join(args, ", "), FunctionCallSuffix))

				case part.FunctionResponse != nil:
					// Convert function response part to json string
					prefix := FunctionResponsePrefix
					if gemini2 != "" {
						prefix = FunctionPrefix
					}
					sb.Reset()
					if err := json.MarshalWrite(sb, part.FunctionResponse, jsontext.SpaceAfterComma(true)); err != nil {
						return "", err
					}
					otuput.WriteString(fmt.Sprintf("%s%v%s", prefix, sb.String(), FunctionResponseSuffix))

				case part.Text != "":
					otuput.WriteString(part.Text + "\n")
				}
			}
		}

		otuput.WriteString(ExampleEnd)
		examplesStr.WriteString(otuput.String())
	}
	pool.String.Put(sb)

	return fmt.Sprintf("%s%s%s", ExamplesIntro, examplesStr.String(), ExamplesEnd), nil
}

// BuildExampleSI builds a system instruction string from examples.
func BuildExampleSI[T any](ctx context.Context, examples T, query, modelStr string) (string, error) {
	switch examples := any(examples).(type) {
	case []*Example:
		return ConvertExamplesToText(examples, modelStr)
	case Provider:
		exmpls, err := examples.GetExamples(ctx, query)
		if err != nil {
			return "", err
		}
		return ConvertExamplesToText(exmpls, modelStr)
	default:
		return "", errors.New("Invalid example configuration")
	}
}
