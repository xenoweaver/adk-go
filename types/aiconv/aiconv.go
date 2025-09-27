// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package aiconv

import (
	"fmt"

	"cloud.google.com/go/aiplatform/apiv1beta1/aiplatformpb"
	"google.golang.org/genai"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/xenoweaver/adk-go/types"
)

// Content Conversions

// ToAIPlatformContent converts genai.Content to aiplatformpb.Content.
// Returns nil if input is nil.
func ToAIPlatformContent(content *genai.Content) *aiplatformpb.Content {
	if content == nil {
		return nil
	}

	result := &aiplatformpb.Content{
		Role: content.Role,
	}

	// Convert parts
	result.Parts = make([]*aiplatformpb.Part, len(content.Parts))
	for i, part := range content.Parts {
		result.Parts[i] = ToAIPlatformPart(part)
	}

	return result
}

// FromAIPlatformContent converts aiplatformpb.Content to genai.Content.
// Returns nil if input is nil.
func FromAIPlatformContent(content *aiplatformpb.Content) *genai.Content {
	if content == nil {
		return nil
	}

	result := &genai.Content{
		Role: content.Role,
	}

	// Convert parts
	result.Parts = make([]*genai.Part, len(content.Parts))
	for i, part := range content.Parts {
		result.Parts[i] = FromAIPlatformPart(part)
	}

	return result
}

// ToAIPlatformContents converts a slice of genai.Content to aiplatformpb.Content.
// Returns nil if input is nil.
func ToAIPlatformContents(contents []*genai.Content) []*aiplatformpb.Content {
	if contents == nil {
		return nil
	}

	result := make([]*aiplatformpb.Content, len(contents))
	for i, content := range contents {
		result[i] = ToAIPlatformContent(content)
	}
	return result
}

// FromAIPlatformContents converts a slice of aiplatformpb.Content to genai.Content.
// Returns nil if input is nil.
func FromAIPlatformContents(contents []*aiplatformpb.Content) []*genai.Content {
	if contents == nil {
		return nil
	}

	result := make([]*genai.Content, len(contents))
	for i, content := range contents {
		result[i] = FromAIPlatformContent(content)
	}
	return result
}

// Part Conversions

// ToAIPlatformPart converts genai.Part to aiplatformpb.Part.
// Returns nil if input is nil.
func ToAIPlatformPart(part *genai.Part) *aiplatformpb.Part {
	if part == nil {
		return nil
	}

	result := &aiplatformpb.Part{}

	switch {
	case part.Text != "":
		result.Data = &aiplatformpb.Part_Text{
			Text: part.Text,
		}

	case part.InlineData != nil:
		result.Data = &aiplatformpb.Part_InlineData{
			InlineData: &aiplatformpb.Blob{
				MimeType: part.InlineData.MIMEType,
				Data:     part.InlineData.Data,
			},
		}

	case part.FileData != nil:
		result.Data = &aiplatformpb.Part_FileData{
			FileData: &aiplatformpb.FileData{
				MimeType: part.FileData.MIMEType,
				FileUri:  part.FileData.FileURI,
			},
		}

	case part.FunctionCall != nil:
		result.Data = &aiplatformpb.Part_FunctionCall{
			FunctionCall: ToAIPlatformFunctionCall(part.FunctionCall),
		}

	case part.FunctionResponse != nil:
		result.Data = &aiplatformpb.Part_FunctionResponse{
			FunctionResponse: ToAIPlatformFunctionResponse(part.FunctionResponse),
		}

	case part.VideoMetadata != nil:
		result.Metadata = &aiplatformpb.Part_VideoMetadata{
			VideoMetadata: ToAIPlatformVideoMetadata(part.VideoMetadata),
		}

	default:
		panic(fmt.Errorf("unsupported genai.Part type: %+v", part))
	}

	return result
}

// FromAIPlatformPart converts aiplatformpb.Part to genai.Part.
// Returns nil if input is nil.
func FromAIPlatformPart(part *aiplatformpb.Part) *genai.Part {
	if part == nil {
		return nil
	}

	result := &genai.Part{}

	switch data := part.Data.(type) {
	case *aiplatformpb.Part_Text:
		result.Text = data.Text

	case *aiplatformpb.Part_InlineData:
		result.InlineData = &genai.Blob{
			MIMEType: data.InlineData.MimeType,
			Data:     data.InlineData.Data,
		}

	case *aiplatformpb.Part_FileData:
		result.FileData = &genai.FileData{
			MIMEType: data.FileData.MimeType,
			FileURI:  data.FileData.FileUri,
		}

	case *aiplatformpb.Part_FunctionCall:
		result.FunctionCall = FromAIPlatformFunctionCall(data.FunctionCall)

	case *aiplatformpb.Part_FunctionResponse:
		result.FunctionResponse = FromAIPlatformFunctionResponse(data.FunctionResponse)

	default:
		panic(fmt.Errorf("unsupported aiplatformpb.Part data type: %T", data))
	}

	// Handle metadata
	switch metadata := part.Metadata.(type) {
	case *aiplatformpb.Part_VideoMetadata:
		result.VideoMetadata = FromAIPlatformVideoMetadata(metadata.VideoMetadata)
	}

	return result
}

// FunctionCall Conversions

// ToAIPlatformFunctionCall converts genai.FunctionCall to aiplatformpb.FunctionCall.
// Returns nil if input is nil.
func ToAIPlatformFunctionCall(fc *genai.FunctionCall) *aiplatformpb.FunctionCall {
	if fc == nil {
		return nil
	}

	// Convert args to structpb.Struct
	var args *structpb.Struct
	if fc.Args != nil {
		var err error
		args, err = structpb.NewStruct(fc.Args)
		if err != nil {
			panic(fmt.Errorf("convert FunctionCall args to structpb.Struct: %w", err))
		}
	}

	return &aiplatformpb.FunctionCall{
		Name: fc.Name,
		Args: args,
	}
}

// FromAIPlatformFunctionCall converts aiplatformpb.FunctionCall to genai.FunctionCall.
// Returns nil if input is nil.
func FromAIPlatformFunctionCall(fc *aiplatformpb.FunctionCall) *genai.FunctionCall {
	if fc == nil {
		return nil
	}

	result := &genai.FunctionCall{
		Name: fc.Name,
	}

	// Convert args from structpb.Struct
	if fc.Args != nil {
		result.Args = fc.Args.AsMap()
	}

	return result
}

// FunctionResponse Conversions

// ToAIPlatformFunctionResponse converts genai.FunctionResponse to aiplatformpb.FunctionResponse.
// Returns nil if input is nil.
func ToAIPlatformFunctionResponse(fr *genai.FunctionResponse) *aiplatformpb.FunctionResponse {
	if fr == nil {
		return nil
	}

	// Convert response to structpb.Struct
	var response *structpb.Struct
	if fr.Response != nil {
		var err error
		response, err = structpb.NewStruct(fr.Response)
		if err != nil {
			panic(fmt.Errorf("convert FunctionResponse response to structpb.Struct: %w", err))
		}
	}

	return &aiplatformpb.FunctionResponse{
		Name:     fr.Name,
		Response: response,
	}
}

// FromAIPlatformFunctionResponse converts aiplatformpb.FunctionResponse to genai.FunctionResponse.
// Returns nil if input is nil.
func FromAIPlatformFunctionResponse(fr *aiplatformpb.FunctionResponse) *genai.FunctionResponse {
	if fr == nil {
		return nil
	}

	result := &genai.FunctionResponse{
		Name: fr.Name,
	}

	// Convert response from structpb.Struct
	if fr.Response != nil {
		result.Response = fr.Response.AsMap()
	}

	return result
}

// VideoMetadata Conversions

