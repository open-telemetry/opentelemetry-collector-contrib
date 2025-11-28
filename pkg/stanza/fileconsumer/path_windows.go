// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"path/filepath"
	"strings"
)

// normalizePath ensures Windows UNC paths are properly formatted for os.Open().
// UNC paths returned by filepath globbing may not work correctly with os.Open().
// Converting them to extended-length format ensures reliable file access.
//
// Windows supports extended-length paths with the \\?\ prefix, which:
// - Disables path parsing and passes the path directly to the filesystem
// - Supports paths longer than MAX_PATH (260 characters)
// - Requires absolute paths and does not support . or .. components
//
// For UNC paths, the format is: \\?\UNC\server\share\path\file.txt
// This is more reliable than the standard \\server\share\path\file.txt format
// when the path has been processed by filepath.Clean() or similar functions.
func normalizePath(path string) string {
	// Check if this is a UNC path (starts with \\)
	if len(path) >= 2 && path[0] == '\\' && path[1] == '\\' {
		// UNC path detected, ensure it's properly formatted
		// Convert to the extended-length path format which is more reliable
		// Extended-length UNC paths use the \\?\UNC\ prefix
		if !strings.HasPrefix(path, `\\?\`) {
			// Remove the leading \\ and prepend \\?\UNC\
			// Clean the path after removing the UNC prefix to normalize it
			cleaned := filepath.Clean(path[2:])
			return `\\?\UNC\` + cleaned
		}
	}
	// For non-UNC paths, just clean normally
	return filepath.Clean(path)
}
