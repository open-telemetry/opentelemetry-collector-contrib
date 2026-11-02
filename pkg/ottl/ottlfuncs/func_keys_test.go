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

func Test_keys(t *testing.T) {
	tests := []struct {
		name     string
		target   map[string]any
		expected []any
	}{
		{
			name: "simple",
			target: map[string]any{
				"name":  "test",
				"value": "test2",
			},
			expected: []any{"name", "value"},
		},
		{
			name:     "empty",
			target:   map[string]any{},
			expected: []any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			err := m.FromRaw(tt.target)
			require.NoError(t, err)
			target := ottl.StandardPMapGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return m, nil
				},
			}
			expected := pcommon.NewSlice()
			err = expected.FromRaw(tt.expected)
			require.NoError(t, err)

			exprFunc := keys[any](target)
			rv, err := exprFunc(nil, nil)
			require.NoError(t, err)
			rvSlice := rv.(pcommon.Slice)
			raw := rvSlice.AsRaw()

			assert.True(t, compareSlices(tt.expected, raw))
		})
	}
}

func Test_KeysFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewKeysFactory[any]()
		assert.Equal(t, "Keys", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewKeysFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &keysArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewKeysFactory[any]()
		args := factory.CreateDefaultArguments()
		keysArgs, ok := args.(*keysArguments[any])
		require.True(t, ok)
		keysArgs.Target = ottl.StandardPMapGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return pcommon.NewMap(), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createKeysFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "KeysFactory args must be of type *keysArguments[K]")
	})
}

func BenchmarkKeys(b *testing.B) {
	m := pcommon.NewMap()
	m.PutStr("name", "test")
	m.PutStr("value", "test2")
	target := ottl.StandardPMapGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return m, nil
		},
	}
	exprFunc := keys[any](target)
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := exprFunc(ctx, nil); err != nil {
			b.Fatal(err)
		}
	}
}
