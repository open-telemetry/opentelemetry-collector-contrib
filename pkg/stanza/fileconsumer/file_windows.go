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
// for reliable file access on Windows.
func normalizePath(path string) string {
	if len(path) == 0 {
		return path
	}

	// Already in extended-length format
	if strings.HasPrefix(path, `\\?\`) {
		return path
	}

	// Convert proper UNC paths (\\server\share) to extended-length format
	if len(path) >= 2 && path[0] == '\\' && path[1] == '\\' {
		return `\\?\UNC\` + filepath.Clean(path[2:])
	}

	// Handle corrupted UNC path that lost one backslash (\server\share)
	// This can happen after certain path operations
	if path[0] == '\\' {
		// Check if it looks like a UNC path: \hostname\share\...
		if idx := strings.Index(path[1:], "\\"); idx > 0 {
			// Has at least \server\share pattern, treat as UNC
			return `\\?\UNC\` + filepath.Clean(path[1:])
		}
	}

	// For non-UNC paths, just clean normally
	return filepath.Clean(path)
}
