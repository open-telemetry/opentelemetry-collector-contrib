// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package finder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixUNCPath(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		match    string
		expected string
	}{
		{
			name:     "UNC pattern with corrupted match",
			pattern:  `\\server\share\*.log`,
			match:    `\server\share\file.log`,
			expected: `\\server\share\file.log`,
		},
		{
			name:     "UNC pattern with corrupted match (FQDN)",
			pattern:  `\\cloud.local\Logs\*.txt`,
			match:    `\cloud.local\Logs\file.txt`,
			expected: `\\cloud.local\Logs\file.txt`,
		},
		{
			name:     "UNC pattern with uncorrupted match",
			pattern:  `\\server\share\*.log`,
			match:    `\\server\share\file.log`,
			expected: `\\server\share\file.log`,
		},
		{
			name:     "Non-UNC pattern with local path",
			pattern:  `C:\Users\*.txt`,
			match:    `C:\Users\file.txt`,
			expected: `C:\Users\file.txt`,
		},
		{
			name:     "Local pattern with single backslash match",
			pattern:  `C:\Logs\*.log`,
			match:    `C:\Logs\file.log`,
			expected: `C:\Logs\file.log`,
		},
		{
			name:     "Local path on current drive (single backslash) should NOT be converted",
			pattern:  `\Logs\App\*.log`,
			match:    `\Logs\App\file.log`,
			expected: `\Logs\App\file.log`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixUNCPath(tt.pattern, tt.match)
			require.Equal(t, tt.expected, result)
		})
	}
}
