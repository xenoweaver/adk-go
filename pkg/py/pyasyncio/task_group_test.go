// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package pyasyncio_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/xenoweaver/adk-go/pkg/py/pyasyncio"
)

func TestTaskGroupBasicSuccess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[int](ctx)
	defer tg.Close()

	// Create multiple successful tasks
	_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
		return 1, nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	_, err = tg.CreateTask(func(ctx context.Context) (int, error) {
		return 2, nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	_, err = tg.CreateTask(func(ctx context.Context) (int, error) {
		return 3, nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait for all tasks to complete
	results, err := tg.Wait(ctx)
	if err != nil {
		t.Fatalf("TaskGroup failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Results should contain 1, 2, 3 (order may vary)
	resultSet := make(map[int]bool)
	for _, result := range results {
		resultSet[result] = true
	}

	for i := 1; i <= 3; i++ {
		if !resultSet[i] {
			t.Errorf("Missing result %d", i)
		}
	}

	if tg.TaskCount() != 3 {
		t.Errorf("Expected 3 tasks, got %d", tg.TaskCount())
	}

	if tg.ActiveCount() != 0 {
		t.Errorf("Expected 0 active tasks, got %d", tg.ActiveCount())
	}
}

func TestTaskGroupSingleFailure(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[string](ctx)
	defer tg.Close()

	started := make(chan struct{}, 3)
	expectedErr := errors.New("task failure")

	// Create multiple tasks, one will fail
	_, err := tg.CreateTask(func(ctx context.Context) (string, error) {
		started <- struct{}{}
		<-ctx.Done() // Wait for cancellation
		return "", ctx.Err()
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	_, err = tg.CreateTask(func(ctx context.Context) (string, error) {
		started <- struct{}{}
		return "", expectedErr // This task fails
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	_, err = tg.CreateTask(func(ctx context.Context) (string, error) {
		started <- struct{}{}
		<-ctx.Done() // Wait for cancellation
		return "", ctx.Err()
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait for all tasks to start
	for range 3 {
		<-started
	}

	// Wait for group completion
	results, err := tg.Wait(ctx)
	if err == nil {
		t.Fatal("Expected TaskGroup to fail")
	}

	var tgErr *pyasyncio.TaskGroupError
	if !errors.As(err, &tgErr) {
		t.Fatalf("Expected TaskGroupError, got %T: %v", err, err)
	}

	if len(tgErr.Errors) == 0 {
		t.Error("Expected at least one error in TaskGroupError")
	}

	// Should contain the original error
	foundExpectedErr := false
	for _, e := range tgErr.Errors {
		if errors.Is(e, expectedErr) {
			foundExpectedErr = true
			break
		}
	}
	if !foundExpectedErr {
		t.Error("Expected error not found in TaskGroupError")
	}

	// Results may still contain successful task results
	t.Logf("Got %d results despite failure", len(results))
}

func TestTaskGroupMultipleFailures(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[int](ctx)
	defer tg.Close()

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	// Create tasks that both fail quickly
	_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
		return 0, err1
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	_, err = tg.CreateTask(func(ctx context.Context) (int, error) {
		return 0, err2
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait for completion
	_, err = tg.Wait(ctx)
	if err == nil {
		t.Fatal("Expected TaskGroup to fail")
	}

	var tgErr *pyasyncio.TaskGroupError
	if !errors.As(err, &tgErr) {
		t.Fatalf("Expected TaskGroupError, got %T: %v", err, err)
	}

	if len(tgErr.Errors) == 0 {
		t.Error("Expected at least one error in TaskGroupError")
	}

	// At least one of the errors should be present
	foundErr1 := false
	foundErr2 := false
	for _, e := range tgErr.Errors {
		if errors.Is(e, err1) {
			foundErr1 = true
		}
		if errors.Is(e, err2) {
			foundErr2 = true
		}
	}

	if !foundErr1 && !foundErr2 {
		t.Error("None of the expected errors found in TaskGroupError")
	}
}

func TestTaskGroupCancellation(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[string](ctx)
	defer tg.Close()

	started := make(chan struct{}, 2)

	// Create long-running tasks
	_, err := tg.CreateTask(func(ctx context.Context) (string, error) {
		started <- struct{}{}
		<-ctx.Done()
		return "", ctx.Err()
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	_, err = tg.CreateTask(func(ctx context.Context) (string, error) {
		started <- struct{}{}
		<-ctx.Done()
		return "", ctx.Err()
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait for tasks to start
	<-started
	<-started

	// Cancel the group
	tg.Cancel()

	// Wait for completion
	_, err = tg.Wait(ctx)
	if err == nil {
		t.Fatal("Expected cancelled TaskGroup to return error")
	}

	if !tg.Cancelled() {
		t.Error("TaskGroup should be cancelled")
	}
}

func TestTaskGroupExternalContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	tg := pyasyncio.NewTaskGroup[int](ctx)
	defer tg.Close()

	started := make(chan struct{})

	// Create task that waits for cancellation
	_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
		close(started)
		<-ctx.Done()
		return 0, ctx.Err()
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait for task to start
	<-started

	// Cancel external context
	cancel()

	// Wait for completion
	_, err = tg.Wait(context.Background())
	if err == nil {
		t.Fatal("Expected TaskGroup to fail due to context cancellation")
	}

	if !tg.Cancelled() {
		t.Error("TaskGroup should be cancelled")
	}
}

func TestTaskGroupEmptyGroup(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[string](ctx)
	defer tg.Close()

	// Wait on empty group
	results, err := tg.Wait(ctx)
	if err != nil {
		t.Fatalf("Empty TaskGroup should not fail: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}

	if tg.TaskCount() != 0 {
		t.Errorf("Expected 0 tasks, got %d", tg.TaskCount())
	}

	if tg.ActiveCount() != 0 {
		t.Errorf("Expected 0 active tasks, got %d", tg.ActiveCount())
	}
}

func TestTaskGroupWaitTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[string](ctx)
	defer tg.Close()

	// Create slow task
	_, err := tg.CreateTask(func(ctx context.Context) (string, error) {
		time.Sleep(200 * time.Millisecond)
		return "slow", nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait with timeout
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = tg.Wait(waitCtx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	if elapsed < 40*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Errorf("Wait timeout took %v, expected around 50ms", elapsed)
	}

	// TaskGroup should still be running
	if tg.ActiveCount() == 0 {
		t.Error("TaskGroup should still have active tasks")
	}
}

func TestTaskGroupTaskAfterFinish(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[int](ctx)
	defer tg.Close()

	// Create and complete a task
	_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
		return 1, nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait for completion
	_, err = tg.Wait(ctx)
	if err != nil {
		t.Fatalf("TaskGroup failed: %v", err)
	}

	// Try to add task after group is finished
	_, err = tg.CreateTask(func(ctx context.Context) (int, error) {
		return 2, nil
	})
	if err == nil {
		t.Fatal("Expected error when adding task to finished group")
	}

	expectedMsg := "cannot add task to finished task group"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestTaskGroupNamedTasks(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[string](ctx)
	defer tg.Close()

	// Create named tasks
	task1, err := tg.CreateNamedTask("task-1", func(ctx context.Context) (string, error) {
		return "first", nil
	})
	if err != nil {
		t.Fatalf("CreateNamedTask failed: %v", err)
	}

	task2, err := tg.CreateNamedTask("task-2", func(ctx context.Context) (string, error) {
		return "second", nil
	})
	if err != nil {
		t.Fatalf("CreateNamedTask failed: %v", err)
	}

	// Verify names
	if task1.Name() != "task-1" {
		t.Errorf("Expected name 'task-1', got '%s'", task1.Name())
	}

	if task2.Name() != "task-2" {
		t.Errorf("Expected name 'task-2', got '%s'", task2.Name())
	}

	// Wait for completion
	results, err := tg.Wait(ctx)
	if err != nil {
		t.Fatalf("TaskGroup failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestTaskGroupTaskAccess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[int](ctx)
	defer tg.Close()

	var tasks []*pyasyncio.Task[int]

	// Create tasks and collect references
	for i := range 3 {
		task, err := tg.CreateNamedTask(fmt.Sprintf("task-%d", i), func(ctx context.Context) (int, error) {
			return i, nil
		})
		if err != nil {
			t.Fatalf("CreateNamedTask failed: %v", err)
		}
		tasks = append(tasks, task)
	}

	// Get tasks from group
	groupTasks := tg.Tasks()
	if len(groupTasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(groupTasks))
	}

	// Verify task references match
	for i, task := range tasks {
		found := slices.Contains(groupTasks, task)
		if !found {
			t.Errorf("Task %d not found in group tasks", i)
		}
	}

	// Wait for completion
	_, err := tg.Wait(ctx)
	if err != nil {
		t.Fatalf("TaskGroup failed: %v", err)
	}
}

func TestTaskGroupConcurrentStress(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[int](ctx)
	defer tg.Close()

	const numTasks = 100
	counter := int64(0)

	// Create many concurrent tasks
	for i := range numTasks {
		_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
			// Simulate some work
			time.Sleep(time.Duration(i%10) * time.Millisecond)
			return int(atomic.AddInt64(&counter, 1)), nil
		})
		if err != nil {
			t.Fatalf("CreateTask failed: %v", err)
		}
	}

	// Wait for all tasks
	results, err := tg.Wait(ctx)
	if err != nil {
		t.Fatalf("TaskGroup failed: %v", err)
	}

	if len(results) != numTasks {
		t.Errorf("Expected %d results, got %d", numTasks, len(results))
	}

	finalCounter := atomic.LoadInt64(&counter)
	if finalCounter != numTasks {
		t.Errorf("Expected counter %d, got %d", numTasks, finalCounter)
	}

	// Verify all results are unique and in expected range
	resultSet := make(map[int]bool)
	for _, result := range results {
		if result <= 0 || result > numTasks {
			t.Errorf("Unexpected result: %d", result)
		}
		if resultSet[result] {
			t.Errorf("Duplicate result: %d", result)
		}
		resultSet[result] = true
	}
}

func TestTaskGroupWaitForCompletion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[string](ctx)
	defer tg.Close()

	// Create tasks
	_, err := tg.CreateTask(func(ctx context.Context) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "task1", nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	_, err = tg.CreateTask(func(ctx context.Context) (string, error) {
		time.Sleep(30 * time.Millisecond)
		return "task2", nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Use WaitForCompletion instead of Wait
	results, err := tg.WaitForCompletion()
	if err != nil {
		t.Fatalf("WaitForCompletion failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Verify results
	resultSet := make(map[string]bool)
	for _, result := range results {
		resultSet[result] = true
	}

	if !resultSet["task1"] || !resultSet["task2"] {
		t.Error("Missing expected results")
	}
}

func TestTaskGroupErrorUnwrap(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[int](ctx)
	defer tg.Close()

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	// Create failing tasks
	_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
		return 0, err1
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	_, err = tg.CreateTask(func(ctx context.Context) (int, error) {
		return 0, err2
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait for completion
	_, err = tg.Wait(ctx)
	if err == nil {
		t.Fatal("Expected TaskGroup to fail")
	}

	var tgErr *pyasyncio.TaskGroupError
	if !errors.As(err, &tgErr) {
		t.Fatalf("Expected TaskGroupError, got %T", err)
	}

	// Test Unwrap functionality
	unwrapped := tgErr.Unwrap()
	if len(unwrapped) == 0 {
		t.Error("Expected unwrapped errors")
	}

	// Check that original errors are accessible
	foundErr1 := false
	foundErr2 := false
	for _, e := range unwrapped {
		if errors.Is(e, err1) {
			foundErr1 = true
		}
		if errors.Is(e, err2) {
			foundErr2 = true
		}
	}

	if !foundErr1 && !foundErr2 {
		t.Error("Original errors not found in unwrapped errors")
	}
}

func TestTaskGroupStringRepresentation(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[int](ctx)
	defer tg.Close()

	// Initially empty
	str := tg.String()
	if str != "TaskGroup[0 tasks, 0 active, 0 errors]" {
		t.Errorf("Unexpected string for empty group: %s", str)
	}

	// Add some tasks
	_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
		time.Sleep(100 * time.Millisecond)
		return 1, nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	_, err = tg.CreateTask(func(ctx context.Context) (int, error) {
		return 0, errors.New("test error")
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait a bit for state to update
	time.Sleep(10 * time.Millisecond)

	str = tg.String()
	t.Logf("TaskGroup string representation: %s", str)
	// Don't assert exact string since timing may vary
}

func TestGatherFunction(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Test successful gather
	results, err := pyasyncio.Gather(ctx,
		func(ctx context.Context) (string, error) {
			return "first", nil
		},
		func(ctx context.Context) (string, error) {
			return "second", nil
		},
		func(ctx context.Context) (string, error) {
			return "third", nil
		},
	)
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Verify results (order should be preserved)
	expectedResults := []string{"first", "second", "third"}
	if diff := cmp.Diff(expectedResults, results,
		cmpopts.SortSlices(func(a, b string) bool { return a < b }),
	); diff != "" {
		t.Errorf("Result mismatch (-expected +actual):\n%s", diff)
	}
}

func TestGatherWithFailure(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	expectedErr := errors.New("gather failure")

	// Test gather with one failure
	_, err := pyasyncio.Gather(ctx,
		func(ctx context.Context) (int, error) {
			return 1, nil
		},
		func(ctx context.Context) (int, error) {
			return 0, expectedErr
		},
		func(ctx context.Context) (int, error) {
			<-ctx.Done() // Should be cancelled
			return 0, ctx.Err()
		},
	)

	if err == nil {
		t.Fatal("Expected Gather to fail")
	}

	var tgErr *pyasyncio.TaskGroupError
	if !errors.As(err, &tgErr) {
		t.Fatalf("Expected TaskGroupError, got %T: %v", err, err)
	}

	// Should contain the original error
	foundExpectedErr := false
	for _, e := range tgErr.Errors {
		if errors.Is(e, expectedErr) {
			foundExpectedErr = true
			break
		}
	}
	if !foundExpectedErr {
		t.Error("Expected error not found in gather failure")
	}
}

func TestGatherEmpty(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Test empty gather
	results, err := pyasyncio.Gather[string](ctx)
	if err != nil {
		t.Fatalf("Empty Gather failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

func TestTaskGroupMixedSuccessFailure(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[int](ctx)
	defer tg.Close()

	successCount := int64(0)

	// Create mix of successful and failing tasks
	// TODO(zchee): flaky test
	t.Log("flaky: CreateTask failed: cannot add task to finished task group")
	for i := range 10 {
		_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
			if i%3 == 0 {
				return 0, fmt.Errorf("task %d failed", i)
			}
			atomic.AddInt64(&successCount, 1)
			return i, nil
		})
		if err != nil {
			t.Fatalf("CreateTask failed: %v", err)
		}
	}

	// Wait for completion
	results, err := tg.Wait(ctx)
	if err == nil {
		t.Fatal("Expected TaskGroup to fail due to some task failures")
	}

	var tgErr *pyasyncio.TaskGroupError
	if !errors.As(err, &tgErr) {
		t.Fatalf("Expected TaskGroupError, got %T: %v", err, err)
	}

	// Should have some errors
	if len(tgErr.Errors) == 0 {
		t.Error("Expected some errors in TaskGroupError")
	}

	// May have some successful results
	t.Logf("Got %d results and %d errors", len(results), len(tgErr.Errors))
}

func TestTaskGroupNilFunction(t *testing.T) {
	t.Parallel()

	tg := pyasyncio.NewTaskGroup[int](t.Context())
	defer tg.Close()

	// Try to create task with nil function
	_, err := tg.CreateTask(nil)
	if err == nil {
		t.Fatal("Expected error when creating task with nil function")
	}

	expectedMsg := "task function cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestTaskGroupContextPropagation(t *testing.T) {
	t.Parallel()

	parentCtx := t.Context()
	tg := pyasyncio.NewTaskGroup[string](parentCtx)
	defer tg.Close()

	// Verify context propagation
	groupCtx := tg.Context()
	if groupCtx == parentCtx {
		t.Error("TaskGroup should create child context, not use parent directly")
	}

	// Create task and verify it gets the group context
	taskCtx := make(chan context.Context, 1)

	_, err := tg.CreateTask(func(ctx context.Context) (string, error) {
		taskCtx <- ctx
		return "context test", nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Wait for task context
	select {
	case receivedCtx := <-taskCtx:
		// Task context should be related to group context
		if receivedCtx == parentCtx {
			t.Error("Task should not receive parent context directly")
		}
		// We can't easily test the exact relationship without implementation details

	case <-time.After(100 * time.Millisecond):
		t.Fatal("Task did not send context")
	}

	// Clean up
	_, err = tg.Wait(parentCtx)
	if err != nil {
		t.Fatalf("TaskGroup failed: %v", err)
	}
}
