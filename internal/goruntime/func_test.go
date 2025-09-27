// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package goruntime

import (
	"testing"
)

type funcTest struct{}

//go:noinline
func (funcTest) method() string { return Func() }

//go:noinline
func (*funcTest) pmethod() string { return Func() }

func (funcTest) method2() string { return funcTest{}.method() }
func (funcTest) method3() string { return funcTest{}.method2() }

func TestFunc(t *testing.T) {
	if Func() != "github.com/xenoweaver/adk-go/internal/goruntime.TestFunc" {
		t.Fatalf("Func() = %s, want: %s", Func(), "github.com/xenoweaver/adk-go/internal/goruntime.TestFunc")
	}

	if (funcTest{}).method() != "github.com/xenoweaver/adk-go/internal/goruntime.funcTest.method" {
		t.Fatalf("Func() = %s, want: %s", (funcTest{}).method(), "github.com/xenoweaver/adk-go/internal/goruntime.funcTest.method")
	}

	if (funcTest{}).method2() != "github.com/xenoweaver/adk-go/internal/goruntime.funcTest.method" {
		t.Fatalf("Func() = %s, want: %s", (funcTest{}).method2(), "github.com/xenoweaver/adk-go/internal/goruntime.funcTest.method")
	}

	if (funcTest{}).method3() != "github.com/xenoweaver/adk-go/internal/goruntime.funcTest.method" {
		t.Fatalf("Func() = %s, want: %s", (funcTest{}).method3(), "github.com/xenoweaver/adk-go/internal/goruntime.funcTest.method")
	}

	if new(funcTest).pmethod() != "github.com/xenoweaver/adk-go/internal/goruntime.(*funcTest).pmethod" {
		t.Fatalf("Func() = %s, want: %s", new(funcTest).pmethod(), "github.com/xenoweaver/adk-go/internal/goruntime.(*funcTest).pmethod")
	}
}

func TestFuncN(t *testing.T) {
	if FuncN(0) != "github.com/xenoweaver/adk-go/internal/goruntime.TestFuncN" {
		t.Fatalf("")
		t.Fatalf("ThisN(0) = %s, want: %s", FuncN(0), "github.com/xenoweaver/adk-go/internal/goruntime.TestFuncN")
	}
	if FuncN(1) != "testing.tRunner" {
		t.Fatalf("")
		t.Fatalf("ThisN(0) = %s, want: %s", FuncN(1), "testing.tRunner")
	}
}

func BenchmarkFunc(b *testing.B) {
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			Func()
		}
	})

	b.Run("Inlined", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			funcTest{}.method()
		}
	})

	b.Run("InlinedTwice", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			funcTest{}.method2()
		}
	})

	b.Run("InlinedThrice", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			funcTest{}.method3()
		}
	})
}

func BenchmarkFuncN(b *testing.B) {
	b.Run("Direct", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			FuncN(1)
		}
	})
}
