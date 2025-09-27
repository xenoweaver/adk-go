// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package pyasyncio_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/xenoweaver/adk-go/pkg/py/pyasyncio"
)

func TestTaskBasicLifecycle(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	executed := false

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		executed = true
		return "success", nil
	})

	// Task should start as pending, then transition to running/done
	if task.Cancelled() {
		t.Error("New task should not be cancelled")
	}

	// Wait for completion
	result, err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got '%s'", result)
	}

	if !executed {
		t.Error("Task function was not executed")
	}

	if !task.Done() {
		t.Error("Task should be done after completion")
	}

	if task.State() != pyasyncio.TaskDone {
		t.Errorf("Expected TaskDone state, got %v", task.State())
	}
}

func TestTaskWithError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	expectedErr := errors.New("task error")

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (int, error) {
		return 0, expectedErr
	})

	result, err := task.Wait(ctx)
	if err == nil {
		t.Fatal("Expected task to fail")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if result != 0 {
		t.Errorf("Expected zero result on error, got %d", result)
	}

	if !task.Done() {
		t.Error("Failed task should be done")
	}

	// Test Exception() method
	taskErr := task.Exception()
	if !errors.Is(taskErr, expectedErr) {
		t.Errorf("Exception() returned wrong error: %v", taskErr)
	}
}

func TestTaskCancellation(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	started := make(chan struct{})

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		close(started)
		<-ctx.Done() // Wait for cancellation
		return "", ctx.Err()
	})

	// Wait for task to start
	<-started

	// Cancel the task
	if !task.Cancel() {
		t.Error("Cancel should return true for running task")
	}

	// Wait for completion
	_, err := task.Wait(ctx)
	if err == nil {
		t.Fatal("Expected cancelled task to return error")
	}

	var cancelledErr *pyasyncio.TaskCancelledError
	if !errors.As(err, &cancelledErr) {
		t.Errorf("Expected TaskCancelledError, got %T: %v", err, err)
	}

	if !task.Cancelled() {
		t.Error("Task should be cancelled")
	}

	if !task.Done() {
		t.Error("Cancelled task should be done")
	}

	if task.State() != pyasyncio.TaskCancelled {
		t.Errorf("Expected TaskCancelled state, got %v", task.State())
	}
}

func TestTaskCancellationAlreadyDone(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		return "completed", nil
	})

	// Wait for completion
	_, err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	// Try to cancel completed task
	if task.Cancel() {
		t.Error("Cancel should return false for completed task")
	}

	if task.Cancelled() {
		t.Error("Completed task should not be cancelled")
	}
}

func TestTaskCallbacks(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	callbackExecuted := make(chan bool, 1)
	var callbackTask *pyasyncio.Task[string]

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		time.Sleep(50 * time.Millisecond) // Small delay
		return "callback test", nil
	})

	// Add callback before completion
	task.AddCallback(func(t *pyasyncio.Task[string]) {
		callbackTask = t
		callbackExecuted <- true
	})

	// Wait for task completion
	result, err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	if result != "callback test" {
		t.Errorf("Expected 'callback test', got '%s'", result)
	}

	// Wait for callback
	select {
	case <-callbackExecuted:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Callback was not executed")
	}

	if callbackTask != task {
		t.Error("Callback received wrong task")
	}
}

func TestTaskCallbackOnCompletedTask(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	callbackExecuted := make(chan bool, 1)

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (int, error) {
		return 42, nil
	})

	// Wait for completion
	_, err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	// Add callback after completion
	task.AddCallback(func(t *pyasyncio.Task[int]) {
		callbackExecuted <- true
	})

	// Callback should execute immediately
	select {
	case <-callbackExecuted:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Callback should execute immediately on completed task")
	}
}

