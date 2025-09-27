// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	aiplatform "cloud.google.com/go/aiplatform/apiv1beta1"
	"google.golang.org/api/option"

	"github.com/xenoweaver/adk-go/pkg/logging"
)

// Service provides comprehensive prompt management functionality for Vertex AI.
//
// The service enables creation, management, versioning, and deployment of prompt templates
// for use with Vertex AI generative models, mirroring the functionality of Python's
// vertexai.prompts module.
type Service interface {
	GetProjectID() string
	GetLocation() string
	GetCacheStats() map[string]any
	ClearCache()
	CreatePrompt(ctx context.Context, req *CreatePromptRequest) (*Prompt, error)
	GetPrompt(ctx context.Context, req *GetPromptRequest) (*Prompt, error)
	UpdatePrompt(ctx context.Context, req *UpdatePromptRequest) (*Prompt, error)
	DeletePrompt(ctx context.Context, req *DeletePromptRequest) error
	ListPrompts(ctx context.Context, req *ListPromptsRequest) (*ListPromptsResponse, error)
	CreateVersion(ctx context.Context, req *CreateVersionRequest) (*PromptVersion, error)
	GetVersion(ctx context.Context, promptID, versionID string) (*PromptVersion, error)
	ListVersions(ctx context.Context, req *ListVersionsRequest) (*ListVersionsResponse, error)
	RestoreVersion(ctx context.Context, req *RestoreVersionRequest) (*PromptVersion, error)
	DeleteVersion(ctx context.Context, promptID, versionID string) error
	Close() error
}

type service struct {
	// AI Platform clients
	predictionClient *aiplatform.PredictionClient
	notebookClient   *aiplatform.NotebookClient

	// Configuration
	projectID string
	location  string
	logger    *slog.Logger

	// Internal storage and caching
	promptCache  map[string]*Prompt
	versionCache map[string][]*PromptVersion
	cacheMutex   sync.RWMutex
	cacheExpiry  time.Duration

	// Template engine
	templateEngine *TemplateProcessor

	// Metrics tracking
	metrics *MetricsCollector

	// Service state
	initialized bool
	mu          sync.RWMutex
}

var _ Service = (*service)(nil)

// ServiceOption is a functional option for configuring the prompts service.
type ServiceOption func(*service)

// WithLogger sets a custom logger for the service.
func WithLogger(logger *slog.Logger) ServiceOption {
	return func(s *service) {
		s.logger = logger
	}
}

// WithCacheExpiry sets the cache expiry duration for prompts.
func WithCacheExpiry(duration time.Duration) ServiceOption {
	return func(s *service) {
		s.cacheExpiry = duration
	}
}

// WithTemplateEngine sets a custom template processor.
func WithTemplateEngine(engine *TemplateProcessor) ServiceOption {
	return func(s *service) {
		s.templateEngine = engine
	}
}

// WithMetricsCollector sets a custom metrics collector.
func WithMetricsCollector(collector *MetricsCollector) ServiceOption {
	return func(s *service) {
		s.metrics = collector
	}
}

// NewService creates a new Vertex AI prompts service.
//
// The service provides comprehensive prompt management capabilities including
// creation, versioning, template processing, and cloud storage integration.
//
// Parameters:
//   - ctx: Context for the initialization
//   - projectID: Google Cloud project ID
//   - location: Geographic location for Vertex AI services (e.g., "us-central1")
//   - opts: Optional configuration options
//
// Returns a fully initialized prompts service or an error if initialization fails.
func NewService(ctx context.Context, projectID, location string, opts ...option.ClientOption) (*service, error) {
	if projectID == "" {
		return nil, NewInvalidRequestError("projectID", "cannot be empty")
	}
	if location == "" {
		return nil, NewInvalidRequestError("location", "cannot be empty")
	}

	service := &service{
		projectID:      projectID,
		location:       location,
		logger:         logging.FromContext(ctx),
		promptCache:    make(map[string]*Prompt),
		versionCache:   make(map[string][]*PromptVersion),
		cacheExpiry:    30 * time.Minute,
		templateEngine: NewTemplateProcessor(),
		metrics:        NewMetricsCollector(),
	}

	// Initialize AI Platform prediction client
	predictionClient, err := aiplatform.NewPredictionClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI Platform prediction client: %w", err)
	}
	service.predictionClient = predictionClient

	// Initialize AI Platform notebook client
	notebookClient, err := aiplatform.NewNotebookClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI Platform notebook client: %w", err)
	}
	service.notebookClient = notebookClient

	service.initialized = true

	service.logger.InfoContext(ctx, "Vertex AI prompts service initialized successfully",
		slog.String("project_id", projectID),
		slog.String("location", location),
	)

	return service, nil
}

