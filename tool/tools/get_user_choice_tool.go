// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"github.com/xenoweaver/adk-go/types"
)

// GetUserChoice provides the options to the user and asks them to choose one.
func GetUserChoice(_ []string, toolCtx *types.ToolContext) {
	toolCtx.Actions().SkipSummarization = true
}
