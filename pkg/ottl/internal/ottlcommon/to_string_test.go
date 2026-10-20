// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlerror"
)

func TestToString(t *testing.T) {
	tests := []struct {
		name   string
		val    any
		want   string
		wantOk bool
	}{
		{
			name: "nil",
			val:  nil,
		},
		{
			name:   "string",
			val:    "hello",
			want:   "hello",
			wantOk: true,
		},
		{
			name:   "bytes",
			val:    []byte{0x01, 0xab},
			want:   "01ab",
			wantOk: true,
		},
		{
			name: "pcommon.Map",
			val: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("a", "b")
				return m
			}(),
			want:   `{"a":"b"}`,
			wantOk: true,
		},
		{
			name: "pcommon.Slice",
			val: func() pcommon.Slice {
				s := pcommon.NewSlice()
				_ = s.FromRaw([]any{1, 2})
				return s
			}(),
			want:   "[1,2]",
			wantOk: true,
		},
		{
			name:   "pcommon.Value bytes",
			val:    pcommon.NewValueBytes(),
			want:   "",
			wantOk: true,
		},
		{
			name:   "pcommon.Value int",
			val:    pcommon.NewValueInt(7),
			want:   "7",
			wantOk: true,
		},
		{
			name:   "int64",
			val:    int64(1),
			want:   "1",
			wantOk: true,
		},
		{
			name:   "float64",
			val:    3.14159,
			want:   "3.14159",
			wantOk: true,
		},
		{
			name:   "bool",
			val:    true,
			want:   "true",
			wantOk: true,
		},
		{
			name:   "map[string]any",
			val:    map[string]any{"a": "b"},
			want:   `{"a":"b"}`,
			wantOk: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := ToString(tt.val)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantOk, ok)
		})
	}
}

func TestToString_UnsupportedType(t *testing.T) {
	_, ok, err := ToString(make(chan int))
	require.Error(t, err)
	assert.False(t, ok)
	var typeErr ottlerror.TypeError
	require.ErrorAs(t, err, &typeErr)
	assert.Equal(t, "unsupported type: chan int", err.Error())
}

func TestIsMapOrList(t *testing.T) {
	mapValue := pcommon.NewValueMap()
	sliceValue := pcommon.NewValueSlice()

	tests := []struct {
		name string
		val  any
		want bool
	}{
		{name: "nil", val: nil, want: false},
		{name: "string", val: "a", want: false},
		{name: "int64", val: int64(1), want: false},
		{name: "float64", val: 1.5, want: false},
		{name: "bool", val: true, want: false},
		{name: "bytes", val: []byte{1, 2}, want: false},
		{name: "pcommon.Map", val: pcommon.NewMap(), want: true},
		{name: "pcommon.Slice", val: pcommon.NewSlice(), want: true},
		{name: "pcommon.Value map", val: mapValue, want: true},
		{name: "pcommon.Value slice", val: sliceValue, want: true},
		{name: "pcommon.Value string", val: pcommon.NewValueStr("a"), want: false},
		{name: "pcommon.Value bytes", val: pcommon.NewValueBytes(), want: false},
		{name: "pcommon.Value empty", val: pcommon.NewValueEmpty(), want: false},
		{name: "map[string]any", val: map[string]any{}, want: true},
		{name: "[]any", val: []any{}, want: true},
		{name: "[]string", val: []string{"a"}, want: true},
		{name: "[]int64", val: []int64{1}, want: true},
		{name: "[]float64", val: []float64{1}, want: true},
		{name: "[]bool", val: []bool{true}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsMapOrList(tt.val))
		})
	}
}
