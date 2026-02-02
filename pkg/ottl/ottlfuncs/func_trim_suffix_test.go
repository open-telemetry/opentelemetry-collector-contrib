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

func Test_TrimSuffix(t *testing.T) {
	tests := []struct {
		name     string
		target   any
		prefix   ottl.StringGetter[any]
		expected string
	}{
		{
			name:     "has prefix true",
			target:   "hello world",
			prefix:   &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "hello ", nil }},
			expected: "hello world",
		},
		{
			name:     "has prefix false",
			target:   "hello world",
			prefix:   &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " world", nil }},
			expected: "hello",
		},
		{
			name:     "target pcommon.Value",
			target:   pcommon.NewValueStr("hello world"),
			prefix:   &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "world", nil }},
			expected: "hello ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewTrimSuffixFactory[any]()
			exprFunc, err := factory.CreateFunction(
				ottl.FunctionContext{},
				&TrimSuffixArguments[any]{
					Target: ottl.StandardStringGetter[any]{
						Getter: func(context.Context, any) (any, error) {
							return tt.target, nil
						},
					},
					Suffix: tt.prefix,
				})
			require.NoError(t, err)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_TrimSuffix_Error(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return true, nil
		},
	}
	prefix := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "test", nil
		},
	}
	exprFunc := trimSuffix[any](target, prefix)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
}

func Test_TrimSuffix_Error_prefix(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return true, nil
		},
	}
	prefix := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return true, nil
		},
	}
	exprFunc := trimSuffix[any](target, prefix)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
}
