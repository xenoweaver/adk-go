// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package vertexai

import (
	"context"
	"fmt"
	"log/slog"

	aiplatform "cloud.google.com/go/aiplatform/apiv1beta1"
	"cloud.google.com/go/auth/credentials"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log"
	nooplog "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/api/option"
	"google.golang.org/api/option/internaloption"

	"github.com/xenoweaver/adk-go/internal/vertexai/extension"
	"github.com/xenoweaver/adk-go/internal/vertexai/generativemodel"
	"github.com/xenoweaver/adk-go/internal/vertexai/preview/rag"
	"github.com/xenoweaver/adk-go/internal/vertexai/prompt"
	"github.com/xenoweaver/adk-go/pkg/logging"
)

// Client provides unified access to all Vertex AI functionality.
//
// The client orchestrates multiple specialized services to provide comprehensive access to GA and preview features of Vertex AI.
// It maintains a single authentication context and configuration across all services.
type Client struct {
	// Base configuration
	projectID string
	location  string

	// telemetry providers
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	loggerProvider log.LoggerProvider
	logger         *slog.Logger

	// Core services
	cacheClient        *aiplatform.GenAiCacheClient
	exampleStoreClient *aiplatform.ExampleStoreClient
	generativeService  generativemodel.Service
	modelGardenClient  *aiplatform.ModelGardenClient
	extensionService   extension.Service
	promptsService     prompt.Service

	// Previwe services
	ragClient             *rag.Service
	evaluationClient      *aiplatform.EvaluationClient
	reasoningengineClient *aiplatform.ReasoningEngineClient
	tuningClient          *aiplatform.GenAiTuningClient
}

// ClientOption is a functional option for configuring the client.
type ClientOption interface {
	apply(*Client)
}

type tracerOption struct {
	*internaloption.EmbeddableAdapter
	trace.TracerProvider
}

func (o tracerOption) apply(c *Client) {
	c.tracerProvider = o.TracerProvider
}

// WithTracerProvider sets the [trace.TracerProvider] for the client.
func WithTracerProvider(tracer trace.TracerProvider) option.ClientOption {
	return tracerOption{TracerProvider: tracer}
}

type meterOption struct {
	*internaloption.EmbeddableAdapter
	metric.MeterProvider
}

func (o meterOption) apply(c *Client) {
	c.meterProvider = o.MeterProvider
}

// WithMeterProvider sets the [metric.MeterProvider] for the client.
func WithMeterProvider(meter metric.MeterProvider) option.ClientOption {
	return meterOption{MeterProvider: meter}
}

type loggerOption struct {
	*internaloption.EmbeddableAdapter
	log.LoggerProvider
}

func (o loggerOption) apply(c *Client) {
	c.loggerProvider = o.LoggerProvider
}

// WithLoggerProvider sets the [log.LoggerProvider] for the client.
func WithLoggerProvider(logger log.LoggerProvider) option.ClientOption {
	return loggerOption{LoggerProvider: logger}
}

