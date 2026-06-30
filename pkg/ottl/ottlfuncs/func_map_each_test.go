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

func Test_mapEach(t *testing.T) {
	tests := []struct {
		name   string
		source ottl.Getter[any]
		mapper *ottl.LambdaExpression[any]
		want   any
	}{
		{
			name: "map map values",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutInt("a", 1)
					m.PutInt("b", 2)
					return m, nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				v := resolveBinding("v")
				return v.(int64) * 2, nil
			}),
			want: map[string]any{"a": int64(2), "b": int64(4)},
		},
		{
			name: "map map keys and values",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("a", "1")
					return m, nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				k := resolveBinding("k")
				v := resolveBinding("v")
				return k.(string) + v.(string), nil
			}),
			want: map[string]any{"a": "a1"},
		},
		{
			name: "map empty map",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewMap(), nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return "unused", nil
			}),
			want: map[string]any{},
		},
		{
			name: "map slice values",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{int64(1), int64(2), int64(3)}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"i", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				i := resolveBinding("i")
				v := resolveBinding("v")
				return i.(int64) + v.(int64), nil
			}),
			want: []any{int64(1), int64(3), int64(5)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := mapEach(tt.source, tt.mapper)
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

func Test_mapEach_error(t *testing.T) {
	exprFunc := mapEach(
		ottl.StandardGetSetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return 123, nil
			},
		},
		ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
			return nil, nil
		}),
	)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported type")
}

func Test_createMapEachFunction(t *testing.T) {
	fCtx := ottl.FunctionContext{}
	mapper := ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return "ok", nil
	})
	source := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return pcommon.NewMap(), nil
		},
	}

	t.Run("valid args", func(t *testing.T) {
		fn, err := createMapEachFunction[any](fCtx, &MapEachArguments[any]{
			Source: source,
			Mapper: *mapper,
		})
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("invalid args type", func(t *testing.T) {
		_, err := createMapEachFunction[any](fCtx, &struct{}{})
		assert.EqualError(t, err, "MapEachFactory args must be of type *MapEachArguments[K]")
	})
}
