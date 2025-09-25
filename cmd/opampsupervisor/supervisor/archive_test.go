// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func TestNewInstallFunc(t *testing.T) {
	testCases := []struct {
		name          string
		archiveFormat config.Archive
		expectedError string
	}{
		{"tar.gz", config.ArchiveTarGzip, ""},
		{"default", config.ArchiveDefault, ""},
		{"unsupported", config.Archive("unsupported"), "unsupported archive format: unsupported"},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			installFunc, err := NewInstallFunc(tt.archiveFormat)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
				assert.Nil(t, installFunc)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, installFunc)
		})
	}
}

func TestInstallFunc(t *testing.T) {
	testCases := []struct {
		name          string
		installFunc   InstallFunc
		archive       func(t *testing.T) []byte
		binaryName    string
		destination   func(dir string) string
		expectedError string
	}{
		{
			name:        "tar.gz install success",
			installFunc: tarGzipInstall,
			archive: func(t *testing.T) []byte {
				// Create a simple test binary content
				testBinaryContent := []byte("test data")
				return createTarGzArchive(t, "test", testBinaryContent)
			},
			binaryName: "test",
			destination: func(dir string) string {
				return filepath.Join(dir, "test")
			},
			expectedError: "",
		},
		{
			name:        "tar.gz install failure",
			installFunc: tarGzipInstall,
			archive: func(t *testing.T) []byte {
				var gzipBuf bytes.Buffer
				gzipWriter := gzip.NewWriter(&gzipBuf)
				defer gzipWriter.Close()
				_, err := gzipWriter.Write([]byte("test data"))
				assert.NoError(t, err)
				return gzipBuf.Bytes()
			},
			binaryName: "test",
			destination: func(dir string) string {
				return filepath.Join(dir, "test")
			},
			expectedError: "first tarball read for collector: unexpected EOF",
		},
		{
			name:        "default install success",
			installFunc: defaultInstall,
			archive: func(_ *testing.T) []byte {
				return []byte("test data")
			},
			binaryName: "test",
			destination: func(dir string) string {
				return filepath.Join(dir, "test")
			},
			expectedError: "",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			destination := tt.destination(t.TempDir())
			err := tt.installFunc(t.Context(), tt.archive(t), tt.binaryName, destination)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.FileExists(t, destination)
			}
		})
	}
}

// createTarGzArchive creates a tar.gz archive containing the specified binary content
func createTarGzArchive(t *testing.T, binaryName string, binaryContent []byte) []byte {
	var tarBuf bytes.Buffer

	// First create the tar archive
	tarWriter := tar.NewWriter(&tarBuf)

	// Create tar header for the binary
	header := &tar.Header{
		Name: binaryName,
		Mode: 0o755,
		Size: int64(len(binaryContent)),
	}

	// Write the header
	if err := tarWriter.WriteHeader(header); err != nil {
		assert.NoError(t, err)
	}

	// Write the binary content
	if _, err := tarWriter.Write(binaryContent); err != nil {
		assert.NoError(t, err)
	}

	// Close the tar writer to finalize the tar archive
	if err := tarWriter.Close(); err != nil {
		assert.NoError(t, err)
	}

	// Now compress the entire tar archive with gzip
	var gzipBuf bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzipBuf)
	defer gzipWriter.Close()

	if _, err := gzipWriter.Write(tarBuf.Bytes()); err != nil {
		assert.NoError(t, err)
	}

	if err := gzipWriter.Close(); err != nil {
		assert.NoError(t, err)
	}

	return gzipBuf.Bytes()
}
