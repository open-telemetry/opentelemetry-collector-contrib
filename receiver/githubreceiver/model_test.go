// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver

import "testing"

func TestFormatString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "converts underscores to hyphens",
			input:    "hello_world",
			expected: "hello-world",
		},
		{
			name:     "converts to lowercase",
			input:    "HELLO_WORLD",
			expected: "hello-world",
		},
		{
			name:     "handles mixed case and multiple underscores",
			input:    "Hello_Big_WORLD",
			expected: "hello-big-world",
		},
		{
			name:     "handles string with no underscores",
			input:    "HelloWorld",
			expected: "helloworld",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatString(tt.input)
			if got != tt.expected {
				t.Errorf("formatString() = %v, want %v", got, tt.expected)
			}
		})
	}
}
