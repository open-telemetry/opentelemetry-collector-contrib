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

func Test_IsMap(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{
			name:     "map",
			value:    make(map[string]any, 0),
			expected: true,
		},
		{
			name:     "ValueTypeMap",
			value:    pcommon.NewValueMap(),
			expected: true,
		},
		{
			name:     "not map",
			value:    "not a map",
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
			exprFunc := isMap[any](&ottl.StandardPMapGetter[any]{
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

func Test_IsMap_Error(t *testing.T) {
	exprFunc := isMap[any](&ottl.StandardPMapGetter[any]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return nil, ottl.TypeError("")
		},
	})
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(ottl.TypeError)
	assert.False(t, ok)
}
