// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInstaller(t *testing.T) {
	testCases := []struct {
		name      string
		format    Format
		expectErr string
		expectNil bool
	}{
		{
			name:   "no archive returns raw installer",
			format: FormatNone,
		},
		{
			name:      "tar.gz is not yet supported",
			format:    Format("tar.gz"),
			expectErr: "unsupported archive format",
			expectNil: true,
		},
		{
			name:      "unsupported archive format",
			format:    Format("zip"),
			expectErr: "unsupported archive format",
			expectNil: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			installer, err := NewInstaller(tc.format)
			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				assert.Nil(t, installer)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, installer)
		})
	}
}

func TestRawInstaller_Install(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "otelcol-contrib")
	contents := []byte("raw collector binary")

	installer, err := NewInstaller(FormatNone)
	require.NoError(t, err)

	require.NoError(t, installer.Install(t.Context(), contents, destination))

	written, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, contents, written)
}

func TestRawInstaller_Install_Size(t *testing.T) {
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

			installer := rawInstaller{maxBytes: maxBytes}
			err := installer.Install(t.Context(), contents, destination)
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
