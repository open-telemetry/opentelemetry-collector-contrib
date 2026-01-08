// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"path/filepath"
	"strings"
)

// Noop on windows because we close files immediately after reading.
func (*Manager) readLostFiles(context.Context) {
}

// normalizePath ensures Windows UNC paths are properly formatted for os.Open().
// It converts UNC paths to extended-length format (\\?\UNC\server\share\path)
// for reliable file access on Windows. Extended-length paths bypass path parsing
// and provide more reliable access to network shares.
// Returns the normalized path and a boolean (always false, kept for API compatibility).
func normalizePath(path string) (string, bool) {
	if path == "" {
		return path, false
	}

	// Already in extended-length format
	if strings.HasPrefix(path, `\\?\`) {
		return path, false
	}

	// Convert proper UNC paths (\\server\share) to extended-length format
	// Note: We check for exactly 2 leading backslashes to identify UNC paths
	if len(path) >= 2 && path[0] == '\\' && path[1] == '\\' {
		// Extract the path after the \\ prefix and clean it
		cleaned := filepath.Clean(path[2:])
		return `\\?\UNC\` + cleaned, false
	}

	// For non-UNC paths (including paths starting with single backslash which are
	// valid local paths on the current drive), just clean normally
	return filepath.Clean(path), false
}
