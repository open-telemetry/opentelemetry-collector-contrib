// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestGetValue(t *testing.T) {
	tests := []struct {
		name string
		val  pcommon.Value
		want any
	}{
		{
			name: "string",
			val:  pcommon.NewValueStr("hello"),
			want: "hello",
		},
		{
			name: "bool",
			val:  pcommon.NewValueBool(true),
			want: true,
		},
		{
			name: "int",
			val:  pcommon.NewValueInt(42),
			want: int64(42),
		},
		{
			name: "double",
			val:  pcommon.NewValueDouble(3.14),
			want: 3.14,
		},
		{
			name: "bytes",
			val: func() pcommon.Value {
				v := pcommon.NewValueBytes()
				v.Bytes().FromRaw([]byte{0x01, 0x02})
				return v
			}(),
			want: []byte{0x01, 0x02},
		},
		{
			name: "empty",
			val:  pcommon.NewValueEmpty(),
			want: nil,
		},
		{
			name: "map",
			val: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("key", "value")
				return v
			}(),
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("key", "value")
				return m
			}(),
		},
		{
			name: "slice",
			val: func() pcommon.Value {
				v := pcommon.NewValueSlice()
				require.NoError(t, v.Slice().FromRaw([]any{"a"}))
				return v
			}(),
			want: func() pcommon.Slice {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"a"}))
				return s
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetValue(tt.val)

			switch want := tt.want.(type) {
			case pcommon.Map:
				assert.Equal(t, want.AsRaw(), got.(pcommon.Map).AsRaw())
			case pcommon.Slice:
				assert.Equal(t, want.AsRaw(), got.(pcommon.Slice).AsRaw())
			default:
				assert.Equal(t, want, got)
			}
		})
	}
}

func TestNormalizeValue(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  any
	}{
		{name: "string passthrough", input: "hello", want: "hello"},
		{name: "int64 passthrough", input: int64(7), want: int64(7)},
		{name: "float64 passthrough", input: float64(1.5), want: float64(1.5)},
		{name: "bool passthrough", input: true, want: true},
		{name: "int to int64", input: int(9), want: int64(9)},
		{name: "uint to int64", input: uint(10), want: int64(10)},
		{name: "int32 to int64", input: int32(11), want: int64(11)},
		{name: "uint32 to int64", input: uint32(12), want: int64(12)},
		{name: "float32 to float64", input: float32(1.25), want: float64(float32(1.25))},
		{name: "pcommon value string", input: pcommon.NewValueStr("key"), want: "key"},
		{name: "pcommon value int", input: pcommon.NewValueInt(3), want: int64(3)},
		{name: "pcommon value empty", input: pcommon.NewValueEmpty(), want: nil},
		{
			name: "pcommon map passthrough",
			input: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("a", "b")
				return m
			}(),
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("a", "b")
				return m
			}(),
		},
		{
			name:  "raw map passthrough",
			input: map[string]any{"a": "b"},
			want:  map[string]any{"a": "b"},
		},
		{
			name:  "raw slice passthrough",
			input: []any{int64(1), int64(2)},
			want:  []any{int64(1), int64(2)},
		},
		{
			name:  "unknown type passthrough",
			input: struct{ X int }{X: 1},
			want:  struct{ X int }{X: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeValue(tt.input)

			switch want := tt.want.(type) {
			case pcommon.Map:
				assert.Equal(t, want.AsRaw(), got.(pcommon.Map).AsRaw())
			default:
				assert.Equal(t, want, got)
			}
		})
	}
}
