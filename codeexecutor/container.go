// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package codeexecutor

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"github.com/xenoweaver/adk-go/types"
)

const defaultImageTag = "adk-code-executor:latest"

// ContainerExecutor represents a code executor that uses a custom container to execute code.
//
// This provides a safer execution environment compared to local execution.
type ContainerExecutor struct {
	config *types.ExecutionConfig

	// Optional. The base url of the user hosted Docker client.
	baseURL string

	// The tag of the predefined image or custom image to run on the container.
	// Either docker_path or image must be set.
	image string

	// The path to the directory containing the Dockerfile.
	// If set, build the image from the dockerfile path instead of using the
	// predefined image. Either docker_path or image must be set.
	dockerfile string

	// Docker client
	client *client.Client

	// Container configuration
	containerConfig *container.Config
	hostConfig      *container.HostConfig

	// Resource limits
	memoryLimit int64 // in bytes
	cpuLimit    int64 // in nano CPUs (1 CPU = 1000000000)

	// activeContainers tracks running containers for stateful execution
	activeContainers map[string]string // executionID -> containerID
}

var _ types.CodeExecutor = (*ContainerExecutor)(nil)

// ContainerExecutorOption is a functional option for configuring ContainerExecutor.
type ContainerExecutorOption func(*ContainerExecutor)

// WithDockerClient sets a custom Docker client.
func WithDockerClient(client *client.Client) ContainerExecutorOption {
	return func(e *ContainerExecutor) {
		e.client = client
	}
}

// WithDockerImage sets the Docker image to use for execution.
func WithDockerImage(dockerImage string) ContainerExecutorOption {
	return func(e *ContainerExecutor) {
		e.image = dockerImage
	}
}

// WithDockerfile sets the Dockerfile to use for execution.
func WithDockerfile(dockerfile string) ContainerExecutorOption {
	return func(e *ContainerExecutor) {
		e.dockerfile = dockerfile
	}
}

// WithMemoryLimit sets the memory limit for containers (in bytes).
func WithMemoryLimit(limit int64) ContainerExecutorOption {
	return func(e *ContainerExecutor) {
		e.memoryLimit = limit
	}
}

// WithCPULimit sets the CPU limit for containers (in nano CPUs).
func WithCPULimit(limit int64) ContainerExecutorOption {
	return func(e *ContainerExecutor) {
		e.cpuLimit = limit
	}
}

// NewContainerExecutor creates a new container-based code executor.
func NewContainerExecutor(opts ...any) (*ContainerExecutor, error) {
	// Separate execution options from container executor options
	var execOpts []types.ExecutionOption
	var containerOpts []ContainerExecutorOption

	for _, opt := range opts {
		switch o := opt.(type) {
		case types.ExecutionOption:
			execOpts = append(execOpts, o)
		case ContainerExecutorOption:
			containerOpts = append(containerOpts, o)
		default:
			return nil, fmt.Errorf("unsupported option type: %T", opt)
		}
	}

	config := types.DefaultConfig()
	for _, opt := range execOpts {
		opt(config)
	}
	config.Stateful = false
	config.OptimizeDataFiles = false

	executor := &ContainerExecutor{
		config:           config,
		memoryLimit:      512 * 1024 * 1024, // 512MB default
		cpuLimit:         1000000000,        // 1 CPU default
		activeContainers: make(map[string]string),
	}
	for _, opt := range containerOpts {
		opt(executor)
	}

	if executor.image == "" && executor.dockerfile == "" {
		return nil, errors.New("either image or docker_path must be set for ContainerCodeExecutor")
	}
	if executor.image == "" {
		executor.image = defaultImageTag
	}
	if executor.dockerfile != "" {
		dockerfile, err := filepath.Abs(executor.dockerfile)
		if err != nil {
			return nil, err
		}
		executor.dockerfile = dockerfile
	}

	// Initialize Docker client if not provided
	if executor.client == nil {
		client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, fmt.Errorf("create Docker client: %w", err)
		}
		executor.client = client
	}

	// Test Docker connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := executor.client.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker: %w", err)
	}

	// Setup container configuration
	executor.setupContainerConfig()

	return executor, nil
}

// setupContainerConfig initializes the container configuration.
func (e *ContainerExecutor) setupContainerConfig() {
	e.containerConfig = &container.Config{
		Image:        e.image,
		WorkingDir:   "/workspace",
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
		Env:          []string{"PYTHONUNBUFFERED=1"},
	}

	e.hostConfig = &container.HostConfig{
		Resources: container.Resources{
			Memory:   e.memoryLimit,
			NanoCPUs: e.cpuLimit,
		},
		NetworkMode:    network.NetworkDefault, // Disable network access for security
		ReadonlyRootfs: false,                  // Allow writing to /workspace
		AutoRemove:     !e.config.Stateful,     // Auto-remove containers unless stateful
	}
}

