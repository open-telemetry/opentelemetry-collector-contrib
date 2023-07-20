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
	tests := []struct {
		name     string
		value    interface{}
		expected int
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := strLen[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, interface{}) (interface{}, error) {
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
	exprFunc := strLen[any](&ottl.StandardStringGetter[any]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return nil, ottl.TypeError("")
		},
	})
	result, err := exprFunc(context.Background(), nil)
	assert.Equal(t, nil, result)
	assert.Error(t, err)
	_, ok := err.(ottl.TypeError)
	assert.False(t, ok)
}
