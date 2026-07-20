// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchValues(t *testing.T) {
	testCases := []struct {
		name        string
		value       string
		regexStr    string
		expected    map[string]any
		expectError bool
	}{
		{
			name:        "match with named groups",
			value:       "2023-10-12 ERROR test log",
			regexStr:    `^(?P<time>\d{4}-\d{2}-\d{2}) (?P<level>[A-Z]+) (?P<msg>.*)$`,
			expected:    map[string]any{"time": "2023-10-12", "level": "ERROR", "msg": "test log"},
			expectError: false,
		},
		{
			name:        "no match",
			value:       "completely different format",
			regexStr:    `^(?P<time>\d{4}-\d{2}-\d{2}) (?P<level>[A-Z]+) (?P<msg>.*)$`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "unamed subexps ignored",
			value:       "some value",
			regexStr:    `^(some) (?P<named>value)$`,
			expected:    map[string]any{"named": "value"},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			re := regexp.MustCompile(tc.regexStr)
			actual, err := MatchValues(tc.value, re)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}