// Close closes the prompts service and releases all resources.
//
// This method should be called when the service is no longer needed to ensure
// proper cleanup of underlying connections and resources.
func (s *service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return nil
	}

	s.logger.Info("Closing Vertex AI prompts service")

	// Close AI Platform clients
	if s.predictionClient != nil {
		if err := s.predictionClient.Close(); err != nil {
			s.logger.Error("Failed to close prediction client", slog.String("error", err.Error()))
			return fmt.Errorf("failed to close prediction client: %w", err)
		}
	}

	if s.notebookClient != nil {
		if err := s.notebookClient.Close(); err != nil {
			s.logger.Error("Failed to close notebook client", slog.String("error", err.Error()))
			return fmt.Errorf("failed to close notebook client: %w", err)
		}
	}

	// Clear caches
	s.cacheMutex.Lock()
	s.promptCache = make(map[string]*Prompt)
	s.versionCache = make(map[string][]*PromptVersion)
	s.cacheMutex.Unlock()

	s.initialized = false
	s.logger.Info("Vertex AI prompts service closed successfully")

	return nil
}

// Prompt CRUD Operations

// CreatePrompt creates a new prompt and optionally saves it to cloud storage.
//
// The prompt is validated before creation, and if save_to_cloud is true,
// it will be stored as an online resource accessible via the Google Cloud console.
func (s *service) CreatePrompt(ctx context.Context, req *CreatePromptRequest) (*Prompt, error) {
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}

	// Validate the prompt template
	if req.ValidateTemplate {
		if err := s.validatePromptTemplate(req.Prompt); err != nil {
			return nil, err
		}
	}

	// Check if prompt with this name already exists
	if req.Prompt.Name != "" {
		if existing, err := s.getPromptByName(ctx, req.Prompt.Name); err == nil && existing != nil {
			return nil, NewPromptAlreadyExistsError(req.Prompt.Name)
		}
	}

	// Generate ID if not provided
	if req.Prompt.ID == "" {
		req.Prompt.ID = s.generatePromptID()
	}

	// Set metadata
	now := time.Now()
	req.Prompt.CreatedAt = now
	req.Prompt.UpdatedAt = now
	req.Prompt.ProjectID = s.projectID
	req.Prompt.Location = s.location

	// Perform dry run if requested
	if req.DryRun {
		return req.Prompt, nil
	}

	// Save to cloud storage (simulated - actual implementation would call Vertex AI APIs)
	if err := s.savePromptToCloud(ctx, req.Prompt); err != nil {
		return nil, fmt.Errorf("failed to save prompt to cloud: %w", err)
	}

	// Create initial version if requested
	if req.CreateVersion {
		versionName := req.VersionName
		if versionName == "" {
			versionName = "v1"
		}

		version, err := s.createPromptVersion(ctx, req.Prompt.ID, req.Prompt, versionName, "Initial version")
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to create initial version",
				slog.String("prompt_id", req.Prompt.ID),
				slog.String("error", err.Error()),
			)
		} else {
			req.Prompt.VersionID = version.VersionID
		}
	}

	// Cache the prompt
	s.cachePrompt(req.Prompt)

	// Track metrics
	s.metrics.IncrementPromptCreated()

	s.logger.InfoContext(ctx, "Prompt created successfully",
		slog.String("prompt_id", req.Prompt.ID),
		slog.String("name", req.Prompt.Name),
	)

	return req.Prompt, nil
}