// NewClient creates a new Vertex AI [*Client].
//
// The client provides unified access to all services including RAG, caching, enhanced generative models, and Model Garden integration.
func NewClient(ctx context.Context, projectID, location string, options ...option.ClientOption) (*Client, error) {
	if projectID == "" {
		return nil, fmt.Errorf("projectID is required")
	}
	if location == "" {
		return nil, fmt.Errorf("location is required")
	}

	// Apply options
	opts := make([]ClientOption, 0, len(options))
	copts := make([]option.ClientOption, 0, len(options))
	for _, opt := range options {
		switch o := opt.(type) {
		case ClientOption:
			opts = append(opts, o)
		case option.ClientOption:
			copts = append(copts, o)
		}
	}

	client := &Client{
		projectID:      projectID,
		location:       location,
		tracerProvider: nooptrace.NewTracerProvider(),
		meterProvider:  noopmetric.NewMeterProvider(),
		loggerProvider: nooplog.NewLoggerProvider(),
	}
	for _, o := range opts {
		o.apply(client)
	}
	logger := otelslog.NewLogger("github.com/xenoweaver/adk-go/internal/vertexai", otelslog.WithLoggerProvider(client.loggerProvider))
	if client.logger == nil {
		client.logger = logger
	}
	copts = append(copts, option.WithLogger(logger))
	ctx = logging.NewContext(ctx, logger)

	// Create credentials
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes: aiplatform.DefaultAuthScopes(),
	})
	if err != nil {
		return nil, fmt.Errorf("detect default credentials: %w", err)
	}
	copts = append(copts, option.WithAuthCredentials(creds))

	// Initialize GenAI cache service client
	cacheClient, err := aiplatform.NewGenAiCacheClient(ctx, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize GenAI cache service: %w", err)
	}
	client.logger.InfoContext(ctx, "GenAI cache service initialized successfully",
		slog.String("project_id", projectID),
		slog.String("location", location),
	)
	client.cacheClient = cacheClient

	// Initialize example store service client
	exampleStoreClient, err := aiplatform.NewExampleStoreClient(ctx, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize example store service client: %w", err)
	}
	client.logger.InfoContext(ctx, "Example Store service initialized successfully",
		slog.String("project_id", projectID),
		slog.String("location", location),
	)
	client.exampleStoreClient = exampleStoreClient

	// Initialize generative models service
	generativeService, err := generativemodel.NewService(ctx, projectID, location, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize generative model service: %w", err)
	}
	client.generativeService = generativeService

	// Initialize model garden service client
	modelGardenClient, err := aiplatform.NewModelGardenClient(ctx, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize Model Garden service: %w", err)
	}
	client.logger.InfoContext(ctx, "Model Garden service initialized successfully",
		slog.String("project_id", projectID),
		slog.String("location", location),
	)
	client.modelGardenClient = modelGardenClient

	// Initialize Extension service
	extensionService, err := extension.NewService(ctx, projectID, location, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize Extension service: %w", err)
	}
	client.extensionService = extensionService

	// Initialize Prompts service
	promptsService, err := prompt.NewService(ctx, projectID, location, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize Prompt service: %w", err)
	}
	client.promptsService = promptsService

	// Initialize RAG client
	ragClient, err := rag.NewService(ctx, projectID, location, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize RAG client: %w", err)
	}
	client.ragClient = ragClient

	// Initialize evaluation service client
	evaluationClient, err := aiplatform.NewEvaluationClient(ctx, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize evaluation service client: %w", err)
	}
	client.logger.InfoContext(ctx, "Evaluation client initialized successfully",
		slog.String("project_id", projectID),
		slog.String("location", location),
	)
	client.evaluationClient = evaluationClient

	// Initialize Reasoning Engine Service
	reasoningengineClient, err := aiplatform.NewReasoningEngineClient(ctx, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize Reasoning Engine service: %w", err)
	}
	client.logger.InfoContext(ctx, "Reasoning Engine client initialized successfully",
		slog.String("project_id", projectID),
		slog.String("location", location),
	)
	client.reasoningengineClient = reasoningengineClient

	// Initialize GenAI Tuning Service
	tuningClient, err := aiplatform.NewGenAiTuningClient(ctx, copts...)
	if err != nil {
		return nil, fmt.Errorf("initialize Reasoning Engine service: %w", err)
	}
	client.logger.InfoContext(ctx, "GenAI Tuning service initialized successfully",
		slog.String("project_id", projectID),
		slog.String("location", location),
	)
	client.tuningClient = tuningClient

	client.logger.InfoContext(ctx, "Vertex AI client initialized successfully",
		slog.String("project_id", projectID),
		slog.String("location", location),
	)

	return client, nil
}