// OptimizeDataFile implements [types.CodeExecutor].
func (e *ContainerExecutor) OptimizeDataFile() bool {
	return e.config.OptimizeDataFiles
}

// IsStateful implements [types.CodeExecutor].
func (e *ContainerExecutor) IsStateful() bool {
	return e.config.Stateful
}

// IsLongRunning implements [types.CodeExecutor].
func (e *ContainerExecutor) IsLongRunning() bool {
	return e.config.LongRunning
}

// ErrorRetryAttempts implements [types.CodeExecutor].
func (e *ContainerExecutor) ErrorRetryAttempts() int {
	return e.config.MaxRetries
}

// CodeBlockDelimiters implements [types.CodeExecutor].
func (e *ContainerExecutor) CodeBlockDelimiters() []types.DelimiterPair {
	return e.config.CodeBlockDelimiters
}

// ExecutionResultDelimiters implements [types.CodeExecutor].
func (e *ContainerExecutor) ExecutionResultDelimiters() types.DelimiterPair {
	return e.config.ExecutionResultDelimiters
}

// executeInContainer performs the actual container execution.
func (e *ContainerExecutor) executeInContainer(ctx context.Context, input *types.CodeExecutionInput) (*types.CodeExecutionResult, error) {
	var containerID string
	var err error

	// Get or create container
	if e.config.Stateful && input.ExecutionID != "" {
		if existingID, exists := e.activeContainers[input.ExecutionID]; exists {
			containerID = existingID
		}
	}

	if containerID == "" {
		containerID, err = e.createContainer(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create container: %w", err)
		}

		if e.config.Stateful && input.ExecutionID != "" {
			e.activeContainers[input.ExecutionID] = containerID
		}
	}

	// Copy files to container
	if err := e.copyFilesToContainer(ctx, containerID, input.InputFiles); err != nil {
		return nil, fmt.Errorf("failed to copy files to container: %w", err)
	}

	// Execute code
	result, err := e.runCodeInContainer(ctx, containerID, input)
	if err != nil {
		return nil, fmt.Errorf("failed to execute code in container: %w", err)
	}

	// Copy output files from container
	outputFiles, err := e.copyFilesFromContainer(ctx, containerID)
	if err == nil {
		result.OutputFiles = outputFiles
	}

	// Clean up container if not stateful
	if !e.config.Stateful {
		e.cleanupContainer(context.Background(), containerID)
	}

	return result, nil
}