// GetPrompt retrieves a prompt by ID or name.
func (s *service) GetPrompt(ctx context.Context, req *GetPromptRequest) (*Prompt, error) {
	if req.PromptID == "" && req.Name == "" {
		return nil, NewInvalidRequestError("prompt_id_or_name", "either prompt_id or name must be specified")
	}

	var prompt *Prompt
	var err error

	// Try to get from cache first
	if req.PromptID != "" {
		prompt = s.getCachedPrompt(req.PromptID)
	} else {
		prompt, err = s.getPromptByName(ctx, req.Name)
		if err != nil {
			return nil, err
		}
	}

	// If not in cache, load from cloud storage
	if prompt == nil {
		prompt, err = s.loadPromptFromCloud(ctx, req.PromptID, req.Name)
		if err != nil {
			if req.PromptID != "" {
				return nil, NewPromptNotFoundError(req.PromptID)
			}
			return nil, NewPromptNotFoundError(req.Name)
		}
		s.cachePrompt(prompt)
	}

	// Load specific version if requested
	if req.VersionID != "" && req.VersionID != prompt.VersionID {
		version, err := s.getPromptVersion(ctx, prompt.ID, req.VersionID)
		if err != nil {
			return nil, err
		}
		prompt = s.promptFromVersion(prompt, version)
	}

	// Include version history if requested
	if req.IncludeVersions {
		versions, err := s.listPromptVersions(ctx, prompt.ID)
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to load version history",
				slog.String("prompt_id", prompt.ID),
				slog.String("error", err.Error()),
			)
		}
		// Add versions to metadata
		if prompt.Metadata == nil {
			prompt.Metadata = make(map[string]any)
		}
		prompt.Metadata["versions"] = versions
	}

	// Include usage metrics if requested
	if req.IncludeUsage {
		usage, err := s.getPromptMetrics(ctx, prompt.ID)
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to load usage metrics",
				slog.String("prompt_id", prompt.ID),
				slog.String("error", err.Error()),
			)
		} else {
			if prompt.Metadata == nil {
				prompt.Metadata = make(map[string]any)
			}
			prompt.Metadata["usage"] = usage
		}
	}

	s.metrics.IncrementPromptRetrieved()

	return prompt, nil
}

// UpdatePrompt updates an existing prompt.
func (s *service) UpdatePrompt(ctx context.Context, req *UpdatePromptRequest) (*Prompt, error) {
	if req.Prompt.ID == "" && req.Prompt.Name == "" {
		return nil, NewInvalidRequestError("prompt_id_or_name", "either prompt_id or name must be specified")
	}

	// Get the current prompt
	existing, err := s.GetPrompt(ctx, &GetPromptRequest{
		PromptID: req.Prompt.ID,
		Name:     req.Prompt.Name,
	})
	if err != nil {
		return nil, err
	}

	// Check ETag for optimistic locking
	if req.IfMatchETag != "" && existing.Metadata != nil {
		if etag, ok := existing.Metadata["etag"].(string); ok && etag != req.IfMatchETag {
			return nil, NewVersionConflictError(existing.ID, req.IfMatchETag, etag)
		}
	}

	// Validate the updated template
	if req.ValidateTemplate {
		if err := s.validatePromptTemplate(req.Prompt); err != nil {
			return nil, err
		}
	}

	// Create new version if requested
	if req.CreateNewVersion {
		versionName := req.VersionName
		if versionName == "" {
			versionName = fmt.Sprintf("v%d", time.Now().Unix())
		}

		version, err := s.createPromptVersion(ctx, existing.ID, req.Prompt, versionName, req.Changelog)
		if err != nil {
			return nil, fmt.Errorf("failed to create new version: %w", err)
		}
		req.Prompt.VersionID = version.VersionID
	}

	// Update metadata
	req.Prompt.ID = existing.ID
	req.Prompt.UpdatedAt = time.Now()
	req.Prompt.CreatedAt = existing.CreatedAt
	req.Prompt.ProjectID = s.projectID
	req.Prompt.Location = s.location

	// Save to cloud storage
	if err := s.savePromptToCloud(ctx, req.Prompt); err != nil {
		return nil, fmt.Errorf("failed to save updated prompt to cloud: %w", err)
	}

	// Update cache
	s.cachePrompt(req.Prompt)

	s.metrics.IncrementPromptUpdated()

	s.logger.InfoContext(ctx, "Prompt updated successfully",
		slog.String("prompt_id", req.Prompt.ID),
		slog.String("name", req.Prompt.Name),
	)

	return req.Prompt, nil
}

