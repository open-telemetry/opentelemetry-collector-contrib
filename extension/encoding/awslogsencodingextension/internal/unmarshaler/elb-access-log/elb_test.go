// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_scanField(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError string
	}{
		{
			name:     "Simple input",
			input:    "value 123",
			expected: "value",
		},
		{
			name:     "Quoted string with space delimiter",
			input:    `"GET http://example.com/index.html HTTP/1.1" otherValue`,
			expected: "GET http://example.com/index.html HTTP/1.1",
		},
		{
			name:     "Multi byte character handling",
			input:    `"GET http://example.com/こんにちは HTTP/1.1" otherValue`,
			expected: "GET http://example.com/こんにちは HTTP/1.1",
		},
		{
			name:     "Quoted array - expects unquoted array",
			input:    `"a","b","c"`,
			expected: "a,b,c",
		},
		{
			name:     "Quoted string only",
			input:    `"This is quoted and with spaces"`,
			expected: "This is quoted and with spaces",
		},
		{
			name:      "Invalid input",
			input:     `"no end quotes`,
			wantError: "log line has no end quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := scanField(tt.input)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
			}

			require.Equal(t, tt.expected, got)
		})
	}
}
