// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_IsDouble(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "float64",
			value:    float64(2.7),
			expected: true,
		},
		{
			name:     "float64 without decimal",
			value:    float64(55),
			expected: true,
		},
		{
			name:     "an integer",
			value:    int64(333),
			expected: false,
		},
		{
			name:     "ValueTypeDouble",
			value:    pcommon.NewValueDouble(5.5),
			expected: true,
		},
		{
			name:     "not a number",
			value:    "string",
			expected: false,
		},
		{
			name:     "ValueTypeSlice",
			value:    pcommon.NewValueSlice(),
			expected: false,
		},
		{
			name:     "nil",
			value:    nil,
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isDouble[any](&ottl.StandardFloatGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// nolint:errorlint
func Test_IsDouble_Error(t *testing.T) {
	exprFunc := isDouble[any](&ottl.StandardFloatGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, ottl.TypeError("")
		},
	})
	result, err := exprFunc(context.Background(), nil)
	assert.Equal(t, false, result)
	assert.Error(t, err)
	_, ok := err.(ottl.TypeError)
	assert.False(t, ok)
}
