// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
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
			name: "map over map values",
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
			name: "map over map keys and values",
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
			name: "map over empty map",
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
			name: "map over slice values",
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
		{
			name: "map over empty slice",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewSlice(), nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"i", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return "unused", nil
			}),
			want: []any{},
		},
		{
			name: "map over slice returns pcommon.Map",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"a", "b"}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"i", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				m := pcommon.NewMap()
				m.PutInt("index", resolveBinding("i").(int64))
				m.PutStr("value", resolveBinding("v").(string))
				return m, nil
			}),
			want: []any{
				map[string]any{"index": int64(0), "value": "a"},
				map[string]any{"index": int64(1), "value": "b"},
			},
		},
		{
			name: "map over slice returns pcommon.Slice",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{int64(1), int64(2)}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"i", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				inner := pcommon.NewSlice()
				inner.AppendEmpty().SetInt(resolveBinding("i").(int64))
				inner.AppendEmpty().SetInt(resolveBinding("v").(int64))
				return inner, nil
			}),
			want: []any{
				[]any{int64(0), int64(1)},
				[]any{int64(1), int64(2)},
			},
		},
		{
			name: "map over slice returns pcommon.Value",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"x", "y"}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				return pcommon.NewValueStr(resolveBinding("v").(string)), nil
			}),
			want: []any{"x", "y"},
		},
		{
			name: "map over map returns pcommon.Map",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("a", "1")
					return m, nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				nested := pcommon.NewMap()
				nested.PutStr("key", resolveBinding("k").(string))
				nested.PutStr("value", resolveBinding("v").(string))
				return nested, nil
			}),
			want: map[string]any{
				"a": map[string]any{"key": "a", "value": "1"},
			},
		},
		{
			name: "map over map returns pcommon.Slice",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutInt("count", 2)
					return m, nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				s := pcommon.NewSlice()
				s.AppendEmpty().SetStr(resolveBinding("k").(string))
				s.AppendEmpty().SetInt(resolveBinding("v").(int64))
				return s, nil
			}),
			want: map[string]any{
				"count": []any{"count", int64(2)},
			},
		},
		{
			name: "map over map returns pcommon.Value",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutInt("n", 7)
					return m, nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				return pcommon.NewValueInt(resolveBinding("v").(int64)), nil
			}),
			want: map[string]any{"n": int64(7)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := mapEach(tt.source, tt.mapper)
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

func Test_mapEach_errors(t *testing.T) {
	tests := []struct {
		name           string
		source         ottl.Getter[any]
		mapper         *ottl.LambdaExpression[any]
		wantErrorsWith []string
	}{
		{
			name: "unsupported source type",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 123, nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return nil, nil
			}),
			wantErrorsWith: []string{"unsupported type"},
		},
		{
			name: "lambda evaluation error on map item",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key", "value")
					return m, nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return nil, errors.New("eval failed")
			}),
			wantErrorsWith: []string{
				"error while evaluating lambda function on map item (key,",
				"eval failed",
			},
		},
		{
			name: "lambda evaluation error on slice item",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{int64(1)}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"i", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return nil, errors.New("eval failed")
			}),
			wantErrorsWith: []string{
				"error while evaluating lambda function on slice item (0,",
				"eval failed",
			},
		},
		{
			name: "lambda result conversion error on map item",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key", "value")
					return m, nil
				},
			},
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return make(chan int), nil
			}),
			wantErrorsWith: []string{
				"error while converting lambda function result on map item (key, ",
				"Invalid value type",
			},
		},
		{
			name: "lambda result conversion error on slice item",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{int64(1)}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			mapper: ottl.NewTestingLambdaExpression[any]([]string{"i", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return make(chan int), nil
			}),
			wantErrorsWith: []string{
				"error while converting lambda function result on slice item (0, ",
				"Invalid value type",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := mapEach(tt.source, tt.mapper)
			require.NoError(t, err)
			_, err = exprFunc(t.Context(), nil)
			require.Error(t, err)
			for _, wantSubstr := range tt.wantErrorsWith {
				assert.ErrorContains(t, err, wantSubstr)
			}
		})
	}
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

	t.Run("creates function with valid args", func(t *testing.T) {
		fn, err := createMapEachFunction[any](fCtx, &MapEachArguments[any]{
			Source: source,
			Mapper: mapper,
		})
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("error on invalid args type", func(t *testing.T) {
		_, err := createMapEachFunction[any](fCtx, &struct{}{})
		assert.EqualError(t, err, "MapEachFactory args must be of type *MapEachArguments[K]")
	})
}
