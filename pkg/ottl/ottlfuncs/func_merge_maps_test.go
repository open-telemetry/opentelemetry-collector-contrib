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

func Test_MergeMaps(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("attr1", "value1")

	tests := []struct {
		name     string
		source   ottl.PMapGetter[pcommon.Map]
		strategy string
		want     func(pcommon.Map)
	}{
		{
			name: "Upsert no conflicting keys",
			source: ottl.StandardPMapGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: UPSERT,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "Upsert conflicting key",
			source: ottl.StandardPMapGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("attr1", "value3")
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: UPSERT,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value3")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "Insert no conflicting keys",
			source: ottl.StandardPMapGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: INSERT,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "Insert conflicting key",
			source: ottl.StandardPMapGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("attr1", "value3")
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: INSERT,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "Update no conflicting keys",
			source: ottl.StandardPMapGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			strategy: UPDATE,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
			},
		},
		{
			name: "Update conflicting key",
			source: ottl.StandardPMapGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("attr1", "value3")
					return m, nil
				},
			},
			strategy: UPDATE,
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value3")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			setterWasCalled := false
			target := &ottl.StandardPMapGetSetter[pcommon.Map]{
				Getter: func(_ context.Context, tCtx pcommon.Map) (pcommon.Map, error) {
					return tCtx, nil
				},
				Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
					setterWasCalled = true
					if v, ok := m.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, err := mergeMaps[pcommon.Map](target, tt.source, tt.strategy)
			require.NoError(t, err)

			result, err := exprFunc(t.Context(), scenarioMap)
			require.NoError(t, err)
			assert.Nil(t, result)
			assert.True(t, setterWasCalled)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_MergeMaps_bad_target(t *testing.T) {
	input := &ottl.StandardPMapGetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	exprFunc, err := mergeMaps[any](target, input, "insert")
	require.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_MergeMaps_bad_input(t *testing.T) {
	input := &ottl.StandardPMapGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return 1, nil
		},
	}
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	exprFunc, err := mergeMaps[any](target, input, "insert")
	require.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_MergeMapsFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewMergeMapsFactory[any]()
		assert.Equal(t, "merge_maps", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewMergeMapsFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &MergeMapsArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target", "Source", "Strategy"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewMergeMapsFactory[any]()
		args := factory.CreateDefaultArguments()
		mergeMapsArgs, ok := args.(*MergeMapsArguments[any])
		require.True(t, ok)
		mergeMapsArgs.Target = &ottl.StandardPMapGetSetter[any]{
			Getter: func(context.Context, any) (pcommon.Map, error) {
				return pcommon.NewMap(), nil
			},
		}
		mergeMapsArgs.Source = ottl.StandardPMapGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return pcommon.NewMap(), nil
			},
		}
		mergeMapsArgs.Strategy = INSERT

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createMergeMapsFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "MergeMapsFactory args must be of type *MergeMapsArguments[K]")
	})
}
