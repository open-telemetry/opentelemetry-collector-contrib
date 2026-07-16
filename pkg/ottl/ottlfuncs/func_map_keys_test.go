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

func Test_mapKeys(t *testing.T) {
	tests := []struct {
		name      string
		source    pcommon.Map
		keyMapper *ottl.LambdaExpression[any]
		want      map[string]any
	}{
		{
			name: "prefix map keys",
			source: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("a", "1")
				m.PutStr("b", "2")
				return m
			}(),
			keyMapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				k := resolveBinding("k")
				return "prefix." + k.(string), nil
			}),
			want: map[string]any{"prefix.a": "1", "prefix.b": "2"},
		},
		{
			name: "keys from value type",
			source: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutInt("count", 10)
				return m
			}(),
			keyMapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
				k := resolveBinding("k")
				v := resolveBinding("v")
				return k.(string) + strings.Repeat("!", int(v.(int64))), nil
			}),
			want: map[string]any{"count!!!!!!!!!!": int64(10)},
		},
		{
			name:   "empty map",
			source: pcommon.NewMap(),
			keyMapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return "unused", nil
			}),
			want: map[string]any{},
		},
		{
			name: "duplicate mapped keys keep later value",
			source: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutInt("number-1", 1)
				m.PutInt("number-2", 2)
				m.PutStr("value-1", "one")
				m.PutInt("number-3", 3)
				m.PutBool("value-2", true)
				return m
			}(),
			keyMapper: ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, getBindings func(string) any) (any, error) {
				key := getBindings("k").(string)
				return strings.Split(key, "-")[0], nil
			}),
			want: map[string]any{
				"number": int64(3),
				"value":  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.source, nil
				},
			}
			exprFunc, err := mapKeys(target, tt.keyMapper)
			require.NoError(t, err)
			got, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.(pcommon.Map).AsRaw())
		})
	}
}

func Test_mapKeys_lambda_type_error(t *testing.T) {
	source := pcommon.NewMap()
	source.PutStr("a", "1")

	target := ottl.StandardPMapGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return source, nil
		},
	}
	keyMapper := ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return 123, nil
	})

	exprFunc, err := mapKeys(target, keyMapper)
	require.NoError(t, err)
	_, err = exprFunc(t.Context(), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error while evaluating lambda function on map item (a,")
	assert.ErrorContains(t, err, "lambda expression must return a value of type string")
}

func Test_createMapKeysFunction(t *testing.T) {
	fCtx := ottl.FunctionContext{}
	keyMapper := ottl.NewTestingLambdaExpression[any]([]string{"k", "_"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		k := resolveBinding("k")
		return k.(string), nil
	})
	source := ottl.StandardPMapGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return pcommon.NewMap(), nil
		},
	}

	t.Run("valid args", func(t *testing.T) {
		fn, err := createMapKeysFunction[any](fCtx, &MapKeysArguments[any]{
			Source:    source,
			KeyMapper: keyMapper,
		})
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("invalid args type", func(t *testing.T) {
		_, err := createMapKeysFunction[any](fCtx, &struct{}{})
		assert.EqualError(t, err, "MapKeysFactory args must be of type *MapKeysArguments[K]")
	})
}
