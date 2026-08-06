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

func Test_TrimPrefix(t *testing.T) {
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
			expected: "world",
		},
		{
			name:     "has prefix false",
			target:   "hello world",
			prefix:   &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " world", nil }},
			expected: "hello world",
		},
		{
			name:     "target pcommon.Value",
			target:   pcommon.NewValueStr("hello world"),
			prefix:   &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "hello", nil }},
			expected: " world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewTrimPrefixFactory[any]()
			exprFunc, err := factory.CreateFunction(
				ottl.FunctionContext{},
				&TrimPrefixArguments[any]{
					Target: ottl.StandardStringGetter[any]{
						Getter: func(context.Context, any) (any, error) {
							return tt.target, nil
						},
					},
					Prefix: tt.prefix,
				})
			require.NoError(t, err)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_TrimPrefix_Error(t *testing.T) {
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
	exprFunc := trimPrefix[any](target, prefix)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
}

func Test_TrimPrefix_Error_prefix(t *testing.T) {
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
	exprFunc := trimPrefix[any](target, prefix)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
}

func Test_TrimPrefixFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewTrimPrefixFactory[any]()
		assert.Equal(t, "TrimPrefix", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewTrimPrefixFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &TrimPrefixArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewTrimPrefixFactory[any]()
		args := factory.CreateDefaultArguments()
		trimPrefixArgs, ok := args.(*TrimPrefixArguments[any])
		require.True(t, ok)
		trimPrefixArgs.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "prefix-hello", nil
			},
		}
		trimPrefixArgs.Prefix = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "prefix-", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createTrimPrefixFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "TrimFactory args must be of type *TrimPrefixArguments[K]")
	})
}
