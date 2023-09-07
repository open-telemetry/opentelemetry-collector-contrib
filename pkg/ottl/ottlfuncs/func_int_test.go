// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Int(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected interface{}
		err      bool
	}{
		{
			name:     "string",
			value:    "50",
			expected: int64(50),
		},
		{
			name:     "empty string",
			value:    "",
			expected: nil,
		},
		{
			name:     "not a number string",
			value:    "test",
			expected: nil,
		},
		{
			name:     "int64",
			value:    int64(333),
			expected: int64(333),
		},
		{
			name:     "float64",
			value:    float64(2.7),
			expected: int64(2),
		},
		{
			name:     "float64 without decimal",
			value:    float64(55),
			expected: int64(55),
		},
		{
			name:     "true",
			value:    true,
			expected: int64(1),
		},
		{
			name:     "false",
			value:    false,
			expected: int64(0),
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := intFunc[interface{}](&ottl.StandardIntLikeGetter[interface{}]{
				Getter: func(context.Context, interface{}) (interface{}, error) {
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
