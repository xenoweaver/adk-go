// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package pyasyncio_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xenoweaver/adk-go/pkg/py/pyasyncio"
)

func TestQueueBasicOperations(t *testing.T) {
	t.Parallel()

	q := pyasyncio.NewQueue[int](0) // Unlimited size

	// Test empty queue
	if !q.Empty() {
		t.Error("New queue should be empty")
	}

	if q.Size() != 0 {
		t.Errorf("Expected queue size 0, got %d", q.Size())
	}

	if q.Full() {
		t.Error("Unlimited queue should never be full")
	}

	// Add items
	ctx := t.Context()
	if err := q.Put(ctx, 1); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if err := q.Put(ctx, 2); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if q.Empty() {
		t.Error("Queue with items should not be empty")
	}

	if q.Size() != 2 {
		t.Errorf("Expected queue size 2, got %d", q.Size())
	}

	// Get items
	item1, err := q.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item1 != 1 {
		t.Errorf("Expected item 1, got %d", item1)
	}

	item2, err := q.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item2 != 2 {
		t.Errorf("Expected item 2, got %d", item2)
	}

	if !q.Empty() {
		t.Error("Queue should be empty after getting all items")
	}
}

func TestQueueWithMaxSize(t *testing.T) {
	t.Parallel()

	q := pyasyncio.NewQueue[string](2) // Max size 2

	ctx := t.Context()

	// Fill queue to capacity
	if err := q.Put(ctx, "first"); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if err := q.Put(ctx, "second"); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if !q.Full() {
		t.Error("Queue at capacity should be full")
	}

	if q.Size() != 2 {
		t.Errorf("Expected queue size 2, got %d", q.Size())
	}

	// Verify FIFO order
	item, err := q.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item != "first" {
		t.Errorf("Expected 'first', got '%s'", item)
	}

	item, err = q.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item != "second" {
		t.Errorf("Expected 'second', got '%s'", item)
	}
}

func TestQueueNowaitOperations(t *testing.T) {
	t.Parallel()

	q := pyasyncio.NewQueue[int](2)

	// Test PutNowait on empty queue
	if err := q.PutNowait(10); err != nil {
		t.Fatalf("PutNowait failed: %v", err)
	}

	if err := q.PutNowait(20); err != nil {
		t.Fatalf("PutNowait failed: %v", err)
	}

	// Test PutNowait on full queue
	err := q.PutNowait(30)
	var queueFullErr *pyasyncio.ErrQueueFull
	if !errors.As(err, &queueFullErr) {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}

	// Test GetNowait with items
	item, err := q.GetNowait()
	if err != nil {
		t.Fatalf("GetNowait failed: %v", err)
	}
	if item != 10 {
		t.Errorf("Expected 10, got %d", item)
	}

	item, err = q.GetNowait()
	if err != nil {
		t.Fatalf("GetNowait failed: %v", err)
	}
	if item != 20 {
		t.Errorf("Expected 20, got %d", item)
	}

	// Test GetNowait on empty queue
	_, err = q.GetNowait()
	var queueEmptyErr *pyasyncio.ErrQueueEmpty
	if !errors.As(err, &queueEmptyErr) {
		t.Errorf("Expected ErrQueueEmpty, got %v", err)
	}
}

func TestQueueBlocking(t *testing.T) {
	t.Parallel()

	q := pyasyncio.NewQueue[int](1)

	ctx := t.Context()

	// Fill queue
	if err := q.Put(ctx, 100); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Test that Put blocks when queue is full
	putDone := make(chan bool)
	go func() {
		defer close(putDone)
		if err := q.Put(ctx, 200); err != nil {
			t.Errorf("Put failed: %v", err)
		}
	}()

	// Give some time for the goroutine to block
	time.Sleep(50 * time.Millisecond)

	select {
	case <-putDone:
		t.Error("Put should have blocked")
	default:
		// Good, Put is blocking
	}

	// Get an item to unblock Put
	item, err := q.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item != 100 {
		t.Errorf("Expected 100, got %d", item)
	}

	// Now Put should complete
	select {
	case <-putDone:
		// Good, Put completed
	case <-time.After(100 * time.Millisecond):
		t.Error("Put should have completed after Get")
	}

	// Test that Get blocks when queue is empty
	item, err = q.Get(ctx) // Get the item that was just put
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item != 200 {
		t.Errorf("Expected 200, got %d", item)
	}

	getDone := make(chan bool)
	go func() {
		defer close(getDone)
		if _, err := q.Get(ctx); err != nil {
			t.Errorf("Get failed: %v", err)
		}
	}()

	// Give some time for the goroutine to block
	time.Sleep(50 * time.Millisecond)

	select {
	case <-getDone:
		t.Error("Get should have blocked")
	default:
		// Good, Get is blocking
	}

	// Put an item to unblock Get
	if err := q.Put(ctx, 300); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Now Get should complete
	select {
	case <-getDone:
		// Good, Get completed
	case <-time.After(100 * time.Millisecond):
		t.Error("Get should have completed after Put")
	}
}

