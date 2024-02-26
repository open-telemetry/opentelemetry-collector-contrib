// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_IsNaN(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "NaN",
			value:    math.NaN(),
			expected: true,
		},
		{
			name:     "ValueTypeDouble NaN",
			value:    pcommon.NewValueDouble(math.NaN()),
			expected: true,
		},
		{
			name:     "positive infinity",
			value:    math.Inf(1),
			expected: false,
		},
		{
			name:     "negative infinity",
			value:    math.Inf(-1),
			expected: false,
		},
		{
			name:     "float64 zero",
			value:    float64(0),
			expected: false,
		},
		{
			name:     "float64 number",
			value:    float64(2.7),
			expected: false,
		},
		{
			name:     "int",
			value:    int64(0),
			expected: false,
		},
		{
			name:     "string number",
			value:    "0",
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
			exprFunc := isNaN[any](&ottl.StandardFloatGetter[any]{
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
func Test_IsNan_Error(t *testing.T) {
	exprFunc := isNaN[any](&ottl.StandardFloatGetter[any]{
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
