// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTraceID(t *testing.T) {
	tests := []struct {
		name     string
		traceID  string
		expected error
	}{
		{
			name:     "Valid",
			traceID:  "ae87dadd90e9935a4bc9660628efd569",
			expected: nil,
		},
		{
			name:     "InvalidLength",
			traceID:  "invalid-length",
			expected: errInvalidTraceIDLength,
		},
		{
			name:     "InvalidTraceID",
			traceID:  "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			expected: errInvalidTraceID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTraceID(tt.traceID)
			assert.ErrorIs(t, err, tt.expected)
		})
	}
}

func TestValidateSpanID(t *testing.T) {
	tests := []struct {
		name     string
		spanID   string
		expected error
	}{
		{
			name:     "Valid",
			spanID:   "5828fa4960140870",
			expected: nil,
		},
		{
			name:     "InvalidLength",
			spanID:   "invalid-length",
			expected: errInvalidSpanIDLength,
		},
		{
			name:     "InvalidTraceID",
			spanID:   "zzzzzzzzzzzzzzzz",
			expected: errInvalidSpanID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpanID(tt.spanID)
			assert.ErrorIs(t, err, tt.expected)
		})
	}
}
