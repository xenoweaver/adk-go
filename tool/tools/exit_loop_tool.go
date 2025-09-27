// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"github.com/xenoweaver/adk-go/types"
)

// ExitLoop exits the loop.
//
// Call this function only when you are instructed to do so.
func ExitLoop(toolCtx *types.ToolContext) {
	toolCtx.Actions().Escalate = true
}
