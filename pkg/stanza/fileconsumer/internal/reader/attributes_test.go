// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
)

// AddFields tests that the `log.file.name` and `log.file.path` fields are included
// when IncludeFileName and IncludeFilePath are set to true
func TestAttributes(t *testing.T) {
	t.Parallel()

	// Create a file, then start
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog\n")

	f, sink := testFactory(t, includeFileName(), includeFilePath())
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	reader, err := f.NewReader(temp, fp)
	require.NoError(t, err)
	defer reader.Close()

	reader.ReadToEnd(context.Background())

	token, attributes := sink.NextCall(t)
	require.Equal(t, []byte("testlog"), token)
	require.Equal(t, filepath.Base(temp.Name()), attributes[attrs.LogFileName])
	require.Equal(t, temp.Name(), attributes[attrs.LogFilePath])
	require.Nil(t, attributes[attrs.LogFileNameResolved])
	require.Nil(t, attributes[attrs.LogFilePathResolved])
}

// TestAttributesResolved tests that the `log.file.name_resolved` and `log.file.path_resolved` fields are included
// when IncludeFileNameResolved and IncludeFilePathResolved are set to true
func TestAttributesResolved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlinks usage disabled for now. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21088")
	}
	t.Parallel()

	// Set up actual file
	actualDir := t.TempDir()
	actualFile, err := os.CreateTemp(actualDir, "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, actualFile.Close()) })

	// Resolve path
	realPath, err := filepath.EvalSymlinks(actualFile.Name())
	require.NoError(t, err)
	resolved, err := filepath.Abs(realPath)
	require.NoError(t, err)

	// Create another directory with a symbolic link to the file
	symlinkDir := t.TempDir()
	symlinkPath := filepath.Join(symlinkDir, "symlink")
	require.NoError(t, os.Symlink(actualFile.Name(), symlinkPath))
	symlinkFile := filetest.OpenFile(t, symlinkPath)

	// Populate data
	filetest.WriteString(t, actualFile, "testlog\n")

	// Read the data
	f, sink := testFactory(t, includeFileName(), includeFilePath(), includeFileNameResolved(), includeFilePathResolved())
	fp, err := f.NewFingerprint(symlinkFile)
	require.NoError(t, err)
	reader, err := f.NewReader(symlinkFile, fp)
	require.NoError(t, err)
	defer reader.Close()
	reader.ReadToEnd(context.Background())

	// Validate expectations
	token, attributes := sink.NextCall(t)
	require.Equal(t, []byte("testlog"), token)
	require.Equal(t, filepath.Base(symlinkPath), attributes[attrs.LogFileName])
	require.Equal(t, symlinkPath, attributes[attrs.LogFilePath])
	require.Equal(t, filepath.Base(resolved), attributes[attrs.LogFileNameResolved])
	require.Equal(t, resolved, attributes[attrs.LogFilePathResolved])

	// Move symlinked file
	newActualPath := actualFile.Name() + "_renamed"
	require.NoError(t, os.Remove(symlinkPath))
	require.NoError(t, os.Rename(actualFile.Name(), newActualPath))
	require.NoError(t, os.Symlink(newActualPath, symlinkPath))

	// Append additional data
	filetest.WriteString(t, actualFile, "testlog2\n")

	// Recreate the reader
	symlinkFile = filetest.OpenFile(t, symlinkPath)
	reader, err = f.NewReaderFromMetadata(symlinkFile, reader.Close())
	require.NoError(t, err)
	reader.ReadToEnd(context.Background())

	token, attributes = sink.NextCall(t)
	require.Equal(t, []byte("testlog2"), token)
	require.Equal(t, filepath.Base(symlinkPath), attributes[attrs.LogFileName])
	require.Equal(t, symlinkPath, attributes[attrs.LogFilePath])
	require.Equal(t, filepath.Base(resolved)+"_renamed", attributes[attrs.LogFileNameResolved])
	require.Equal(t, resolved+"_renamed", attributes[attrs.LogFilePathResolved])

	// Append additional data
	filetest.WriteString(t, actualFile, "testlog3\n")

	// Recreate the factory with the attributes disabled
	f, sink = testFactory(t)

	// Recreate the reader and read new data
	symlinkFile = filetest.OpenFile(t, symlinkPath)
	reader, err = f.NewReaderFromMetadata(symlinkFile, reader.Close())
	require.NoError(t, err)
	reader.ReadToEnd(context.Background())

	token, attributes = sink.NextCall(t)
	require.Equal(t, []byte("testlog3"), token)
	require.Nil(t, attributes[attrs.LogFileName])
	require.Nil(t, attributes[attrs.LogFilePath])
	require.Nil(t, attributes[attrs.LogFileNameResolved])
	require.Nil(t, attributes[attrs.LogFilePathResolved])
}
