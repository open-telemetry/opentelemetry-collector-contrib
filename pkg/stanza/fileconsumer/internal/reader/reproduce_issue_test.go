// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateGzipReader_ResetDecompressedBytesToSkip verifies that
// decompressedBytesToSkip is reset to 0 at the start of createGzipReader,
// ensuring clean state for each file processing attempt.
// This fixes the bug where concurrent processing of gzipped files with
// matching fingerprints could cause "gzip: invalid header" errors.
func TestCreateGzipReader_ResetDecompressedBytesToSkip(t *testing.T) {
	tempDir := t.TempDir()
	lines := []string{
		"Log line 1",
		"Log line 2",
	}

	// Create a gzip file
	gzipPath := filepath.Join(tempDir, "test.log.gz")
	gzipFile, err := os.Create(gzipPath)
	require.NoError(t, err)
	gzWriter := gzip.NewWriter(gzipFile)
	_, err = gzWriter.Write([]byte(strings.Join(lines, "\n") + "\n"))
	require.NoError(t, err)
	require.NoError(t, gzWriter.Close())
	require.NoError(t, gzipFile.Close())

	f, _ := testFactory(t, withCompression("gzip"))

	gzipFileRead, err := os.Open(gzipPath)
	require.NoError(t, err)
	defer gzipFileRead.Close()

	fp, err := f.NewFingerprint(gzipFileRead)
	require.NoError(t, err)

	r, err := f.NewReader(gzipFileRead, fp)
	require.NoError(t, err)

	// Manually set decompressedBytesToSkip to a non-zero value.
	// This simulates a state where it was set but createGzipReader is called.
	r.decompressedBytesToSkip = 10

	// First call to createGzipReader should work and reset it.
	_, err = r.createGzipReader()
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.decompressedBytesToSkip)

	// Verify that after the first call, r.Offset was updated to the compressed EOF.
	// This ensures subsequent calls will have the correct starting position.
	assert.True(t, r.Offset > 0, "Offset should be updated after createGzipReader")
}

// TestCreateGzipReader_ConcurrentReset verifies that reset at the start of
// createGzipReader ensures clean state when called multiple times.
// This addresses the issue where decompressedBytesToSkip wasn't reset between
// file processing attempts, causing "gzip: invalid header" errors.
func TestCreateGzipReader_ConcurrentReset(t *testing.T) {
	tempDir := t.TempDir()
	lines := []string{
		"Log line 1",
		"Log line 2",
	}

	// Create a gzip file
	gzipPath := filepath.Join(tempDir, "test.log.gz")
	gzipFile, err := os.Create(gzipPath)
	require.NoError(t, err)
	gzWriter := gzip.NewWriter(gzipFile)
	_, err = gzWriter.Write([]byte(strings.Join(lines, "\n") + "\n"))
	require.NoError(t, err)
	require.NoError(t, gzWriter.Close())
	require.NoError(t, gzipFile.Close())

	f, _ := testFactory(t, withCompression("gzip"))

	gzipFileRead, err := os.Open(gzipPath)
	require.NoError(t, err)
	defer gzipFileRead.Close()

	fp, err := f.NewFingerprint(gzipFileRead)
	require.NoError(t, err)

	r, err := f.NewReader(gzipFileRead, fp)
	require.NoError(t, err)

	// Simulate the scenario: set decompressedBytesToSkip and call createGzipReader multiple times.
	// Without the fix at the start, the second call would use the stale value.
	r.decompressedBytesToSkip = 5

	// First call - should work and reset the value.
	_, err = r.createGzipReader()
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.decompressedBytesToSkip)

	// Second call - without the fix at the start, this would fail or use stale state.
	// With the fix, it starts fresh and works correctly.
	_, err = r.createGzipReader()
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.decompressedBytesToSkip)

	// Third call - verify consistent behavior.
	_, err = r.createGzipReader()
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.decompressedBytesToSkip)
}