// ToAIPlatformVideoMetadata converts genai.VideoMetadata to aiplatformpb.VideoMetadata.
// Returns nil if input is nil.
func ToAIPlatformVideoMetadata(vm *genai.VideoMetadata) *aiplatformpb.VideoMetadata {
	if vm == nil {
		return nil
	}

	result := &aiplatformpb.VideoMetadata{}

	if vm.StartOffset != 0 {
		result.StartOffset = durationpb.New(vm.StartOffset)
	}
	if vm.EndOffset != 0 {
		result.EndOffset = durationpb.New(vm.EndOffset)
	}

	return result
}

// FromAIPlatformVideoMetadata converts aiplatformpb.VideoMetadata to genai.VideoMetadata.
// Returns nil if input is nil.
func FromAIPlatformVideoMetadata(vm *aiplatformpb.VideoMetadata) *genai.VideoMetadata {
	if vm == nil {
		return nil
	}

	result := &genai.VideoMetadata{}

	if vm.StartOffset != nil {
		result.StartOffset = vm.StartOffset.AsDuration()
	}
	if vm.EndOffset != nil {
		result.EndOffset = vm.EndOffset.AsDuration()
	}

	return result
}

// FunctionDeclaration Conversions

// ToAIPlatformFunctionDeclaration converts genai.FunctionDeclaration to aiplatformpb.FunctionDeclaration.
// Returns nil if input is nil.
func ToAIPlatformFunctionDeclaration(fd *genai.FunctionDeclaration) *aiplatformpb.FunctionDeclaration {
	if fd == nil {
		return nil
	}

	return &aiplatformpb.FunctionDeclaration{
		Name:        fd.Name,
		Description: fd.Description,
		Parameters:  ToAIPlatformSchema(fd.Parameters),
	}
}

// FromAIPlatformFunctionDeclaration converts aiplatformpb.FunctionDeclaration to genai.FunctionDeclaration.
// Returns nil if input is nil.
func FromAIPlatformFunctionDeclaration(fd *aiplatformpb.FunctionDeclaration) *genai.FunctionDeclaration {
	if fd == nil {
		return nil
	}

	return &genai.FunctionDeclaration{
		Name:        fd.Name,
		Description: fd.Description,
		Parameters:  FromAIPlatformSchema(fd.Parameters),
	}
}

// Schema Conversions

// ToAIPlatformSchema converts genai.Schema to aiplatformpb.Schema.
// Returns nil if input is nil.
func ToAIPlatformSchema(schema *genai.Schema) *aiplatformpb.Schema {
	if schema == nil {
		return nil
	}

	result := &aiplatformpb.Schema{
		Type:        ToAIPlatformType(schema.Type),
		Format:      schema.Format,
		Description: schema.Description,
		Enum:        schema.Enum,
		Example:     ToAIPlatformValue(schema.Example),
		Items:       ToAIPlatformSchema(schema.Items),
		Properties:  make(map[string]*aiplatformpb.Schema),
		Required:    schema.Required,
		Pattern:     schema.Pattern,
	}

	// Handle nullable
	if schema.Nullable != nil {
		result.Nullable = *schema.Nullable
	}

	// Handle optional int64 fields
	if schema.MinLength != nil {
		result.MinLength = *schema.MinLength
	}
	if schema.MaxLength != nil {
		result.MaxLength = *schema.MaxLength
	}
	if schema.MinItems != nil {
		result.MinItems = *schema.MinItems
	}
	if schema.MaxItems != nil {
		result.MaxItems = *schema.MaxItems
	}
	if schema.MinProperties != nil {
		result.MinProperties = *schema.MinProperties
	}
	if schema.MaxProperties != nil {
		result.MaxProperties = *schema.MaxProperties
	}

	// Handle optional float64 fields
	if schema.Minimum != nil {
		result.Minimum = *schema.Minimum
	}
	if schema.Maximum != nil {
		result.Maximum = *schema.Maximum
	}

	// Convert properties map
	for k, v := range schema.Properties {
		result.Properties[k] = ToAIPlatformSchema(v)
	}

	return result
}

// FromAIPlatformSchema converts aiplatformpb.Schema to genai.Schema.
// Returns nil if input is nil.
func FromAIPlatformSchema(schema *aiplatformpb.Schema) *genai.Schema {
	if schema == nil {
		return nil
	}

	result := &genai.Schema{
		Type:        FromAIPlatformType(schema.Type),
		Format:      schema.Format,
		Description: schema.Description,
		Enum:        schema.Enum,
		Example:     FromAIPlatformValue(schema.Example),
		Items:       FromAIPlatformSchema(schema.Items),
		Properties:  make(map[string]*genai.Schema),
		Required:    schema.Required,
		Pattern:     schema.Pattern,
	}

	// Handle nullable
	result.Nullable = &schema.Nullable

	// Handle int64 fields (convert to pointers)
	if schema.MinLength != 0 {
		result.MinLength = &schema.MinLength
	}
	if schema.MaxLength != 0 {
		result.MaxLength = &schema.MaxLength
	}
	if schema.MinItems != 0 {
		result.MinItems = &schema.MinItems
	}
	if schema.MaxItems != 0 {
		result.MaxItems = &schema.MaxItems
	}
	if schema.MinProperties != 0 {
		result.MinProperties = &schema.MinProperties
	}
	if schema.MaxProperties != 0 {
		result.MaxProperties = &schema.MaxProperties
	}

	// Handle float64 fields (convert to pointers)
	if schema.Minimum != 0 {
		result.Minimum = &schema.Minimum
	}
	if schema.Maximum != 0 {
		result.Maximum = &schema.Maximum
	}

	// Convert properties map
	for k, v := range schema.Properties {
		result.Properties[k] = FromAIPlatformSchema(v)
	}

	return result
}

// Type Conversions

// ToAIPlatformType converts genai.Type to aiplatformpb.Type.
func ToAIPlatformType(t genai.Type) aiplatformpb.Type {
	switch t {
	case genai.TypeUnspecified:
		return aiplatformpb.Type_TYPE_UNSPECIFIED
	case genai.TypeString:
		return aiplatformpb.Type_STRING
	case genai.TypeNumber:
		return aiplatformpb.Type_NUMBER
	case genai.TypeInteger:
		return aiplatformpb.Type_INTEGER
	case genai.TypeBoolean:
		return aiplatformpb.Type_BOOLEAN
	case genai.TypeArray:
		return aiplatformpb.Type_ARRAY
	case genai.TypeObject:
		return aiplatformpb.Type_OBJECT
	default:
		panic(fmt.Errorf("unknown genai.Type: %v", t))
	}
}

// FromAIPlatformType converts aiplatformpb.Type to genai.Type.
func FromAIPlatformType(t aiplatformpb.Type) genai.Type {
	switch t {
	case aiplatformpb.Type_TYPE_UNSPECIFIED:
		return genai.TypeUnspecified
	case aiplatformpb.Type_STRING:
		return genai.TypeString
	case aiplatformpb.Type_NUMBER:
		return genai.TypeNumber
	case aiplatformpb.Type_INTEGER:
		return genai.TypeInteger
	case aiplatformpb.Type_BOOLEAN:
		return genai.TypeBoolean
	case aiplatformpb.Type_ARRAY:
		return genai.TypeArray
	case aiplatformpb.Type_OBJECT:
		return genai.TypeObject
	default:
		panic(fmt.Errorf("unknown aiplatformpb.Type: %v", t))
	}
}

// Tool Conversions

// ToAIPlatformTool converts genai.Tool to aiplatformpb.Tool.
// Returns nil if input is nil.
func ToAIPlatformTool(tool *genai.Tool) *aiplatformpb.Tool {
	if tool == nil {
		return nil
	}

	result := &aiplatformpb.Tool{}

	// Convert function declarations
	if len(tool.FunctionDeclarations) > 0 {
		result.FunctionDeclarations = make([]*aiplatformpb.FunctionDeclaration, len(tool.FunctionDeclarations))
		for i, fd := range tool.FunctionDeclarations {
			result.FunctionDeclarations[i] = ToAIPlatformFunctionDeclaration(fd)
		}
	}

	// Convert CodeExecution if present
	if tool.CodeExecution != nil {
		result.CodeExecution = &aiplatformpb.Tool_CodeExecution{}
	}

	// Convert GoogleSearchRetrieval if present
	if tool.GoogleSearchRetrieval != nil {
		result.GoogleSearchRetrieval = ToAIPlatformGoogleSearchRetrieval(tool.GoogleSearchRetrieval)
	}

	return result
}