// DeletePrompt deletes a prompt and optionally all its versions.
func (s *service) DeletePrompt(ctx context.Context, req *DeletePromptRequest) error {
	if req.PromptID == "" && req.Name == "" {
		return NewInvalidRequestError("prompt_id_or_name", "either prompt_id or name must be specified")
	}

	// Get the prompt to ensure it exists
	prompt, err := s.GetPrompt(ctx, &GetPromptRequest{
		PromptID: req.PromptID,
		Name:     req.Name,
	})
	if err != nil {
		return err
	}

	// Check ETag for optimistic locking
	if req.IfMatchETag != "" && prompt.Metadata != nil {
		if etag, ok := prompt.Metadata["etag"].(string); ok && etag != req.IfMatchETag {
			return NewVersionConflictError(prompt.ID, req.IfMatchETag, etag)
		}
	}

	// Delete all versions if requested
	if req.DeleteVersions {
		versions, err := s.listPromptVersions(ctx, prompt.ID)
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to list versions for deletion",
				slog.String("prompt_id", prompt.ID),
				slog.String("error", err.Error()),
			)
		} else {
			for _, version := range versions {
				if err := s.deletePromptVersion(ctx, prompt.ID, version.VersionID); err != nil {
					s.logger.WarnContext(ctx, "Failed to delete version",
						slog.String("prompt_id", prompt.ID),
						slog.String("version_id", version.VersionID),
						slog.String("error", err.Error()),
					)
				}
			}
		}
	}

	// Delete from cloud storage
	if err := s.deletePromptFromCloud(ctx, prompt.ID); err != nil {
		return fmt.Errorf("failed to delete prompt from cloud: %w", err)
	}

	// Remove from cache
	s.removeCachedPrompt(prompt.ID)

	s.metrics.IncrementPromptDeleted()

	s.logger.InfoContext(ctx, "Prompt deleted successfully",
		slog.String("prompt_id", prompt.ID),
		slog.String("name", prompt.Name),
	)

	return nil
}

// ListPrompts lists prompts with filtering, searching, and pagination.
func (s *service) ListPrompts(ctx context.Context, req *ListPromptsRequest) (*ListPromptsResponse, error) {
	// Load prompts from cloud storage (simulated)
	prompts, nextToken, totalSize, err := s.listPromptsFromCloud(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts from cloud: %w", err)
	}

	// Apply additional filtering and processing
	filteredPrompts := s.filterPrompts(prompts, req)

	// Cache the loaded prompts
	for _, prompt := range filteredPrompts {
		s.cachePrompt(prompt)
	}

	s.metrics.IncrementPromptsListed(int64(len(filteredPrompts)))

	return &ListPromptsResponse{
		Prompts:       filteredPrompts,
		NextPageToken: nextToken,
		TotalSize:     totalSize,
	}, nil
}

// Helper methods for the service implementation

