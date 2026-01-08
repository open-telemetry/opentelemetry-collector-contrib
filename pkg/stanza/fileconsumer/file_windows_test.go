// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer

import (
	"testing"

	"github.com/stretchr/testify/require"
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
