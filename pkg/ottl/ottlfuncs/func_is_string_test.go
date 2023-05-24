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

func Test_IsString(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{
			name:     "string",
			value:    "a string",
			expected: true,
		},
		{
			name:     "ValueTypeString",
			value:    pcommon.NewValueStr("a string"),
			expected: true,
		},
		{
			name:     "not String",
			value:    1,
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
			exprFunc := isStringFunc[any](&ottl.StandardGetSetter[any]{
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
