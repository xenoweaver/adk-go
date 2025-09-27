// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/tool"
)

// FunctionOption represents configuration options for function declaration building.
type FunctionOption func(*functionConfig)

// functionConfig holds configuration for building function declarations.
type functionConfig struct {
	name            string
	description     string
	paramDescs      map[string]string
	includeResponse bool
}

// WithName sets a custom name for the function declaration.
func WithName(name string) FunctionOption {
	return func(c *functionConfig) {
		c.name = name
	}
}

// WithDescription sets a description for the function declaration.
func WithDescription(description string) FunctionOption {
	return func(c *functionConfig) {
		c.description = description
	}
}

// WithParameterDescription sets a description for a specific parameter.
func WithParameterDescription(paramName, description string) FunctionOption {
	return func(c *functionConfig) {
		if c.paramDescs == nil {
			c.paramDescs = make(map[string]string)
		}
		c.paramDescs[paramName] = description
	}
}

// WithResponseSchema includes response schema in the function declaration.
func WithResponseSchema() FunctionOption {
	return func(c *functionConfig) {
		c.includeResponse = true
	}
}

// buildFunctionDeclaration automatically generates a [genai.FunctionDeclaration]
// from a Go function using reflection. It analyzes the function signature
// and maps Go types to JSON Schema types compatible with LLM function calling.
//
// The function should follow the pattern:
//
//	func MyTool(ctx context.Context, param1 Type1, param2 Type2) (ReturnType, error)
//
// Context parameters are automatically skipped. The function supports:
//   - Basic types: string, int, float, bool
//   - Complex types: slices, maps, structs
//   - Pointer types (treated as optional)
//   - Struct field tags for JSON property names
//
// Example:
//
//	func SearchTool(ctx context.Context, query string, limit int) ([]string, error) {
//	    // implementation
//	}
//
//	decl, err := buildFunctionDeclaration(SearchTool,
//	    WithName("search"),
//	    WithDescription("Search for items"),
//	    WithParameterDescription("query", "Search query string"),
//	    WithParameterDescription("limit", "Maximum number of results"),
//	)
func buildFunctionDeclaration(fn any, opts ...FunctionOption) (*genai.FunctionDeclaration, error) {
	// Validate input
	if fn == nil {
		return nil, fmt.Errorf("function cannot be nil")
	}

	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected function, got %T", fn)
	}

	funcType := v.Type()

	// Apply configuration options
	config := &functionConfig{}
	for _, opt := range opts {
		opt(config)
	}

	// Generate function name if not provided
	if config.name == "" {
		config.name = getFunctionName(v)
	}

	// Analyze function parameters
	paramsSchema, err := buildParametersSchema(funcType, config)
	if err != nil {
		return nil, fmt.Errorf("failed to build parameters schema: %w", err)
	}

	// Create function declaration
	decl := &genai.FunctionDeclaration{
		Name:        config.name,
		Description: config.description,
		Parameters:  paramsSchema,
		Behavior:    genai.BehaviorBlocking,
	}

	// Add response schema if requested
	if config.includeResponse {
		responseSchema, err := buildResponseSchema(funcType)
		if err != nil {
			return nil, fmt.Errorf("failed to build response schema: %w", err)
		}
		decl.Response = responseSchema
	}

	return decl, nil
}

// getFunctionName extracts the function name from reflection.
func getFunctionName(v reflect.Value) string {
	if !v.IsValid() {
		return "function"
	}

	ptr := v.Pointer()
	if ptr == 0 {
		return "function"
	}

	if fn := runtime.FuncForPC(ptr); fn != nil {
		name := fn.Name()
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[idx+1:]
		}
		// Remove closure suffixes like .func1
		if idx := strings.Index(name, ".func"); idx >= 0 {
			name = name[:idx]
		}
		if name != "" {
			return name
		}
	}

	return "function"
}

