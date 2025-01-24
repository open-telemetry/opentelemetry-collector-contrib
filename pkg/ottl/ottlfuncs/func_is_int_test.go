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

func Test_IsInt(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "int",
			value:    int64(0),
			expected: true,
		},
		{
			name:     "ValueTypeInt",
			value:    pcommon.NewValueInt(0),
			expected: true,
		},
		{
			name:     "float64",
			value:    float64(2.7),
			expected: false,
		},
		{
			name:     "ValueTypeString",
			value:    pcommon.NewValueStr("a string"),
			expected: false,
		},
		{
			name:     "not Int",
			value:    "string",
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
			exprFunc := isInt[any](&ottl.StandardIntGetter[any]{
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

//nolint:errorlint
func Test_IsInt_Error(t *testing.T) {
	exprFunc := isInt[any](&ottl.StandardIntGetter[any]{
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
