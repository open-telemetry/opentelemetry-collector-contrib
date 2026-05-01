// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Bool(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "string true",
			value:    "true",
			expected: true,
		},
		{
			name:     "string false",
			value:    "false",
			expected: false,
		},
		{
			name:     "string TRUE",
			value:    "TRUE",
			expected: true,
		},
		{
			name:     "string FALSE",
			value:    "FALSE",
			expected: false,
		},
		{
			name:     "string True",
			value:    "True",
			expected: true,
		},
		{
			name:     "string False",
			value:    "False",
			expected: false,
		},
		{
			name:     "string TrUe",
			value:    "TrUe",
			expected: nil,
			err:      true,
		},
		{
			name:     "string FaLsE",
			value:    "FaLsE",
			expected: nil,
			err:      true,
		},
		{
			name:     "string t",
			value:    "t",
			expected: true,
		},
		{
			name:     "string f",
			value:    "f",
			expected: false,
		},
		{
			name:     "string T",
			value:    "T",
			expected: true,
		},
		{
			name:     "string F",
			value:    "F",
			expected: false,
		},
		{
			name:     "string 6",
			value:    "6",
			expected: nil,
			err:      true,
		},
		{
			name:     "string 1",
			value:    "1",
			expected: true,
		},
		{
			name:     "string 0",
			value:    "0",
			expected: false,
		},
		{
			name:     "empty string",
			value:    "",
			expected: nil,
			err:      true,
		},
		{
			name:     "invalid string",
			value:    "test",
			expected: nil,
			err:      true,
		},
		{
			name:     "int64 non-zero",
			value:    int64(5),
			expected: true,
		},
		{
			name:     "int64 zero",
			value:    int64(0),
			expected: false,
		},
		{
			name:     "float64 non-zero",
			value:    float64(2.7),
			expected: true,
		},
		{
			name:     "float64 zero",
			value:    float64(0),
			expected: false,
		},
		{
			name:     "true",
			value:    true,
			expected: true,
		},
		{
			name:     "false",
			value:    false,
			expected: false,
		},
		{
			name:     "nil",
			value:    nil,
			expected: nil,
		},
		{
			name:     "some struct",
			value:    struct{}{},
			expected: nil,
			err:      true,
		},
		{
			name:     "some slice",
			value:    []any{},
			expected: nil,
			err:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := boolFunc[any](&ottl.StandardBoolLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
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