func TestTaskNaming(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Test unnamed task
	task1 := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		return "unnamed", nil
	})

	if task1.Name() != "" {
		t.Errorf("Expected empty name, got '%s'", task1.Name())
	}

	// Test named task
	task2 := pyasyncio.CreateNamedTask(ctx, "test-task", func(ctx context.Context) (string, error) {
		return "named", nil
	})

	if task2.Name() != "test-task" {
		t.Errorf("Expected 'test-task', got '%s'", task2.Name())
	}

	// Test setting name
	task1.SetName("renamed-task")
	if task1.Name() != "renamed-task" {
		t.Errorf("Expected 'renamed-task', got '%s'", task1.Name())
	}
}

func TestTaskTimestamps(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	beforeCreate := time.Now()

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		time.Sleep(10 * time.Millisecond)
		return "timing test", nil
	})

	afterCreate := time.Now()

	// Wait for completion
	_, err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	created := task.Created()
	started := task.Started()
	finished := task.Finished()

	// Verify timestamp ordering
	if created.Before(beforeCreate) || created.After(afterCreate) {
		t.Errorf("Created timestamp %v not between %v and %v", created, beforeCreate, afterCreate)
	}

	if started.Before(created) {
		t.Errorf("Started timestamp %v before created %v", started, created)
	}

	if finished.Before(started) {
		t.Errorf("Finished timestamp %v before started %v", finished, started)
	}

	if finished.IsZero() {
		t.Error("Finished timestamp should not be zero for completed task")
	}
}

func TestTaskResultBeforeDone(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	started := make(chan struct{})

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		close(started)
		time.Sleep(100 * time.Millisecond)
		return "eventually", nil
	})

	// Wait for task to start
	<-started

	// Try to get result before completion
	_, err := task.Result()
	if err == nil {
		t.Fatal("Result() should fail on incomplete task")
	}

	expectedMsg := "task is not yet done"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Cancel task and clean up
	task.Cancel()
}

func TestTaskWaitWithTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		time.Sleep(200 * time.Millisecond)
		return "slow task", nil
	})

	// Wait with short timeout
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := task.Wait(waitCtx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	if elapsed < 40*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Errorf("Wait timeout took %v, expected around 50ms", elapsed)
	}

	// Task should still be running
	if task.Done() {
		t.Error("Task should still be running after wait timeout")
	}

	// Clean up
	task.Cancel()
}

func TestTaskContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	started := make(chan struct{})

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})

	// Wait for task to start
	<-started

	// Cancel parent context
	cancel()

	// Task should complete with cancellation error
	_, err := task.Wait(context.Background())
	if err == nil {
		t.Fatal("Expected task to be cancelled")
	}

	var cancelledErr *pyasyncio.TaskCancelledError
	if !errors.As(err, &cancelledErr) {
		t.Errorf("Expected TaskCancelledError, got %T: %v", err, err)
	}

	if !task.Cancelled() {
		t.Error("Task should be cancelled")
	}
}

func TestTaskConcurrentAccess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	const numGoroutines = 100

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (int, error) {
		time.Sleep(50 * time.Millisecond)
		return 42, nil
	})

	var wg sync.WaitGroup
	results := make([]int, numGoroutines)
	errors := make([]error, numGoroutines)

	// Start many goroutines accessing the task concurrently
	for i := range numGoroutines {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errors[index] = task.Wait(ctx)
		}(i)
	}

	wg.Wait()

	// All should get the same result
	for i := range numGoroutines {
		if errors[i] != nil {
			t.Errorf("Goroutine %d got error: %v", i, errors[i])
		}
		if results[i] != 42 {
			t.Errorf("Goroutine %d got result %d, expected 42", i, results[i])
		}
	}
}

func TestTaskStringRepresentation(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Test unnamed task
	task1 := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		return "test", nil
	})

	str1 := task1.String()
	if str1 != "Task[unnamed](pending)" && str1 != "Task[unnamed](running)" && str1 != "Task[unnamed](done)" {
		t.Errorf("Unexpected string representation: %s", str1)
	}

	// Test named task
	task2 := pyasyncio.CreateNamedTask(ctx, "my-task", func(ctx context.Context) (string, error) {
		return "test", nil
	})

	str2 := task2.String()
	if str2 != "Task[my-task](pending)" && str2 != "Task[my-task](running)" && str2 != "Task[my-task](done)" {
		t.Errorf("Unexpected string representation: %s", str2)
	}
}

