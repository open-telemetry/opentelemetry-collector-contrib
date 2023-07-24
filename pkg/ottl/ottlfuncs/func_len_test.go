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

func Test_Len(t *testing.T) {
	pcommonSlice := pcommon.NewSlice()
	err := pcommonSlice.FromRaw(make([]any, 5))
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name     string
		value    interface{}
		expected int64
	}{
		{
			name:     "string",
			value:    "a string",
			expected: 8,
		},
		{
			name:     "ValueTypeString",
			value:    pcommon.NewValueStr("a string"),
			expected: 8,
		},
		{
			name:     "string slice",
			value:    make([]string, 5),
			expected: 5,
		},
		{
			name:     "int slice",
			value:    make([]int, 5),
			expected: 5,
		},
		{
			name:     "pcommon slice",
			value:    pcommonSlice,
			expected: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := computeLen[any](&ottl.StandardGetSetter[any]{
				Getter: func(context context.Context, tCtx any) (interface{}, error) {
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
func Test_Len_Error(t *testing.T) {
	exprFunc := computeLen[any](&ottl.StandardGetSetter[any]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return 24, nil
		},
	})
	result, err := exprFunc(context.Background(), nil)
	assert.Equal(t, nil, result)
	assert.Error(t, err)
	_, ok := err.(ottl.TypeError)
	assert.False(t, ok)
}
