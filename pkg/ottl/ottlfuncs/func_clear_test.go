// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_clear(t *testing.T) {
	tests := []struct {
		name          string
		initialValue  any
		expectedValue any
	}{
		{
			name:          "string",
			initialValue:  "test string",
			expectedValue: "",
		},
		{
			name:          "int64",
			initialValue:  int64(42),
			expectedValue: int64(0),
		},
		{
			name:          "float64",
			initialValue:  float64(3.14),
			expectedValue: float64(0),
		},
		{
			name:          "bool",
			initialValue:  true,
			expectedValue: false,
		},
		{
			name:          "slice",
			initialValue:  []any{"a", "b"},
			expectedValue: []any(nil),
		},
		{
			name:          "map",
			initialValue:  map[string]any{"key": "value"},
			expectedValue: map[string]any(nil),
		},
		{
			name:          "pcommon.Value",
			initialValue:  pcommon.NewValueStr("test"),
			expectedValue: pcommon.NewValueEmpty(),
		},
		{
			name:          "pcommon.Map",
			initialValue:  pcommon.NewMap(),
			expectedValue: pcommon.NewMap(),
		},
		{
			name:          "pcommon.Slice",
			initialValue:  pcommon.NewSlice(),
			expectedValue: pcommon.NewSlice(),
		},
		{
			name:          "[]byte",
			initialValue:  []byte{1, 2, 3},
			expectedValue: []byte(nil),
		},
		{
			name:          "pcommon.SpanID",
			initialValue:  pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
			expectedValue: pcommon.SpanID([8]byte{}),
		},
		{
			name:          "pcommon.TraceID",
			initialValue:  pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
			expectedValue: pcommon.TraceID([16]byte{}),
		},
		{
			name:          "time.Time",
			initialValue:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedValue: time.Time{},
		},
		{
			name:          "time.Duration",
			initialValue:  10 * time.Second,
			expectedValue: time.Duration(0),
		},
		{
			name:          "nil",
			initialValue:  nil,
			expectedValue: nil,
		},
		{
			name:          "pointer",
			initialValue:  new(string),
			expectedValue: (*string)(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedValue any
			setterCalled := false

			target := &ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return tt.initialValue, nil
				},
				Setter: func(_ context.Context, _, val any) error {
					setterCalled = true
					capturedValue = val
					return nil
				},
			}

			exprFunc := clearFunc[any](target)
			result, err := exprFunc(t.Context(), nil)

			require.NoError(t, err)
			assert.Nil(t, result)
			assert.True(t, setterCalled)
			assert.Equal(t, tt.expectedValue, capturedValue, "clear should pass the correct zero-value to the setter")
		})
	}
}

func Test_clear_error_getter(t *testing.T) {
	expectedErr := errors.New("getter error")
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, expectedErr
		},
	}

	exprFunc := clearFunc[any](target)

	result, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error getting target value to infer zero value in clear")
	assert.Nil(t, result)
}

func Test_clear_error_setter(t *testing.T) {
	expectedErr := errors.New("setter error")
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "initial value", nil
		},
		Setter: func(_ context.Context, _, _ any) error {
			return expectedErr
		},
	}

	exprFunc := clearFunc[any](target)

	result, err := exprFunc(t.Context(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, result)
}
