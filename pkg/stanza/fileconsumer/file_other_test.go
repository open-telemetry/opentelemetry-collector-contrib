// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

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
			name:     "Absolute path",
			input:    "/var/log/app.log",
			expected: "/var/log/app.log",
		},
		{
			name:     "Path with dots",
			input:    "/var/log/../log/app.log",
			expected: "/var/log/app.log",
		},
		{
			name:     "Relative path",
			input:    "./test/file.txt",
			expected: "test/file.txt",
		},
		{
			name:     "Path with trailing slash",
			input:    "/var/log/",
			expected: "/var/log",
		},
		{
			name:     "Path with multiple slashes",
			input:    "/var//log///app.log",
			expected: "/var/log/app.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := normalizePath(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