// FromAIPlatformTool converts aiplatformpb.Tool to genai.Tool.
// Returns nil if input is nil.
func FromAIPlatformTool(tool *aiplatformpb.Tool) *genai.Tool {
	if tool == nil {
		return nil
	}

	result := &genai.Tool{}

	// Convert function declarations
	if len(tool.FunctionDeclarations) > 0 {
		result.FunctionDeclarations = make([]*genai.FunctionDeclaration, len(tool.FunctionDeclarations))
		for i, fd := range tool.FunctionDeclarations {
			result.FunctionDeclarations[i] = FromAIPlatformFunctionDeclaration(fd)
		}
	}

	// Convert CodeExecution if present
	if tool.CodeExecution != nil {
		result.CodeExecution = &genai.ToolCodeExecution{}
	}

	// Convert GoogleSearchRetrieval if present
	if tool.GoogleSearchRetrieval != nil {
		result.GoogleSearchRetrieval = FromAIPlatformGoogleSearchRetrieval(tool.GoogleSearchRetrieval)
	}

	return result
}

// ToAIPlatformTools converts a slice of genai.Tool to aiplatformpb.Tool.
// Returns nil if input is nil.
func ToAIPlatformTools(tools []*genai.Tool) []*aiplatformpb.Tool {
	if tools == nil {
		return nil
	}

	result := make([]*aiplatformpb.Tool, len(tools))
	for i, tool := range tools {
		result[i] = ToAIPlatformTool(tool)
	}
	return result
}

// FromAIPlatformTools converts a slice of aiplatformpb.Tool to genai.Tool.
// Returns nil if input is nil.
func FromAIPlatformTools(tools []*aiplatformpb.Tool) []*genai.Tool {
	if tools == nil {
		return nil
	}

	result := make([]*genai.Tool, len(tools))
	for i, tool := range tools {
		result[i] = FromAIPlatformTool(tool)
	}
	return result
}

// GoogleSearchRetrieval Conversions

// ToAIPlatformGoogleSearchRetrieval converts genai.GoogleSearchRetrieval to aiplatformpb.GoogleSearchRetrieval.
// Returns nil if input is nil.
func ToAIPlatformGoogleSearchRetrieval(gsr *genai.GoogleSearchRetrieval) *aiplatformpb.GoogleSearchRetrieval {
	if gsr == nil {
		return nil
	}

	return &aiplatformpb.GoogleSearchRetrieval{}
}

// FromAIPlatformGoogleSearchRetrieval converts aiplatformpb.GoogleSearchRetrieval to genai.GoogleSearchRetrieval.
// Returns nil if input is nil.
func FromAIPlatformGoogleSearchRetrieval(gsr *aiplatformpb.GoogleSearchRetrieval) *genai.GoogleSearchRetrieval {
	if gsr == nil {
		return nil
	}

	return &genai.GoogleSearchRetrieval{}
}

// Helper Functions

// ToAIPlatformValue converts any to structpb.Value.
// Returns nil if input is nil.
func ToAIPlatformValue(v any) *structpb.Value {
	if v == nil {
		return nil
	}

	value, err := structpb.NewValue(v)
	if err != nil {
		panic(fmt.Errorf("convert value to structpb.Value: %w", err))
	}
	return value
}

// FromAIPlatformValue converts structpb.Value to any.
// Returns nil if input is nil.
func FromAIPlatformValue(v *structpb.Value) any {
	if v == nil {
		return nil
	}

	return v.AsInterface()
}

// GenerationConfig Conversions

// ToAIPlatformGenerationConfig converts genai.GenerationConfig to aiplatformpb.GenerationConfig.
// Returns nil if input is nil.
func ToAIPlatformGenerationConfig(gc *genai.GenerationConfig) *aiplatformpb.GenerationConfig {
	if gc == nil {
		return nil
	}

	result := &aiplatformpb.GenerationConfig{}

	if gc.Temperature != nil {
		result.Temperature = gc.Temperature
	}
	if gc.TopP != nil {
		result.TopP = gc.TopP
	}
	if gc.TopK != nil {
		result.TopK = gc.TopK
	}
	if gc.CandidateCount != 0 {
		result.CandidateCount = &gc.CandidateCount
	}
	if gc.MaxOutputTokens != 0 {
		result.MaxOutputTokens = &gc.MaxOutputTokens
	}

	// Convert stop sequences
	if gc.StopSequences != nil {
		result.StopSequences = make([]string, len(gc.StopSequences))
		copy(result.StopSequences, gc.StopSequences)
	}

	// Convert response MIME type
	if gc.ResponseMIMEType != "" {
		result.ResponseMimeType = gc.ResponseMIMEType
	}

	// Convert response schema
	if gc.ResponseSchema != nil {
		result.ResponseSchema = ToAIPlatformSchema(gc.ResponseSchema)
	}

	return result
}

// FromAIPlatformGenerationConfig converts aiplatformpb.GenerationConfig to genai.GenerationConfig.
// Returns nil if input is nil.
func FromAIPlatformGenerationConfig(gc *aiplatformpb.GenerationConfig) *genai.GenerationConfig {
	if gc == nil {
		return nil
	}

	result := &genai.GenerationConfig{}

	if gc.Temperature != nil {
		result.Temperature = gc.Temperature
	}
	if gc.TopP != nil {
		result.TopP = gc.TopP
	}
	if gc.TopK != nil {
		result.TopK = gc.TopK
	}
	if gc.CandidateCount != nil {
		result.CandidateCount = *gc.CandidateCount
	}
	if gc.MaxOutputTokens != nil {
		result.MaxOutputTokens = *gc.MaxOutputTokens
	}

	// Convert stop sequences
	if gc.StopSequences != nil {
		result.StopSequences = make([]string, len(gc.StopSequences))
		copy(result.StopSequences, gc.StopSequences)
	}

	// Convert response MIME type
	if gc.ResponseMimeType != "" {
		result.ResponseMIMEType = gc.ResponseMimeType
	}

	// Convert response schema
	if gc.ResponseSchema != nil {
		result.ResponseSchema = FromAIPlatformSchema(gc.ResponseSchema)
	}

	return result
}

// SafetySettings Conversions

// ToAIPlatformSafetySetting converts genai.SafetySetting to aiplatformpb.SafetySetting.
// Returns nil if input is nil.
func ToAIPlatformSafetySetting(ss *genai.SafetySetting) *aiplatformpb.SafetySetting {
	if ss == nil {
		return nil
	}

	return &aiplatformpb.SafetySetting{
		Category:  ToAIPlatformHarmCategory(ss.Category),
		Threshold: ToAIPlatformHarmBlockThreshold(ss.Threshold),
	}
}

// FromAIPlatformSafetySetting converts aiplatformpb.SafetySetting to genai.SafetySetting.
// Returns nil if input is nil.
func FromAIPlatformSafetySetting(ss *aiplatformpb.SafetySetting) *genai.SafetySetting {
	if ss == nil {
		return nil
	}

	return &genai.SafetySetting{
		Category:  FromAIPlatformHarmCategory(ss.Category),
		Threshold: FromAIPlatformHarmBlockThreshold(ss.Threshold),
	}
}

