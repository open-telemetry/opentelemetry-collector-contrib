// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_find(t *testing.T) {
	tests := []struct {
		name      string
		source    ottl.Getter[any]
		predicate *ottl.LambdaExpression[any]
		mapper    ottl.Optional[*ottl.LambdaExpression[any]]
		want      any
	}{
		{
			name: "find map value by key",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("target", "found")
					m.PutStr("other", "miss")
					return m, nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				k := resolveBinding("k")
				return k.(string) == "target", nil
			}),
			want: "found",
		},
		{
			name: "find map value not found",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("a", "b")
					return m, nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return false, nil
			}),
			want: nil,
		},
		{
			name: "find slice value",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"a", "b", "c"}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"i", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				i := resolveBinding("i")
				return i.(int64) == 1, nil
			}),
			want: "b",
		},
		{
			name: "find slice value not found",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"a"}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"i", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return false, nil
			}),
			want: nil,
		},
		{
			name: "empty map returns nil",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewMap(), nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return true, nil
			}),
			want: nil,
		},
		{
			name: "empty slice returns nil",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewSlice(), nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"i", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return true, nil
			}),
			want: nil,
		},
		{
			name: "find first matching slice value",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"match1", "match2", "match3"}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				v := resolveBinding("v")
				return v.(string) == "match1" || v.(string) == "match2", nil
			}),
			want: "match1",
		},
		{
			name: "find map value with mapper",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("target", "found")
					m.PutStr("other", "miss")
					return m, nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				k := resolveBinding("k")
				return k.(string) == "target", nil
			}),
			mapper: ottl.NewTestingOptional(ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				k := resolveBinding("k")
				v := resolveBinding("v")
				return k.(string) + ":" + v.(string), nil
			})),
			want: "target:found",
		},
		{
			name: "find slice value with mapper",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"a", "b", "c"}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"i", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				i := resolveBinding("i")
				return i.(int64) == 1, nil
			}),
			mapper: ottl.NewTestingOptional(ottl.NewTestingLambdaExpression[any]([]string{"i", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				i := resolveBinding("i")
				v := resolveBinding("v")
				return fmt.Sprintf("%d:%s", i.(int64), v.(string)), nil
			})),
			want: "1:b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := find(tt.source, tt.predicate, &tt.mapper)
			require.NoError(t, err)
			got, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_find_error(t *testing.T) {
	t.Run("unsupported source type", func(t *testing.T) {
		exprFunc, err := find(
			ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "not a collection", nil
				},
			},
			ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return true, nil
			}),
			&ottl.Optional[*ottl.LambdaExpression[any]]{},
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

		exprFunc, err := find(source, predicate, &ottl.Optional[*ottl.LambdaExpression[any]]{})
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

		exprFunc, err := find(source, predicate, &ottl.Optional[*ottl.LambdaExpression[any]]{})
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error while evaluating lambda function on slice item (0,")
		assert.ErrorContains(t, err, "lambda expression must return a value of type bool")
	})

	t.Run("predicate eval error on map item", func(t *testing.T) {
		source := ottl.StandardGetSetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				m := pcommon.NewMap()
				m.PutStr("a", "b")
				return m, nil
			},
		}

		exprFunc, err := find(
			source,
			ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return nil, errors.New("eval failed")
			}),
			&ottl.Optional[*ottl.LambdaExpression[any]]{},
		)
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error while evaluating lambda function on map item (a,")
		assert.ErrorContains(t, err, "eval failed")
	})

	t.Run("predicate eval error on slice item", func(t *testing.T) {
		s := pcommon.NewSlice()
		require.NoError(t, s.FromRaw([]any{"b"}))
		source := ottl.StandardGetSetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return s, nil
			},
		}

		exprFunc, err := find(
			source,
			ottl.NewTestingLambdaExpression[any]([]string{"i", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return nil, errors.New("eval failed")
			}),
			&ottl.Optional[*ottl.LambdaExpression[any]]{},
		)
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error while evaluating lambda function on slice item (0,")
		assert.ErrorContains(t, err, "eval failed")
	})
}

func Test_find_mapper_error(t *testing.T) {
	source := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			m := pcommon.NewMap()
			m.PutStr("target", "found")
			return m, nil
		},
	}
	predicate := ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		k := resolveBinding("k")
		return k.(string) == "target", nil
	})
	mapper := ottl.NewTestingOptional(ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return nil, errors.New("mapper failed")
	}))

	exprFunc, err := find(source, predicate, &mapper)
	require.NoError(t, err)
	_, err = exprFunc(t.Context(), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error while evaluating mapper lambda function on item (target,")
	assert.ErrorContains(t, err, "mapper failed")
}

func Test_createFindFunction(t *testing.T) {
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
		fn, err := createFindFunction[any](fCtx, &FindArguments[any]{
			Source:    source,
			Predicate: predicate,
		})
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("valid args with mapper", func(t *testing.T) {
		mapper := ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
			return "mapped", nil
		})
		fn, err := createFindFunction[any](fCtx, &FindArguments[any]{
			Source:    source,
			Predicate: predicate,
			Mapper:    ottl.NewTestingOptional(mapper),
		})
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("invalid args type", func(t *testing.T) {
		_, err := createFindFunction[any](fCtx, &struct{}{})
		assert.EqualError(t, err, "FindFactory args must be of type *FindArguments[K]")
	})
}
