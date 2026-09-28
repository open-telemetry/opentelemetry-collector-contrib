// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExtractor(t *testing.T) {
	testCases := []struct {
		name      string
		format    Format
		expectErr string
	}{
		{
			name:   "no archive returns raw extractor",
			format: FormatNone,
		},
		{
			name:   "tar.gz returns tar.gz installer",
			format: FormatTarGzip,
		},
		{
			name:      "unsupported archive format",
			format:    Format("zip"),
			expectErr: "unsupported archive format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			extractor, err := NewExtractor(tc.format)
			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				assert.Nil(t, extractor)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, extractor)
		})
	}
}

func TestRawExtractor_Extract(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "otelcol-contrib")
	contents := []byte("raw collector binary")

	extractor, err := NewExtractor(FormatNone)
	require.NoError(t, err)

	require.NoError(t, extractor.Extract(t.Context(), contents, "otelcol-contrib", destination))

	written, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, contents, written)
}

func TestRawExtractor_Extract_Size(t *testing.T) {
	const maxBytes = 16

	testCases := []struct {
		name      string
		size      int
		expectErr string
	}{
		{
			name: "under limit",
			size: maxBytes - 1,
		},
		{
			name: "at limit",
			size: maxBytes,
		},
		{
			name:      "over limit is rejected",
			size:      maxBytes + 1,
			expectErr: "binary exceeds maximum size",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "otelcol-contrib")
			contents := bytes.Repeat([]byte("a"), tc.size)

			extractor := rawExtractor{maxBytes: maxBytes}
			err := extractor.Extract(t.Context(), contents, "otelcol-contrib", destination)
			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)

			written, err := os.ReadFile(destination)
			require.NoError(t, err)
			assert.Equal(t, contents, written)
		})
	}
}

func TestTarGzipExtractor_Extract(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "otelcol-contrib")
		contents := []byte("collector binary inside a tarball")
		archive := createTarGzArchive(t, map[string][]byte{
			"README.md":       []byte("not the binary"),
			"otelcol-contrib": contents,
		})

		extractor, err := NewExtractor(FormatTarGzip)
		require.NoError(t, err)

		require.NoError(t, extractor.Extract(t.Context(), archive, "otelcol-contrib", destination))

		written, err := os.ReadFile(destination)
		require.NoError(t, err)
		assert.Equal(t, contents, written)
	})

	t.Run("missing binary name", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "otelcol-contrib")
		archive := createTarGzArchive(t, map[string][]byte{"otelcol-contrib": []byte("binary")})

		err := tarGzipExtractor{maxBytes: maxAgentBytes}.Extract(t.Context(), archive, "", destination)
		require.ErrorContains(t, err, "agent binary name is required")
	})

	t.Run("binary not in archive", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "otelcol-contrib")
		archive := createTarGzArchive(t, map[string][]byte{"some-other-file": []byte("binary")})

		err := tarGzipExtractor{maxBytes: maxAgentBytes}.Extract(t.Context(), archive, "otelcol-contrib", destination)
		require.ErrorContains(t, err, `read tarball looking for "otelcol-contrib"`)
	})

	t.Run("not a gzip archive", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "otelcol-contrib")

		err := tarGzipExtractor{maxBytes: maxAgentBytes}.Extract(t.Context(), []byte("not gzip data"), "otelcol-contrib", destination)
		require.ErrorContains(t, err, "create gzip reader")
	})

	t.Run("binary exceeds max size", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "otelcol-contrib")
		archive := createTarGzArchive(t, map[string][]byte{
			"otelcol-contrib": []byte("contents larger than the configured cap"),
		})

		err := tarGzipExtractor{maxBytes: 8}.Extract(t.Context(), archive, "otelcol-contrib", destination)
		require.ErrorContains(t, err, "binary exceeds maximum size of 8 bytes")

		// The size check happens before the destination is opened.
		_, statErr := os.Stat(destination)
		require.True(t, os.IsNotExist(statErr))
	})
}

// createTarGzArchive builds an in-memory gzipped tarball containing the given
// files (name -> contents).
func createTarGzArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	for name, contents := range files {
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(contents)),
		}))
		_, err := tarWriter.Write(contents)
		require.NoError(t, err)
	}

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	return buf.Bytes()
}
