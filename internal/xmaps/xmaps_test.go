// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package xmaps_test

import (
	"fmt"
	"testing"

	"github.com/xenoweaver/adk-go/internal/xmaps"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]int
		key  string
		want bool
	}{
		{
			name: "key exists",
			m:    map[string]int{"a": 1, "b": 2, "c": 3},
			key:  "b",
			want: true,
		},
		{
			name: "key does not exist",
			m:    map[string]int{"a": 1, "b": 2, "c": 3},
			key:  "d",
			want: false,
		},
		{
			name: "empty map",
			m:    map[string]int{},
			key:  "a",
			want: false,
		},
		{
			name: "case sensitivity",
			m:    map[string]int{"a": 1, "B": 2, "c": 3},
			key:  "b",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xmaps.Contains(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsWithIntegers(t *testing.T) {
	tests := []struct {
		name string
		m    map[int]string
		key  int
		want bool
	}{
		{
			name: "key exists",
			m:    map[int]string{1: "a", 2: "b", 3: "c"},
			key:  2,
			want: true,
		},
		{
			name: "key does not exist",
			m:    map[int]string{1: "a", 2: "b", 3: "c"},
			key:  4,
			want: false,
		},
		{
			name: "empty map",
			m:    map[int]string{},
			key:  1,
			want: false,
		},
		{
			name: "negative key",
			m:    map[int]string{-1: "a", 0: "b", 1: "c"},
			key:  -1,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xmaps.Contains(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

var (
	benchBool   bool
	benchString string
)

// Benchmark to compare our Contains function with direct map lookup
func BenchmarkContains(b *testing.B) {
	mapSizes := []int{10, 100, 1000, 10000}

	for _, size := range mapSizes {
		b.Run(fmt.Sprintf("map size %d", size), func(b *testing.B) {
			// Setup: create a map with 'size' elements
			m := make(map[int]string, size)
			for i := range size {
				m[i] = "value"
			}

			// Key that exists in the map
			existingKey := size / 2
			// Key that doesn't exist in the map
			missingKey := size + 1

			b.Run("Contains-existing", func(b *testing.B) {
				b.ResetTimer()
				for b.Loop() {
					benchBool = xmaps.Contains(m, existingKey)
				}
			})

			b.Run("Direct-existing", func(b *testing.B) {
				b.ResetTimer()
				for b.Loop() {
					benchString, benchBool = m[existingKey]
				}
			})

			b.Run("Contains-missing", func(b *testing.B) {
				b.ResetTimer()
				for b.Loop() {
					benchBool = xmaps.Contains(m, missingKey)
				}
			})

			b.Run("Direct-missing", func(b *testing.B) {
				b.ResetTimer()
				for b.Loop() {
					benchString, benchBool = m[missingKey]
				}
			})
		})
	}
}
