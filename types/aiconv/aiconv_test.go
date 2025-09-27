// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package aiconv_test

import (
	"testing"
	"time"

	"cloud.google.com/go/aiplatform/apiv1beta1/aiplatformpb"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/types"
	"github.com/xenoweaver/adk-go/types/aiconv"
)

// Test helper functions.
func TestToPtr(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		str := "test"
		ptr := types.ToPtr(str)
		if ptr == nil {
			t.Fatal("expected non-nil pointer")
		}
		if *ptr != str {
			t.Errorf("expected %q, got %q", str, *ptr)
		}
	})

	t.Run("int", func(t *testing.T) {
		val := 42
		ptr := types.ToPtr(val)
		if ptr == nil {
			t.Fatal("expected non-nil pointer")
		}
		if *ptr != val {
			t.Errorf("expected %d, got %d", val, *ptr)
		}
	})
}

func TestDeref(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		str := "test"
		ptr := &str
		result := types.Deref(ptr, "default")
		if result != str {
			t.Errorf("expected %q, got %q", str, result)
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		var ptr *string
		def := "default"
		result := types.Deref(ptr, def)
		if result != def {
			t.Errorf("expected %q, got %q", def, result)
		}
	})
}

func TestFromFloat32Ptr(t *testing.T) {
	t.Run("non-nil", func(t *testing.T) {
		val := float32(3.14)
		ptr := &val
		result := aiconv.FromFloat32Ptr(ptr)
		if result != val {
			t.Errorf("expected %f, got %f", val, result)
		}
	})

	t.Run("nil", func(t *testing.T) {
		var ptr *float32
		result := aiconv.FromFloat32Ptr(ptr)
		if result != 0 {
			t.Errorf("expected 0, got %f", result)
		}
	})
}

func TestFromInt32PtrToInt32(t *testing.T) {
	t.Run("non-nil", func(t *testing.T) {
		val := int32(42)
		ptr := &val
		result := aiconv.FromInt32PtrToInt32(ptr)
		if result != val {
			t.Errorf("expected %d, got %d", val, result)
		}
	})

	t.Run("nil", func(t *testing.T) {
		var ptr *int32
		result := aiconv.FromInt32PtrToInt32(ptr)
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})
}

