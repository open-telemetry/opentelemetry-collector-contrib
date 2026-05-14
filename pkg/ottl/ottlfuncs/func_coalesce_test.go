// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_coalesce(t *testing.T) {
	tests := []struct {
		name     string
		values   []ottl.Getter[any]
		expected any
	}{
		{
			name: "first value non-nil",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return "first", nil
				}},
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return "second", nil
				}},
			},
			expected: "first",
		},
		{
			name: "first nil second non-nil",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, nil
				}},
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return "second", nil
				}},
			},
			expected: "second",
		},
		{
			name: "first two nil third non-nil",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, nil
				}},
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, nil
				}},
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return "third", nil
				}},
			},
			expected: "third",
		},
		{
			name: "all nil",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, nil
				}},
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, nil
				}},
			},
			expected: nil,
		},
		{
			name: "single value non-nil",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return "only", nil
				}},
			},
			expected: "only",
		},
		{
			name: "single value nil",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, nil
				}},
			},
			expected: nil,
		},
		{
			name: "returns int64 value",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, nil
				}},
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return int64(42), nil
				}},
			},
			expected: int64(42),
		},
		{
			name: "returns bool value",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, nil
				}},
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return true, nil
				}},
			},
			expected: true,
		},
		{
			name: "returns float64 value",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, nil
				}},
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return 3.14, nil
				}},
			},
			expected: 3.14,
		},
		{
			name: "does not evaluate past first non-nil",
			values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return "found", nil
				}},
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return nil, errors.New("should not be reached")
				}},
			},
			expected: "found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := coalesce[any](tt.values)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_coalesce_error(t *testing.T) {
	values := []ottl.Getter[any]{
		&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
			return nil, errors.New("getter error")
		}},
		&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
			return "value", nil
		}},
	}

	exprFunc := coalesce[any](values)
	result, err := exprFunc(t.Context(), nil)
	assert.Nil(t, result)
	assert.EqualError(t, err, "getter error")
}

func Test_createCoalesceFunction(t *testing.T) {
	factory := NewCoalesceFactory[any]()
	fCtx := ottl.FunctionContext{}

	t.Run("valid args", func(t *testing.T) {
		args := &CoalesceArguments[any]{
			Values: []ottl.Getter[any]{
				&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
					return "test", nil
				}},
			},
		}
		fn, err := factory.CreateFunction(fCtx, args)
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("empty values", func(t *testing.T) {
		args := &CoalesceArguments[any]{
			Values: []ottl.Getter[any]{},
		}
		_, err := factory.CreateFunction(fCtx, args)
		assert.EqualError(t, err, "Coalesce requires at least one argument")
	})

	t.Run("wrong args type", func(t *testing.T) {
		args := &ConcatArguments[any]{}
		_, err := factory.CreateFunction(fCtx, args)
		assert.EqualError(t, err, "CoalesceFactory args must be of type *CoalesceArguments[K]")
	})
}
