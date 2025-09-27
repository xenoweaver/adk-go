// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package example

import (
	"context"

	aiplatform "cloud.google.com/go/aiplatform/apiv1beta1"
	"cloud.google.com/go/aiplatform/apiv1beta1/aiplatformpb"
	"google.golang.org/api/option"
	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/model"
	"github.com/xenoweaver/adk-go/pkg/logging"
)

// VertexAiExampleStore provides examples from Vertex example store.
type VertexAIExampleStore struct {
	client       *aiplatform.ExampleStoreClient
	exampleStore string
}

var _ Provider = (*VertexAIExampleStore)(nil)

// NewVertexAIExampleStore creates a new VertexAiExampleStore client from the given examplesStoreName.
//
// examplesStoreName is the resource name of the vertex example store, in the format of
//
//	projects/{project}/locations/{location}/exampleStores/{example_store}
func NewVertexAIExampleStore(ctx context.Context, exampleStore string, opts ...option.ClientOption) (*VertexAIExampleStore, error) {
	logger := logging.FromContext(ctx).WithGroup("example.VertexAIExampleStore")
	opts = append(opts, option.WithLogger(logger))

	client, err := aiplatform.NewExampleStoreClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &VertexAIExampleStore{
		client:       client,
		exampleStore: exampleStore,
	}, nil
}

// GetExamples returns the slice of [*Example] from Vertex AI example store for a given query.
func (e *VertexAIExampleStore) GetExamples(ctx context.Context, query string) ([]*Example, error) {
	req := &aiplatformpb.SearchExamplesRequest{
		Parameters: &aiplatformpb.SearchExamplesRequest_StoredContentsExampleParameters{
			StoredContentsExampleParameters: &aiplatformpb.StoredContentsExampleParameters{
				Query: &aiplatformpb.StoredContentsExampleParameters_ContentSearchKey_{
					ContentSearchKey: &aiplatformpb.StoredContentsExampleParameters_ContentSearchKey{
						Contents: []*aiplatformpb.Content{
							{
								Role: model.RoleUser,
								Parts: []*aiplatformpb.Part{
									{
										Data: &aiplatformpb.Part_Text{
											Text: query,
										},
									},
								},
							},
						},
						SearchKeyGenerationMethod: &aiplatformpb.StoredContentsExample_SearchKeyGenerationMethod{
							Method: &aiplatformpb.StoredContentsExample_SearchKeyGenerationMethod_LastEntry_{
								LastEntry: &aiplatformpb.StoredContentsExample_SearchKeyGenerationMethod_LastEntry{},
							},
						},
					},
				},
			},
		},
		ExampleStore: e.exampleStore,
		TopK:         10,
	}
	resp, err := e.client.SearchExamples(ctx, req)
	if err != nil {
		return nil, err
	}

	returnedExamples := []*Example{}
	// Convert results to genai formats
	for _, result := range resp.GetResults() {
		if result.SimilarityScore < 0.5 {
			continue
		}
		contents := result.GetExample().GetStoredContentsExample().GetContentsExample().GetContents()
		expectedContents := make([]*aiplatformpb.Content, len(contents))
		copy(expectedContents, result.GetExample().GetStoredContentsExample().GetContentsExample().GetContents())

		expectedOutput := make([]*genai.Content, len(expectedContents))
		for i, content := range expectedContents {
			expectedParts := make([]*genai.Part, 0, len(expectedContents))
			for _, part := range content.GetParts() {
				switch {
				case part.GetText() != "":
					expectedParts[i] = genai.NewPartFromText(part.GetText())
					expectedParts = append(expectedParts,
						genai.NewPartFromText(part.GetText()),
					)
				case part.GetFunctionCall() != nil:
					funcCall := part.GetFunctionCall()
					expectedParts = append(expectedParts,
						genai.NewPartFromFunctionCall(funcCall.GetName(), funcCall.GetArgs().AsMap()),
					)
				case part.GetFunctionResponse() != nil:
					funcResponse := part.GetFunctionResponse()
					expectedParts = append(expectedParts,
						genai.NewPartFromFunctionResponse(funcResponse.GetName(), funcResponse.GetResponse().AsMap()),
					)
				}
			}
			expectedOutput[i] = genai.NewContentFromParts(expectedParts, model.ToGenAIRole((content.GetRole())))
		}

		returnedExamples = append(returnedExamples, &Example{
			Input: genai.NewContentFromParts(
				[]*genai.Part{
					genai.NewPartFromText(result.GetExample().GetStoredContentsExample().GetSearchKey()),
				},
				model.ToGenAIRole(model.RoleUser),
			),
			Output: expectedOutput,
		})
	}

	return returnedExamples, nil
}