// Close closes the client and releases all resources.
//
// This method should be called when the client is no longer needed to ensure
// proper cleanup of underlying connections and resources.
func (c *Client) Close() error {
	c.logger.Info("Closing Vertex AI client")

	if err := c.cacheClient.Close(); err != nil {
		c.logger.Error("close caching service", slog.String("error", err.Error()))
		return fmt.Errorf("close caching service: %w", err)
	}

	if err := c.exampleStoreClient.Close(); err != nil {
		c.logger.Error("close example store service", slog.String("error", err.Error()))
		return fmt.Errorf("close example store service: %w", err)
	}

	if err := c.generativeService.Close(); err != nil {
		c.logger.Error("close generative models service", slog.String("error", err.Error()))
		return fmt.Errorf("close generative models service: %w", err)
	}

	if err := c.modelGardenClient.Close(); err != nil {
		c.logger.Error("close Model Garden service", slog.String("error", err.Error()))
		return fmt.Errorf("close Model Garden service: %w", err)
	}

	if err := c.extensionService.Close(); err != nil {
		c.logger.Error("close Extension service", slog.String("error", err.Error()))
		return fmt.Errorf("close Extension service: %w", err)
	}

	if err := c.promptsService.Close(); err != nil {
		c.logger.Error("close Prompts service", slog.String("error", err.Error()))
		return fmt.Errorf("close Prompts service: %w", err)
	}

	// Close all services
	if err := c.ragClient.Close(); err != nil {
		c.logger.Error("close RAG client", slog.String("error", err.Error()))
		return fmt.Errorf("close RAG client: %w", err)
	}

	if err := c.evaluationClient.Close(); err != nil {
		c.logger.Error("close Evaluation service", slog.String("error", err.Error()))
		return fmt.Errorf("close Evaluation service: %w", err)
	}

	if err := c.reasoningengineClient.Close(); err != nil {
		c.logger.Error("close Reasoning Engine service", slog.String("error", err.Error()))
		return fmt.Errorf("close Evaluation service: %w", err)
	}

	if err := c.tuningClient.Close(); err != nil {
		c.logger.Error("close Tuning service", slog.String("error", err.Error()))
		return fmt.Errorf("close Tuning service: %w", err)
	}

	c.logger.Info("Vertex AI client closed successfully")
	return nil
}

// Configuration Access Methods

// GetProjectID returns the configured Google Cloud project ID.
func (c *Client) GetProjectID() string {
	return c.projectID
}

// GetLocation returns the configured geographic location.
func (c *Client) GetLocation() string {
	return c.location
}

// GetLogger returns the configured logger instance.
func (c *Client) GetLogger() *slog.Logger {
	return c.logger
}

// Service Access Methods
//
// These methods provide access to individual services while maintaining
// the unified client context and configuration.

// Cache returns the caching service.
//
// The content caching service provides optimized caching for large content
// contexts, reducing token usage and improving performance for repeated queries.
func (c *Client) Cache() *aiplatform.GenAiCacheClient {
	return c.cacheClient
}

// ExampleStore returns the example store service.
//
// The example store service provides functionality for managing Example Stores,
// uploading examples, and performing similarity-based retrieval for few-shot learning.
func (c *Client) ExampleStore() *aiplatform.ExampleStoreClient {
	return c.exampleStoreClient
}

// GenerativeModel returns the enhanced generative models service.
//
// This service provides access to features for generative AI models,
// including advanced configuration options and experimental capabilities.
func (c *Client) GenerativeModel() generativemodel.Service {
	return c.generativeService
}

// ModelGarden returns the Model Garden service.
//
// The Model Garden service provides access to experimental and community models,
// including deployment and management capabilities.
func (c *Client) ModelGarden() *aiplatform.ModelGardenClient {
	return c.modelGardenClient
}

// Extension returns the Extension service.
//
// The Extension service provides access to Vertex AI Extension functionality,
// including creating, managing, and executing both custom and prebuilt extensions.
func (c *Client) Extension() extension.Service {
	return c.extensionService
}

