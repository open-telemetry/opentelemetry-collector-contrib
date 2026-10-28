// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlcommon

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
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
			name:  "string slice via convertAnySlice",
			input: []string{"a", "b"},
			want:  []any{"a", "b"},
		},
		{
			name:  "bool slice via convertAnySlice",
			input: []bool{true, false},
			want:  []any{true, false},
		},
		{
			name:  "float64 slice via convertAnySlice",
			input: []float64{1.1, 2.2},
			want:  []any{1.1, 2.2},
		},
		{
			name:  "int64 slice via convertAnySlice",
			input: []int64{1, 2},
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

func TestCopyValueTo(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  any
	}{
		{
			name:  "string via FromRaw",
			value: "hello",
			want:  "hello",
		},
		{
			name:  "int64 via FromRaw",
			value: int64(42),
			want:  int64(42),
		},
		{
			name:  "bool via FromRaw",
			value: true,
			want:  true,
		},
		{
			name:  "any slice via FromRaw",
			value: []any{"a", int64(1)},
			want:  []any{"a", int64(1)},
		},
		{
			name:  "string slice via copyValuesTo",
			value: []string{"a", "b"},
			want:  []any{"a", "b"},
		},
		{
			name:  "bool slice via copyValuesTo",
			value: []bool{true, false},
			want:  []any{true, false},
		},
		{
			name:  "float64 slice via copyValuesTo",
			value: []float64{1.1, 2.2},
			want:  []any{1.1, 2.2},
		},
		{
			name:  "int64 slice via copyValuesTo",
			value: []int64{1, 2},
			want:  []any{int64(1), int64(2)},
		},
		{
			name: "pcommon.Map via CopyTo",
			value: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("key", "value")
				return m
			}(),
			want: map[string]any{"key": "value"},
		},
		{
			name: "pcommon.Slice via CopyTo",
			value: func() pcommon.Slice {
				s := pcommon.NewSlice()
				require.NoError(t, s.FromRaw([]any{"a", int64(1)}))
				return s
			}(),
			want: []any{"a", int64(1)},
		},
		{
			name:  "pcommon.Value via CopyTo",
			value: pcommon.NewValueStr("key"),
			want:  "key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := pcommon.NewValueEmpty()
			err := CopyValueTo(tt.value, dest)
			require.NoError(t, err)

			switch want := tt.want.(type) {
			case map[string]any:
				assert.Equal(t, want, dest.Map().AsRaw())
			case []any:
				assert.Equal(t, want, dest.Slice().AsRaw())
			default:
				assert.Equal(t, want, GetValue(dest))
			}
		})
	}
}

func TestCopyValueTo_errors(t *testing.T) {
	dest := pcommon.NewValueEmpty()
	err := CopyValueTo(make(chan int), dest)
	require.Error(t, err)
	assert.ErrorContains(t, err, "Invalid value type")
}

func newTestMap(t *testing.T) pcommon.Map {
	m := pcommon.NewMap()
	require.NoError(t, m.FromRaw(map[string]any{"k": "v", "n": int64(1)}))
	return m
}

func newTestSlice(t *testing.T) pcommon.Slice {
	s := pcommon.NewSlice()
	require.NoError(t, s.FromRaw([]any{"a", int64(2)}))
	return s
}

