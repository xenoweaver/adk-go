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

func TestTimeoutError(t *testing.T) {
	t.Parallel()

	// Test basic TimeoutError
	err := pyasyncio.NewTimeoutError(5 * time.Second)
	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T", err)
	}

	expectedMsg := "operation timed out after 5s"
	if timeoutErr.Error() != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, timeoutErr.Error())
	}

	// Test TimeoutError with custom message
	customErr := pyasyncio.NewTimeoutErrorWithMessage("custom timeout", 3*time.Second)
	var customTimeoutErr *pyasyncio.TimeoutError
	if !errors.As(customErr, &customTimeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T", customErr)
	}

	if customTimeoutErr.Error() != "custom timeout" {
		t.Errorf("Expected custom message, got '%s'", customTimeoutErr.Error())
	}
}

func TestWaitForSuccess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	result, err := pyasyncio.WaitFor(ctx, 1*time.Second, func(ctx context.Context) (string, error) {
		// Quick operation that completes within timeout
		time.Sleep(50 * time.Millisecond)
		return "success", nil
	})
	if err != nil {
		t.Fatalf("WaitFor failed: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got '%s'", result)
	}
}

func TestWaitForTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	timeout := 100 * time.Millisecond

	start := time.Now()
	result, err := pyasyncio.WaitFor(ctx, timeout, func(ctx context.Context) (string, error) {
		// Operation that takes longer than timeout
		select {
		case <-time.After(500 * time.Millisecond):
			return "too slow", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	elapsed := time.Since(start)

	// Should timeout
	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T: %v", err, err)
	}

	if result != "" {
		t.Errorf("Expected empty result on timeout, got '%s'", result)
	}

	// Should timeout around the specified duration
	if elapsed < timeout || elapsed > timeout*2 {
		t.Errorf("Expected timeout around %v, got %v", timeout, elapsed)
	}
}

func TestWaitForFunctionError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	expectedErr := errors.New("function error")

	result, err := pyasyncio.WaitFor(ctx, 1*time.Second, func(ctx context.Context) (int, error) {
		return 0, expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected original error, got %v", err)
	}

	if result != 0 {
		t.Errorf("Expected zero result on error, got %d", result)
	}
}

func TestWaitForNilFunction(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	_, err := pyasyncio.WaitFor[string](ctx, 1*time.Second, nil)

	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T", err)
	}

	expectedMsg := "function cannot be nil"
	if timeoutErr.Error() != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, timeoutErr.Error())
	}
}

func TestWaitForInvalidTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	_, err := pyasyncio.WaitFor(ctx, 0, func(ctx context.Context) (string, error) {
		return "test", nil
	})

	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T", err)
	}

	expectedMsg := "timeout must be positive"
	if timeoutErr.Error() != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, timeoutErr.Error())
	}
}

func TestWaitForContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	started := make(chan struct{})
	go func() {
		<-started
		time.Sleep(50 * time.Millisecond)
		cancel() // Cancel parent context
	}()

	_, err := pyasyncio.WaitFor(ctx, 5*time.Second, func(ctx context.Context) (string, error) {
		close(started)
		<-ctx.Done() // Wait for cancellation
		return "", ctx.Err()
	})

	// Should get the original context cancellation error, not timeout
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestWaitForTask(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Create a task
	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "task result", nil
	})

	// Wait for it with timeout
	result, err := pyasyncio.WaitForTask(ctx, 1*time.Second, task)
	if err != nil {
		t.Fatalf("WaitForTask failed: %v", err)
	}

	if result != "task result" {
		t.Errorf("Expected 'task result', got '%s'", result)
	}
}

func TestWaitForTaskTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Create a slow task that periodically checks for cancellation
	task := pyasyncio.CreateTask(ctx, func(ctx context.Context) (string, error) {
		for range 50 {
			select {
			case <-time.After(10 * time.Millisecond):
				// Continue
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		return "slow task", nil
	})

	timeout := 100 * time.Millisecond
	start := time.Now()

	_, err := pyasyncio.WaitForTask(ctx, timeout, task)
	elapsed := time.Since(start)

	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T: %v", err, err)
	}

	if elapsed < timeout || elapsed > timeout*2 {
		t.Errorf("Expected timeout around %v, got %v", timeout, elapsed)
	}

	// Wait for task to complete (give it reasonable time to process cancellation)
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer waitCancel()

	_, taskErr := task.Wait(waitCtx)

	// Task should be cancelled/done and return an error
	if !task.Done() {
		t.Error("Task should be done after being cancelled")
	}

	if taskErr == nil {
		t.Error("Cancelled task should return an error")
	}
}

func TestWaitForTaskNil(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	_, err := pyasyncio.WaitForTask[string](ctx, 1*time.Second, nil)

	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T", err)
	}

	expectedMsg := "task cannot be nil"
	if timeoutErr.Error() != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, timeoutErr.Error())
	}
}

