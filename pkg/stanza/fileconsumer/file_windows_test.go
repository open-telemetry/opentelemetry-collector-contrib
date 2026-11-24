// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/sys/windows"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "UNC path",
			input:    `\\server\share\file.txt`,
			expected: `\\?\UNC\server\share\file.txt`,
		},
		{
			name:     "UNC path with subdirectories",
			input:    `\\networkShare2\BLAM\Logs\stdout_46_2025.log`,
			expected: `\\?\UNC\networkShare2\BLAM\Logs\stdout_46_2025.log`,
		},
		{
			name:     "Already extended UNC path",
			input:    `\\?\UNC\server\share\file.txt`,
			expected: `\\?\UNC\server\share\file.txt`,
		},
		{
			name:     "UNC path with forward slashes",
			input:    `\\server\share/path/to/file.txt`,
			expected: `\\?\UNC\server\share\path\to\file.txt`,
		},
		{
			name:     "UNC path with dot segments",
			input:    `\\server\share\path\.\to\..\file.txt`,
			expected: `\\?\UNC\server\share\path\file.txt`,
		},
		{
			name:     "Regular local path",
			input:    `C:\Users\test\file.txt`,
			expected: `C:\Users\test\file.txt`,
		},
		{
			name:     "Relative path",
			input:    `.\test\file.txt`,
			expected: `test\file.txt`,
		},
		{
			name:     "Absolute path on current drive (single backslash start)",
			input:    `\Users\test\file.txt`,
			expected: `\Users\test\file.txt`,
		},
		{
			name:     "Local path starting with single backslash and subdirectories",
			input:    `\Program Files\app\data.log`,
			expected: `\Program Files\app\data.log`,
		},
		{
			name:     "Extended-length local path",
			input:    `\\?\C:\very\long\path\file.txt`,
			expected: `\\?\C:\very\long\path\file.txt`,
		},
		{
			name:     "Empty path",
			input:    ``,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := normalizePath(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestOpenFileWithShareDelete verifies that files opened with openFile
// can be deleted or renamed by other processes while still open.
func TestOpenFileWithShareDelete(t *testing.T) {
	t.Parallel()

	// Create a temporary file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.log")

	// Write some content to the file
	err := os.WriteFile(filePath, []byte("test content\n"), 0o600)
	require.NoError(t, err)

	// Open the file using our Windows-specific openFile function
	file, err := openFile(filePath)
	require.NoError(t, err)
	require.NotNil(t, file)
	defer file.Close()

	// Verify we can read from it
	buf := make([]byte, 13)
	n, err := file.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 13, n)
	require.Equal(t, "test content\n", string(buf))

	// Now try to delete the file while it's still open
	// This should succeed because we opened it with FILE_SHARE_DELETE
	err = os.Remove(filePath)
	require.NoError(t, err, "File should be deletable while open with FILE_SHARE_DELETE")

	// Verify the file is deleted (or marked for deletion)
	_, err = os.Stat(filePath)
	require.True(t, os.IsNotExist(err), "File should not exist after deletion")
}

// TestOpenFileWithShareDeleteRename verifies that files opened with openFile
// can be renamed by other processes while still open.
func TestOpenFileWithShareDeleteRename(t *testing.T) {
	t.Parallel()

	// Create a temporary file
	tempDir := t.TempDir()
	originalPath := filepath.Join(tempDir, "original.log")
	renamedPath := filepath.Join(tempDir, "renamed.log")

	// Write some content to the file
	err := os.WriteFile(originalPath, []byte("test content\n"), 0o600)
	require.NoError(t, err)

	// Open the file using our Windows-specific openFile function
	file, err := openFile(originalPath)
	require.NoError(t, err)
	require.NotNil(t, file)
	defer file.Close()

	// Verify we can read from it
	buf := make([]byte, 13)
	n, err := file.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 13, n)

	// Now try to rename the file while it's still open
	// This should succeed because we opened it with FILE_SHARE_DELETE
	err = os.Rename(originalPath, renamedPath)
	require.NoError(t, err, "File should be renameable while open with FILE_SHARE_DELETE")

	// Verify the original path no longer exists
	_, err = os.Stat(originalPath)
	require.True(t, os.IsNotExist(err), "Original file should not exist after rename")

	// Verify the new path exists
	_, err = os.Stat(renamedPath)
	require.NoError(t, err, "Renamed file should exist")
}

// TestReadAfterFileDeleted verifies behavior when reading from a file
// that was deleted while open. The entire file should remain readable
// through EOF since FILE_SHARE_DELETE allows deletion while the handle
// remains valid.
func TestReadAfterFileDeleted(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.log")

	// Write initial content
	err := os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o600)
	require.NoError(t, err)

	// Open with FILE_SHARE_DELETE
	file, err := openFile(filePath)
	require.NoError(t, err)
	defer file.Close()

	// Read first line
	buf := make([]byte, 6)
	_, err = file.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "line1\n", string(buf))

	// Delete the file while open
	err = os.Remove(filePath)
	require.NoError(t, err)

	// Verify the file is no longer visible on the filesystem
	_, err = os.Stat(filePath)
	require.True(t, os.IsNotExist(err), "File should not exist after deletion")

	// Continue reading - should still work since handle is still valid
	buf = make([]byte, 6)
	n, err := file.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 6, n)
	require.Equal(t, "line2\n", string(buf))

	// Read third line
	buf = make([]byte, 6)
	n, err = file.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 6, n)
	require.Equal(t, "line3\n", string(buf))

	// Verify EOF is reached normally
	_, err = file.Read(buf)
	require.ErrorIs(t, err, io.EOF, "Should reach EOF after reading all content")
}

func TestDeletePendingFileNotReportedUnreadable(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManagerKeepFilesOpen(t, cfg)
	core, observedLogs := observer.New(zap.ErrorLevel)
	operator.set.Logger = zap.New(core)

	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog\n")
	require.NoError(t, temp.Close())

	operator.poll(t.Context())
	sink.ExpectToken(t, []byte("testlog"))
	operator.wg.Wait()
	require.Len(t, operator.tracker.PreviousPollFiles(), 1)

	markDeletePending(t, temp.Name())
	_, err := openFile(temp.Name())
	require.ErrorIs(t, err, fs.ErrPermission, "file should be pending deletion while the operator holds its handle")

	operator.poll(t.Context())
	operator.wg.Wait()
	require.Empty(t, operator.unreadable)
	require.Zero(t, observedLogs.Len())
	require.Empty(t, operator.tracker.PreviousPollFiles())
}

// markDeletePending deletes path without POSIX semantics, so it stays delete-pending while other handles are open.
func markDeletePending(t *testing.T, path string) {
	t.Helper()

	pathp, err := windows.UTF16PtrFromString(path)
	require.NoError(t, err)
	sharemode := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	handle, err := windows.CreateFile(pathp, windows.DELETE, sharemode, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	require.NoError(t, err)

	deleteFile := byte(1)
	require.NoError(t, windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, &deleteFile, 1))
	require.NoError(t, windows.CloseHandle(handle))
}
