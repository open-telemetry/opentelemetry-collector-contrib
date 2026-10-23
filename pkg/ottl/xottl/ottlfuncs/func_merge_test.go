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
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl"
)

func newTestMap(t *testing.T, raw map[string]any) pcommon.Map {
	t.Helper()
	m := pcommon.NewMap()
	require.NoError(t, m.FromRaw(raw))
	return m
}

func mapGetters(maps ...pcommon.Map) []ottl.PMapGetter[any] {
	getters := make([]ottl.PMapGetter[any], 0, len(maps))
	for _, m := range maps {
		getters = append(getters, ottl.StandardPMapGetter[any]{
			Getter: func(context.Context, any) (any, error) { return m, nil },
		})
	}
	return getters
}

func rawGetter(v any) ottl.PMapGetter[any] {
	return ottl.StandardPMapGetter[any]{
		Getter: func(context.Context, any) (any, error) { return v, nil },
	}
}

func resolverLambda(fn func(key string, merged, next any) (any, error)) ottl.Optional[*xottl.LambdaExpression[any]] {
	return ottl.NewTestingOptional(xottl.NewTestingLambdaExpression[any]([]string{"key", "merged", "next"},
		func(_ context.Context, _ any, resolve func(string) any) (any, error) {
			return fn(resolve("key").(string), resolve("merged"), resolve("next"))
		}))
}

func sumInts(_ string, merged, next any) (any, error) {
	return merged.(int64) + next.(int64), nil
}

func Test_merge(t *testing.T) {
	tests := []struct {
		name     string
		maps     []map[string]any
		resolver ottl.Optional[*xottl.LambdaExpression[any]]
		want     map[string]any
	}{
		{
			name: "disjoint keys",
			maps: []map[string]any{{"a": int64(1)}, {"b": int64(2)}},
			want: map[string]any{"a": int64(1), "b": int64(2)},
		},
		{
			name: "conflict without resolver last wins",
			maps: []map[string]any{{"a": int64(1), "b": "x"}, {"a": int64(2)}, {"a": "three"}},
			want: map[string]any{"a": "three", "b": "x"},
		},
		{
			name:     "left fold carries previous resolver result",
			maps:     []map[string]any{{"a": int64(1)}, {"a": int64(2)}, {"a": int64(3)}},
			resolver: resolverLambda(sumInts),
			want:     map[string]any{"a": int64(6)},
		},
		{
			name: "resolver only called on conflicts",
			maps: []map[string]any{{"a": int64(1), "b": int64(10)}, {"a": int64(2), "c": int64(20)}},
			resolver: resolverLambda(func(key string, merged, next any) (any, error) {
				assert.Equal(t, "a", key)
				return sumInts(key, merged, next)
			}),
			want: map[string]any{"a": int64(3), "b": int64(10), "c": int64(20)},
		},
		{
			name:     "resolver first wins",
			maps:     []map[string]any{{"a": "first"}, {"a": "second"}},
			resolver: resolverLambda(func(_ string, merged, _ any) (any, error) { return merged, nil }),
			want:     map[string]any{"a": "first"},
		},
		{
			name:     "resolver returns nil keeps key with empty value",
			maps:     []map[string]any{{"a": int64(1), "b": int64(1)}, {"a": int64(2)}},
			resolver: resolverLambda(func(string, any, any) (any, error) { return nil, nil }),
			want:     map[string]any{"a": nil, "b": int64(1)},
		},
		{
			name:     "resolver returning merged map keeps data",
			maps:     []map[string]any{{"a": map[string]any{"x": "1", "y": int64(2)}}, {"a": map[string]any{"z": true}}},
			resolver: resolverLambda(func(_ string, merged, _ any) (any, error) { return merged, nil }),
			want:     map[string]any{"a": map[string]any{"x": "1", "y": int64(2)}},
		},
		{
			name:     "resolver returning next map",
			maps:     []map[string]any{{"a": map[string]any{"x": "1"}}, {"a": map[string]any{"z": true}}},
			resolver: resolverLambda(func(_ string, _, next any) (any, error) { return next, nil }),
			want:     map[string]any{"a": map[string]any{"z": true}},
		},
		{
			name:     "resolver returning merged slice keeps data",
			maps:     []map[string]any{{"a": []any{"x", int64(2)}}, {"a": []any{"z"}}},
			resolver: resolverLambda(func(_ string, merged, _ any) (any, error) { return merged, nil }),
			want:     map[string]any{"a": []any{"x", int64(2)}},
		},
		{
			name:     "resolver returning merged map across three maps",
			maps:     []map[string]any{{"a": map[string]any{"x": "1"}}, {"a": "b"}, {"a": "c"}},
			resolver: resolverLambda(func(_ string, merged, _ any) (any, error) { return merged, nil }),
			want:     map[string]any{"a": map[string]any{"x": "1"}},
		},
		{
			name: "nested maps merged shallowly",
			maps: []map[string]any{
				{"a": map[string]any{"x": "1", "y": "2"}},
				{"a": map[string]any{"y": "3", "z": "4"}},
			},
			want: map[string]any{"a": map[string]any{"y": "3", "z": "4"}},
		},
		{
			name: "empty slice",
			maps: []map[string]any{},
			want: map[string]any{},
		},
		{
			name:     "empty slice with resolver",
			maps:     []map[string]any{},
			resolver: resolverLambda(sumInts),
			want:     map[string]any{},
		},
		{
			name: "single map",
			maps: []map[string]any{{"a": int64(1), "b": "c"}},
			want: map[string]any{"a": int64(1), "b": "c"},
		},
	}

	// The "resolver returning merged" cases only lose data when proto pooling is enabled,
	// so the tests run with it both disabled and enabled.
	for _, pooling := range []bool{false, true} {
		t.Run(fmt.Sprintf("pooling=%t", pooling), func(t *testing.T) {
			setProtoPoolingForTest(t, pooling)
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					inputs := make([]pcommon.Map, 0, len(tt.maps))
					originals := make([]map[string]any, 0, len(tt.maps))
					for _, raw := range tt.maps {
						m := newTestMap(t, raw)
						inputs = append(inputs, m)
						originals = append(originals, m.AsRaw())
					}
					resolver := tt.resolver
					exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, mapGetters(inputs...)), &resolver)
					require.NoError(t, err)

					got, err := exprFunc(t.Context(), nil)
					require.NoError(t, err)
					gotMap, ok := got.(pcommon.Map)
					require.True(t, ok)
					assert.Equal(t, tt.want, gotMap.AsRaw())

					for i, m := range inputs {
						assert.Equal(t, originals[i], m.AsRaw(), "input map %d was modified", i)
					}
				})
			}
		})
	}
}

