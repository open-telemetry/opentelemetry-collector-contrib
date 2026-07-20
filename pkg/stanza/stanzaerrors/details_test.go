// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stanzaerrors

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

type mockEncoder struct {
	zapcore.ObjectEncoder
	added map[string]string
}

func (e *mockEncoder) AddString(key, value string) {
	if e.added == nil {
		e.added = make(map[string]string)
	}
	e.added[key] = value
}

func TestErrorDetailsMarshalLogObject(t *testing.T) {
	details := ErrorDetails{
		"key1": "value1",
		"key2": "value2",
	}

	encoder := &mockEncoder{}
	err := details.MarshalLogObject(encoder)
	require.NoError(t, err)

	require.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, encoder.added)
}

func TestCreateDetails(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected ErrorDetails
	}{
		{
			name:     "empty input",
			input:    []string{},
			expected: ErrorDetails{},
		},
		{
			name:     "even number of elements",
			input:    []string{"key1", "val1", "key2", "val2"},
			expected: ErrorDetails{"key1": "val1", "key2": "val2"},
		},
		{
			name:     "odd number of elements",
			input:    []string{"key1", "val1", "key2"},
			expected: ErrorDetails{"key1": "val1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			details := createDetails(tc.input)
			require.Equal(t, tc.expected, details)
		})
	}
}
