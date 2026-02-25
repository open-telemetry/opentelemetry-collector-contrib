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

func Test_IsNil(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "Nil",
			value:    nil,
			expected: true,
		},
		{
			name:     "not Nil",
			value:    "Not Nil",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isNil[any](&ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

//nolint:errorlint
func Test_IsNil_Error(t *testing.T) {
    exprFunc := isNil[any](&ottl.StandardGetSetter[any]{
        Getter: func(context.Context, any) (any, error) {
            return nil, ottl.TypeError("")
        },
    })
    result, err := exprFunc(t.Context(), nil)
    assert.Equal(t, true, result)
    require.NoError(t, err)
}