// ToAIPlatformSafetySettings converts a slice of genai.SafetySetting to aiplatformpb.SafetySetting.
// Returns nil if input is nil.
func ToAIPlatformSafetySettings(settings []*genai.SafetySetting) []*aiplatformpb.SafetySetting {
	if settings == nil {
		return nil
	}

	result := make([]*aiplatformpb.SafetySetting, len(settings))
	for i, setting := range settings {
		result[i] = ToAIPlatformSafetySetting(setting)
	}
	return result
}

// FromAIPlatformSafetySettings converts a slice of aiplatformpb.SafetySetting to genai.SafetySetting.
// Returns nil if input is nil.
func FromAIPlatformSafetySettings(settings []*aiplatformpb.SafetySetting) []*genai.SafetySetting {
	if settings == nil {
		return nil
	}

	result := make([]*genai.SafetySetting, len(settings))
	for i, setting := range settings {
		result[i] = FromAIPlatformSafetySetting(setting)
	}
	return result
}

// HarmCategory Conversions

// ToAIPlatformHarmCategory converts genai.HarmCategory to aiplatformpb.HarmCategory.
func ToAIPlatformHarmCategory(hc genai.HarmCategory) aiplatformpb.HarmCategory {
	switch hc {
	case genai.HarmCategoryUnspecified:
		return aiplatformpb.HarmCategory_HARM_CATEGORY_UNSPECIFIED
	case genai.HarmCategoryHarassment:
		return aiplatformpb.HarmCategory_HARM_CATEGORY_HARASSMENT
	case genai.HarmCategoryHateSpeech:
		return aiplatformpb.HarmCategory_HARM_CATEGORY_HATE_SPEECH
	case genai.HarmCategorySexuallyExplicit:
		return aiplatformpb.HarmCategory_HARM_CATEGORY_SEXUALLY_EXPLICIT
	case genai.HarmCategoryDangerousContent:
		return aiplatformpb.HarmCategory_HARM_CATEGORY_DANGEROUS_CONTENT
	default:
		panic(fmt.Errorf("unknown genai.HarmCategory: %v", hc))
	}
}

// FromAIPlatformHarmCategory converts aiplatformpb.HarmCategory to genai.HarmCategory.
func FromAIPlatformHarmCategory(hc aiplatformpb.HarmCategory) genai.HarmCategory {
	switch hc {
	case aiplatformpb.HarmCategory_HARM_CATEGORY_UNSPECIFIED:
		return genai.HarmCategoryUnspecified
	case aiplatformpb.HarmCategory_HARM_CATEGORY_HARASSMENT:
		return genai.HarmCategoryHarassment
	case aiplatformpb.HarmCategory_HARM_CATEGORY_HATE_SPEECH:
		return genai.HarmCategoryHateSpeech
	case aiplatformpb.HarmCategory_HARM_CATEGORY_SEXUALLY_EXPLICIT:
		return genai.HarmCategorySexuallyExplicit
	case aiplatformpb.HarmCategory_HARM_CATEGORY_DANGEROUS_CONTENT:
		return genai.HarmCategoryDangerousContent
	default:
		panic(fmt.Errorf("unknown aiplatformpb.HarmCategory: %v", hc))
	}
}

// HarmBlockThreshold Conversions

// ToAIPlatformHarmBlockThreshold converts genai.HarmBlockThreshold to aiplatformpb.SafetySetting_HarmBlockThreshold.
func ToAIPlatformHarmBlockThreshold(st genai.HarmBlockThreshold) aiplatformpb.SafetySetting_HarmBlockThreshold {
	switch st {
	case genai.HarmBlockThresholdUnspecified:
		return aiplatformpb.SafetySetting_HARM_BLOCK_THRESHOLD_UNSPECIFIED
	case genai.HarmBlockThresholdBlockLowAndAbove:
		return aiplatformpb.SafetySetting_BLOCK_LOW_AND_ABOVE
	case genai.HarmBlockThresholdBlockMediumAndAbove:
		return aiplatformpb.SafetySetting_BLOCK_MEDIUM_AND_ABOVE
	case genai.HarmBlockThresholdBlockOnlyHigh:
		return aiplatformpb.SafetySetting_BLOCK_ONLY_HIGH
	case genai.HarmBlockThresholdBlockNone:
		return aiplatformpb.SafetySetting_BLOCK_NONE
	default:
		panic(fmt.Errorf("unknown genai.HarmBlockThreshold: %v", st))
	}
}

// FromAIPlatformHarmBlockThreshold converts aiplatformpb.SafetySetting_HarmBlockThreshold to genai.HarmBlockThreshold.
func FromAIPlatformHarmBlockThreshold(st aiplatformpb.SafetySetting_HarmBlockThreshold) genai.HarmBlockThreshold {
	switch st {
	case aiplatformpb.SafetySetting_HARM_BLOCK_THRESHOLD_UNSPECIFIED:
		return genai.HarmBlockThresholdUnspecified
	case aiplatformpb.SafetySetting_BLOCK_LOW_AND_ABOVE:
		return genai.HarmBlockThresholdBlockLowAndAbove
	case aiplatformpb.SafetySetting_BLOCK_MEDIUM_AND_ABOVE:
		return genai.HarmBlockThresholdBlockMediumAndAbove
	case aiplatformpb.SafetySetting_BLOCK_ONLY_HIGH:
		return genai.HarmBlockThresholdBlockOnlyHigh
	case aiplatformpb.SafetySetting_BLOCK_NONE:
		return genai.HarmBlockThresholdBlockNone
	default:
		panic(fmt.Errorf("unknown aiplatformpb.SafetySetting_HarmBlockThreshold: %v", st))
	}
}

// Candidate Conversions

// ToAIPlatformCandidate converts genai.Candidate to aiplatformpb.Candidate.
// Returns nil if input is nil.
func ToAIPlatformCandidate(c *genai.Candidate) *aiplatformpb.Candidate {
	if c == nil {
		return nil
	}

	result := &aiplatformpb.Candidate{
		Index:              c.Index,
		Content:            ToAIPlatformContent(c.Content),
		FinishReason:       ToAIPlatformFinishReason(c.FinishReason),
		FinishMessage:      types.ToPtr(c.FinishMessage),
		SafetyRatings:      ToAIPlatformSafetyRatings(c.SafetyRatings),
		CitationMetadata:   ToAIPlatformCitationMetadata(c.CitationMetadata),
		AvgLogprobs:        c.AvgLogprobs,
		LogprobsResult:     ToAIPlatformLogprobsResult(c.LogprobsResult),
		GroundingMetadata:  ToAIPlatformGroundingMetadata(c.GroundingMetadata),
		UrlContextMetadata: ToAIPlatformURLContextMetadata(c.URLContextMetadata),
	}

	return result
}

// FromAIPlatformCandidate converts aiplatformpb.Candidate to genai.Candidate.
// Returns nil if input is nil.
func FromAIPlatformCandidate(c *aiplatformpb.Candidate) *genai.Candidate {
	if c == nil {
		return nil
	}

	result := &genai.Candidate{
		Index:              c.Index,
		Content:            FromAIPlatformContent(c.Content),
		FinishReason:       FromAIPlatformFinishReason(c.FinishReason),
		FinishMessage:      types.Deref(c.FinishMessage, ""),
		SafetyRatings:      FromAIPlatformSafetyRatings(c.SafetyRatings),
		CitationMetadata:   FromAIPlatformCitationMetadata(c.CitationMetadata),
		AvgLogprobs:        c.AvgLogprobs,
		LogprobsResult:     FromAIPlatformLogprobsResult(c.LogprobsResult),
		GroundingMetadata:  FromAIPlatformGroundingMetadata(c.GroundingMetadata),
		URLContextMetadata: FromAIPlatformURLContextMetadata(c.UrlContextMetadata),
	}

	return result
}

