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

func Test_reduce(t *testing.T) {
	tests := []struct {
		name        string
		source      ottl.Getter[any]
		seed        any
		accumulator *ottl.LambdaExpression[any]
		want        any
	}{
		{
			name: "reduce map sum values",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutInt("a", 1)
					m.PutInt("b", 2)
					return m, nil
				},
			},
			seed: int64(0),
			accumulator: ottl.NewTestingLambdaExpression[any]([]string{"acc", "_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				acc := resolveBinding("acc")
				v := resolveBinding("v")
				return acc.(int64) + v.(int64), nil
			}),
			want: int64(3),
		},
		{
			name: "reduce map builds key value string",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("env", "prod")
					return m, nil
				},
			},
			seed: "",
			accumulator: ottl.NewTestingLambdaExpression[any]([]string{"acc", "k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				k := resolveBinding("k")
				v := resolveBinding("v")
				return k.(string) + "=" + v.(string), nil
			}),
			want: "env=prod",
		},
		{
			name: "reduce empty map returns seed",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewMap(), nil
				},
			},
			seed: "seed",
			accumulator: ottl.NewTestingLambdaExpression[any]([]string{"acc", "k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				acc := resolveBinding("acc")
				return acc, nil
			}),
			want: "seed",
		},
		{
			name: "reduce slice sum",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{int64(1), int64(2), int64(3)}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			seed: int64(0),
			accumulator: ottl.NewTestingLambdaExpression[any]([]string{"acc", "_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				acc := resolveBinding("acc")
				v := resolveBinding("v")
				return acc.(int64) + v.(int64), nil
			}),
			want: int64(6),
		},
		{
			name: "reduce empty slice returns seed",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			seed: int64(42),
			accumulator: ottl.NewTestingLambdaExpression[any]([]string{"acc", "_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				acc := resolveBinding("acc")
				return acc, nil
			}),
			want: int64(42),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed := tt.seed
			exprFunc := reduce(tt.source, ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return seed, nil
				},
			}, tt.accumulator)
			got, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_reduce_error(t *testing.T) {
	seed := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return int64(0), nil
		},
	}
	accumulator := ottl.NewTestingLambdaExpression[any]([]string{"acc", "_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		acc := resolveBinding("acc")
		return acc, nil
	})

	t.Run("unsupported source type", func(t *testing.T) {
		exprFunc := reduce(
			ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "not a collection", nil
				},
			},
			seed,
			accumulator,
		)
		_, err := exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "unsupported type")
	})

	t.Run("seed getter error", func(t *testing.T) {
		seedErr := errors.New("seed failed")
		exprFunc := reduce(
			ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewSlice()
					require.NoError(t, s.FromRaw([]any{int64(1)}))
					return s, nil
				},
			},
			ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, seedErr
				},
			},
			accumulator,
		)
		_, err := exprFunc(t.Context(), nil)
		require.ErrorIs(t, err, seedErr)
	})
}

func Test_reduce_accumulator_error(t *testing.T) {
	source := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			m := pcommon.NewMap()
			m.PutInt("a", 1)
			return m, nil
		},
	}
	seed := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return int64(0), nil
		},
	}
	accumulator := ottl.NewTestingLambdaExpression[any]([]string{"acc", "_", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return nil, errors.New("accumulator failed")
	})

	exprFunc := reduce(source, seed, accumulator)
	_, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "accumulator failed")
}

func Test_createReduceFunction(t *testing.T) {
	fCtx := ottl.FunctionContext{}
	accumulator := ottl.NewTestingLambdaExpression[any]([]string{"acc", "_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		acc := resolveBinding("acc")
		return acc, nil
	})
	source := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return pcommon.NewMap(), nil
		},
	}
	seed := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return int64(0), nil
		},
	}

	t.Run("valid args", func(t *testing.T) {
		fn, err := createReduceFunction[any](fCtx, &ReduceArguments[any]{
			Source:      source,
			Seed:        seed,
			Accumulator: *accumulator,
		})
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("invalid args type", func(t *testing.T) {
		_, err := createReduceFunction[any](fCtx, &struct{}{})
		assert.EqualError(t, err, "ReduceFactory args must be of type *ReduceArguments[K]")
	})
}
