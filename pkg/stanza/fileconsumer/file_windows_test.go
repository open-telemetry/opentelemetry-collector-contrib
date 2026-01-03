// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