// ToAIPlatformCandidates converts a slice of genai.Candidate to aiplatformpb.Candidate.
// Returns nil if input is nil.
func ToAIPlatformCandidates(candidates []*genai.Candidate) []*aiplatformpb.Candidate {
	if candidates == nil {
		return nil
	}

	result := make([]*aiplatformpb.Candidate, len(candidates))
	for i, candidate := range candidates {
		result[i] = ToAIPlatformCandidate(candidate)
	}
	return result
}

// FromAIPlatformCandidates converts a slice of aiplatformpb.Candidate to genai.Candidate.
// Returns nil if input is nil.
func FromAIPlatformCandidates(candidates []*aiplatformpb.Candidate) []*genai.Candidate {
	if candidates == nil {
		return nil
	}

	result := make([]*genai.Candidate, len(candidates))
	for i, candidate := range candidates {
		result[i] = FromAIPlatformCandidate(candidate)
	}
	return result
}

// FinishReason Conversions

// ToAIPlatformFinishReason converts genai.FinishReason to aiplatformpb.Candidate_FinishReason.
func ToAIPlatformFinishReason(fr genai.FinishReason) aiplatformpb.Candidate_FinishReason {
	switch fr {
	case genai.FinishReasonUnspecified:
		return aiplatformpb.Candidate_FINISH_REASON_UNSPECIFIED
	case genai.FinishReasonStop:
		return aiplatformpb.Candidate_STOP
	case genai.FinishReasonMaxTokens:
		return aiplatformpb.Candidate_MAX_TOKENS
	case genai.FinishReasonSafety:
		return aiplatformpb.Candidate_SAFETY
	case genai.FinishReasonRecitation:
		return aiplatformpb.Candidate_RECITATION
	case genai.FinishReasonOther:
		return aiplatformpb.Candidate_OTHER
	default:
		panic(fmt.Errorf("unknown genai.FinishReason: %v", fr))
	}
}

// FromAIPlatformFinishReason converts aiplatformpb.Candidate_FinishReason to genai.FinishReason.
func FromAIPlatformFinishReason(fr aiplatformpb.Candidate_FinishReason) genai.FinishReason {
	switch fr {
	case aiplatformpb.Candidate_FINISH_REASON_UNSPECIFIED:
		return genai.FinishReasonUnspecified
	case aiplatformpb.Candidate_STOP:
		return genai.FinishReasonStop
	case aiplatformpb.Candidate_MAX_TOKENS:
		return genai.FinishReasonMaxTokens
	case aiplatformpb.Candidate_SAFETY:
		return genai.FinishReasonSafety
	case aiplatformpb.Candidate_RECITATION:
		return genai.FinishReasonRecitation
	case aiplatformpb.Candidate_OTHER:
		return genai.FinishReasonOther
	default:
		panic(fmt.Errorf("unknown aiplatformpb.Candidate_FinishReason: %v", fr))
	}
}

// SafetyRating Conversions

// ToAIPlatformSafetyRating converts genai.SafetyRating to aiplatformpb.SafetyRating.
// Returns nil if input is nil.
func ToAIPlatformSafetyRating(sr *genai.SafetyRating) *aiplatformpb.SafetyRating {
	if sr == nil {
		return nil
	}

	return &aiplatformpb.SafetyRating{
		Category:    ToAIPlatformHarmCategory(sr.Category),
		Probability: ToAIPlatformHarmProbability(sr.Probability),
		Blocked:     sr.Blocked,
	}
}

// FromAIPlatformSafetyRating converts aiplatformpb.SafetyRating to genai.SafetyRating.
// Returns nil if input is nil.
func FromAIPlatformSafetyRating(sr *aiplatformpb.SafetyRating) *genai.SafetyRating {
	if sr == nil {
		return nil
	}

	return &genai.SafetyRating{
		Category:    FromAIPlatformHarmCategory(sr.Category),
		Probability: FromAIPlatformHarmProbability(sr.Probability),
		Blocked:     sr.Blocked,
	}
}

// ToAIPlatformSafetyRatings converts a slice of genai.SafetyRating to aiplatformpb.SafetyRating.
// Returns nil if input is nil.
func ToAIPlatformSafetyRatings(ratings []*genai.SafetyRating) []*aiplatformpb.SafetyRating {
	if ratings == nil {
		return nil
	}

	result := make([]*aiplatformpb.SafetyRating, len(ratings))
	for i, rating := range ratings {
		result[i] = ToAIPlatformSafetyRating(rating)
	}
	return result
}

// FromAIPlatformSafetyRatings converts a slice of aiplatformpb.SafetyRating to genai.SafetyRating.
// Returns nil if input is nil.
func FromAIPlatformSafetyRatings(ratings []*aiplatformpb.SafetyRating) []*genai.SafetyRating {
	if ratings == nil {
		return nil
	}

	result := make([]*genai.SafetyRating, len(ratings))
	for i, rating := range ratings {
		result[i] = FromAIPlatformSafetyRating(rating)
	}
	return result
}

// HarmProbability Conversions

// ToAIPlatformHarmProbability converts genai.HarmProbability to aiplatformpb.SafetyRating_HarmProbability.
func ToAIPlatformHarmProbability(hp genai.HarmProbability) aiplatformpb.SafetyRating_HarmProbability {
	switch hp {
	case genai.HarmProbabilityUnspecified:
		return aiplatformpb.SafetyRating_HARM_PROBABILITY_UNSPECIFIED
	case genai.HarmProbabilityNegligible:
		return aiplatformpb.SafetyRating_NEGLIGIBLE
	case genai.HarmProbabilityLow:
		return aiplatformpb.SafetyRating_LOW
	case genai.HarmProbabilityMedium:
		return aiplatformpb.SafetyRating_MEDIUM
	case genai.HarmProbabilityHigh:
		return aiplatformpb.SafetyRating_HIGH
	default:
		panic(fmt.Errorf("unknown genai.HarmProbability: %v", hp))
	}
}

// FromAIPlatformHarmProbability converts aiplatformpb.SafetyRating_HarmProbability to genai.HarmProbability.
func FromAIPlatformHarmProbability(hp aiplatformpb.SafetyRating_HarmProbability) genai.HarmProbability {
	switch hp {
	case aiplatformpb.SafetyRating_HARM_PROBABILITY_UNSPECIFIED:
		return genai.HarmProbabilityUnspecified
	case aiplatformpb.SafetyRating_NEGLIGIBLE:
		return genai.HarmProbabilityNegligible
	case aiplatformpb.SafetyRating_LOW:
		return genai.HarmProbabilityLow
	case aiplatformpb.SafetyRating_MEDIUM:
		return genai.HarmProbabilityMedium
	case aiplatformpb.SafetyRating_HIGH:
		return genai.HarmProbabilityHigh
	default:
		panic(fmt.Errorf("unknown aiplatformpb.SafetyRating_HarmProbability: %v", hp))
	}
}

// CitationMetadata Conversions

// ToAIPlatformCitationMetadata converts genai.CitationMetadata to aiplatformpb.CitationMetadata.
// Returns nil if input is nil.
func ToAIPlatformCitationMetadata(cm *genai.CitationMetadata) *aiplatformpb.CitationMetadata {
	if cm == nil {
		return nil
	}

	result := &aiplatformpb.CitationMetadata{}

	// Convert citations
	if len(cm.Citations) > 0 {
		result.Citations = make([]*aiplatformpb.Citation, len(cm.Citations))
		for i, citation := range cm.Citations {
			result.Citations[i] = ToAIPlatformCitation(citation)
		}
	}

	return result
}

// FromAIPlatformCitationMetadata converts aiplatformpb.CitationMetadata to genai.CitationMetadata.
// Returns nil if input is nil.
func FromAIPlatformCitationMetadata(cm *aiplatformpb.CitationMetadata) *genai.CitationMetadata {
	if cm == nil {
		return nil
	}

	result := &genai.CitationMetadata{}

	// Convert citations
	if len(cm.Citations) > 0 {
		result.Citations = make([]*genai.Citation, len(cm.Citations))
		for i, citation := range cm.Citations {
			result.Citations[i] = FromAIPlatformCitation(citation)
		}
	}

	return result
}

