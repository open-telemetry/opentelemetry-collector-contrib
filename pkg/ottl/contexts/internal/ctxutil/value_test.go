// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

func Test_SetIndexableValue_InvalidValue(t *testing.T) {
	keys := []ottl.Key[any]{
		&pathtest.Key[any]{},
	}
	err := ctxutil.SetIndexableValue[any](t.Context(), nil, pcommon.NewValueStr("str"), nil, keys)
	assert.Error(t, err)
}

func Test_SetValue(t *testing.T) {
	// valueFromRaw builds a pcommon.Value from a raw representation, failing the test on error.
	valueFromRaw := func(t *testing.T, raw any) pcommon.Value {
		v := pcommon.NewValueEmpty()
		require.NoError(t, v.FromRaw(raw))
		return v
	}
	sliceFromRaw := func(t *testing.T, raw []any) pcommon.Slice {
		s := pcommon.NewSlice()
		require.NoError(t, s.FromRaw(raw))
		return s
	}
	mapFromRaw := func(t *testing.T, raw map[string]any) pcommon.Map {
		m := pcommon.NewMap()
		require.NoError(t, m.FromRaw(raw))
		return m
	}

	tests := []struct {
		name            string
		initial         func(t *testing.T) pcommon.Value
		val             func(t *testing.T) any
		want            func(t *testing.T) pcommon.Value
		wantErrContains string
	}{
		{
			name: "nil clears to empty value",
			val:  func(*testing.T) any { return nil },
			want: func(*testing.T) pcommon.Value { return pcommon.NewValueEmpty() },
		},
		{
			name: "string",
			val:  func(*testing.T) any { return "foo" },
			want: func(*testing.T) pcommon.Value { return pcommon.NewValueStr("foo") },
		},
		{
			name: "bool",
			val:  func(*testing.T) any { return true },
			want: func(*testing.T) pcommon.Value { return pcommon.NewValueBool(true) },
		},
		{
			name: "int64",
			val:  func(*testing.T) any { return int64(5) },
			want: func(*testing.T) pcommon.Value { return pcommon.NewValueInt(5) },
		},
		{
			name: "float64",
			val:  func(*testing.T) any { return 1.5 },
			want: func(*testing.T) pcommon.Value { return pcommon.NewValueDouble(1.5) },
		},
		{
			name: "[]byte",
			val:  func(*testing.T) any { return []byte{1, 2, 3} },
			want: func(*testing.T) pcommon.Value {
				v := pcommon.NewValueEmpty()
				v.SetEmptyBytes().FromRaw([]byte{1, 2, 3})
				return v
			},
		},
		{
			name: "[]string",
			val:  func(*testing.T) any { return []string{"a", "b"} },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{"a", "b"}) },
		},
		{
			name: "[]bool",
			val:  func(*testing.T) any { return []bool{true, false} },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{true, false}) },
		},
		{
			name: "[]int64",
			val:  func(*testing.T) any { return []int64{1, 2} },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{int64(1), int64(2)}) },
		},
		{
			name: "[]float64",
			val:  func(*testing.T) any { return []float64{1.5, 2.5} },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{1.5, 2.5}) },
		},
		{
			name: "[][]byte",
			val:  func(*testing.T) any { return [][]byte{{1, 2}, {3}} },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{[]byte{1, 2}, []byte{3}}) },
		},
		{
			name: "[]any",
			val:  func(*testing.T) any { return []any{"x", int64(2)} },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{"x", int64(2)}) },
		},
		{
			name: "[]any with nil element becomes empty",
			val:  func(*testing.T) any { return []any{"x", nil} },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{"x", nil}) },
		},
		{
			name:            "[]any with unsupported element",
			val:             func(*testing.T) any { return []any{struct{}{}} },
			wantErrContains: "unsupported type struct {} for set operation",
		},
		{
			name: "pcommon.Slice into non-slice value",
			val:  func(t *testing.T) any { return sliceFromRaw(t, []any{"a", "b"}) },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{"a", "b"}) },
		},
		{
			name:    "pcommon.Slice into existing slice value",
			initial: func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{int64(999)}) },
			val:     func(t *testing.T) any { return sliceFromRaw(t, []any{"a", "b"}) },
			want:    func(t *testing.T) pcommon.Value { return valueFromRaw(t, []any{"a", "b"}) },
		},
		{
			name: "pcommon.Map into non-map value",
			val:  func(t *testing.T) any { return mapFromRaw(t, map[string]any{"k": "v"}) },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, map[string]any{"k": "v"}) },
		},
		{
			name:    "pcommon.Map into existing map value",
			initial: func(t *testing.T) pcommon.Value { return valueFromRaw(t, map[string]any{"old": "gone"}) },
			val:     func(t *testing.T) any { return mapFromRaw(t, map[string]any{"k": "v"}) },
			want:    func(t *testing.T) pcommon.Value { return valueFromRaw(t, map[string]any{"k": "v"}) },
		},
		{
			name: "map[string]any",
			val:  func(*testing.T) any { return map[string]any{"k": "v"} },
			want: func(t *testing.T) pcommon.Value { return valueFromRaw(t, map[string]any{"k": "v"}) },
		},
		{
			name:            "map[string]any with unsupported value",
			val:             func(*testing.T) any { return map[string]any{"k": struct{}{}} },
			wantErrContains: "",
		},
		{
			name:            "unsupported type",
			val:             func(*testing.T) any { return struct{}{} },
			wantErrContains: "unsupported type struct {} for set operation; current value type is Str",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := pcommon.NewValueStr("original")
			if tt.initial != nil {
				value = tt.initial(t)
			}

			err := ctxutil.SetValue(value, tt.val(t))

			// The "map[string]any with unsupported value" case relies only on FromRaw
			// returning an error, whose message may vary; assert an error without a message.
			if tt.wantErrContains == "" && tt.want == nil {
				require.Error(t, err)
				return
			}
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want(t), value)
		})
	}
}