func TestCopyValueTo_containers(t *testing.T) {
	tests := []struct {
		name  string
		value func(t *testing.T) any
		want  any
	}{
		{
			name: "raw any slice",
			value: func(*testing.T) any {
				return []any{"a", int64(1), 1.5, true, nil, []byte{1}, []any{"x"}, map[string]any{"k": "v"}}
			},
			want: []any{"a", int64(1), 1.5, true, nil, []byte{1}, []any{"x"}, map[string]any{"k": "v"}},
		},
		{
			name:  "raw map",
			value: func(*testing.T) any { return map[string]any{"a": "b", "c": []any{map[string]any{"d": int64(1)}}} },
			want:  map[string]any{"a": "b", "c": []any{map[string]any{"d": int64(1)}}},
		},
		{
			name:  "empty any slice",
			value: func(*testing.T) any { return []any{} },
			want:  []any{},
		},
		{
			name:  "empty map",
			value: func(*testing.T) any { return map[string]any{} },
			want:  map[string]any{},
		},
		{
			name:  "any slice with pcommon.Map",
			value: func(t *testing.T) any { return []any{newTestMap(t)} },
			want:  []any{map[string]any{"k": "v", "n": int64(1)}},
		},
		{
			name:  "any slice with pcommon.Slice",
			value: func(t *testing.T) any { return []any{newTestSlice(t)} },
			want:  []any{[]any{"a", int64(2)}},
		},
		{
			name:  "any slice with pcommon.Value",
			value: func(*testing.T) any { return []any{"s", pcommon.NewValueInt(5)} },
			want:  []any{"s", int64(5)},
		},
		{
			name:  "map with pcommon.Map",
			value: func(t *testing.T) any { return map[string]any{"m": newTestMap(t), "s": "x"} },
			want:  map[string]any{"m": map[string]any{"k": "v", "n": int64(1)}, "s": "x"},
		},
		{
			name:  "map with pcommon.Slice",
			value: func(t *testing.T) any { return map[string]any{"s": newTestSlice(t)} },
			want:  map[string]any{"s": []any{"a", int64(2)}},
		},
		{
			name:  "map with pcommon.Value",
			value: func(*testing.T) any { return map[string]any{"v": pcommon.NewValueStr("x")} },
			want:  map[string]any{"v": "x"},
		},
		{
			name:  "any slice with nested typed slices",
			value: func(*testing.T) any { return []any{[]any{[]string{"a", "b"}}, []int64{1}} },
			want:  []any{[]any{[]any{"a", "b"}}, []any{int64(1)}},
		},
		{
			name: "map with typed slices at depth",
			value: func(*testing.T) any {
				return map[string]any{
					"s": []string{"a"},
					"b": []bool{true},
					"f": []float64{1.5},
					"i": []int64{2},
					"n": []any{map[string]any{"m": []string{"x", "y"}}},
				}
			},
			want: map[string]any{
				"s": []any{"a"},
				"b": []any{true},
				"f": []any{1.5},
				"i": []any{int64(2)},
				"n": []any{map[string]any{"m": []any{"x", "y"}}},
			},
		},
		{
			name: "nested combination",
			value: func(t *testing.T) any {
				return map[string]any{
					"a": []any{map[string]any{"b": []any{newTestMap(t), "raw"}}, newTestSlice(t)},
					"c": map[string]any{"d": pcommon.NewValueBool(true), "e": []any{"x"}},
				}
			},
			want: map[string]any{
				"a": []any{map[string]any{"b": []any{map[string]any{"k": "v", "n": int64(1)}, "raw"}}, []any{"a", int64(2)}},
				"c": map[string]any{"d": true, "e": []any{"x"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := pcommon.NewValueEmpty()
			require.NoError(t, CopyValueTo(tt.value(t), dest))
			switch want := tt.want.(type) {
			case map[string]any:
				assert.Equal(t, want, dest.Map().AsRaw())
			case []any:
				assert.Equal(t, want, dest.Slice().AsRaw())
			}
		})
	}
}

func TestCopyValueTo_rawMatchesFromRaw(t *testing.T) {
	raws := []any{
		[]any{"a", int64(1), []any{"b"}, map[string]any{"c": 1.5}},
		map[string]any{"a": "b", "c": []any{int64(1), map[string]any{"d": nil}}},
	}
	for _, raw := range raws {
		want := pcommon.NewValueEmpty()
		require.NoError(t, want.FromRaw(raw))
		got := pcommon.NewValueEmpty()
		require.NoError(t, CopyValueTo(raw, got))
		assert.Equal(t, want.AsRaw(), got.AsRaw())
	}
}

func TestCopyValueTo_containerErrors(t *testing.T) {
	tests := []struct {
		name  string
		value func(t *testing.T) any
	}{
		{"raw any slice", func(*testing.T) any { return []any{"a", make(chan int)} }},
		{"raw map", func(*testing.T) any { return map[string]any{"a": make(chan int)} }},
		{"nested raw", func(*testing.T) any { return map[string]any{"a": []any{map[string]any{"b": make(chan int)}}} }},
		{"any slice with pdata and invalid", func(t *testing.T) any { return []any{newTestMap(t), make(chan int)} }},
		{"map with pdata and invalid", func(t *testing.T) any { return map[string]any{"m": newTestMap(t), "c": make(chan int)} }},
		{"nested pdata and invalid", func(t *testing.T) any {
			return map[string]any{"a": []any{newTestSlice(t), map[string]any{"b": make(chan int)}}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CopyValueTo(tt.value(t), pcommon.NewValueEmpty())
			require.Error(t, err)
			assert.ErrorContains(t, err, "Invalid value type")
		})
	}
}

func TestCopyValueTo_failingElementLeavesDestEmpty(t *testing.T) {
	dest := pcommon.NewValueEmpty()
	err := CopyValueTo(map[string]any{"m": newTestMap(t), "bad": make(chan int)}, dest)
	require.Error(t, err)
	assert.Equal(t, pcommon.ValueTypeMap, dest.Type())
	assert.Equal(t, 0, dest.Map().Len())
}

func setProtoPoolingForTest(t *testing.T, enabled bool) {
	t.Helper()
	const id = "pdata.useProtoPooling"
	var prev bool
	featuregate.GlobalRegistry().VisitAll(func(g *featuregate.Gate) {
		if g.ID() == id {
			prev = g.IsEnabled()
		}
	})
	require.NoError(t, featuregate.GlobalRegistry().Set(id, enabled))
	t.Cleanup(func() { require.NoError(t, featuregate.GlobalRegistry().Set(id, prev)) })
}

func TestCopyValueTo_protoPooling(t *testing.T) {
	setProtoPoolingForTest(t, true)
	want := map[string]any{
		"m": newTestMap(t).AsRaw(),
		"a": []any{map[string]any{"b": newTestSlice(t).AsRaw()}, "raw"},
	}
	// dest is reused so that each CopyValueTo releases the previous contents to the pools,
	// and the next one may get them back.
	dest := pcommon.NewValueEmpty()
	for range 3 {
		src := map[string]any{
			"m": newTestMap(t),
			"a": []any{map[string]any{"b": newTestSlice(t)}, "raw"},
		}
		require.NoError(t, CopyValueTo(src, dest))
		assert.Equal(t, want, dest.Map().AsRaw())
	}
}

func TestCopyValueTo_noAliasing(t *testing.T) {
	newSrc := func() (map[string]any, pcommon.Map, pcommon.Slice, pcommon.Value) {
		m := pcommon.NewMap()
		m.PutStr("k", "v")
		s := pcommon.NewSlice()
		s.AppendEmpty().SetStr("a")
		v := pcommon.NewValueStr("orig")
		return map[string]any{"l": []any{m, s, v}}, m, s, v
	}

	t.Run("mutating result does not change source", func(t *testing.T) {
		src, m, s, v := newSrc()
		dest := pcommon.NewValueEmpty()
		require.NoError(t, CopyValueTo(src, dest))

		l, ok := dest.Map().Get("l")
		require.True(t, ok)
		l.Slice().At(0).Map().PutStr("k", "changed")
		l.Slice().At(1).Slice().At(0).SetStr("changed")
		l.Slice().At(2).SetStr("changed")

		assert.Equal(t, map[string]any{"k": "v"}, m.AsRaw())
		assert.Equal(t, []any{"a"}, s.AsRaw())
		assert.Equal(t, "orig", v.Str())
	})

	t.Run("mutating source does not change result", func(t *testing.T) {
		src, m, s, v := newSrc()
		dest := pcommon.NewValueEmpty()
		require.NoError(t, CopyValueTo(src, dest))

		m.PutStr("k", "changed")
		s.At(0).SetStr("changed")
		v.SetStr("changed")

		assert.Equal(t, map[string]any{"l": []any{map[string]any{"k": "v"}, []any{"a"}, "orig"}}, dest.Map().AsRaw())
	})
}

// benchmarkMap returns a map with n entries. If withPdata is true, one of them is a pcommon.Value.
func benchmarkMap(n int, withPdata bool) map[string]any {
	m := make(map[string]any, n)
	for i := range n {
		m["key"+strconv.Itoa(i)] = "value"
	}
	if withPdata && n > 0 {
		m["key0"] = pcommon.NewValueStr("value")
	}
	return m
}

// benchmarkNestedMap returns a map nested depth levels deep. Each level has 3 entries plus the
// next level, and the innermost level contains a pcommon.Value.
func benchmarkNestedMap(depth int) map[string]any {
	m := benchmarkMap(3, true)
	for range depth - 1 {
		parent := benchmarkMap(3, false)
		parent["child"] = m
		m = parent
	}
	return m
}

func BenchmarkCopyValueTo(b *testing.B) {
	for _, bc := range []struct {
		name string
		src  map[string]any
	}{
		{"pdata_map_1", benchmarkMap(1, true)},
		{"pdata_map_3", benchmarkMap(3, true)},
		{"pdata_map_10", benchmarkMap(10, true)},
		{"pdata_map_1000", benchmarkMap(1000, true)},
		{"pdata_nested_2", benchmarkNestedMap(2)},
		{"pdata_nested_4", benchmarkNestedMap(4)},
		{"pdata_nested_8", benchmarkNestedMap(8)},
	} {
		b.Run(bc.name, func(b *testing.B) {
			dest := pcommon.NewValueEmpty()
			b.ReportAllocs()
			for b.Loop() {
				if err := CopyValueTo(bc.src, dest); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