// Citation Conversions

// ToAIPlatformCitation converts genai.Citation to aiplatformpb.Citation.
// Returns nil if input is nil.
func ToAIPlatformCitation(c *genai.Citation) *aiplatformpb.Citation {
	if c == nil {
		return nil
	}

	return &aiplatformpb.Citation{
		StartIndex: c.StartIndex,
		EndIndex:   c.EndIndex,
		Uri:        c.URI,
		Title:      c.Title,
		License:    c.License,
	}
}

// FromAIPlatformCitation converts aiplatformpb.Citation to genai.Citation.
// Returns nil if input is nil.
func FromAIPlatformCitation(cs *aiplatformpb.Citation) *genai.Citation {
	if cs == nil {
		return nil
	}

	result := &genai.Citation{
		StartIndex: cs.StartIndex,
		EndIndex:   cs.EndIndex,
		URI:        cs.Uri,
		Title:      cs.Title,
		License:    cs.License,
	}

	return result
}

// UsageMetadata Conversions

// ToAIPlatformUsageMetadata converts genai.GenerateContentResponseUsageMetadata to aiplatformpb.GenerateContentResponse_UsageMetadata.
// Returns nil if input is nil.
func ToAIPlatformUsageMetadata(um *genai.GenerateContentResponseUsageMetadata) *aiplatformpb.GenerateContentResponse_UsageMetadata {
	if um == nil {
		return nil
	}

	return &aiplatformpb.GenerateContentResponse_UsageMetadata{
		PromptTokenCount:        um.PromptTokenCount,
		CandidatesTokenCount:    um.CandidatesTokenCount,
		TotalTokenCount:         um.TotalTokenCount,
		CachedContentTokenCount: um.CachedContentTokenCount,
		ThoughtsTokenCount:      um.ThoughtsTokenCount,
		PromptTokensDetails:     ToAIPlatformModalityTokenCounts(um.PromptTokensDetails),
		CacheTokensDetails:      ToAIPlatformModalityTokenCounts(um.CacheTokensDetails),
		CandidatesTokensDetails: ToAIPlatformModalityTokenCounts(um.CandidatesTokensDetails),
	}
}

// FromAIPlatformUsageMetadata converts aiplatformpb.GenerateContentResponse_UsageMetadata to genai.GenerateContentResponseUsageMetadata.
// Returns nil if input is nil.
func FromAIPlatformUsageMetadata(um *aiplatformpb.GenerateContentResponse_UsageMetadata) *genai.GenerateContentResponseUsageMetadata {
	if um == nil {
		return nil
	}

	result := &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:        um.PromptTokenCount,
		CandidatesTokenCount:    um.CandidatesTokenCount,
		TotalTokenCount:         um.TotalTokenCount,
		CachedContentTokenCount: um.CachedContentTokenCount,
		ThoughtsTokenCount:      um.ThoughtsTokenCount,
		PromptTokensDetails:     FromAIPlatformModalityTokenCounts(um.PromptTokensDetails),
		CacheTokensDetails:      FromAIPlatformModalityTokenCounts(um.CacheTokensDetails),
		CandidatesTokensDetails: FromAIPlatformModalityTokenCounts(um.CandidatesTokensDetails),
	}

	return result
}

// PromptFeedback Conversions

// ToAIPlatformPromptFeedback converts genai.GenerateContentResponsePromptFeedback to aiplatformpb.GenerateContentResponse_PromptFeedback.
// Returns nil if input is nil.
func ToAIPlatformPromptFeedback(pf *genai.GenerateContentResponsePromptFeedback) *aiplatformpb.GenerateContentResponse_PromptFeedback {
	if pf == nil {
		return nil
	}

	result := &aiplatformpb.GenerateContentResponse_PromptFeedback{
		BlockReason:   ToAIPlatformBlockedReason(pf.BlockReason),
		SafetyRatings: ToAIPlatformSafetyRatings(pf.SafetyRatings),
	}

	return result
}

// FromAIPlatformPromptFeedback converts aiplatformpb.GenerateContentResponse_PromptFeedback to genai.GenerateContentResponsePromptFeedback.
// Returns nil if input is nil.
func FromAIPlatformPromptFeedback(pf *aiplatformpb.GenerateContentResponse_PromptFeedback) *genai.GenerateContentResponsePromptFeedback {
	if pf == nil {
		return nil
	}

	return &genai.GenerateContentResponsePromptFeedback{
		BlockReason:   FromAIPlatformBlockedReason(pf.BlockReason),
		SafetyRatings: FromAIPlatformSafetyRatings(pf.SafetyRatings),
	}
}

// BlockReason Conversions

// ToAIPlatformBlockedReason converts genai.BlockedReason to aiplatformpb.GenerateContentResponse_PromptFeedback_BlockedReason.
func ToAIPlatformBlockedReason(br genai.BlockedReason) aiplatformpb.GenerateContentResponse_PromptFeedback_BlockedReason {
	switch br {
	case genai.BlockedReasonUnspecified:
		return aiplatformpb.GenerateContentResponse_PromptFeedback_BLOCKED_REASON_UNSPECIFIED
	case genai.BlockedReasonSafety:
		return aiplatformpb.GenerateContentResponse_PromptFeedback_SAFETY
	case genai.BlockedReasonOther:
		return aiplatformpb.GenerateContentResponse_PromptFeedback_OTHER
	default:
		panic(fmt.Errorf("unknown genai.BlockedReason: %v", br))
	}
}

// FromAIPlatformBlockedReason converts aiplatformpb.GenerateContentResponse_PromptFeedback_BlockedReason to genai.BlockedReason.
func FromAIPlatformBlockedReason(br aiplatformpb.GenerateContentResponse_PromptFeedback_BlockedReason) genai.BlockedReason {
	switch br {
	case aiplatformpb.GenerateContentResponse_PromptFeedback_BLOCKED_REASON_UNSPECIFIED:
		return genai.BlockedReasonUnspecified
	case aiplatformpb.GenerateContentResponse_PromptFeedback_SAFETY:
		return genai.BlockedReasonSafety
	case aiplatformpb.GenerateContentResponse_PromptFeedback_OTHER:
		return genai.BlockedReasonOther
	default:
		panic(fmt.Errorf("unknown aiplatformpb.GenerateContentResponse_PromptFeedback_BlockedReason: %v", br))
	}
}

// GenerateContentResponse Conversions

// ToAIPlatformGenerateContentResponse converts genai.GenerateContentResponse to aiplatformpb.GenerateContentResponse.
// Returns nil if input is nil.
func ToAIPlatformGenerateContentResponse(resp *genai.GenerateContentResponse) *aiplatformpb.GenerateContentResponse {
	if resp == nil {
		return nil
	}

	result := &aiplatformpb.GenerateContentResponse{
		Candidates:     ToAIPlatformCandidates(resp.Candidates),
		PromptFeedback: ToAIPlatformPromptFeedback(resp.PromptFeedback),
		UsageMetadata:  ToAIPlatformUsageMetadata(resp.UsageMetadata),
	}

	return result
}

// FromAIPlatformGenerateContentResponse converts aiplatformpb.GenerateContentResponse to genai.GenerateContentResponse.
// Returns nil if input is nil.
func FromAIPlatformGenerateContentResponse(resp *aiplatformpb.GenerateContentResponse) *genai.GenerateContentResponse {
	if resp == nil {
		return nil
	}

	return &genai.GenerateContentResponse{
		Candidates:     FromAIPlatformCandidates(resp.Candidates),
		PromptFeedback: FromAIPlatformPromptFeedback(resp.PromptFeedback),
		UsageMetadata:  FromAIPlatformUsageMetadata(resp.UsageMetadata),
	}
}

// GroundingMetadata Conversions

