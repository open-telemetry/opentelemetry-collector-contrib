// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_SHA1(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "string",
			value:    "hello world",
			expected: "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := SHA1HashString[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_SHA1Error(t *testing.T) {
	tests := []struct {
		name          string
		value         any
		err           bool
		expectedError string
	}{
		{
			name:          "non-string",
			value:         10,
			expectedError: "expected string but got int",
		},
		{
			name:          "nil",
			value:         nil,
			expectedError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := SHA1HashString[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			assert.NoError(t, err)
			_, err = exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
