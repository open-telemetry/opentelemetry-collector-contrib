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

func Test_allMatch(t *testing.T) {
	tests := []struct {
		name      string
		source    ottl.Getter[any]
		predicate *ottl.LambdaExpression[any]
		want      bool
	}{
		{
			name: "all map values match",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutInt("a", 2)
					m.PutInt("b", 4)
					return m, nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				v := resolveBinding("v")
				return v.(int64)%2 == 0, nil
			}),
			want: true,
		},
		{
			name: "not all map values match",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutInt("a", 2)
					m.PutInt("b", 3)
					return m, nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				v := resolveBinding("v")
				return v.(int64)%2 == 0, nil
			}),
			want: false,
		},
		{
			name: "empty map matches",
			source: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewMap(), nil
				},
			},
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return false, nil
			}),
			want: true,
		},
		{
			name: "all slice values match",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{int64(2), int64(4)}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				v := resolveBinding("v")
				return v.(int64)%2 == 0, nil
			}),
			want: true,
		},
		{
			name: "not all slice values match",
			source: func() ottl.Getter[any] {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{int64(2), int64(3)}))
				return ottl.StandardGetSetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return s, nil
					},
				}
			}(),
			predicate: ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				v := resolveBinding("v")
				return v.(int64)%2 == 0, nil
			}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := allMatch(tt.source, tt.predicate)
			got, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_createAllFunction(t *testing.T) {
	fCtx := ottl.FunctionContext{}
	predicated := ottl.NewTestingLambdaExpression[any]([]string{"_", "v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return true, nil
	})
	source := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return pcommon.NewMap(), nil
		},
	}

	t.Run("valid args", func(t *testing.T) {
		fn, err := createAllFunction[any](fCtx, &AllArguments[any]{
			Source:    source,
			Predicate: *predicated,
		})
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("invalid args type", func(t *testing.T) {
		_, err := createAllFunction[any](fCtx, &struct{}{})
		assert.EqualError(t, err, "AllFactory args must be of type *AllArguments[K]")
	})
}