// ToAIPlatformGroundingMetadata converts genai.GroundingMetadata to aiplatformpb.GroundingMetadata.
// Returns nil if input is nil.
func ToAIPlatformGroundingMetadata(gm *genai.GroundingMetadata) *aiplatformpb.GroundingMetadata {
	if gm == nil {
		return nil
	}

	result := &aiplatformpb.GroundingMetadata{}

	// Convert grounding supports
	if len(gm.GroundingSupports) > 0 {
		result.GroundingSupports = make([]*aiplatformpb.GroundingSupport, len(gm.GroundingSupports))
		for i, support := range gm.GroundingSupports {
			result.GroundingSupports[i] = ToAIPlatformGroundingSupport(support)
		}
	}

	// Convert web search queries
	if len(gm.WebSearchQueries) > 0 {
		result.WebSearchQueries = make([]string, len(gm.WebSearchQueries))
		copy(result.WebSearchQueries, gm.WebSearchQueries)
	}

	return result
}

// FromAIPlatformGroundingMetadata converts aiplatformpb.GroundingMetadata to genai.GroundingMetadata.
// Returns nil if input is nil.
func FromAIPlatformGroundingMetadata(gm *aiplatformpb.GroundingMetadata) *genai.GroundingMetadata {
	if gm == nil {
		return nil
	}

	result := &genai.GroundingMetadata{}

	// Convert grounding supports
	if len(gm.GroundingSupports) > 0 {
		result.GroundingSupports = make([]*genai.GroundingSupport, len(gm.GroundingSupports))
		for i, support := range gm.GroundingSupports {
			result.GroundingSupports[i] = FromAIPlatformGroundingSupport(support)
		}
	}

	// Convert web search queries
	if len(gm.WebSearchQueries) > 0 {
		result.WebSearchQueries = make([]string, len(gm.WebSearchQueries))
		copy(result.WebSearchQueries, gm.WebSearchQueries)
	}

	return result
}

// GroundingSupport Conversions

// ToAIPlatformGroundingSupport converts genai.GroundingSupport to aiplatformpb.GroundingSupport.
// Returns nil if input is nil.
func ToAIPlatformGroundingSupport(gs *genai.GroundingSupport) *aiplatformpb.GroundingSupport {
	if gs == nil {
		return nil
	}

	result := &aiplatformpb.GroundingSupport{
		ConfidenceScores:      gs.ConfidenceScores,
		GroundingChunkIndices: gs.GroundingChunkIndices,
	}

	// Handle segment
	if gs.Segment != nil {
		result.Segment = &aiplatformpb.Segment{
			PartIndex:  gs.Segment.PartIndex,
			StartIndex: gs.Segment.StartIndex,
			EndIndex:   gs.Segment.EndIndex,
			Text:       gs.Segment.Text,
		}
	}

	return result
}

// FromAIPlatformGroundingSupport converts aiplatformpb.GroundingSupport to genai.GroundingSupport.
// Returns nil if input is nil.
func FromAIPlatformGroundingSupport(gs *aiplatformpb.GroundingSupport) *genai.GroundingSupport {
	if gs == nil {
		return nil
	}

	result := &genai.GroundingSupport{
		ConfidenceScores:      gs.ConfidenceScores,
		GroundingChunkIndices: gs.GroundingChunkIndices,
	}

	// Handle segment
	if gs.Segment != nil {
		result.Segment = &genai.Segment{
			PartIndex:  gs.Segment.PartIndex,
			StartIndex: gs.Segment.StartIndex,
			EndIndex:   gs.Segment.EndIndex,
			Text:       gs.Segment.Text,
		}
	}

	return result
}

// FromFloat32Ptr converts *float32 to float32.
// Returns 0 if input is nil.
func FromFloat32Ptr(f *float32) float32 {
	if f == nil {
		return 0
	}
	return *f
}

// FromInt32PtrToInt32 converts *int32 to int32.
// Returns 0 if input is nil.
func FromInt32PtrToInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

// LogprobsResult Conversions

// ToAIPlatformLogprobsResult converts genai.LogprobsResult to aiplatformpb.LogprobsResult.
// Returns nil if input is nil.
func ToAIPlatformLogprobsResult(lr *genai.LogprobsResult) *aiplatformpb.LogprobsResult {
	if lr == nil {
		return nil
	}

	result := &aiplatformpb.LogprobsResult{}

	// Convert top candidates
	if len(lr.TopCandidates) > 0 {
		result.TopCandidates = make([]*aiplatformpb.LogprobsResult_TopCandidates, len(lr.TopCandidates))
		for i, tc := range lr.TopCandidates {
			result.TopCandidates[i] = &aiplatformpb.LogprobsResult_TopCandidates{}
			if len(tc.Candidates) > 0 {
				result.TopCandidates[i].Candidates = make([]*aiplatformpb.LogprobsResult_Candidate, len(tc.Candidates))
				for j, c := range tc.Candidates {
					result.TopCandidates[i].Candidates[j] = &aiplatformpb.LogprobsResult_Candidate{
						Token:          genai.Ptr(c.Token),
						TokenId:        &c.TokenID,
						LogProbability: genai.Ptr(c.LogProbability),
					}
				}
			}
		}
	}

	// Convert chosen candidates
	if len(lr.ChosenCandidates) > 0 {
		result.ChosenCandidates = make([]*aiplatformpb.LogprobsResult_Candidate, len(lr.ChosenCandidates))
		for i, c := range lr.ChosenCandidates {
			result.ChosenCandidates[i] = &aiplatformpb.LogprobsResult_Candidate{
				Token:          genai.Ptr(c.Token),
				TokenId:        &c.TokenID,
				LogProbability: genai.Ptr(c.LogProbability),
			}
		}
	}

	return result
}

// FromAIPlatformLogprobsResult converts aiplatformpb.LogprobsResult to genai.LogprobsResult.
// Returns nil if input is nil.
func FromAIPlatformLogprobsResult(lr *aiplatformpb.LogprobsResult) *genai.LogprobsResult {
	if lr == nil {
		return nil
	}

	result := &genai.LogprobsResult{}

	// Convert top candidates
	if len(lr.TopCandidates) > 0 {
		result.TopCandidates = make([]*genai.LogprobsResultTopCandidates, len(lr.TopCandidates))
		for i, tc := range lr.TopCandidates {
			result.TopCandidates[i] = &genai.LogprobsResultTopCandidates{}
			if len(tc.Candidates) > 0 {
				result.TopCandidates[i].Candidates = make([]*genai.LogprobsResultCandidate, len(tc.Candidates))
				for j, c := range tc.Candidates {
					result.TopCandidates[i].Candidates[j] = &genai.LogprobsResultCandidate{
						Token:          types.Deref(c.Token, ""),
						TokenID:        FromInt32PtrToInt32(c.TokenId),
						LogProbability: FromFloat32Ptr(c.LogProbability),
					}
				}
			}
		}
	}

	// Convert chosen candidates
	if len(lr.ChosenCandidates) > 0 {
		result.ChosenCandidates = make([]*genai.LogprobsResultCandidate, len(lr.ChosenCandidates))
		for i, c := range lr.ChosenCandidates {
			result.ChosenCandidates[i] = &genai.LogprobsResultCandidate{
				Token:          types.Deref(c.Token, ""),
				TokenID:        FromInt32PtrToInt32(c.TokenId),
				LogProbability: FromFloat32Ptr(c.LogProbability),
			}
		}
	}

	return result
}

// URLContextMetadata Conversions

