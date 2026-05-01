// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// openFile opens a file on Windows with FILE_SHARE_DELETE flag to allow
// other processes to delete or rename the file while it's open.
func openFile(path string) (*os.File, error) {
	if path == "" {
		return nil, &fs.PathError{Op: "open", Path: path, Err: syscall.ENOENT}
	}

	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}

	access := uint32(syscall.GENERIC_READ)
	sharemode := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE)
	createmode := uint32(syscall.OPEN_EXISTING)
	flags := uint32(syscall.FILE_ATTRIBUTE_NORMAL)

	handle, err := syscall.CreateFile(pathp, access, sharemode, nil, createmode, flags, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}

	if handle == syscall.InvalidHandle {
		return nil, &fs.PathError{Op: "open", Path: path, Err: syscall.EINVAL}
	}

	return os.NewFile(uintptr(handle), path), nil
}