// buildParametersSchema creates a JSON schema for function parameters.
func buildParametersSchema(funcType reflect.Type, config *functionConfig) (*genai.Schema, error) {
	numParams := funcType.NumIn()
	properties := make(map[string]*genai.Schema)
	var required []string

	// Skip context.Context parameter if present
	startIdx := 0
	if numParams > 0 {
		firstParam := funcType.In(0)
		if isContextType(firstParam) {
			startIdx = 1
		}
	}

	// Process each parameter
	for i := startIdx; i < numParams; i++ {
		paramType := funcType.In(i)
		paramName := fmt.Sprintf("param%d", i-startIdx+1)

		// Try to get parameter name from function signature (limited in Go reflection)
		// In practice, you might need additional metadata or struct tags

		schema, err := typeToSchema(paramType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parameter %d type %v: %w", i, paramType, err)
		}

		// Add parameter description if provided
		if desc, ok := config.paramDescs[paramName]; ok {
			schema.Description = desc
		}

		properties[paramName] = schema

		// Non-pointer types are required
		if paramType.Kind() != reflect.Pointer {
			required = append(required, paramName)
		}
	}

	schema := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: properties,
	}

	if len(required) > 0 {
		schema.Required = required
	}

	return schema, nil
}

// buildResponseSchema creates a JSON schema for function return type.
func buildResponseSchema(funcType reflect.Type) (*genai.Schema, error) {
	numOut := funcType.NumOut()
	if numOut == 0 {
		return &genai.Schema{Type: genai.TypeObject}, nil
	}

	// Handle (result, error) pattern
	if numOut == 2 {
		returnType := funcType.Out(0)
		errorType := funcType.Out(1)

		// Check if second return is error interface
		if errorType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			return typeToSchema(returnType)
		}
	}

	// Handle single return type
	if numOut == 1 {
		return typeToSchema(funcType.Out(0))
	}

	// Multiple return values - create object with indexed properties
	properties := make(map[string]*genai.Schema)
	for i := range numOut {
		schema, err := typeToSchema(funcType.Out(i))
		if err != nil {
			return nil, err
		}
		properties[fmt.Sprintf("result%d", i)] = schema
	}

	return &genai.Schema{
		Type:       genai.TypeObject,
		Properties: properties,
	}, nil
}

// typeToSchema converts a Go reflect.Type to a genai.Schema.
func typeToSchema(t reflect.Type) (*genai.Schema, error) {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		return typeToSchema(t.Elem())
	}

	switch t.Kind() {
	case reflect.String:
		return &genai.Schema{Type: genai.TypeString}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &genai.Schema{Type: genai.TypeInteger}, nil

	case reflect.Float32, reflect.Float64:
		return &genai.Schema{Type: genai.TypeNumber}, nil

	case reflect.Bool:
		return &genai.Schema{Type: genai.TypeBoolean}, nil

	case reflect.Slice, reflect.Array:
		elemSchema, err := typeToSchema(t.Elem())
		if err != nil {
			return nil, err
		}
		return &genai.Schema{
			Type:  genai.TypeArray,
			Items: elemSchema,
		}, nil

	case reflect.Map:
		// Only support string keys for JSON compatibility
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map keys must be strings, got %v", t.Key().Kind())
		}

		// For maps, we represent them as objects without specific properties
		// The LLM will understand this as a map/dictionary structure
		return &genai.Schema{
			Type:        genai.TypeObject,
			Description: "Map with string keys",
		}, nil

	case reflect.Struct:
		return structToSchema(t)

	case reflect.Interface:
		// Handle interface{} / any type
		if t == reflect.TypeOf((*any)(nil)).Elem() {
			return &genai.Schema{}, nil // No type constraint
		}
		return &genai.Schema{Type: genai.TypeObject}, nil

	default:
		return nil, fmt.Errorf("unsupported type: %v", t.Kind())
	}
}