// setProtoPoolingForTest sets pdata's pdata.useProtoPooling feature gate for the duration of the test.
// pdata does not export the gate, so it is looked up by ID.
func setProtoPoolingForTest(t *testing.T, enabled bool) {
	t.Helper()
	var gate *featuregate.Gate
	featuregate.GlobalRegistry().VisitAll(func(g *featuregate.Gate) {
		if g.ID() == "pdata.useProtoPooling" {
			gate = g
		}
	})
	require.NotNil(t, gate, "pdata.useProtoPooling feature gate is not registered")
	t.Cleanup(testutil.SetFeatureGateForTest(t, gate, enabled))
}

func Test_merge_resolver_receives_key(t *testing.T) {
	m1 := newTestMap(t, map[string]any{"a": int64(1), "b": int64(1), "c": int64(1)})
	m2 := newTestMap(t, map[string]any{"b": int64(2), "a": int64(2), "d": int64(2)})
	var seenKeys []string
	resolver := resolverLambda(func(key string, merged, next any) (any, error) {
		seenKeys = append(seenKeys, key)
		return sumInts(key, merged, next)
	})
	exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, mapGetters(m1, m2)), &resolver)
	require.NoError(t, err)
	got, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"a", "b"}, seenKeys)
	assert.Equal(t, map[string]any{"a": int64(3), "b": int64(3), "c": int64(1), "d": int64(2)}, got.(pcommon.Map).AsRaw())
}

func Test_merge_key_order(t *testing.T) {
	m1 := pcommon.NewMap()
	m1.PutInt("z", 1)
	m1.PutInt("b", 1)
	m2 := pcommon.NewMap()
	m2.PutInt("c", 2)
	m2.PutInt("z", 2)
	resolver := ottl.Optional[*xottl.LambdaExpression[any]]{}
	exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, mapGetters(m1, m2)), &resolver)
	require.NoError(t, err)
	got, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)

	var keys []string
	for k := range got.(pcommon.Map).All() {
		keys = append(keys, k)
	}
	assert.Equal(t, []string{"z", "b", "c"}, keys)
}

func Test_merge_same_map_twice(t *testing.T) {
	m := newTestMap(t, map[string]any{"a": int64(1), "n": map[string]any{"x": "y"}})
	original := m.AsRaw()

	t.Run("no resolver", func(t *testing.T) {
		resolver := ottl.Optional[*xottl.LambdaExpression[any]]{}
		exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, mapGetters(m, m)), &resolver)
		require.NoError(t, err)
		got, err := exprFunc(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, original, got.(pcommon.Map).AsRaw())
		assert.Equal(t, original, m.AsRaw())
	})

	t.Run("with resolver", func(t *testing.T) {
		resolver := resolverLambda(func(_ string, merged, next any) (any, error) {
			if mm, ok := merged.(pcommon.Map); ok {
				return mm, nil
			}
			return sumInts("", merged, next)
		})
		exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, mapGetters(m, m)), &resolver)
		require.NoError(t, err)
		got, err := exprFunc(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"a": int64(2), "n": map[string]any{"x": "y"}}, got.(pcommon.Map).AsRaw())
		assert.Equal(t, original, m.AsRaw())
	})
}

func Test_merge_result_independent_of_inputs(t *testing.T) {
	m := newTestMap(t, map[string]any{"a": map[string]any{"x": "y"}})
	resolver := ottl.Optional[*xottl.LambdaExpression[any]]{}
	exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, mapGetters(m)), &resolver)
	require.NoError(t, err)
	got, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)

	m.PutStr("new", "value")
	inner, _ := m.Get("a")
	inner.Map().PutStr("extra", "1")
	assert.Equal(t, map[string]any{"a": map[string]any{"x": "y"}}, got.(pcommon.Map).AsRaw())
}

