// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operationsmanagement

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeEntityValue(t *testing.T) {
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
			name:     "value with allowed special chars",
			input:    "test~!@#$^&*()-_=[];'?./\\value",
			expected: "test~!@#$^&*()-_=[];'?./\\value",
		},
		{
			name:     "value with colons replaced",
			input:    "host:port:path",
			expected: "host_port_path",
		},
		{
			name:     "value with disallowed chars",
			input:    "test<>|value",
			expected: "test___value",
		},
		{
			name:     "value with percent",
			input:    "50%",
			expected: "50_",
		},
		{
			name:     "value with curly braces",
			input:    "{test}",
			expected: "_test_",
		},
		{
			name:     "value with mixed chars",
			input:    "device=eth0",
			expected: "device=eth0",
		},
		{
			name:     "path-like value",
			input:    "/var/log/test.log",
			expected: "/var/log/test.log",
		},
		{
			name:     "entityTypeId with colons",
			input:    "container:docker:nginx",
			expected: "container_docker_nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeEntityValue(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeEntityValueRune(t *testing.T) {
	// Valid runes should be returned as-is
	require.Equal(t, 'a', sanitizeEntityValueRune('a'))
	require.Equal(t, 'Z', sanitizeEntityValueRune('Z'))
	require.Equal(t, '5', sanitizeEntityValueRune('5'))
	require.Equal(t, ' ', sanitizeEntityValueRune(' '))
	require.Equal(t, '\t', sanitizeEntityValueRune('\t'))

	// Allowed special chars should be returned as-is (excluding ":")
	allowedSpecials := []rune{'~', '!', '@', '#', '$', '^', '&', '*', '(', ')', '-', '_', '=', '[', ']', ';', '\'', '?', '.', '/', '\\'}
	for _, r := range allowedSpecials {
		require.Equal(t, r, sanitizeEntityValueRune(r), "Expected %c to be allowed", r)
	}

	// Colon should be replaced with underscore
	require.Equal(t, '_', sanitizeEntityValueRune(':'))

	// Invalid runes should be replaced with '_'
	require.Equal(t, '_', sanitizeEntityValueRune('<'))
	require.Equal(t, '_', sanitizeEntityValueRune('>'))
	require.Equal(t, '_', sanitizeEntityValueRune('|'))
	require.Equal(t, '_', sanitizeEntityValueRune('%'))
	require.Equal(t, '_', sanitizeEntityValueRune('{'))
	require.Equal(t, '_', sanitizeEntityValueRune('}'))
}