func (s *service) validateRequest(req *CreatePromptRequest) error {
	if req == nil {
		return NewInvalidRequestError("request", "cannot be nil")
	}
	if req.Prompt == nil {
		return NewInvalidRequestError("prompt", "cannot be nil")
	}
	if req.Prompt.Template == "" {
		return NewInvalidRequestError("template", "cannot be empty")
	}
	return nil
}

func (s *service) validatePromptTemplate(prompt *Prompt) error {
	return s.templateEngine.ValidateTemplate(prompt.Template, prompt.Variables)
}

func (s *service) generatePromptID() string {
	return fmt.Sprintf("prompt_%d", time.Now().UnixNano())
}

func (s *service) cachePrompt(prompt *Prompt) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.promptCache[prompt.ID] = prompt
}

func (s *service) getCachedPrompt(promptID string) *Prompt {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return s.promptCache[promptID]
}

func (s *service) removeCachedPrompt(promptID string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	delete(s.promptCache, promptID)
}

// Placeholder methods for cloud operations (to be implemented with actual Vertex AI APIs)

func (s *service) savePromptToCloud(ctx context.Context, prompt *Prompt) error {
	// This would implement the actual Vertex AI API call to save the prompt
	s.logger.InfoContext(ctx, "Saving prompt to cloud storage", slog.String("prompt_id", prompt.ID))
	return nil
}

func (s *service) loadPromptFromCloud(ctx context.Context, promptID, name string) (*Prompt, error) {
	// This would implement the actual Vertex AI API call to load the prompt
	s.logger.InfoContext(ctx, "Loading prompt from cloud storage",
		slog.String("prompt_id", promptID),
		slog.String("name", name))
	return nil, ErrPromptNotFound
}

func (s *service) deletePromptFromCloud(ctx context.Context, promptID string) error {
	// This would implement the actual Vertex AI API call to delete the prompt
	s.logger.InfoContext(ctx, "Deleting prompt from cloud storage", slog.String("prompt_id", promptID))
	return nil
}

func (s *service) listPromptsFromCloud(ctx context.Context, req *ListPromptsRequest) ([]*Prompt, string, int32, error) {
	// This would implement the actual Vertex AI API call to list prompts
	s.logger.InfoContext(ctx, "Listing prompts from cloud storage")
	return []*Prompt{}, "", 0, nil
}

func (s *service) getPromptByName(ctx context.Context, name string) (*Prompt, error) {
	// Check cache first by iterating through cached prompts
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	for _, prompt := range s.promptCache {
		if prompt.Name == name {
			return prompt, nil
		}
	}

	// This would implement actual name-based lookup from cloud storage
	s.logger.InfoContext(ctx, "Loading prompt by name from cloud storage", slog.String("name", name))
	return nil, ErrPromptNotFound
}

func (s *service) filterPrompts(prompts []*Prompt, req *ListPromptsRequest) []*Prompt {
	// Apply additional client-side filtering
	return prompts
}

// Configuration and utility methods

// GetProjectID returns the configured Google Cloud project ID.
func (s *service) GetProjectID() string {
	return s.projectID
}

// GetLocation returns the configured geographic location.
func (s *service) GetLocation() string {
	return s.location
}

// GetLogger returns the configured logger instance.
func (s *service) GetLogger() *slog.Logger {
	return s.logger
}

// IsInitialized returns whether the service is properly initialized.
func (s *service) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

// GetCacheStats returns cache statistics.
func (s *service) GetCacheStats() map[string]any {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	return map[string]any{
		"prompt_cache_size":  len(s.promptCache),
		"version_cache_size": len(s.versionCache),
		"cache_expiry":       s.cacheExpiry,
	}
}

// ClearCache clears all cached data.
func (s *service) ClearCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.promptCache = make(map[string]*Prompt)
	s.versionCache = make(map[string][]*PromptVersion)

	s.logger.Info("Prompt cache cleared")
}