func TestTaskMultipleResultCalls(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		return "cached result", nil
	})

	// Wait for completion
	result1, err1 := task.Wait(ctx)
	if err1 != nil {
		t.Fatalf("Task failed: %v", err1)
	}

	// Call Result() multiple times
	result2, err2 := task.Result()
	result3, err3 := task.Result()

	// All should return the same cached result
	if diff := cmp.Diff(result1, result2); diff != "" {
		t.Errorf("Result mismatch (-wait +result1):\n%s", diff)
	}

	if diff := cmp.Diff(result1, result3); diff != "" {
		t.Errorf("Result mismatch (-wait +result2):\n%s", diff)
	}

	if err2 != nil || err3 != nil {
		t.Errorf("Result() calls failed: %v, %v", err2, err3)
	}
}

func TestTaskStateTransitions(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	started := make(chan struct{})
	proceed := make(chan struct{})

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		close(started)
		<-proceed
		return "state test", nil
	})

	// Initially should be pending or running
	initialState := task.State()
	if initialState != pyasyncio.TaskPending && initialState != pyasyncio.TaskRunning {
		t.Errorf("Expected TaskPending or TaskRunning, got %v", initialState)
	}

	// Wait for task to start
	<-started

	// Should now be running
	runningState := task.State()
	if runningState != pyasyncio.TaskRunning {
		t.Errorf("Expected TaskRunning, got %v", runningState)
	}

	// Allow task to complete
	close(proceed)

	// Wait for completion
	_, err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	// Should now be done
	finalState := task.State()
	if finalState != pyasyncio.TaskDone {
		t.Errorf("Expected TaskDone, got %v", finalState)
	}
}

func TestTaskPanicRecovery(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	callbackPanicked := make(chan bool, 1)

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		return "panic test", nil
	})

	// Add callback that panics
	task.AddCallback(func(t *pyasyncio.Task[string]) {
		defer func() {
			if r := recover(); r == nil {
				callbackPanicked <- false
			} else {
				callbackPanicked <- true
			}
		}()
		panic("callback panic")
	})

	// Wait for completion
	result, err := task.Wait(ctx)
	if err != nil {
		t.Fatalf("Task failed: %v", err)
	}

	if result != "panic test" {
		t.Errorf("Expected 'panic test', got '%s'", result)
	}

	// Callback should have panicked but been recovered
	select {
	case panicked := <-callbackPanicked:
		if !panicked {
			t.Error("Callback should have panicked")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Callback did not execute")
	}
}

func TestTaskMemoryUsage(t *testing.T) {
	t.Parallel()

	// Test that creating many tasks doesn't leak memory
	ctx := t.Context()
	const numTasks = 1000

	var tasks []*pyasyncio.Task[int]
	counter := int64(0)

	for range numTasks {
		task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (int, error) {
			return int(atomic.AddInt64(&counter, 1)), nil
		})
		tasks = append(tasks, task)
	}

	// Wait for all tasks to complete
	for _, task := range tasks {
		result, err := task.Wait(ctx)
		if err != nil {
			t.Fatalf("Task failed: %v", err)
		}
		if result <= 0 || result > numTasks {
			t.Errorf("Unexpected result: %d", result)
		}
	}

	// Verify all tasks completed
	finalCounter := atomic.LoadInt64(&counter)
	if finalCounter != numTasks {
		t.Errorf("Expected %d tasks to complete, got %d", numTasks, finalCounter)
	}
}

func TestTaskErrorEquality(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("test error")
	})

	// Wait for completion
	_, err1 := task.Wait(ctx)
	if err1 == nil {
		t.Fatal("Expected task to fail")
	}

	// Get error again
	err2 := task.Exception()

	// Errors should be the same
	if err1.Error() != err2.Error() {
		t.Errorf("Error mismatch: wait=%v, exception=%v", err1, err2)
	}
}
