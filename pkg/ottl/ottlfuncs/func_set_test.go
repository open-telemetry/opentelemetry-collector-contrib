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

func Test_set(t *testing.T) {
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(_ context.Context, tCtx pcommon.Value, val any) error {
			if val == nil {
				tCtx.SetStr("nil was set")
			} else {
				tCtx.SetStr(val.(string))
			}
			return nil
		},
	}

	tests := []struct {
		name   string
		setter ottl.Setter[pcommon.Value]
		getter ottl.Getter[pcommon.Value]
		want   func(pcommon.Value)
	}{
		{
			name:   "set name",
			setter: target,
			getter: &ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(_ context.Context, _ pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("new name")
			},
		},
		{
			name:   "set nil",
			setter: target,
			getter: &ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(_ context.Context, _ pcommon.Value) (any, error) {
					return nil, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("nil was set")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := set(tt.setter, tt.getter)
			input := pcommon.NewValueStr("original name")

			result, err := exprFunc(t.Context(), input)
			require.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueStr("original name")
			tt.want(expected)

			assert.Equal(t, expected, input)
		})
	}
}

func Test_set_get_nil(t *testing.T) {
	setterCalled := false
	setter := &ottl.StandardGetSetter[any]{
		Setter: func(_ context.Context, _, _ any) error {
			setterCalled = true
			return nil
		},
	}

	getter := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, nil
		},
	}

	exprFunc := set[any](setter, getter)

	result, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)
	assert.Nil(t, result)

	assert.True(t, setterCalled, "setter should have been called with nil")
}

func Test_SetFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewSetFactory[any]()
		assert.Equal(t, "set", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewSetFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &setArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target", "Value"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewSetFactory[any]()
		args := factory.CreateDefaultArguments()
		setArgs, ok := args.(*setArguments[any])
		require.True(t, ok)
		setArgs.Target = &ottl.StandardGetSetter[any]{
			Setter: func(context.Context, any, any) error {
				return nil
			},
		}
		setArgs.Value = &ottl.StandardGetSetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "value", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createSetFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "SetFactory args must be of type *setArguments[K]")
	})
}

func BenchmarkSet(b *testing.B) {
	target := &ottl.StandardGetSetter[any]{
		Setter: func(context.Context, any, any) error {
			return nil
		},
	}
	getter := &ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "new value", nil
		},
	}
	exprFunc := set[any](target, getter)
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := exprFunc(ctx, nil); err != nil {
			b.Fatal(err)
		}
	}
}
