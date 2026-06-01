// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive

import (
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
