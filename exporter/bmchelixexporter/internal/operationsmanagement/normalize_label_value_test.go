// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operationsmanagement

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeLabelValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "simple alphanumeric",
			input:    "value123",
			expected: "value123",
		},
		{
			name:     "value with spaces",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "value with special chars preserved",
			input:    "test~!@#$^&*()-_=[];'?./\\:value",
			expected: "test~!@#$^&*()-_=[];'?./\\:value",
		},
		{
			name:     "value with comma replaced",
			input:    "value1,value2,value3",
			expected: "value1 value2 value3",
		},
		{
			name:     "value with colons preserved",
			input:    "host:port:path",
			expected: "host:port:path",
		},
		{
			name:     "path-like value",
			input:    "/var/log/test.log",
			expected: "/var/log/test.log",
		},
		{
			name:     "complex value with comma",
			input:    "cpu=0,mode=idle",
			expected: "cpu=0 mode=idle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeLabelValue(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