// Prompt returns the Prompt service.
//
// The Prompt service provides comprehensive prompt management functionality,
// including creation, versioning, template processing, and cloud storage integration.
func (c *Client) Prompt() prompt.Service {
	return c.promptsService
}

// RAG returns the RAG (Retrieval-Augmented Generation) client.
//
// The RAG client provides comprehensive functionality for managing corpora,
// importing documents, and performing retrieval-augmented generation.
func (c *Client) RAG() *rag.Service {
	return c.ragClient
}

// Evaluation returns the Evaluation client
func (c *Client) Evaluation() *aiplatform.EvaluationClient {
	return c.evaluationClient
}

// ReasoningEngine returns the Reasoning Engine client.
func (c *Client) ReasoningEngine() *aiplatform.ReasoningEngineClient {
	return c.reasoningengineClient
}

// Tuning returns the Tuning client.
func (c *Client) Tuning() *aiplatform.GenAiTuningClient {
	return c.tuningClient
}

// Health Check and Status Methods

// HealthCheck performs a basic health check across all services.
//
// This method verifies that all underlying services are accessible and
// functioning correctly. It's useful for monitoring and debugging.
func (c *Client) HealthCheck(ctx context.Context) error {
	c.logger.InfoContext(ctx, "Performing client health check")

	// TODO(zchee): In a full implementation, you would perform actual health checks
	// against each service. For now, we just verify the services are initialized.

	if c.cacheClient == nil {
		return fmt.Errorf("content caching service not initialized")
	}

	if c.exampleStoreClient == nil {
		return fmt.Errorf("example store service not initialized")
	}

	if c.generativeService == nil {
		return fmt.Errorf("generative models service not initialized")
	}

	if c.modelGardenClient == nil {
		return fmt.Errorf("Model Garden service not initialized")
	}

	if c.extensionService == nil {
		return fmt.Errorf("Extension service not initialized")
	}

	if c.promptsService == nil {
		return fmt.Errorf("Prompts service not initialized")
	}

	if c.ragClient == nil {
		return fmt.Errorf("RAG client not initialized")
	}

	if c.evaluationClient == nil {
		return fmt.Errorf("Evaluation service not initialized")
	}

	if c.reasoningengineClient == nil {
		return fmt.Errorf("Reasoning Engine service not initialized")
	}

	if c.tuningClient == nil {
		return fmt.Errorf("Tuning service not initialized")
	}

	c.logger.InfoContext(ctx, "Preview client health check passed")
	return nil
}

// GetServiceStatus returns the status of all services.
func (c *Client) GetServiceStatus() map[string]string {
	status := make(map[string]string)

	if c.cacheClient != nil {
		status["cache"] = "initialized"
	} else {
		status["cache"] = "not_initialized"
	}

	if c.exampleStoreClient != nil {
		status["example_store"] = "initialized"
	} else {
		status["example_store"] = "not_initialized"
	}

	if c.generativeService != nil {
		status["generative_model"] = "initialized"
	} else {
		status["generative_model"] = "not_initialized"
	}

	if c.modelGardenClient != nil {
		status["model_garden"] = "initialized"
	} else {
		status["model_garden"] = "not_initialized"
	}

	if c.extensionService != nil {
		status["extension"] = "initialized"
	} else {
		status["extension"] = "not_initialized"
	}

	if c.promptsService != nil {
		status["prompt"] = "initialized"
	} else {
		status["prompt"] = "not_initialized"
	}

	if c.ragClient != nil {
		status["rag"] = "initialized"
	} else {
		status["rag"] = "not_initialized"
	}

	if c.evaluationClient != nil {
		status["evaluation"] = "initialized"
	} else {
		status["evaluation"] = "not_initialized"
	}

	if c.reasoningengineClient != nil {
		status["reasoning_engine"] = "initialized"
	} else {
		status["reasoning_engine"] = "not_initialized"
	}

	if c.tuningClient != nil {
		status["tuning"] = "initialized"
	} else {
		status["tuning"] = "not_initialized"
	}

	return status
}
