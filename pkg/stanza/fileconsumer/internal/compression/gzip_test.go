// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package compression

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestIsGzipFile(t *testing.T) {
	t.Run("file is gzip compressed", func(t *testing.T) {
		temp, err := os.Create(filepath.Join(t.TempDir(), "test.log"))
		require.NoError(t, err)
		defer temp.Close()

		tempWrite := gzip.NewWriter(temp)
		_, err = tempWrite.Write([]byte("this is test data and the header should prove this is gzip"))
		require.NoError(t, err)
		tempWrite.Close()

		// set offset to start
		_, err = temp.Seek(0, io.SeekStart)
		require.NoError(t, err)

		require.True(t, IsGzipFile(temp, zap.NewNop()), "expected file to be detected as gzip compressed")
	})

	t.Run("file is NOT gzip compressed", func(t *testing.T) {
		tempFile, err := os.Create(filepath.Join(t.TempDir(), "test1.log"))
		require.NoError(t, err)
		defer tempFile.Close()

		_, err = tempFile.WriteString(
			"this is test data and the header should prove this is not gzip compressed")
		require.NoError(t, err)

		_, err = tempFile.Seek(0, io.SeekStart)
		require.NoError(t, err)

		require.False(t, IsGzipFile(tempFile, zap.NewNop()), "expected file to not be detected as gzip compressed")
	})
}