func TestWaitForAnyFirstSuccess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	result, err := pyasyncio.WaitForAny(ctx, 1*time.Second,
		func(ctx context.Context) (string, error) {
			time.Sleep(200 * time.Millisecond)
			return "slow", nil
		},
		func(ctx context.Context) (string, error) {
			time.Sleep(50 * time.Millisecond)
			return "fast", nil // This should complete first
		},
		func(ctx context.Context) (string, error) {
			time.Sleep(300 * time.Millisecond)
			return "slower", nil
		},
	)
	if err != nil {
		t.Fatalf("WaitForAny failed: %v", err)
	}

	if result != "fast" {
		t.Errorf("Expected 'fast' (first to complete), got '%s'", result)
	}
}

func TestWaitForAnyTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	timeout := 100 * time.Millisecond

	start := time.Now()
	_, err := pyasyncio.WaitForAny(ctx, timeout,
		func(ctx context.Context) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
		func(ctx context.Context) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	)
	elapsed := time.Since(start)

	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T: %v", err, err)
	}

	if elapsed < timeout || elapsed > timeout*2 {
		t.Errorf("Expected timeout around %v, got %v", timeout, elapsed)
	}
}

func TestWaitForAnyEmptyFunctions(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	_, err := pyasyncio.WaitForAny[string](ctx, 1*time.Second)

	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T", err)
	}

	expectedMsg := "at least one function must be provided"
	if timeoutErr.Error() != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, timeoutErr.Error())
	}
}

func TestWaitForAllSuccess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	results, err := pyasyncio.WaitForAll(ctx, 1*time.Second,
		func(ctx context.Context) (string, error) {
			time.Sleep(50 * time.Millisecond)
			return "first", nil
		},
		func(ctx context.Context) (string, error) {
			time.Sleep(30 * time.Millisecond)
			return "second", nil
		},
		func(ctx context.Context) (string, error) {
			time.Sleep(40 * time.Millisecond)
			return "third", nil
		},
	)
	if err != nil {
		t.Fatalf("WaitForAll failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Results should contain all expected values (order may vary)
	resultSet := make(map[string]bool)
	for _, result := range results {
		resultSet[result] = true
	}

	expected := []string{"first", "second", "third"}
	for _, exp := range expected {
		if !resultSet[exp] {
			t.Errorf("Missing expected result: %s", exp)
		}
	}
}

func TestWaitForAllTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	timeout := 100 * time.Millisecond

	start := time.Now()
	_, err := pyasyncio.WaitForAll(ctx, timeout,
		func(ctx context.Context) (string, error) {
			time.Sleep(50 * time.Millisecond)
			return "fast", nil
		},
		func(ctx context.Context) (string, error) {
			<-ctx.Done() // Wait for timeout
			return "", ctx.Err()
		},
	)
	elapsed := time.Since(start)

	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T: %v", err, err)
	}

	if elapsed < timeout || elapsed > timeout*2 {
		t.Errorf("Expected timeout around %v, got %v", timeout, elapsed)
	}
}

func TestWaitForAllEmpty(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	results, err := pyasyncio.WaitForAll[string](ctx, 1*time.Second)
	if err != nil {
		t.Fatalf("WaitForAll with empty functions should succeed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

func TestWaitForAllSingleFailure(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	expectedErr := errors.New("function failure")

	_, err := pyasyncio.WaitForAll(ctx, 1*time.Second,
		func(ctx context.Context) (string, error) {
			return "success", nil
		},
		func(ctx context.Context) (string, error) {
			return "", expectedErr // This fails
		},
	)

	if err == nil {
		t.Fatal("Expected WaitForAll to fail")
	}

	// Should contain the original error
	var tgErr *pyasyncio.TaskGroupError
	if errors.As(err, &tgErr) {
		// Check if original error is in the group error
		found := false
		for _, e := range tgErr.Errors {
			if errors.Is(e, expectedErr) {
				found = true
				break
			}
		}
		if !found {
			t.Error("Original error not found in TaskGroupError")
		}
	} else {
		// Direct error should be the original error
		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected original error, got %v", err)
		}
	}
}

func TestShieldBasicUsage(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	result, err := pyasyncio.Shield(ctx, func(ctx context.Context) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "shielded", nil
	})
	if err != nil {
		t.Fatalf("Shield failed: %v", err)
	}

	if result != "shielded" {
		t.Errorf("Expected 'shielded', got '%s'", result)
	}
}

func TestShieldWithCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	started := make(chan struct{})
	completed := make(chan struct{})

	go func() {
		defer close(completed)

		_, err := pyasyncio.Shield(ctx, func(shieldCtx context.Context) (string, error) {
			close(started)
			// Shield context should not be cancelled even if parent is
			time.Sleep(100 * time.Millisecond)
			return "protected", nil
		})
		if err != nil {
			t.Errorf("Shield should protect from parent cancellation: %v", err)
		}
	}()

	// Wait for function to start
	<-started

	// Cancel parent context
	cancel()

	// Function should still complete despite parent cancellation
	select {
	case <-completed:
		// Good, function completed
	case <-time.After(200 * time.Millisecond):
		t.Error("Shielded function should complete despite parent cancellation")
	}
}