// ToAIPlatformURLContextMetadata converts genai.URLContextMetadata to aiplatformpb.UrlContextMetadata.
// Returns nil if input is nil.
func ToAIPlatformURLContextMetadata(ucm *genai.URLContextMetadata) *aiplatformpb.UrlContextMetadata {
	if ucm == nil {
		return nil
	}

	result := &aiplatformpb.UrlContextMetadata{}

	// Convert URL metadata
	if len(ucm.URLMetadata) > 0 {
		result.UrlMetadata = make([]*aiplatformpb.UrlMetadata, len(ucm.URLMetadata))
		for i, um := range ucm.URLMetadata {
			result.UrlMetadata[i] = ToAIPlatformURLMetadata(um)
		}
	}

	return result
}

// FromAIPlatformURLContextMetadata converts aiplatformpb.UrlContextMetadata to genai.URLContextMetadata.
// Returns nil if input is nil.
func FromAIPlatformURLContextMetadata(ucm *aiplatformpb.UrlContextMetadata) *genai.URLContextMetadata {
	if ucm == nil {
		return nil
	}

	result := &genai.URLContextMetadata{}

	// Convert URL metadata
	if len(ucm.UrlMetadata) > 0 {
		result.URLMetadata = make([]*genai.URLMetadata, len(ucm.UrlMetadata))
		for i, um := range ucm.UrlMetadata {
			result.URLMetadata[i] = FromAIPlatformURLMetadata(um)
		}
	}

	return result
}

// ModalityTokenCount Conversions

// ToAIPlatformModalityTokenCount converts genai.ModalityTokenCount to aiplatformpb.ModalityTokenCount.
// Returns nil if input is nil.
func ToAIPlatformModalityTokenCount(mtc *genai.ModalityTokenCount) *aiplatformpb.ModalityTokenCount {
	if mtc == nil {
		return nil
	}

	return &aiplatformpb.ModalityTokenCount{
		Modality:   ToAIPlatformModality(mtc.Modality),
		TokenCount: mtc.TokenCount,
	}
}

// FromAIPlatformModalityTokenCount converts aiplatformpb.ModalityTokenCount to genai.ModalityTokenCount.
// Returns nil if input is nil.
func FromAIPlatformModalityTokenCount(mtc *aiplatformpb.ModalityTokenCount) *genai.ModalityTokenCount {
	if mtc == nil {
		return nil
	}

	return &genai.ModalityTokenCount{
		Modality:   FromAIPlatformModality(mtc.Modality),
		TokenCount: mtc.TokenCount,
	}
}

// ToAIPlatformModalityTokenCounts converts a slice of genai.ModalityTokenCount to aiplatformpb.ModalityTokenCount.
// Returns nil if input is nil.
func ToAIPlatformModalityTokenCounts(mtcs []*genai.ModalityTokenCount) []*aiplatformpb.ModalityTokenCount {
	if mtcs == nil {
		return nil
	}

	result := make([]*aiplatformpb.ModalityTokenCount, len(mtcs))
	for i, mtc := range mtcs {
		result[i] = ToAIPlatformModalityTokenCount(mtc)
	}
	return result
}

// FromAIPlatformModalityTokenCounts converts a slice of aiplatformpb.ModalityTokenCount to genai.ModalityTokenCount.
// Returns nil if input is nil.
func FromAIPlatformModalityTokenCounts(mtcs []*aiplatformpb.ModalityTokenCount) []*genai.ModalityTokenCount {
	if mtcs == nil {
		return nil
	}

	result := make([]*genai.ModalityTokenCount, len(mtcs))
	for i, mtc := range mtcs {
		result[i] = FromAIPlatformModalityTokenCount(mtc)
	}
	return result
}

// Modality Conversions

// ToAIPlatformModality converts genai.MediaModality to aiplatformpb.Modality.
func ToAIPlatformModality(modality genai.MediaModality) aiplatformpb.Modality {
	switch modality {
	case genai.MediaModalityUnspecified:
		return aiplatformpb.Modality_MODALITY_UNSPECIFIED
	case genai.MediaModalityText:
		return aiplatformpb.Modality_TEXT
	case genai.MediaModalityImage:
		return aiplatformpb.Modality_IMAGE
	case genai.MediaModalityAudio:
		return aiplatformpb.Modality_AUDIO
	case genai.MediaModalityVideo:
		return aiplatformpb.Modality_VIDEO
	default:
		panic(fmt.Errorf("unknown genai.MediaModality: %v", modality))
	}
}

// FromAIPlatformModality converts aiplatformpb.Modality to genai.MediaModality.
func FromAIPlatformModality(modality aiplatformpb.Modality) genai.MediaModality {
	switch modality {
	case aiplatformpb.Modality_MODALITY_UNSPECIFIED:
		return genai.MediaModalityUnspecified
	case aiplatformpb.Modality_TEXT:
		return genai.MediaModalityText
	case aiplatformpb.Modality_IMAGE:
		return genai.MediaModalityImage
	case aiplatformpb.Modality_AUDIO:
		return genai.MediaModalityAudio
	case aiplatformpb.Modality_VIDEO:
		return genai.MediaModalityVideo
	default:
		panic(fmt.Errorf("unknown aiplatformpb.Modality: %v", modality))
	}
}

// URLMetadata Conversions

// ToAIPlatformURLMetadata converts genai.URLMetadata to aiplatformpb.UrlMetadata.
// Returns nil if input is nil.
func ToAIPlatformURLMetadata(um *genai.URLMetadata) *aiplatformpb.UrlMetadata {
	if um == nil {
		return nil
	}

	return &aiplatformpb.UrlMetadata{
		RetrievedUrl:       um.RetrievedURL,
		UrlRetrievalStatus: ToAIPlatformURLRetrievalStatus(um.URLRetrievalStatus),
	}
}

// FromAIPlatformURLMetadata converts aiplatformpb.UrlMetadata to genai.URLMetadata.
// Returns nil if input is nil.
func FromAIPlatformURLMetadata(um *aiplatformpb.UrlMetadata) *genai.URLMetadata {
	if um == nil {
		return nil
	}

	return &genai.URLMetadata{
		RetrievedURL:       um.RetrievedUrl,
		URLRetrievalStatus: FromAIPlatformURLRetrievalStatus(um.UrlRetrievalStatus),
	}
}

// URL Retrieval Status Conversions

// ToAIPlatformURLRetrievalStatus converts genai.UrlRetrievalStatus to aiplatformpb.UrlMetadata_UrlRetrievalStatus.
func ToAIPlatformURLRetrievalStatus(status genai.UrlRetrievalStatus) aiplatformpb.UrlMetadata_UrlRetrievalStatus {
	switch status {
	case genai.URLRetrievalStatusUnspecified:
		return aiplatformpb.UrlMetadata_URL_RETRIEVAL_STATUS_UNSPECIFIED
	case genai.URLRetrievalStatusSuccess:
		return aiplatformpb.UrlMetadata_URL_RETRIEVAL_STATUS_SUCCESS
	case genai.URLRetrievalStatusError:
		return aiplatformpb.UrlMetadata_URL_RETRIEVAL_STATUS_ERROR
	default:
		panic(fmt.Errorf("unknown genai.UrlRetrievalStatus: %v", status))
	}
}

// FromAIPlatformURLRetrievalStatus converts aiplatformpb.UrlMetadata_UrlRetrievalStatus to genai.UrlRetrievalStatus.
func FromAIPlatformURLRetrievalStatus(status aiplatformpb.UrlMetadata_UrlRetrievalStatus) genai.UrlRetrievalStatus {
	switch status {
	case aiplatformpb.UrlMetadata_URL_RETRIEVAL_STATUS_UNSPECIFIED:
		return genai.URLRetrievalStatusUnspecified
	case aiplatformpb.UrlMetadata_URL_RETRIEVAL_STATUS_SUCCESS:
		return genai.URLRetrievalStatusSuccess
	case aiplatformpb.UrlMetadata_URL_RETRIEVAL_STATUS_ERROR:
		return genai.URLRetrievalStatusError
	default:
		panic(fmt.Errorf("unknown aiplatformpb.UrlMetadata_UrlRetrievalStatus: %v", status))
	}
}
