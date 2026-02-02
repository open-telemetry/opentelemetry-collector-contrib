// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_MD5(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "string",
			value:    "hello world",
			expected: "5eb63bbbe01eeed093cb22bb8f5acdc3",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "d41d8cd98f00b204e9800998ecf8427e",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := MD5HashString[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			if tt.err {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_MD5Error(t *testing.T) {
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
			exprFunc, err := MD5HashString[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			require.NoError(t, err)
			_, err = exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
