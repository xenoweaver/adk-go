// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

// Package xiter provides extended utility functions for working with Go 1.23+ iterators, complementing the standard iter package.
//
// The xiter package extends Go's standard iter package with additional utility functions
// that are commonly needed when working with iterators in real-world applications. It provides
// both adapted utilities from the Go tools repository and custom utilities specific to the ADK framework.
//
// # Overview
//
// This package provides utility functions that work with Go's iterator types:
//   - iter.Seq[T]: Single-value iterators
//   - iter.Seq2[T, E]: Two-value iterators (commonly used for value/error pairs)
//
// # Core Functionality
//
// The package provides several categories of iterator utilities:
//
// ## Sequence Operations
//   - First: Extract the first value from an iterator
//   - Contains: Check if a value exists in an iterator sequence
//
// ## Predicate Functions
//   - Every: Check if all elements satisfy a condition
//   - Any: Check if any element satisfies a condition
//
// ## Error Handling
//   - Error: Create iterators that yield errors
//   - EndError: Create iterators that yield errors at the end of iteration
//
// # Basic Usage
//
// ## First Element Extraction
//
// Extract the first value from any iterator:
//
//	// Get first value from a sequence
//	numbers := slices.Values([]int{1, 2, 3, 4, 5})
//	first, ok := xiter.First(numbers)
//	if ok {
//		fmt.Printf("First number: %d\n", first) // First number: 1
//	}
//
//	// Handle empty sequences
//	empty := slices.Values([]string{})
//	firstStr, ok := xiter.First(empty)
//	if !ok {
//		fmt.Println("Sequence is empty") // Sequence is empty
//	}
//
// ## Contains Check
//
// Check if a value exists in an iterator:
//
//	// Check if value exists in sequence
//	names := slices.Values([]string{"alice", "bob", "charlie"})
//	exists := xiter.Contains(names, "bob")   // true
//	missing := xiter.Contains(names, "david") // false
//
//	// Works with any comparable type
//	ages := slices.Values([]int{25, 30, 35, 40})
//	hasAge := xiter.Contains(ages, 35) // true
//
// ## Predicate Operations
//
// Test conditions across all or any elements:
//
//	// Check if all elements satisfy a condition
//	numbers := slices.Values([]int{2, 4, 6, 8})
//	allEven := xiter.Every(numbers, func(n int) bool {
//		return n%2 == 0
//	}) // true
//
//	// Check if any element satisfies a condition
//	mixed := slices.Values([]int{1, 3, 5, 6})
//	hasEven := xiter.Any(mixed, func(n int) bool {
//		return n%2 == 0
//	}) // true (because of 6)
//
// # Error Iterator Utilities
//
// ## Error Iterator Creation
//
// Create iterators that yield specific errors:
//
//	// Create an iterator that immediately yields an error
//	errorIter := xiter.Error[string](fmt.Errorf("something went wrong"))
//
//	for value, err := range errorIter {
//		if err != nil {
//			fmt.Printf("Error: %v\n", err) // Error: something went wrong
//			break
//		}
//		// value will be nil
//	}
//
// ## End Error Iterators
//
// Create iterators that yield an error at the end of iteration:
//
//	// Create an iterator that yields an error at the end
//	endErrorIter := xiter.EndError[int](fmt.Errorf("end of iteration"))
//
//	for value, err := range endErrorIter {
//		if err != nil {
//			fmt.Printf("End error: %v\n", err) // End error: end of iteration
//			break
//		}
//		// Process value (will be nil in this case)
//	}
//
// # Integration with ADK Framework
//
// ## Agent Event Streams
//
// The xiter utilities are used throughout the ADK for processing agent event streams:
//
//	// Check if any event contains an error
//	hasError := xiter.Any(agent.Run(ctx, ictx), func(eventErr struct{*types.Event; error}) bool {
//		return eventErr.error != nil
//	})
//
//	// Get the first successful event
//	firstEvent, ok := xiter.First(agent.Run(ctx, ictx))
//	if ok && firstEvent.error == nil {
//		// Process first successful event
//	}
//
// ## Error Propagation
//
// Create error iterators for consistent error handling:
//
//	func processAgent(ctx context.Context) iter.Seq2[*types.Event, error] {
//		if ctx.Err() != nil {
//			// Return an error iterator if context is cancelled
//			return xiter.Error[types.Event](ctx.Err())
//		}
//
//		// Normal processing...
//		return agent.Run(ctx, ictx)
//	}
//
// ## Stream Processing Validation
//
// Use predicate functions for stream validation:
//
//	// Validate that all events in a stream are well-formed
//	eventsValid := xiter.Every(eventStream, func(eventErr struct{*types.Event; error}) bool {
//		event, err := eventErr.*types.Event, eventErr.error
//		return err == nil && event != nil && event.Timestamp.After(startTime)
//	})
//
// # Performance Characteristics
//
// ## Time Complexity
//
//   - First: O(1) - Returns immediately after first element
//   - Contains: O(n) - May need to examine all elements in worst case
//   - Every: O(n) - May need to examine all elements, stops at first false
//   - Any: O(n) - May need to examine all elements, stops at first true
//   - Error/EndError: O(1) - Create iterator with constant time
//
// ## Memory Usage
//
// All functions use minimal memory:
//   - No additional allocations for predicate functions
//   - Iterator state is maintained efficiently
//   - Error iterators have minimal overhead
//
// # Common Patterns
//
// ## Safe First Element Access
//
//	// Pattern: Safe access with default value
//	func getFirstOrDefault[T any](seq iter.Seq[T], defaultValue T) T {
//		if first, ok := xiter.First(seq); ok {
//			return first
//		}
//		return defaultValue
//	}
//
//	// Usage
//	firstNumber := getFirstOrDefault(numbers, 0)
//	firstName := getFirstOrDefault(names, "unknown")
//
// ## Validation Chains
//
//	// Pattern: Multiple validation checks
//	func validateData[T any](seq iter.Seq[T], validators ...func(T) bool) bool {
//		for _, validator := range validators {
//			if !xiter.Every(seq, validator) {
//				return false
//			}
//		}
//		return true
//	}
//
//	// Usage
//	isValid := validateData(numbers,
//		func(n int) bool { return n > 0 },        // All positive
//		func(n int) bool { return n < 1000 },     // All less than 1000
//		func(n int) bool { return n%2 == 0 },     // All even
//	)
//
// ## Error Stream Handling
//
//	// Pattern: Graceful error handling in streams
//	func processWithErrorRecovery[T any](seq iter.Seq2[T, error]) iter.Seq2[T, error] {
//		return func(yield func(T, error) bool) {
//			for value, err := range seq {
//				if err != nil {
//					// Try to recover or provide fallback
//					if fallbackValue, recovered := attemptRecovery(err); recovered {
//						if !yield(fallbackValue, nil) {
//							return
//						}
//						continue
//					}
//				}
//				if !yield(value, err) {
//					return
//				}
//			}
//		}
//	}
//
// # Thread Safety
//
// All functions in this package are safe for concurrent use:
//   - Functions are stateless and operate on iterator parameters
//   - No shared mutable state between function calls
//   - Iterators themselves maintain their own thread safety guarantees
//
// # Best Practices
//
//  1. Use First() for safe access to the first element instead of manual iteration
//  2. Prefer Every()/Any() over manual loops for predicate testing
//  3. Use Contains() for membership testing in sequences
//  4. Create error iterators consistently for error propagation
//  5. Combine utilities for more complex iterator operations
//  6. Handle empty sequences appropriately in your code
//
// # Integration with Standard Library
//
// The package works seamlessly with Go's standard library:
//
//	import (
//		"iter"
//		"slices"
//		"maps"
//		"github.com/xenoweaver/adk-go/internal/xiter"
//	)
//
//	// Works with slices.Values
//	slice := []int{1, 2, 3, 4, 5}
//	first, ok := xiter.First(slices.Values(slice))
//
//	// Works with maps.Keys
//	m := map[string]int{"a": 1, "b": 2, "c": 3}
//	hasKey := xiter.Contains(maps.Keys(m), "b")
//
//	// Works with custom iterators
//	customSeq := func(yield func(int) bool) {
//		for i := 0; i < 10; i++ {
//			if !yield(i * i) {
//				return
//			}
//		}
//	}
//	anyLarge := xiter.Any(customSeq, func(n int) bool { return n > 50 })
//
// # Attribution
//
// Part of this package (moreiters.go) is adapted from the Go tools repository:
// https://github.com/golang/tools/blob/master/gopls/internal/util/moreiters/iters.go@2835a17831c9
//
// The original code is copyright The Go Authors and used under the BSD license.
// Additional utilities are original implementations for the ADK framework.
//
// # Future Extensions
//
// The package is designed to be extended with additional iterator utilities as needed:
//   - Transformation functions (Map, Filter, Reduce)
//   - Combination functions (Zip, Chain, Interleave)
//   - Aggregation functions (Count, Sum, GroupBy)
//   - Splitting and batching utilities
//   - Advanced error handling patterns
//
// The xiter package provides essential utilities for working with Go's modern iterator
// patterns while maintaining simplicity and performance.
package xiter
