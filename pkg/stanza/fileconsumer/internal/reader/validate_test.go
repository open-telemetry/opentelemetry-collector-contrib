// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
)

// When a file it moved, we should detect that our old handle is still valid.
func TestValidateMoved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Moving files while open is unsupported on Windows")
	}
	t.Parallel()

	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	_, err := temp.WriteString("testlog1\n")
	require.NoError(t, err)

	f, sink := testFactory(t)
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	reader, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	reader.ReadToEnd(context.Background())
	sink.ExpectToken(t, []byte("testlog1"))

	// Validate before moving
	assert.True(t, reader.Validate())

	// Move the file
	require.NoError(t, os.Rename(temp.Name(), temp.Name()+".old"))

	// Validate after moving
	assert.True(t, reader.Validate())

	_, err = temp.WriteString("testlog2\n")
	require.NoError(t, err)

	reader.ReadToEnd(context.Background())
	sink.ExpectToken(t, []byte("testlog2"))

	// Validate after writing to the moved file
	assert.True(t, reader.Validate())
}

func TestInvalidateTruncated(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	_, err := temp.WriteString("testlog1\n")
	require.NoError(t, err)

	f, sink := testFactory(t)
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	reader, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	reader.ReadToEnd(context.Background())
	sink.ExpectToken(t, []byte("testlog1"))

	// Validate before truncating
	assert.True(t, reader.Validate())

	// Truncate the file
	require.NoError(t, temp.Truncate(0))

	// Invalidate after truncating
	assert.False(t, reader.Validate())

	// Write different content to the file
	_, err = temp.WriteString("testlog2\n")
	require.NoError(t, err)

	// Still invalid
	assert.False(t, reader.Validate())
}

func TestInvalidateClosed(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	_, err := temp.WriteString("testlog1\n")
	require.NoError(t, err)

	f, _ := testFactory(t)
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	reader, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	// Validate before closing
	assert.True(t, reader.Validate())

	// Close the file using the reader to drop the handle.
	reader.Close()

	// Invalidate after closing
	assert.False(t, reader.Validate())
}

func TestInvalidateUnreadable(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	_, err := temp.WriteString("testlog1\n")
	require.NoError(t, err)

	f, _ := testFactory(t)
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	reader, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	// Validate before closing
	assert.True(t, reader.Validate())

	// Close the file using our direct handle. The reader still has a handle but cannot be read.
	require.NoError(t, temp.Close())

	// Invalidate unreadable file
	assert.False(t, reader.Validate())
}
