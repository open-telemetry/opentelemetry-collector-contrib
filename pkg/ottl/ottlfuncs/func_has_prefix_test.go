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
		prefix   ottl.StringGetter[any]
		expected bool
	}{
		{
			name:     "has prefix true",
			target:   "hello world",
			prefix:   &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "hello ", nil }},
			expected: true,
		},
		{
			name:     "has prefix false",
			target:   "hello world",
			prefix:   &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " world", nil }},
			expected: false,
		},
		{
			name:     "target pcommon.Value",
			target:   pcommon.NewValueStr("hello world"),
			prefix:   &ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "hello", nil }},
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
						Getter: func(context.Context, any) (any, error) {
							return tt.target, nil
						},
					},
					Prefix: tt.prefix,
				},
			)
			require.NoError(t, err)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_HasPrefix_Error(t *testing.T) {
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
	exprFunc := HasPrefix[any](target, prefix)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
}

func Test_HasPrefix_Error_prefix(t *testing.T) {
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
	exprFunc := HasPrefix[any](target, prefix)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
}

func Test_HasPrefixFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewHasPrefixFactory[any]()
		assert.Equal(t, "HasPrefix", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewHasPrefixFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &HasPrefixArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target", "Prefix"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewHasPrefixFactory[any]()
		args := factory.CreateDefaultArguments()
		hasPrefixArgs, ok := args.(*HasPrefixArguments[any])
		require.True(t, ok)
		hasPrefixArgs.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "value", nil
			},
		}
		hasPrefixArgs.Prefix = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "val", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createHasPrefixFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "HasPrefixFactory args must be of type *HasPrefixArguments[K]")
	})
}