func Test_merge_dynamic_slice(t *testing.T) {
	m1 := newTestMap(t, map[string]any{"a": int64(1)})
	m2 := newTestMap(t, map[string]any{"a": int64(2), "b": int64(3)})

	for _, withResolver := range []bool{false, true} {
		resolver := ottl.Optional[*xottl.LambdaExpression[any]]{}
		want := map[string]any{"a": int64(2), "b": int64(3)}
		if withResolver {
			resolver = resolverLambda(sumInts)
			want["a"] = int64(3)
		}
		exprFunc, err := merge(xottl.NewTestingSliceGetter[any](false, mapGetters(m1, m2)), &resolver)
		require.NoError(t, err)
		got, err := exprFunc(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, want, got.(pcommon.Map).AsRaw())
	}
}

func Test_merge_errors(t *testing.T) {
	good := newTestMap(t, map[string]any{"a": int64(1)})
	conflicting := newTestMap(t, map[string]any{"a": int64(2)})

	t.Run("non-map element", func(t *testing.T) {
		resolver := ottl.Optional[*xottl.LambdaExpression[any]]{}
		getters := []ottl.PMapGetter[any]{mapGetters(good)[0], rawGetter("not a map")}
		exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, getters), &resolver)
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
	})

	t.Run("nil element", func(t *testing.T) {
		resolver := ottl.Optional[*xottl.LambdaExpression[any]]{}
		getters := []ottl.PMapGetter[any]{mapGetters(good)[0], rawGetter(nil)}
		exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, getters), &resolver)
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
	})

	t.Run("getter error", func(t *testing.T) {
		resolver := ottl.Optional[*xottl.LambdaExpression[any]]{}
		getters := []ottl.PMapGetter[any]{ottl.StandardPMapGetter[any]{
			Getter: func(context.Context, any) (any, error) { return nil, errors.New("boom") },
		}}
		exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, getters), &resolver)
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("wrong lambda arity", func(t *testing.T) {
		resolver := ottl.NewTestingOptional(xottl.NewTestingLambdaExpression[any]([]string{"a", "b"},
			func(context.Context, any, func(string) any) (any, error) { return nil, nil }))
		_, err := merge(xottl.NewTestingSliceGetter[any](true, mapGetters(good)), &resolver)
		assert.ErrorContains(t, err, "invalid resolver")
	})

	t.Run("resolver error", func(t *testing.T) {
		resolver := resolverLambda(func(string, any, any) (any, error) { return nil, errors.New("resolver failed") })
		exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, mapGetters(good, conflicting)), &resolver)
		require.NoError(t, err)
		_, err = exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, `error while evaluating conflict resolver on key "a"`)
		assert.ErrorContains(t, err, "resolver failed")
	})

	t.Run("resolver error does not modify inputs", func(t *testing.T) {
		resolver := resolverLambda(func(string, any, any) (any, error) { return nil, errors.New("resolver failed") })
		exprFunc, err := merge(xottl.NewTestingSliceGetter[any](true, mapGetters(good, conflicting)), &resolver)
		require.NoError(t, err)
		_, _ = exprFunc(t.Context(), nil)
		assert.Equal(t, map[string]any{"a": int64(1)}, good.AsRaw())
		assert.Equal(t, map[string]any{"a": int64(2)}, conflicting.AsRaw())
	})
}

func Test_MergeFactory(t *testing.T) {
	maps := xottl.NewTestingSliceGetter[any](true, mapGetters(pcommon.NewMap()))

	t.Run("factory creation", func(t *testing.T) {
		factory := NewMergeFactory[any]()
		assert.Equal(t, "Merge", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewMergeFactory[any]()
		args := factory.CreateDefaultArguments()
		assert.IsType(t, &mergeArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Maps", "Resolver"})
	})

	t.Run("function creation without resolver", func(t *testing.T) {
		factory := NewMergeFactory[any]()
		args, ok := factory.CreateDefaultArguments().(*mergeArguments[any])
		require.True(t, ok)
		args.Maps = *maps

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("function creation with resolver", func(t *testing.T) {
		factory := NewMergeFactory[any]()
		args, ok := factory.CreateDefaultArguments().(*mergeArguments[any])
		require.True(t, ok)
		args.Maps = *maps
		args.Resolver = resolverLambda(sumInts)

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("function creation with invalid resolver arity", func(t *testing.T) {
		factory := NewMergeFactory[any]()
		args, ok := factory.CreateDefaultArguments().(*mergeArguments[any])
		require.True(t, ok)
		args.Maps = *maps
		args.Resolver = ottl.NewTestingOptional(xottl.NewTestingLambdaExpression[any]([]string{"a", "b", "c", "d"},
			func(context.Context, any, func(string) any) (any, error) { return nil, nil }))

		_, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		assert.ErrorContains(t, err, "invalid resolver")
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createMergeFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "MergeFactory args must be of type *mergeArguments[K]")
	})
}
