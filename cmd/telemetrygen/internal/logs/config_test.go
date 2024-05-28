// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected error
	}{
		{
			name: "ValidConfig",
			config: &Config{
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
				SpanID:  "5828fa4960140870",
			},
			expected: nil,
		},
		{
			name: "InvalidInvalidTraceIDLenght",
			config: &Config{
				TraceID: "invalid-length",
				SpanID:  "5828fa4960140870",
			},
			expected: errInvalidTraceIDLenght,
		},
		{
			name: "InvalidInvalidSpanIDLenght",
			config: &Config{
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
				SpanID:  "invalid-length",
			},
			expected: errInvalidSpanIDLenght,
		},
		{
			name: "InvalidTraceID",
			config: &Config{
				TraceID: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
				SpanID:  "5828fa4960140870",
			},
			expected: errInvalidTraceID,
		},
		{
			name: "InvalidSpanID",
			config: &Config{
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
				SpanID:  "zzzzzzzzzzzzzzzz",
			},
			expected: errInvalidSpanID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.ErrorIs(t, err, tt.expected)
		})
	}
}
