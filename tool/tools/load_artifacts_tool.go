// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"fmt"

	"github.com/go-json-experiment/json"
	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/internal/pool"
	"github.com/xenoweaver/adk-go/model"
	"github.com/xenoweaver/adk-go/tool"
	"github.com/xenoweaver/adk-go/types"
)

// LoadArtifactsTool represents a tool that loads the artifacts and adds them to the session.
type LoadArtifactsTool struct {
	*tool.Tool
}

var _ types.Tool = (*LoadArtifactsTool)(nil)

// NewLoadArtifactsTool returns the new [LoadArtifactsTool].
func NewLoadArtifactsTool() *LoadArtifactsTool {
	return &LoadArtifactsTool{
		Tool: tool.NewTool("load_artifacts", "Loads the artifacts and adds them to the session.", false),
	}
}

// Name implements [types.Tool].
func (t *LoadArtifactsTool) Name() string {
	return t.Tool.Name()
}

// Description implements [types.Tool].
func (t *LoadArtifactsTool) Description() string {
	return t.Tool.Description()
}

// IsLongRunning implements [types.Tool].
func (t *LoadArtifactsTool) IsLongRunning() bool {
	return t.Tool.IsLongRunning()
}

// GetDeclaration implements [types.Tool].
func (t *LoadArtifactsTool) GetDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"artifact_names": {
					Type: genai.TypeArray,
					Items: &genai.Schema{
						Type: genai.TypeString,
					},
				},
			},
		},
	}
}

// Run implements [types.Tool].
func (t *LoadArtifactsTool) Run(ctx context.Context, args map[string]any, toolCtx *types.ToolContext) (any, error) {
	artifactNames, ok := args["artifact_names"]
	if !ok {
		artifactNames = []string{}
	}

	result := map[string]any{
		"artifact_names": artifactNames,
	}

	return result, nil
}

// ProcessLLMRequest implements [types.Tool].
func (t *LoadArtifactsTool) ProcessLLMRequest(ctx context.Context, toolCtx *types.ToolContext, request *types.LLMRequest) error {
	if err := t.Tool.ProcessLLMRequest(ctx, toolCtx, request); err != nil {
		return err
	}

	if err := t.appendArtifactsToLLMRequest(ctx, toolCtx, request); err != nil {
		return err
	}

	return nil
}

func (t *LoadArtifactsTool) appendArtifactsToLLMRequest(ctx context.Context, toolCtx *types.ToolContext, request *types.LLMRequest) error {
	artifactNames, err := toolCtx.ListArtifacts(ctx)
	if err != nil {
		return err
	}
	if len(artifactNames) == 0 {
		return nil
	}

	// Tell the model about the available artifacts.
	sb := pool.String.Get()
	if err := json.MarshalWrite(sb, artifactNames, json.DefaultOptionsV2()); err != nil {
		return err
	}
	s := sb.String()
	pool.String.Put(sb)

	instructions := `You have a list of artifacts:
  ` + s + `

  When the user asks questions about any of the artifacts, you should call the
  ` + "`load_artifacts`" + ` function to load the artifact. Do not generate any text other
  than the function call.
`

	request.AppendInstructions(instructions)

	// Attach the content of the artifacts if the model requests them.
	// This only adds the content to the model request, instead of the session.
	if len(request.Contents) > 0 && len(request.Contents[len(request.Contents)-1].Parts) > 0 {
		funcResponse := request.Contents[len(request.Contents)-1].Parts[0].FunctionResponse
		if funcResponse != nil && funcResponse.Name == "load_artifacts" {
			artifactNames := funcResponse.Response["artifact_names"]
			for _, artifactName := range artifactNames.([]string) {
				artifact, err := toolCtx.LoadArtifact(ctx, artifactName, 0)
				if err != nil {
					return err
				}
				parts := []*genai.Part{
					genai.NewPartFromText(fmt.Sprintf("Artifact %s is:", artifactNames)),
					artifact,
				}
				request.Contents = append(request.Contents, genai.NewContentFromParts(parts, model.ToGenAIRole(model.RoleUser)))
			}
		}
	}

	return nil
}