// Test Content conversions.
func TestContentConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformContent(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformContent(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("round-trip", func(t *testing.T) {
		original := &genai.Content{
			Role: "user",
			Parts: []*genai.Part{
				{Text: "Hello, world!"},
				{
					InlineData: &genai.Blob{
						MIMEType: "image/png",
						Data:     []byte("fake-image-data"),
					},
				},
			},
		}

		// Convert to AI Platform and back
		aiPlatform := aiconv.ToAIPlatformContent(original)
		roundTrip := aiconv.FromAIPlatformContent(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		original := &genai.Content{
			Role:  "user",
			Parts: []*genai.Part{},
		}

		aiPlatform := aiconv.ToAIPlatformContent(original)
		roundTrip := aiconv.FromAIPlatformContent(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestContentsConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformContents(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformContents(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("round-trip", func(t *testing.T) {
		original := []*genai.Content{
			{
				Role:  "user",
				Parts: []*genai.Part{{Text: "First message"}},
			},
			{
				Role:  "model",
				Parts: []*genai.Part{{Text: "Second message"}},
			},
		}

		aiPlatform := aiconv.ToAIPlatformContents(original)
		roundTrip := aiconv.FromAIPlatformContents(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test Part conversions.
func TestPartConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformPart(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformPart(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("text part", func(t *testing.T) {
		original := &genai.Part{Text: "Hello, world!"}

		aiPlatform := aiconv.ToAIPlatformPart(original)
		roundTrip := aiconv.FromAIPlatformPart(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("inline data part", func(t *testing.T) {
		original := &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/jpeg",
				Data:     []byte("fake-image-data"),
			},
		}

		aiPlatform := aiconv.ToAIPlatformPart(original)
		roundTrip := aiconv.FromAIPlatformPart(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("file data part", func(t *testing.T) {
		original := &genai.Part{
			FileData: &genai.FileData{
				MIMEType: "video/mp4",
				FileURI:  "gs://bucket/file.mp4",
			},
		}

		aiPlatform := aiconv.ToAIPlatformPart(original)
		roundTrip := aiconv.FromAIPlatformPart(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("function call part", func(t *testing.T) {
		original := &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: "test_function",
				Args: map[string]any{
					"param1": "value1",
					"param2": float64(42), // JSON conversion converts numbers to float64
				},
			},
		}

		aiPlatform := aiconv.ToAIPlatformPart(original)
		roundTrip := aiconv.FromAIPlatformPart(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("function response part", func(t *testing.T) {
		original := &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name: "test_function",
				Response: map[string]any{
					"result": "success",
					"data":   []any{"item1", "item2"},
				},
			},
		}

		aiPlatform := aiconv.ToAIPlatformPart(original)
		roundTrip := aiconv.FromAIPlatformPart(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	// Note: VideoMetadata-only parts are not supported in the current implementation
	// due to the switch statement structure that requires a Data field to be set.
	// VideoMetadata should be combined with other part types like FileData.
}

// Test FunctionCall conversions.
func TestFunctionCallConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformFunctionCall(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformFunctionCall(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("round-trip", func(t *testing.T) {
		original := &genai.FunctionCall{
			Name: "calculate",
			Args: map[string]any{
				"operation": "add",
				"numbers":   []any{float64(1), float64(2), float64(3)},
				"precision": float64(2),
			},
		}

		aiPlatform := aiconv.ToAIPlatformFunctionCall(original)
		roundTrip := aiconv.FromAIPlatformFunctionCall(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("nil args", func(t *testing.T) {
		original := &genai.FunctionCall{
			Name: "no_args_function",
			Args: nil,
		}

		aiPlatform := aiconv.ToAIPlatformFunctionCall(original)
		roundTrip := aiconv.FromAIPlatformFunctionCall(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test FunctionResponse conversions.
func TestFunctionResponseConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformFunctionResponse(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformFunctionResponse(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("round-trip", func(t *testing.T) {
		original := &genai.FunctionResponse{
			Name: "calculate",
			Response: map[string]any{
				"result": float64(6),
				"status": "success",
				"metadata": map[string]any{
					"execution_time": "0.5ms",
				},
			},
		}

		aiPlatform := aiconv.ToAIPlatformFunctionResponse(original)
		roundTrip := aiconv.FromAIPlatformFunctionResponse(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test VideoMetadata conversions.
func TestVideoMetadataConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformVideoMetadata(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformVideoMetadata(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("round-trip", func(t *testing.T) {
		original := &genai.VideoMetadata{
			StartOffset: 2 * time.Minute,
			EndOffset:   5 * time.Minute,
		}

		aiPlatform := aiconv.ToAIPlatformVideoMetadata(original)
		roundTrip := aiconv.FromAIPlatformVideoMetadata(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("zero durations", func(t *testing.T) {
		original := &genai.VideoMetadata{
			StartOffset: 0,
			EndOffset:   0,
		}

		aiPlatform := aiconv.ToAIPlatformVideoMetadata(original)
		if aiPlatform.StartOffset != nil {
			t.Error("expected nil StartOffset for zero duration")
		}
		if aiPlatform.EndOffset != nil {
			t.Error("expected nil EndOffset for zero duration")
		}

		roundTrip := aiconv.FromAIPlatformVideoMetadata(aiPlatform)
		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test Schema conversions.
func TestSchemaConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformSchema(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformSchema(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("complex schema round-trip", func(t *testing.T) {
		minLength := int64(1)
		maxLength := int64(100)
		maximum := 100.0
		nullable := true

		original := &genai.Schema{
			Type:        genai.TypeObject,
			Description: "A complex object schema",
			Properties: map[string]*genai.Schema{
				"name": {
					Type:        genai.TypeString,
					Description: "Name field",
					MinLength:   &minLength,
					MaxLength:   &maxLength,
					Properties:  map[string]*genai.Schema{}, // Set to empty map to match conversion behavior
					Nullable:    &[]bool{false}[0],          // Conversion sets this to false
				},
				"age": {
					Type:        genai.TypeNumber,
					Description: "Age field",
					Maximum:     &maximum,
					Properties:  map[string]*genai.Schema{}, // Set to empty map to match conversion behavior
					Nullable:    &[]bool{false}[0],          // Conversion sets this to false
				},
				"tags": {
					Type:        genai.TypeArray,
					Description: "Tags array",
					Properties:  map[string]*genai.Schema{}, // Set to empty map to match conversion behavior
					Nullable:    &[]bool{false}[0],          // Conversion sets this to false
					Items: &genai.Schema{
						Type:       genai.TypeString,
						Properties: map[string]*genai.Schema{}, // Set to empty map to match conversion behavior
						Nullable:   &[]bool{false}[0],          // Conversion sets this to false
					},
				},
			},
			Required: []string{"name", "age"},
			Nullable: &nullable,
		}

		aiPlatform := aiconv.ToAIPlatformSchema(original)
		roundTrip := aiconv.FromAIPlatformSchema(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("simple schema", func(t *testing.T) {
		original := &genai.Schema{
			Type:        genai.TypeString,
			Description: "A simple string",
			Properties:  map[string]*genai.Schema{}, // Conversion creates empty map
			Nullable:    &[]bool{false}[0],          // Conversion sets this to false
		}

		aiPlatform := aiconv.ToAIPlatformSchema(original)
		roundTrip := aiconv.FromAIPlatformSchema(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test Type enum conversions.
func TestTypeConversions(t *testing.T) {
	tests := []struct {
		genaiType      genai.Type
		aiplatformType aiplatformpb.Type
	}{
		{genai.TypeUnspecified, aiplatformpb.Type_TYPE_UNSPECIFIED},
		{genai.TypeString, aiplatformpb.Type_STRING},
		{genai.TypeNumber, aiplatformpb.Type_NUMBER},
		{genai.TypeInteger, aiplatformpb.Type_INTEGER},
		{genai.TypeBoolean, aiplatformpb.Type_BOOLEAN},
		{genai.TypeArray, aiplatformpb.Type_ARRAY},
		{genai.TypeObject, aiplatformpb.Type_OBJECT},
	}

	for _, test := range tests {
		t.Run(test.aiplatformType.String(), func(t *testing.T) {
			// Test ToAIPlatformType
			aiPlatform := aiconv.ToAIPlatformType(test.genaiType)
			if aiPlatform != test.aiplatformType {
				t.Errorf("ToAIPlatformType: expected %v, got %v", test.aiplatformType, aiPlatform)
			}

			// Test FromAIPlatformType
			genaiResult := aiconv.FromAIPlatformType(test.aiplatformType)
			if genaiResult != test.genaiType {
				t.Errorf("FromAIPlatformType: expected %v, got %v", test.genaiType, genaiResult)
			}

			// Test round-trip
			roundTrip := aiconv.FromAIPlatformType(aiconv.ToAIPlatformType(test.genaiType))
			if roundTrip != test.genaiType {
				t.Errorf("round-trip: expected %v, got %v", test.genaiType, roundTrip)
			}
		})
	}
}

// Test Tool conversions.
func TestToolConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformTool(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformTool(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("function declarations tool", func(t *testing.T) {
		original := &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "get_weather",
					Description: "Get current weather",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"location": {
								Type:        genai.TypeString,
								Description: "City name",
								Properties:  map[string]*genai.Schema{}, // Conversion creates empty map
								Nullable:    &[]bool{false}[0],          // Conversion sets this to false
							},
						},
						Required: []string{"location"},
						Nullable: &[]bool{false}[0], // Conversion sets this to false
					},
				},
			},
		}

		aiPlatform := aiconv.ToAIPlatformTool(original)
		roundTrip := aiconv.FromAIPlatformTool(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("code execution tool", func(t *testing.T) {
		original := &genai.Tool{
			CodeExecution: &genai.ToolCodeExecution{},
		}

		aiPlatform := aiconv.ToAIPlatformTool(original)
		roundTrip := aiconv.FromAIPlatformTool(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("google search retrieval tool", func(t *testing.T) {
		original := &genai.Tool{
			GoogleSearchRetrieval: &genai.GoogleSearchRetrieval{},
		}

		aiPlatform := aiconv.ToAIPlatformTool(original)
		roundTrip := aiconv.FromAIPlatformTool(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test GenerationConfig conversions.
func TestGenerationConfigConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformGenerationConfig(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformGenerationConfig(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("full config round-trip", func(t *testing.T) {
		temperature := float32(0.7)
		topP := float32(0.9)
		topK := float32(40)

		original := &genai.GenerationConfig{
			Temperature:      &temperature,
			TopP:             &topP,
			TopK:             &topK,
			CandidateCount:   1,
			MaxOutputTokens:  1024,
			StopSequences:    []string{"STOP", "END"},
			ResponseMIMEType: "application/json",
			ResponseSchema: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"answer": {
						Type:       genai.TypeString,
						Properties: map[string]*genai.Schema{}, // Conversion creates empty map
						Nullable:   &[]bool{false}[0],          // Conversion sets this to false
					},
				},
				Nullable: &[]bool{false}[0], // Conversion sets this to false
			},
		}

		aiPlatform := aiconv.ToAIPlatformGenerationConfig(original)
		roundTrip := aiconv.FromAIPlatformGenerationConfig(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test SafetySettings conversions.
func TestSafetySettingsConversions(t *testing.T) {
	t.Run("single safety setting", func(t *testing.T) {
		original := &genai.SafetySetting{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockThresholdBlockMediumAndAbove,
		}

		aiPlatform := aiconv.ToAIPlatformSafetySetting(original)
		roundTrip := aiconv.FromAIPlatformSafetySetting(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("multiple safety settings", func(t *testing.T) {
		original := []*genai.SafetySetting{
			{
				Category:  genai.HarmCategoryHarassment,
				Threshold: genai.HarmBlockThresholdBlockMediumAndAbove,
			},
			{
				Category:  genai.HarmCategoryHateSpeech,
				Threshold: genai.HarmBlockThresholdBlockLowAndAbove,
			},
		}

		aiPlatform := aiconv.ToAIPlatformSafetySettings(original)
		roundTrip := aiconv.FromAIPlatformSafetySettings(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test HarmCategory enum conversions.
func TestHarmCategoryConversions(t *testing.T) {
	tests := []struct {
		genaiCategory      genai.HarmCategory
		aiplatformCategory aiplatformpb.HarmCategory
	}{
		{genai.HarmCategoryUnspecified, aiplatformpb.HarmCategory_HARM_CATEGORY_UNSPECIFIED},
		{genai.HarmCategoryHarassment, aiplatformpb.HarmCategory_HARM_CATEGORY_HARASSMENT},
		{genai.HarmCategoryHateSpeech, aiplatformpb.HarmCategory_HARM_CATEGORY_HATE_SPEECH},
		{genai.HarmCategorySexuallyExplicit, aiplatformpb.HarmCategory_HARM_CATEGORY_SEXUALLY_EXPLICIT},
		{genai.HarmCategoryDangerousContent, aiplatformpb.HarmCategory_HARM_CATEGORY_DANGEROUS_CONTENT},
	}

	for _, test := range tests {
		t.Run(test.aiplatformCategory.String(), func(t *testing.T) {
			// Test round-trip
			roundTrip := aiconv.FromAIPlatformHarmCategory(aiconv.ToAIPlatformHarmCategory(test.genaiCategory))
			if roundTrip != test.genaiCategory {
				t.Errorf("round-trip: expected %v, got %v", test.genaiCategory, roundTrip)
			}
		})
	}
}

// Test Candidate conversions.
func TestCandidateConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformCandidate(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformCandidate(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("full candidate round-trip", func(t *testing.T) {
		original := &genai.Candidate{
			Index: 0,
			Content: &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: "Generated response"}},
			},
			FinishReason:  genai.FinishReasonStop,
			FinishMessage: "Completed successfully",
			SafetyRatings: []*genai.SafetyRating{
				{
					Category:    genai.HarmCategoryHarassment,
					Probability: genai.HarmProbabilityNegligible,
					Blocked:     false,
				},
			},
			CitationMetadata: &genai.CitationMetadata{
				Citations: []*genai.Citation{
					{
						StartIndex: 0,
						EndIndex:   10,
						URI:        "https://example.com",
						Title:      "Example Source",
						License:    "MIT",
					},
				},
			},
		}

		aiPlatform := aiconv.ToAIPlatformCandidate(original)
		roundTrip := aiconv.FromAIPlatformCandidate(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test GenerateContentResponse conversions.
func TestGenerateContentResponseConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformGenerateContentResponse(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformGenerateContentResponse(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("full response round-trip", func(t *testing.T) {
		original := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Index: 0,
					Content: &genai.Content{
						Role:  "model",
						Parts: []*genai.Part{{Text: "Response text"}},
					},
					FinishReason: genai.FinishReasonStop,
				},
			},
			PromptFeedback: &genai.GenerateContentResponsePromptFeedback{
				BlockReason: genai.BlockedReasonUnspecified,
				SafetyRatings: []*genai.SafetyRating{
					{
						Category:    genai.HarmCategoryHarassment,
						Probability: genai.HarmProbabilityNegligible,
						Blocked:     false,
					},
				},
			},
			UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:        10,
				CandidatesTokenCount:    20,
				TotalTokenCount:         30,
				CachedContentTokenCount: 5,
			},
		}

		aiPlatform := aiconv.ToAIPlatformGenerateContentResponse(original)
		roundTrip := aiconv.FromAIPlatformGenerateContentResponse(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test Value conversions.
func TestValueConversions(t *testing.T) {
	t.Run("nil handling", func(t *testing.T) {
		if result := aiconv.ToAIPlatformValue(nil); result != nil {
			t.Error("expected nil for nil input")
		}
		if result := aiconv.FromAIPlatformValue(nil); result != nil {
			t.Error("expected nil for nil input")
		}
	})

	tests := []struct {
		name  string
		value any
	}{
		{"string", "test"},
		{"int", float64(42)}, // JSON conversion converts numbers to float64
		{"float", 3.14},
		{"bool", true},
		{"slice", []any{"a", "b", "c"}},
		{"map", map[string]any{"key": "value"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			aiPlatform := aiconv.ToAIPlatformValue(test.value)
			roundTrip := aiconv.FromAIPlatformValue(aiPlatform)

			if diff := cmp.Diff(test.value, roundTrip); diff != "" {
				t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Test panic cases for unknown enum values.
func TestEnumPanicCases(t *testing.T) {
	t.Run("unknown genai.Type", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for unknown genai.Type")
			}
		}()
		aiconv.ToAIPlatformType(genai.Type("UNKNOWN_TYPE"))
	})

	t.Run("unknown aiplatformpb.Type", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for unknown aiplatformpb.Type")
			}
		}()
		aiconv.FromAIPlatformType(aiplatformpb.Type(999))
	})

	t.Run("unknown genai.HarmCategory", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for unknown genai.HarmCategory")
			}
		}()
		aiconv.ToAIPlatformHarmCategory(genai.HarmCategory("UNKNOWN_HARM_CATEGORY"))
	})

	t.Run("unknown genai.FinishReason", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for unknown genai.FinishReason")
			}
		}()
		aiconv.ToAIPlatformFinishReason(genai.FinishReason("UNKNOWN_FINISH_REASON"))
	})
}

// Test empty slice vs nil slice handling.
func TestSliceHandling(t *testing.T) {
	t.Run("nil vs empty slices - Contents", func(t *testing.T) {
		// Nil slice
		if result := aiconv.ToAIPlatformContents(nil); result != nil {
			t.Error("expected nil for nil slice")
		}

		// Empty slice
		empty := []*genai.Content{}
		result := aiconv.ToAIPlatformContents(empty)
		if result == nil {
			t.Error("expected non-nil for empty slice")
		}
		if len(result) != 0 {
			t.Errorf("expected empty slice, got length %d", len(result))
		}
	})

	t.Run("nil vs empty slices - Tools", func(t *testing.T) {
		// Nil slice
		if result := aiconv.ToAIPlatformTools(nil); result != nil {
			t.Error("expected nil for nil slice")
		}

		// Empty slice
		empty := []*genai.Tool{}
		result := aiconv.ToAIPlatformTools(empty)
		if result == nil {
			t.Error("expected non-nil for empty slice")
		}
		if len(result) != 0 {
			t.Errorf("expected empty slice, got length %d", len(result))
		}
	})
}

// Test complex nested structures.
func TestComplexStructures(t *testing.T) {
	t.Run("deeply nested content", func(t *testing.T) {
		// Create a complex structure with multiple levels of nesting
		original := &genai.Content{
			Role: "user",
			Parts: []*genai.Part{
				{Text: "Simple text"},
				{
					FunctionCall: &genai.FunctionCall{
						Name: "complex_function",
						Args: map[string]any{
							"nested_object": map[string]any{
								"level1": map[string]any{
									"level2": map[string]any{
										"data": []any{"item1", "item2", "item3"},
									},
								},
							},
							"array_of_objects": []any{
								map[string]any{"id": float64(1), "name": "first"},
								map[string]any{"id": float64(2), "name": "second"},
							},
						},
					},
				},
				{
					FunctionResponse: &genai.FunctionResponse{
						Name: "complex_function",
						Response: map[string]any{
							"success": true,
							"results": []any{
								map[string]any{
									"processed": true,
									"metadata": map[string]any{
										"timestamp": "2024-01-01T00:00:00Z",
										"version":   "1.0",
									},
								},
							},
						},
					},
				},
			},
		}

		aiPlatform := aiconv.ToAIPlatformContent(original)
		roundTrip := aiconv.FromAIPlatformContent(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("complex structure round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}

// Test edge cases with pointer fields.
func TestPointerFieldEdgeCases(t *testing.T) {
	t.Run("schema with all nil pointers", func(t *testing.T) {
		original := &genai.Schema{
			Type:        genai.TypeString,
			Description: "String with no constraints",
			Properties:  map[string]*genai.Schema{}, // Conversion creates empty map
			Nullable:    &[]bool{false}[0],          // Conversion sets this to false
		}

		aiPlatform := aiconv.ToAIPlatformSchema(original)
		roundTrip := aiconv.FromAIPlatformSchema(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("generation config with selective nil fields", func(t *testing.T) {
		temperature := float32(0.5)
		// topP and topK are nil

		original := &genai.GenerationConfig{
			Temperature:     &temperature,
			TopP:            nil,
			TopK:            nil,
			CandidateCount:  1,
			MaxOutputTokens: 0, // Zero value
		}

		aiPlatform := aiconv.ToAIPlatformGenerationConfig(original)
		roundTrip := aiconv.FromAIPlatformGenerationConfig(aiPlatform)

		if diff := cmp.Diff(original, roundTrip); diff != "" {
			t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
		}
	})
}
