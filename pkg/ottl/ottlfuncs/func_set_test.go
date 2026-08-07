// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/metadata"
)

func Test_set(t *testing.T) {
	input := pcommon.NewValueStr("original name")

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
			name:   "set nil value",
			setter: target,
			getter: &ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(_ context.Context, _ pcommon.Value) (any, error) {
					return nil, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				if metadata.OttlSetAllowNilFeatureGate.IsEnabled() {
					expectedValue.SetStr("nil was set")
				} else {
					expectedValue.SetStr("original name")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())

			fCtx := ottl.FunctionContext{
				Set: componenttest.NewNopTelemetrySettings(),
			}
			exprFunc := set[pcommon.Value](tt.setter, tt.getter, fCtx)

			result, err := exprFunc(nil, scenarioValue)
			require.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueStr("")
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
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

	fCtx := ottl.FunctionContext{
		Set: componenttest.NewNopTelemetrySettings(),
	}
	exprFunc := set[any](setter, getter, fCtx)

	result, err := exprFunc(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, result)

	if metadata.OttlSetAllowNilFeatureGate.IsEnabled() {
		assert.True(t, setterCalled, "setter should have been called with nil")
	} else {
		assert.False(t, setterCalled, "setter should not have been called")
	}
}

func Test_SetFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewSetFactory[any]()
		assert.Equal(t, "set", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewSetFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &SetArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewSetFactory[any]()
		args := factory.CreateDefaultArguments()
		setArgs, ok := args.(*SetArguments[any])
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
		assert.ErrorContains(t, err, "SetFactory args must be of type *SetArguments[K]")
	})
}