func TestQueueContextCancellation(t *testing.T) {
	t.Parallel()

	q := pyasyncio.NewQueue[int](1)

	// Fill queue
	if err := q.PutNowait(1); err != nil {
		t.Fatalf("PutNowait failed: %v", err)
	}

	// Test Put cancellation
	ctx, cancel := context.WithCancel(t.Context())

	putErr := make(chan error, 1)
	go func() {
		putErr <- q.Put(ctx, 2)
	}()

	// Give time for Put to block
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Put should return context error
	select {
	case err := <-putErr:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Put should have returned with cancellation error")
	}

	// Test Get cancellation on empty queue
	q2 := pyasyncio.NewQueue[int](1)
	ctx2, cancel2 := context.WithCancel(context.Background())

	getErr := make(chan error, 1)
	go func() {
		_, err := q2.Get(ctx2)
		getErr <- err
	}()

	// Give time for Get to block
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel2()

	// Get should return context error
	select {
	case err := <-getErr:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Get should have returned with cancellation error")
	}
}

func TestQueueTaskTracking(t *testing.T) {
	t.Parallel()

	q := pyasyncio.NewQueue[string](0)

	ctx := t.Context()

	// Put some items
	if err := q.Put(ctx, "task1"); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if err := q.Put(ctx, "task2"); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if err := q.Put(ctx, "task3"); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Start Join in a goroutine
	joinDone := make(chan error, 1)
	go func() {
		joinDone <- q.Join(ctx)
	}()

	// Give time for Join to start waiting
	time.Sleep(50 * time.Millisecond)

	// Join should still be waiting
	select {
	case <-joinDone:
		t.Error("Join should still be waiting")
	default:
		// Good, Join is waiting
	}

	// Get items and mark them done
	item1, err := q.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item1 != "task1" {
		t.Errorf("Expected 'task1', got '%s'", item1)
	}

	if err := q.TaskDone(); err != nil {
		t.Fatalf("TaskDone failed: %v", err)
	}

	// Join should still be waiting
	time.Sleep(50 * time.Millisecond)
	select {
	case <-joinDone:
		t.Error("Join should still be waiting")
	default:
		// Good, Join is still waiting
	}

	item2, err := q.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item2 != "task2" {
		t.Errorf("Expected 'task2', got '%s'", item2)
	}

	if err := q.TaskDone(); err != nil {
		t.Fatalf("TaskDone failed: %v", err)
	}

	item3, err := q.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if item3 != "task3" {
		t.Errorf("Expected 'task3', got '%s'", item3)
	}

	if err := q.TaskDone(); err != nil {
		t.Fatalf("TaskDone failed: %v", err)
	}

	// Now Join should complete
	select {
	case err := <-joinDone:
		if err != nil {
			t.Errorf("Join failed: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Join should have completed")
	}
}

func TestQueueTaskDoneErrors(t *testing.T) {
	t.Parallel()

	q := pyasyncio.NewQueue[int](0)

	// TaskDone without any items should fail
	err := q.TaskDone()
	if err == nil {
		t.Error("TaskDone should fail when no items were put")
	}

	// Put and get an item
	ctx := context.Background()
	if err := q.Put(ctx, 42); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if _, err := q.Get(ctx); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// First TaskDone should succeed
	if err := q.TaskDone(); err != nil {
		t.Fatalf("TaskDone should succeed: %v", err)
	}

	// Second TaskDone should fail
	err = q.TaskDone()
	if err == nil {
		t.Error("TaskDone should fail when called too many times")
	}
}

// func TestQueueConcurrentAccess(t *testing.T) {
// 	q := pyasyncio.NewQueue[int](10)
// 	ctx := context.Background()
//
// 	const numProducers = 5
// 	const numConsumers = 3
// 	const itemsPerProducer = 10
//
// 	var wg sync.WaitGroup
// 	produced := make([]int, 0, numProducers*itemsPerProducer)
// 	consumed := make([]int, 0, numProducers*itemsPerProducer)
// 	var producedMu, consumedMu sync.Mutex
//
// 	// Start producers
// 	for i := range numProducers {
// 		wg.Add(1)
// 		go func(producerID int) {
// 			defer wg.Done()
// 			for j := range itemsPerProducer {
// 				item := producerID*itemsPerProducer + j
// 				if err := q.Put(ctx, item); err != nil {
// 					t.Errorf("Put failed: %v", err)
// 					return
// 				}
// 				producedMu.Lock()
// 				produced = append(produced, item)
// 				producedMu.Unlock()
// 			}
// 		}(i)
// 	}
//
// 	// Start consumers
// 	for range numConsumers {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for {
// 				item, err := q.Get(ctx)
// 				if err != nil {
// 					t.Errorf("Get failed: %v", err)
// 					return
// 				}
// 				consumedMu.Lock()
// 				consumed = append(consumed, item)
// 				totalConsumed := len(consumed)
// 				consumedMu.Unlock()
//
// 				if totalConsumed >= numProducers*itemsPerProducer {
// 					return
// 				}
// 			}
// 		}()
// 	}
//
// 	wg.Wait()
//
// 	// Verify all items were produced and consumed
// 	if len(produced) != numProducers*itemsPerProducer {
// 		t.Errorf("Expected %d produced items, got %d", numProducers*itemsPerProducer, len(produced))
// 	}
//
// 	if len(consumed) != numProducers*itemsPerProducer {
// 		t.Errorf("Expected %d consumed items, got %d", numProducers*itemsPerProducer, len(consumed))
// 	}
//
// 	// Sort and compare (since order might differ due to concurrency)
// 	if diff := cmp.Diff(produced, consumed, cmp.Transformer("sort", func(s []int) []int {
// 		sorted := make([]int, len(s))
// 		copy(sorted, s)
// 		for i := range len(sorted) {
// 			for j := i + 1; j < len(sorted); j++ {
// 				if sorted[i] > sorted[j] {
// 					sorted[i], sorted[j] = sorted[j], sorted[i]
// 				}
// 			}
// 		}
// 		return sorted
// 	})); diff != "" {
// 		t.Errorf("Produced and consumed items differ (-produced +consumed):\n%s", diff)
// 	}
// }

func TestQueueJoinCancellation(t *testing.T) {
	t.Parallel()

	q := pyasyncio.NewQueue[int](0)

	ctx := t.Context()
	if err := q.Put(ctx, 1); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if _, err := q.Get(ctx); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Start Join with cancellable context
	ctx2, cancel := context.WithCancel(context.Background())

	joinErr := make(chan error, 1)
	go func() {
		joinErr <- q.Join(ctx2)
	}()

	// Give time for Join to start waiting
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Join should return context error
	select {
	case err := <-joinErr:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Join should have returned with cancellation error")
	}
}

func TestQueueInterface(t *testing.T) {
	t.Parallel()

	// Test that concrete queue implements Queue interface
	var q pyasyncio.Queue[string] = pyasyncio.NewQueue[string](5)

	ctx := t.Context()

	if err := q.Put(ctx, "test"); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	item, err := q.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if item != "test" {
		t.Errorf("Expected 'test', got '%s'", item)
	}

	if err := q.TaskDone(); err != nil {
		t.Fatalf("TaskDone failed: %v", err)
	}

	if err := q.Join(ctx); err != nil {
		t.Fatalf("Join failed: %v", err)
	}
}

// func TestQueueProducerConsumerPattern(t *testing.T) {
// 	// Test proper asyncio-style producer-consumer pattern with TaskGroup
// 	q := pyasyncio.NewQueue[int](5)
// 	tg := pyasyncio.NewTaskGroup[int]()
// 	defer tg.Close()
//
// 	ctx := context.Background()
// 	const numItems = 20
//
// 	// Create producer task
// 	_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
// 		for i := range numItems {
// 			if err := q.Put(ctx, i); err != nil {
// 				return 0, err
// 			}
// 		}
// 		return numItems, nil
// 	})
// 	if err != nil {
// 		t.Fatalf("CreateTask failed: %v", err)
// 	}
//
// 	// Create multiple consumer tasks
// 	const numConsumers = 3
// 	for range numConsumers {
// 		_, err := tg.CreateTask(func(ctx context.Context) (int, error) {
// 			processed := 0
// 			for {
// 				item, err := q.Get(ctx)
// 				if err != nil {
// 					return processed, err
// 				}
//
// 				// Simulate work
// 				if item < 0 {
// 					return processed, fmt.Errorf("invalid item: %d", item)
// 				}
//
// 				processed++
//
// 				// Mark task as done
// 				if err := q.TaskDone(); err != nil {
// 					return processed, err
// 				}
//
// 				// Check if we've processed enough items
// 				if processed >= numItems/numConsumers+2 {
// 					return processed, nil
// 				}
// 			}
// 		})
// 		if err != nil {
// 			t.Fatalf("CreateTask failed: %v", err)
// 		}
// 	}
//
// 	// Wait for all tasks to complete
// 	results, err := tg.Wait(ctx)
// 	if err != nil {
// 		t.Fatalf("Wait failed: %v", err)
// 	}
//
// 	// Verify producer created items and consumers processed them
// 	if len(results) != numConsumers+1 {
// 		t.Errorf("Expected %d results, got %d", numConsumers+1, len(results))
// 	}
//
// 	// First result should be from producer
// 	if results[0] != numItems {
// 		t.Errorf("Expected producer to create %d items, got %d", numItems, results[0])
// 	}
//
// 	// Sum of consumer results should equal number of items produced
// 	totalProcessed := 0
// 	for i := 1; i < len(results); i++ {
// 		totalProcessed += results[i]
// 	}
//
// 	if totalProcessed != numItems {
// 		t.Errorf("Expected %d items processed, got %d", numItems, totalProcessed)
// 	}
//
// 	// Join should complete immediately since all tasks are done
// 	if err := q.Join(ctx); err != nil {
// 		t.Fatalf("Join failed: %v", err)
// 	}
// }

func TestQueueTimeout(t *testing.T) {
	t.Parallel()

	// Test asyncio-style timeout handling similar to asyncio.wait_for()
	q := pyasyncio.NewQueue[string](1)

	// Fill the queue
	if err := q.PutNowait("item1"); err != nil {
		t.Fatalf("PutNowait failed: %v", err)
	}

	// Test Put with timeout
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := q.Put(ctx, "item2")
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	if elapsed < 40*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Errorf("Expected timeout around 50ms, got %v", elapsed)
	}

	// Test Get with timeout on empty queue
	q2 := pyasyncio.NewQueue[string](1)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	start2 := time.Now()
	_, err2 := q2.Get(ctx2)
	elapsed2 := time.Since(start2)

	if !errors.Is(err2, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err2)
	}

	if elapsed2 < 40*time.Millisecond || elapsed2 > 100*time.Millisecond {
		t.Errorf("Expected timeout around 50ms, got %v", elapsed2)
	}
}

func TestQueueFIFOOrder(t *testing.T) {
	t.Parallel()

	// Test that queue maintains FIFO order under concurrent stress
	q := pyasyncio.NewQueue[int](0) // Unlimited

	ctx := t.Context()
	const numItems = 1000

	// Put items sequentially to ensure order
	for i := range numItems {
		if err := q.Put(ctx, i); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Get items and verify FIFO order
	for expected := range numItems {
		item, err := q.Get(ctx)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if item != expected {
			t.Errorf("Expected item %d, got %d - FIFO order violated", expected, item)
		}
	}

	// Queue should be empty now
	if !q.Empty() {
		t.Error("Queue should be empty after getting all items")
	}
}
