// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_HasPrefix(t *testing.T) {
	tests := []struct {
		name     string
		target   any
		prefix   string
		expected bool
	}{
		{
			name:     "has prefix true",
			target:   "hello world",
			prefix:   "hello ",
			expected: true,
		},
		{
			name:     "has prefix false",
			target:   "hello world",
			prefix:   " world",
			expected: false,
		},
		{
			name:     "target pcommon.Value",
			target:   pcommon.NewValueStr("hello world"),
			prefix:   `hello`,
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewHasPrefixFactory[any]()
			exprFunc, err := factory.CreateFunction(
				ottl.FunctionContext{},
				&HasPrefixArguments[any]{
					Target: ottl.StandardStringGetter[any]{
						Getter: func(_ context.Context, _ any) (any, error) {
							return tt.target, nil
						},
					},
					Prefix: tt.prefix,
				})
			assert.NoError(t, err)
			result, err := exprFunc(context.Background(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_HasPrefix_Error(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return true, nil
		},
	}
	exprFunc, err := HasPrefix[any](target, "test")
	assert.NoError(t, err)
	_, err = exprFunc(context.Background(), nil)
	require.Error(t, err)
}