// structToSchema converts a struct type to a genai.Schema.
func structToSchema(t reflect.Type) (*genai.Schema, error) {
	properties := make(map[string]*genai.Schema)
	var required []string

	for i := range t.NumField() {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON field name
		fieldName := getJSONFieldName(field)
		if fieldName == "-" {
			continue // Skip fields with json:"-"
		}

		// Convert field type to schema
		fieldSchema, err := typeToSchema(field.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		properties[fieldName] = fieldSchema

		// Check if field is required (non-pointer, no omitempty)
		if isRequiredField(field) {
			required = append(required, fieldName)
		}
	}

	schema := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: properties,
	}

	if len(required) > 0 {
		schema.Required = required
	}

	return schema, nil
}

// getJSONFieldName extracts the JSON field name from struct field tags.
func getJSONFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return strings.ToLower(field.Name)
	}

	// Parse json tag (e.g., "field_name,omitempty")
	parts := strings.Split(tag, ",")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}

	return strings.ToLower(field.Name)
}

// isRequiredField determines if a struct field should be required in the schema.
func isRequiredField(field reflect.StructField) bool {
	// Pointer types are optional
	if field.Type.Kind() == reflect.Ptr {
		return false
	}

	// Check for omitempty tag
	tag := field.Tag.Get("json")
	if strings.Contains(tag, "omitempty") {
		return false
	}

	return true
}

// isContextType checks if a type is context.Context.
func isContextType(t reflect.Type) bool {
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	return t == contextType || t.Implements(contextType)
}

// newFunctionToolWithDeclaration creates a FunctionTool with automatically generated declaration.
func newFunctionToolWithDeclaration(fn Function, options ...FunctionOption) (*FunctionTool, error) {
	decl, err := buildFunctionDeclaration(fn, options...)
	if err != nil {
		return nil, err
	}

	// Create tool with the declaration's name and description
	functionTool := &FunctionTool{
		Tool:        tool.NewTool(decl.Name, decl.Description, false),
		fn:          fn,
		declaration: decl,
	}

	return functionTool, nil
}

// wrapFunction creates a Function compatible with the existing FunctionTool interface
// from a more naturally typed Go function.
//
// Example:
//
//	func SearchAPI(ctx context.Context, query string) ([]string, error) {
//	    // implementation
//	}
//
//	wrapped := wrapFunction(SearchAPI)
//	tool := NewFunctionTool(wrapped)
func wrapFunction[T, R any](fn func(context.Context, T) (R, error)) Function {
	return func(ctx context.Context, args map[string]any) (any, error) {
		// Convert args map to typed parameter
		var param T

		// Handle simple cases where T is a basic type
		rt := reflect.TypeOf(param)
		if rt.Kind() != reflect.Struct {
			// For basic types, expect a single parameter in the args
			if len(args) != 1 {
				return nil, fmt.Errorf("expected 1 parameter for type %T, got %d", param, len(args))
			}

			// Get the first (and only) value
			for _, v := range args {
				if converted, ok := v.(T); ok {
					param = converted
				} else {
					return nil, fmt.Errorf("cannot convert %T to %T", v, param)
				}
				break
			}
		} else {
			// For struct types, map fields from args
			paramValue := reflect.New(rt).Elem()
			for i := range rt.NumField() {
				field := rt.Field(i)
				if !field.IsExported() {
					continue
				}

				fieldName := getJSONFieldName(field)
				if fieldName == "-" {
					continue
				}

				if value, ok := args[fieldName]; ok {
					fieldValue := paramValue.Field(i)
					if fieldValue.CanSet() {
						valueReflect := reflect.ValueOf(value)
						if valueReflect.Type().ConvertibleTo(field.Type) {
							fieldValue.Set(valueReflect.Convert(field.Type))
						}
					}
				}
			}
			param = paramValue.Interface().(T)
		}

		result, err := fn(ctx, param)
		return result, err
	}
}