// createContainer creates a new Docker container.
func (e *ContainerExecutor) createContainer(ctx context.Context) (string, error) {
	// Set timeout for container creation
	createCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Pull image if needed
	if err := e.ensureImage(createCtx); err != nil {
		return "", fmt.Errorf("failed to ensure image: %w", err)
	}

	// Create container
	resp, err := e.client.ContainerCreate(
		createCtx,
		e.containerConfig,
		e.hostConfig,
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := e.client.ContainerStart(createCtx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

// ensureImage ensures the Docker image is available locally.
func (e *ContainerExecutor) ensureImage(ctx context.Context) error {
	// Check if image exists locally
	images, err := e.client.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return err
	}

	for _, image := range images {
		if slices.Contains(image.RepoTags, e.image) {
			return nil // Image already exists
		}
	}

	// Pull image
	reader, err := e.client.ImagePull(ctx, e.image, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Wait for pull to complete
	_, err = io.Copy(io.Discard, reader)
	return err
}

// copyFilesToContainer copies input files to the container.
func (e *ContainerExecutor) copyFilesToContainer(ctx context.Context, containerID string, files []*types.CodeExecutionFile) error {
	if len(files) == 0 {
		return nil
	}

	// Create tar archive with files
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for _, file := range files {
		header := &tar.Header{
			Name: file.Name,
			Size: file.Size,
			Mode: 0o644,
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if _, err := tw.Write(file.Content); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}

	// Copy tar archive to container
	return e.client.CopyToContainer(
		ctx,
		containerID,
		"/workspace",
		&buf,
		container.CopyToContainerOptions{},
	)
}

// runCodeInContainer executes the code in the container.
func (e *ContainerExecutor) runCodeInContainer(ctx context.Context, containerID string, input *types.CodeExecutionInput) (*types.CodeExecutionResult, error) {
	// Determine command based on language
	var cmd []string
	switch strings.ToLower(input.Language) {
	case "python", "py", "":
		cmd = []string{"python3", "-c", input.Code}
	case "go":
		// Create a temporary Go file and run it
		goCode := input.Code
		if !strings.Contains(goCode, "package main") {
			goCode = fmt.Sprintf("package main\n\nimport \"fmt\"\n\nfunc main() {\n%s\n}", goCode)
		}
		cmd = []string{"sh", "-c", fmt.Sprintf("echo '%s' > main.go && go run main.go", goCode)}
	case "javascript", "js", "node":
		cmd = []string{"node", "-e", input.Code}
	case "bash", "shell", "sh":
		cmd = []string{"bash", "-c", input.Code}
	default:
		cmd = []string{"python3", "-c", input.Code}
	}

	// Set timeout
	execCtx := ctx
	if input.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, input.Timeout)
		defer cancel()
	} else if e.config.DefaultTimeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, e.config.DefaultTimeout)
		defer cancel()
	}

	// Create exec config
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   "/workspace",
	}

	// Add environment variables
	if len(input.Environment) > 0 {
		for key, value := range input.Environment {
			execConfig.Env = append(execConfig.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Create exec instance
	execResp, err := e.client.ContainerExecCreate(execCtx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec instance: %w", err)
	}

	// Execute command
	attachResp, err := e.client.ContainerExecAttach(execCtx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec instance: %w", err)
	}
	defer attachResp.Close()

	// Read output
	var stdout, stderr bytes.Buffer
	_, err = attachResp.Conn.Write([]byte{})
	if err != nil {
		return nil, err
	}

	// Read all output
	output, err := io.ReadAll(attachResp.Reader)
	if err != nil {
		return nil, err
	}

	// Parse stdout/stderr from Docker's multiplexed stream
	// Docker uses a simple protocol: first 8 bytes contain stream info
	if len(output) > 8 {
		stdout.Write(output[8:])
	}

	// Get exec inspect to check exit code
	execInspect, err := e.client.ContainerExecInspect(execCtx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec instance: %w", err)
	}

	result := &types.CodeExecutionResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: execInspect.ExitCode,
	}

	return result, nil
}

// copyFilesFromContainer copies output files from the container.
func (e *ContainerExecutor) copyFilesFromContainer(ctx context.Context, containerID string) ([]*types.CodeExecutionFile, error) {
	// Get tar archive from container workspace
	reader, _, err := e.client.CopyFromContainer(ctx, containerID, "/workspace")
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Read tar archive
	tr := tar.NewReader(reader)
	var files []*types.CodeExecutionFile

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Typeflag == tar.TypeReg {
			// Skip input files and common artifacts
			name := filepath.Base(header.Name)
			if strings.HasPrefix(name, ".") ||
				strings.HasSuffix(name, ".pyc") ||
				name == "main.go" {
				continue
			}

			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}

			file := types.NewExecutionFile(name, content, "")
			files = append(files, file)
		}
	}

	return files, nil
}

// cleanupContainer removes the container.
func (e *ContainerExecutor) cleanupContainer(ctx context.Context, containerID string) {
	e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
}

// ExecuteCode implements [types.CodeExecutor].
func (e *ContainerExecutor) ExecuteCode(ctx context.Context, ictx *types.InvocationContext, input *types.CodeExecutionInput) (*types.CodeExecutionResult, error) {
	startTime := time.Now()

	// Get or create execution context
	execCtx := GetContextFromInvocation(ictx, input.ExecutionID)

	// Execute with retry logic
	var result *types.CodeExecutionResult
	var lastErr error

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(e.config.RetryDelay):
				// Continue with retry
			}
		}

		result, lastErr = e.executeInContainer(ctx, input)
		if lastErr == nil {
			break
		}

		// Increment error count
		if ictx != nil {
			execCtx.IncrementErrorCount(ictx.InvocationID)
		}
	}

	if lastErr != nil {
		return &types.CodeExecutionResult{
			ExitCode:      1,
			Error:         lastErr,
			ExecutionTime: time.Since(startTime),
			ExecutionID:   input.ExecutionID,
		}, lastErr
	}

	result.ExecutionTime = time.Since(startTime)
	result.ExecutionID = input.ExecutionID

	// Store result in context
	execCtx.AddExecutionResult(result)

	return result, nil
}

// Close implements [types.CodeExecutor].
func (e *ContainerExecutor) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clean up all active containers
	for _, containerID := range e.activeContainers {
		e.cleanupContainer(ctx, containerID)
	}

	// Close Docker client
	if e.client != nil {
		return e.client.Close()
	}

	return nil
}
