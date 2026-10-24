// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ErrorMode_UnmarshalText(t *testing.T) {
	tests := []struct {
		input     string
		expected  ErrorMode
		expectErr string
	}{
		{"ignore", IgnoreError, ""},
		{"propagate", PropagateError, ""},
		{"silent", SilentError, ""},
		{"IGNORE", IgnoreError, ""},
		{"bogus", "", "unknown error mode bogus"},
		{"", "", "unknown error mode "},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var m ErrorMode
			err := m.UnmarshalText([]byte(tt.input))
			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, m)
		})
	}
}

func Test_LogicOperation_UnmarshalText(t *testing.T) {
	tests := []struct {
		input     string
		expected  LogicOperation
		expectErr string
	}{
		{"and", And, ""},
		{"or", Or, ""},
		{"AND", And, ""},
		{"bogus", "", "unknown LogicOperation bogus"},
		{"", "", "unknown LogicOperation "},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var l LogicOperation
			err := l.UnmarshalText([]byte(tt.input))
			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, l)
		})
	}
}
