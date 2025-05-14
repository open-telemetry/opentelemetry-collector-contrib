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

func Test_HasSuffix(t *testing.T) {
	tests := []struct {
		name     string
		target   any
		suffix   string
		expected bool
	}{
		{
			name:     "has suffix true",
			target:   "hello world",
			suffix:   " world",
			expected: true,
		},
		{
			name:     "has suffix false",
			target:   "hello world",
			suffix:   "hello ",
			expected: false,
		},
		{
			name:     "target pcommon.Value",
			target:   pcommon.NewValueStr("hello world"),
			suffix:   `world`,
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewHasSuffixFactory[any]()
			exprFunc, err := factory.CreateFunction(
				ottl.FunctionContext{},
				&HasSuffixArguments[any]{
					Target: ottl.StandardStringGetter[any]{
						Getter: func(_ context.Context, _ any) (any, error) {
							return tt.target, nil
						},
					},
					Suffix: tt.suffix,
				})
			assert.NoError(t, err)
			result, err := exprFunc(context.Background(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_HasSuffix_Error(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return true, nil
		},
	}
	exprFunc, err := HasSuffix[any](target, "test")
	assert.NoError(t, err)
	_, err = exprFunc(context.Background(), nil)
	require.Error(t, err)
}