func TestShieldWithDeadline(t *testing.T) {
	t.Parallel()

	// Create context with deadline
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := pyasyncio.Shield(ctx, func(shieldCtx context.Context) (string, error) {
		// Shield should inherit deadline from parent
		<-shieldCtx.Done()
		return "", shieldCtx.Err()
	})
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected deadline exceeded, got %v", err)
	}

	if elapsed < 80*time.Millisecond || elapsed > 150*time.Millisecond {
		t.Errorf("Expected deadline around 100ms, got %v", elapsed)
	}
}

func TestShieldNilFunction(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	_, err := pyasyncio.Shield[string](ctx, nil)

	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Expected TimeoutError, got %T", err)
	}

	expectedMsg := "function cannot be nil"
	if timeoutErr.Error() != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, timeoutErr.Error())
	}
}

func TestWaitForConcurrentStress(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	const numOperations = 50

	counter := int64(0)
	var mu sync.Mutex
	results := make([]string, numOperations)
	errors := make([]error, numOperations)

	// Run many WaitFor operations concurrently
	for i := range numOperations {
		go func(index int) {
			result, err := pyasyncio.WaitFor(ctx, 1*time.Second, func(ctx context.Context) (string, error) {
				val := atomic.AddInt64(&counter, 1)
				time.Sleep(time.Duration(index%10) * time.Millisecond)
				return fmt.Sprintf("result-%d", val), nil
			})
			mu.Lock()
			results[index] = result
			errors[index] = err
			mu.Unlock()
		}(i)
	}

	// Wait for all to complete
	time.Sleep(2 * time.Second)

	// Check results
	mu.Lock()
	for i := range numOperations {
		if errors[i] != nil {
			t.Errorf("Operation %d failed: %v", i, errors[i])
		}
		if results[i] == "" {
			t.Errorf("Operation %d returned empty result", i)
		}
	}
	mu.Unlock()

	finalCounter := atomic.LoadInt64(&counter)
	if finalCounter != numOperations {
		t.Errorf("Expected %d operations, got %d", numOperations, finalCounter)
	}
}

func TestWaitForTaskIntegration(t *testing.T) {
	t.Parallel()

	// Test integration with TaskGroup patterns
	ctx := t.Context()
	tg := pyasyncio.NewTaskGroup[string](ctx)
	defer tg.Close()

	// Create tasks normally
	task1, err := tg.CreateTask(func(ctx context.Context) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "task1", nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	task2, err := tg.CreateTask(func(ctx context.Context) (string, error) {
		time.Sleep(30 * time.Millisecond)
		return "task2", nil
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Use WaitForTask to add timeout semantics to existing tasks
	result1, err1 := pyasyncio.WaitForTask(ctx, 1*time.Second, task1)
	result2, err2 := pyasyncio.WaitForTask(ctx, 1*time.Second, task2)

	if err1 != nil {
		t.Errorf("WaitForTask 1 failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("WaitForTask 2 failed: %v", err2)
	}

	if result1 != "task1" {
		t.Errorf("Expected 'task1', got '%s'", result1)
	}
	if result2 != "task2" {
		t.Errorf("Expected 'task2', got '%s'", result2)
	}

	// TaskGroup should complete successfully
	_, err = tg.Wait(ctx)
	if err != nil {
		t.Fatalf("TaskGroup failed: %v", err)
	}
}

func TestWaitForRealWorldPattern(t *testing.T) {
	t.Parallel()

	// Simulate a real-world scenario: API call with timeout
	ctx := t.Context()

	// Simulated API call
	apiCall := func(ctx context.Context) (map[string]any, error) {
		// Simulate network delay
		select {
		case <-time.After(200 * time.Millisecond):
			return map[string]any{
				"status": "success",
				"data":   "api response",
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Use WaitFor to add timeout to API call
	result, err := pyasyncio.WaitFor(ctx, 500*time.Millisecond, apiCall)
	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	if diff := cmp.Diff(map[string]any{
		"status": "success",
		"data":   "api response",
	}, result); diff != "" {
		t.Errorf("API response mismatch (-expected +actual):\n%s", diff)
	}

	// Test timeout scenario
	slowApiCall := func(ctx context.Context) (map[string]any, error) {
		select {
		case <-time.After(1 * time.Second):
			return map[string]any{"status": "success"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	_, err = pyasyncio.WaitFor(ctx, 100*time.Millisecond, slowApiCall)
	var timeoutErr *pyasyncio.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Errorf("Expected timeout for slow API call, got %v", err)
	}
}
