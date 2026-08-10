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
		name      string
		values    []any
		expected  any
		expectErr string
	}{
		{
			name:     "first value non-nil",
			values:   []any{"first", "second"},
			expected: "first",
		},
		{
			name:     "first nil second non-nil",
			values:   []any{nil, "second"},
			expected: "second",
		},
		{
			name:     "first two nil third non-nil",
			values:   []any{nil, nil, "third"},
			expected: "third",
		},
		{
			name:     "all nil",
			values:   []any{nil, nil},
			expected: nil,
		},
		{
			name:     "single value non-nil",
			values:   []any{"only"},
			expected: "only",
		},
		{
			name:     "single value nil",
			values:   []any{nil},
			expected: nil,
		},
		{
			name:     "returns int64 value",
			values:   []any{nil, int64(42)},
			expected: int64(42),
		},
		{
			name:     "returns bool value",
			values:   []any{nil, true},
			expected: true,
		},
		{
			name:     "returns float64 value",
			values:   []any{nil, 3.14},
			expected: 3.14,
		},
		{
			name:      "empty list",
			values:    []any{},
			expectErr: "Coalesce requires at least one value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := tt.values
			exprFunc := coalesce[any](ottl.StandardSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return values, nil
				},
			})
			result, err := exprFunc(t.Context(), nil)
			if tt.expectErr != "" {
				assert.EqualError(t, err, tt.expectErr)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_coalesce_error(t *testing.T) {
	exprFunc := coalesce[any](ottl.StandardSliceGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, errors.New("getter error")
		},
	})
	result, err := exprFunc(t.Context(), nil)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "getter error")
}

// When backed by a literal list, Coalesce evaluates element getters lazily and stops at the
// first non-nil value.
func Test_coalesce_literalList(t *testing.T) {
	t.Run("does not evaluate past first non-nil", func(t *testing.T) {
		values := ottl.NewTestingSliceGetter[any](
			&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return "found", nil
			}},
			&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return nil, errors.New("should not be reached")
			}},
		)
		exprFunc := coalesce[any](values)
		result, err := exprFunc(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "found", result)
	})

	t.Run("skips nil then returns non-nil", func(t *testing.T) {
		values := ottl.NewTestingSliceGetter[any](
			&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return nil, nil
			}},
			&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return "second", nil
			}},
		)
		exprFunc := coalesce[any](values)
		result, err := exprFunc(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "second", result)
	})

	t.Run("returns error from evaluated element", func(t *testing.T) {
		values := ottl.NewTestingSliceGetter[any](
			&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return nil, errors.New("getter error")
			}},
			&ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return "second", nil
			}},
		)
		exprFunc := coalesce[any](values)
		result, err := exprFunc(t.Context(), nil)
		assert.Nil(t, result)
		assert.EqualError(t, err, "getter error")
	})

	t.Run("empty list errors", func(t *testing.T) {
		exprFunc := coalesce[any](ottl.NewTestingSliceGetter[any]())
		result, err := exprFunc(t.Context(), nil)
		assert.Nil(t, result)
		assert.EqualError(t, err, "Coalesce requires at least one value")
	})
}

func Test_createCoalesceFunction(t *testing.T) {
	factory := NewCoalesceFactory[any]()
	fCtx := ottl.FunctionContext{}

	t.Run("valid args", func(t *testing.T) {
		args := &CoalesceArguments[any]{
			Values: ottl.StandardSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"test"}, nil
				},
			},
		}
		fn, err := factory.CreateFunction(fCtx, args)
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("wrong args type", func(t *testing.T) {
		args := &ConcatArguments[any]{}
		_, err := factory.CreateFunction(fCtx, args)
		assert.EqualError(t, err, "CoalesceFactory args must be of type *CoalesceArguments[K]")
	})
}
