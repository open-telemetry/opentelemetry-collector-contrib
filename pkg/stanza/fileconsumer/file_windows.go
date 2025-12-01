// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"
)

// Noop on windows because we close files immediately after reading.
func (*Manager) readLostFiles(context.Context) {
}

// normalizePath ensures Windows UNC paths are properly formatted for os.Open().
// It uses syscall.FullPath to resolve paths and converts UNC paths to extended-length
// format (\\?\UNC\server\share\path) for reliable file access on Windows.
func normalizePath(path string) string {
	// Already in extended-length format, use as-is
	if strings.HasPrefix(path, `\\?\`) {
		return path
	}

	// Use syscall.FullPath to resolve the absolute path
	// This handles relative paths, cleans the path, and normalizes UNC paths
	fullPath, err := syscall.FullPath(path)
	if err != nil {
		// If FullPath fails, return cleaned path as fallback
		return filepath.Clean(path)
	}

	// Convert UNC paths (\\server\share) to extended-length format (\\?\UNC\server\share)
	// This ensures reliable file access and supports paths longer than MAX_PATH
	if len(fullPath) >= 2 && fullPath[0] == '\\' && fullPath[1] == '\\' {
		return `\\?\UNC\` + fullPath[2:]
	}

	return fullPath
}
