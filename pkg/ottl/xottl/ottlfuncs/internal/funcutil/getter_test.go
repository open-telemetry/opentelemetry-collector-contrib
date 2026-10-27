// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package funcutil

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestGetSliceOrMapValue(t *testing.T) {
	tests := []struct {
		name     string
		getter   ottl.Getter[any]
		wantType string
		wantLen  int
	}{
		{
			name: "pcommon map",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("a", "b")
					return m, nil
				},
			},
			wantType: "Map",
			wantLen:  1,
		},
		{
			name: "pcommon slice",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewSlice()
					require.NoError(t, s.FromRaw([]any{1, 2}))
					return s, nil
				},
			},
			wantType: "Slice",
			wantLen:  2,
		},
		{
			name: "pcommon value map",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueMap()
					v.Map().PutStr("a", "b")
					return v, nil
				},
			},
			wantType: "Map",
			wantLen:  1,
		},
		{
			name: "pcommon value slice",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueSlice()
					require.NoError(t, v.Slice().FromRaw([]any{1}))
					return v, nil
				},
			},
			wantType: "Slice",
			wantLen:  1,
		},
		{
			name: "raw map",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return map[string]any{"a": "b"}, nil
				},
			},
			wantType: "Map",
			wantLen:  1,
		},
		{
			name: "raw slice",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{1, 2, 3}, nil
				},
			},
			wantType: "Slice",
			wantLen:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSliceOrMapValue(t.Context(), nil, tt.getter)
			require.NoError(t, err)

			switch tt.wantType {
			case "Map":
				m := got.(pcommon.Map)
				assert.Equal(t, tt.wantLen, m.Len())
			case "Slice":
				s := got.(pcommon.Slice)
				assert.Equal(t, tt.wantLen, s.Len())
			}
		})
	}
}

func TestGetSliceOrMapValue_unsupported(t *testing.T) {
	getter := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "not a map or slice", nil
		},
	}
	_, err := GetSliceOrMapValue(t.Context(), nil, getter)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported type")
}

func TestGetSliceOrMapValue_getterError(t *testing.T) {
	getterErr := errors.New("getter failed")
	getter := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, getterErr
		},
	}

	_, err := GetSliceOrMapValue(t.Context(), nil, getter)
	require.ErrorIs(t, err, getterErr)
}

func TestGetSliceOrMapValue_nonMapOrSliceValue(t *testing.T) {
	getter := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return pcommon.NewValueStr("not a map or slice"), nil
		},
	}

	_, err := GetSliceOrMapValue(t.Context(), nil, getter)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported type provided: pcommon.Value")
}

func TestGetSliceOrMapValue_sliceGetterNoTypeError(t *testing.T) {
	getter := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return []any{make(chan int)}, nil
		},
	}

	_, err := GetSliceOrMapValue(t.Context(), nil, getter)
	require.Error(t, err)
	assert.ErrorContains(t, err, "Invalid value type")
	var typeErr ottl.TypeError
	assert.NotErrorAs(t, err, &typeErr)
}

func TestGetSliceOrMapValue_mapGetterNoTypeError(t *testing.T) {
	getter := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return map[string]any{"invalid": make(chan int)}, nil
		},
	}

	_, err := GetSliceOrMapValue(t.Context(), nil, getter)
	require.Error(t, err)
	assert.ErrorContains(t, err, "Invalid value type")
	var typeErr ottl.TypeError
	assert.NotErrorAs(t, err, &typeErr)
}

func TestGetSliceOrMapValue_fallsBackFromSliceTypeErrorToMap(t *testing.T) {
	getter := ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			// not a pcommon.Map, so std slice getter returns TypeError first
			return map[string]any{"a": "b"}, nil
		},
	}

	got, err := GetSliceOrMapValue(t.Context(), nil, getter)
	require.NoError(t, err)
	m := got.(pcommon.Map)
	assert.Equal(t, map[string]any{"a": "b"}, m.AsRaw())
}
