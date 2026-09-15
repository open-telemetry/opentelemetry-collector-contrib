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

func Test_keepMatchingKeys(t *testing.T) {
	in := pcommon.NewMap()
	in.PutStr("foo", "bar")
	in.PutStr("foo1", "bar")
	in.PutInt("foo2", 3)

	tests := []struct {
		name      string
		pattern   string
		want      func() *pcommon.Map
		wantError bool
	}{
		{
			name:    "keep everything that ends with a number",
			pattern: "\\d$",
			want: func() *pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo1", "bar")
				m.PutInt("foo2", 3)
				return &m
			},
		},
		{
			name:    "keep nothing",
			pattern: "bar.*",
			want: func() *pcommon.Map {
				m := pcommon.NewMap()
				// add and remove something to have an empty map instead of nil
				m.PutStr("k", "")
				m.Remove("k")
				return &m
			},
		},
		{
			name:    "keep everything",
			pattern: "foo.*",
			want: func() *pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo", "bar")
				m.PutStr("foo1", "bar")
				m.PutInt("foo2", 3)
				return &m
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			in.CopyTo(scenarioMap)

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

			pattern := &ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					return tt.pattern, nil
				},
			}
			exprFunc, err := keepMatchingKeys(target, pattern)

			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			assert.Equal(t, *tt.want(), scenarioMap)
		})
	}
}

func Test_keepMatchingKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	pattern := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "anything", nil
		},
	}

	exprFunc, err := keepMatchingKeys[any](target, pattern)
	require.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_keepMatchingKeys_invalid_pattern(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	pattern := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "*", nil
		},
	}

	exprFunc, err := keepMatchingKeys[any](target, pattern)
	require.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_keepMatchingKeys_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	pattern := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "anything", nil
		},
	}

	exprFunc, err := keepMatchingKeys[any](target, pattern)
	require.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}

func Test_KeepMatchingKeysFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewKeepMatchingKeysFactory[any]()
		assert.Equal(t, "keep_matching_keys", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewKeepMatchingKeysFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &KeepMatchingKeysArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target", "Pattern"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewKeepMatchingKeysFactory[any]()
		args := factory.CreateDefaultArguments()
		keepMatchingKeysArgs, ok := args.(*KeepMatchingKeysArguments[any])
		require.True(t, ok)
		keepMatchingKeysArgs.Target = &ottl.StandardPMapGetSetter[any]{
			Getter: func(context.Context, any) (pcommon.Map, error) {
				return pcommon.NewMap(), nil
			},
		}
		keepMatchingKeysArgs.Pattern = ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "foo.*", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createKeepMatchingKeysFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "KeepMatchingKeysFactory args must be of type *KeepMatchingKeysArguments[K")
	})
}
