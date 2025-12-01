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
			name:     "Corrupted UNC path (single backslash)",
			input:    `\networkShare2\BLAM\Logs\stdout_46_2025.log`,
			expected: `\\?\UNC\networkShare2\BLAM\Logs\stdout_46_2025.log`,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
