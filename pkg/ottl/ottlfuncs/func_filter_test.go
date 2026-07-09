// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_filter(t *testing.T) {
	tests := []struct {
		name      string
		source    ottl.Getter[any]
		predicate *ottl.LambdaExpression[any]
		want      any
	}{
		{
			name: "filter map by key",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("keep.me", "yes")
					m.PutStr("drop.me", "no")
					return m, nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				k := resolveBinding("k")
				return strings.HasPrefix(k.(string), "keep"), nil
			}),
			want: map[string]any{"keep.me": "yes"},
		},
		{
			name: "filter map by value",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("a", "keep")
					m.PutStr("b", "drop")
					return m, nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				v := resolveBinding("v")
				return v.(string) == "keep", nil
			}),
			want: map[string]any{"a": "keep"},
		},
		{
			name: "filter empty map",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewMap(), nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return true, nil
			}),
			want: map[string]any{},
		},
		{
			name: "filter slice by value",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"keep", "drop", "keep"}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				v := resolveBinding("v")
				return v.(string) == "keep", nil
			}),
			want: []any{"keep", "keep"},
		},
		{
			name: "filter slice by index",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"a", "b", "c"}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"i", "_"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				i := resolveBinding("i")
				return i.(int64)%2 == 0, nil
			}),
			want: []any{"a", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := filter(tt.source, tt.predicate)
			require.NoError(t, err)
			got, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)

			switch want := tt.want.(type) {
			case map[string]any:
				gotMap := got.(pcommon.Map)
				assert.Equal(t, want, gotMap.AsRaw())
			case []any:
				gotSlice := got.(pcommon.Slice)
				assert.Equal(t, want, gotSlice.AsRaw())
			}
		})
	}
}

func Test_filter_error(t *testing.T) {
	t.Run("unsupported source type", func(t *testing.T) {
		exprFunc, err := filter(
			ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "not a collection", nil
				},
			},
			ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return true, nil
			}),
		)
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "unsupported type")
	})

	t.Run("non-boolean predicate on map item", func(t *testing.T) {
		source := ottl.StandardGetSetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				m := pcommon.NewMap()
				m.PutInt("a", 1)
				return m, nil
			},
		}
		predicate := ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
			return 123, nil
		})

		exprFunc, err := filter(source, predicate)
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error while evaluating lambda function on map item (a,")
		assert.ErrorContains(t, err, "lambda expression must return a value of type bool")
	})

	t.Run("non-boolean predicate on slice item", func(t *testing.T) {
		s := pcommon.NewSlice()
		require.NoError(t, s.FromRaw([]any{int64(1)}))
		source := ottl.StandardGetSetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return s, nil
			},
		}
		predicate := ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
			return 123, nil
		})

		exprFunc, err := filter(source, predicate)
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error while evaluating lambda function on slice item (0,")
		assert.ErrorContains(t, err, "lambda expression must return a value of type bool")
	})
}

func Test_createFilterFunction(t *testing.T) {
	fCtx := ottl.FunctionContext{}
	predicate := ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return true, nil
	})
	source := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return pcommon.NewMap(), nil
		},
	}

	t.Run("valid args", func(t *testing.T) {
		fn, err := createFilterFunction[any](fCtx, &FilterArguments[any]{
			Source:    source,
			Predicate: predicate,
		})
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("invalid args type", func(t *testing.T) {
		_, err := createFilterFunction[any](fCtx, &struct{}{})
		assert.EqualError(t, err, "FilterFactory args must be of type *FilterArguments[K]")
	})
}
