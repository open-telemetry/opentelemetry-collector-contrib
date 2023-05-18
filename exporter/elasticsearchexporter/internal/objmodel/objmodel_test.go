// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-structform/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

var dijkstra = time.Date(1930, 5, 11, 16, 33, 11, 123456789, time.UTC)

func TestObjectModel_CreateMap(t *testing.T) {
	tests := map[string]struct {
		build func() Document
		want  Document
	}{
		"from empty map": {
			build: func() Document {
				return DocumentFromAttributes(pcommon.NewMap())
			},
		},
		"from map": {
			build: func() Document {
				m := pcommon.NewMap()
				m.PutInt("i", 42)
				m.PutStr("str", "test")
				return DocumentFromAttributes(m)
			},
			want: Document{[]field{{"i", IntValue(42)}, {"str", StringValue("test")}}},
		},
		"ignores nil values": {
			build: func() Document {
				m := pcommon.NewMap()
				m.PutEmpty("null")
				m.PutStr("str", "test")
				return DocumentFromAttributes(m)
			},
			want: Document{[]field{{"str", StringValue("test")}}},
		},
		"from map with prefix": {
			build: func() Document {
				m := pcommon.NewMap()
				m.PutInt("i", 42)
				m.PutStr("str", "test")
				return DocumentFromAttributesWithPath("prefix", m)
			},
			want: Document{[]field{{"prefix.i", IntValue(42)}, {"prefix.str", StringValue("test")}}},
		},
		"add attributes with key": {
			build: func() (doc Document) {
				m := pcommon.NewMap()
				m.PutInt("i", 42)
				m.PutStr("str", "test")
				doc.AddAttributes("prefix", m)
				return doc
			},
			want: Document{[]field{{"prefix.i", IntValue(42)}, {"prefix.str", StringValue("test")}}},
		},
		"add attribute flattens a map value": {
			build: func() (doc Document) {
				mapVal := pcommon.NewValueMap()
				m := mapVal.Map()
				m.PutInt("i", 42)
				m.PutStr("str", "test")
				doc.AddAttribute("prefix", mapVal)
				return doc
			},
			want: Document{[]field{{"prefix.i", IntValue(42)}, {"prefix.str", StringValue("test")}}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := test.build()
			doc.Sort()
			assert.Equal(t, test.want, doc)
		})
	}
}

func TestDocument_Sort(t *testing.T) {
	tests := map[string]struct {
		build func() Document
		want  Document
	}{
		"keys are sorted": {
			build: func() (doc Document) {
				doc.AddInt("z", 26)
				doc.AddInt("a", 1)
				return doc
			},
			want: Document{[]field{{"a", IntValue(1)}, {"z", IntValue(26)}}},
		},
		"sorting is stable": {
			build: func() (doc Document) {
				doc.AddInt("a", 1)
				doc.AddInt("c", 3)
				doc.AddInt("a", 2)
				return doc
			},
			want: Document{[]field{{"a", IntValue(1)}, {"a", IntValue(2)}, {"c", IntValue(3)}}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := test.build()
			doc.Sort()
			assert.Equal(t, test.want, doc)
		})
	}

}

func TestObjectModel_Dedup(t *testing.T) {
	tests := map[string]struct {
		build func() Document
		want  Document
	}{
		"no duplicates": {
			build: func() (doc Document) {
				doc.AddInt("a", 1)
				doc.AddInt("c", 3)
				return doc
			},
			want: Document{[]field{{"a", IntValue(1)}, {"c", IntValue(3)}}},
		},
		"duplicate keys": {
			build: func() (doc Document) {
				doc.AddInt("a", 1)
				doc.AddInt("c", 3)
				doc.AddInt("a", 2)
				return doc
			},
			want: Document{[]field{{"a", ignoreValue}, {"a", IntValue(2)}, {"c", IntValue(3)}}},
		},
		"duplicate after flattening from map: namespace object at end": {
			build: func() Document {
				am := pcommon.NewMap()
				am.PutInt("namespace.a", 42)
				am.PutStr("toplevel", "test")
				am.PutEmptyMap("namespace").PutInt("a", 23)
				return DocumentFromAttributes(am)
			},
			want: Document{[]field{{"namespace.a", ignoreValue}, {"namespace.a", IntValue(23)}, {"toplevel", StringValue("test")}}},
		},
		"duplicate after flattening from map: namespace object at beginning": {
			build: func() Document {
				am := pcommon.NewMap()
				am.PutEmptyMap("namespace").PutInt("a", 23)
				am.PutInt("namespace.a", 42)
				am.PutStr("toplevel", "test")
				return DocumentFromAttributes(am)
			},
			want: Document{[]field{{"namespace.a", ignoreValue}, {"namespace.a", IntValue(42)}, {"toplevel", StringValue("test")}}},
		},
		"dedup in arrays": {
			build: func() (doc Document) {
				var embedded Document
				embedded.AddInt("a", 1)
				embedded.AddInt("c", 3)
				embedded.AddInt("a", 2)

				doc.Add("arr", ArrValue(Value{kind: KindObject, doc: embedded}))
				return doc
			},
			want: Document{[]field{{"arr", ArrValue(Value{kind: KindObject, doc: Document{[]field{
				{"a", ignoreValue},
				{"a", IntValue(2)},
				{"c", IntValue(3)},
			}}})}}},
		},
		"dedup mix of primitive and object lifts primitive": {
			build: func() (doc Document) {
				doc.AddInt("namespace", 1)
				doc.AddInt("namespace.a", 2)
				return doc
			},
			want: Document{[]field{{"namespace.a", IntValue(2)}, {"namespace.value", IntValue(1)}}},
		},
		"dedup removes primitive if value exists": {
			build: func() (doc Document) {
				doc.AddInt("namespace", 1)
				doc.AddInt("namespace.a", 2)
				doc.AddInt("namespace.value", 3)
				return doc
			},
			want: Document{[]field{{"namespace.a", IntValue(2)}, {"namespace.value", ignoreValue}, {"namespace.value", IntValue(3)}}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := test.build()
			doc.Sort()
			doc.Dedup()
			assert.Equal(t, test.want, doc)
		})
	}
}

func TestValue_FromAttribute(t *testing.T) {
	tests := map[string]struct {
		in   pcommon.Value
		want Value
	}{
		"null": {
			in:   pcommon.NewValueEmpty(),
			want: nilValue,
		},
		"string": {
			in:   pcommon.NewValueStr("test"),
			want: StringValue("test"),
		},
		"int": {
			in:   pcommon.NewValueInt(23),
			want: IntValue(23),
		},
		"double": {
			in:   pcommon.NewValueDouble(3.14),
			want: DoubleValue(3.14),
		},
		"bool": {
			in:   pcommon.NewValueBool(true),
			want: BoolValue(true),
		},
		"empty array": {
			in:   pcommon.NewValueSlice(),
			want: Value{kind: KindArr},
		},
		"non-empty array": {
			in: func() pcommon.Value {
				v := pcommon.NewValueSlice()
				tgt := v.Slice().AppendEmpty()
				pcommon.NewValueInt(1).CopyTo(tgt)
				return v
			}(),
			want: ArrValue(IntValue(1)),
		},
		"empty map": {
			in:   pcommon.NewValueMap(),
			want: Value{kind: KindObject},
		},
		"non-empty map": {
			in: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutInt("a", 1)
				return v
			}(),
			want: Value{kind: KindObject, doc: Document{[]field{{"a", IntValue(1)}}}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			v := ValueFromAttribute(test.in)
			assert.Equal(t, test.want, v)
		})
	}
}

func TestDocument_Serialize_Flat(t *testing.T) {
	tests := map[string]struct {
		attrs map[string]interface{}
		want  string
	}{
		"no nesting with multiple fields": {
			attrs: map[string]interface{}{
				"a": "test",
				"b": 1,
			},
			want: `{"a":"test","b":1}`,
		},
		"shared prefix": {
			attrs: map[string]interface{}{
				"a.str": "test",
				"a.i":   1,
			},
			want: `{"a.i":1,"a.str":"test"}`,
		},
		"multiple namespaces with dot": {
			attrs: map[string]interface{}{
				"a.str": "test",
				"b.i":   1,
			},
			want: `{"a.str":"test","b.i":1}`,
		},
		"nested maps": {
			attrs: map[string]interface{}{
				"a": map[string]interface{}{
					"str": "test",
					"i":   1,
				},
			},
			want: `{"a.i":1,"a.str":"test"}`,
		},
		"multi-level nested namespace maps": {
			attrs: map[string]interface{}{
				"a": map[string]interface{}{
					"b.str": "test",
					"i":     1,
				},
			},
			want: `{"a.b.str":"test","a.i":1}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var buf strings.Builder
			m := pcommon.NewMap()
			assert.NoError(t, m.FromRaw(test.attrs))
			doc := DocumentFromAttributes(m)
			doc.Dedup()
			err := doc.Serialize(&buf, false)
			require.NoError(t, err)

			assert.Equal(t, test.want, buf.String())
		})
	}
}

func TestDocument_Serialize_Dedot(t *testing.T) {
	tests := map[string]struct {
		attrs map[string]interface{}
		want  string
	}{
		"no nesting with multiple fields": {
			attrs: map[string]interface{}{
				"a": "test",
				"b": 1,
			},
			want: `{"a":"test","b":1}`,
		},
		"shared prefix": {
			attrs: map[string]interface{}{
				"a.str": "test",
				"a.i":   1,
			},
			want: `{"a":{"i":1,"str":"test"}}`,
		},
		"multiple namespaces": {
			attrs: map[string]interface{}{
				"a.str": "test",
				"b.i":   1,
			},
			want: `{"a":{"str":"test"},"b":{"i":1}}`,
		},
		"nested maps": {
			attrs: map[string]interface{}{
				"a": map[string]interface{}{
					"str": "test",
					"i":   1,
				},
			},
			want: `{"a":{"i":1,"str":"test"}}`,
		},
		"multi-level nested namespace maps": {
			attrs: map[string]interface{}{
				"a": map[string]interface{}{
					"b.c.str": "test",
					"i":       1,
				},
			},
			want: `{"a":{"b":{"c":{"str":"test"}},"i":1}}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var buf strings.Builder
			m := pcommon.NewMap()
			assert.NoError(t, m.FromRaw(test.attrs))
			doc := DocumentFromAttributes(m)
			doc.Dedup()
			err := doc.Serialize(&buf, true)
			require.NoError(t, err)

			assert.Equal(t, test.want, buf.String())
		})
	}
}

func TestValue_Serialize(t *testing.T) {
	tests := map[string]struct {
		value Value
		want  string
	}{
		"nil value":         {value: nilValue, want: "null"},
		"bool value: true":  {value: BoolValue(true), want: "true"},
		"bool value: false": {value: BoolValue(false), want: "false"},
		"int value":         {value: IntValue(42), want: "42"},
		"double value":      {value: DoubleValue(3.14), want: "3.14"},
		"NaN is undefined":  {value: DoubleValue(math.NaN()), want: "null"},
		"Inf is undefined":  {value: DoubleValue(math.Inf(0)), want: "null"},
		"string value":      {value: StringValue("Hello World!"), want: `"Hello World!"`},
		"timestamp": {
			value: TimestampValue(dijkstra),
			want:  `"1930-05-11T16:33:11.123456789Z"`,
		},
		"array": {
			value: ArrValue(BoolValue(true), IntValue(23)),
			want:  `[true,23]`,
		},
		"object": {
			value: func() Value {
				doc := Document{}
				doc.AddString("a", "b")
				return Value{kind: KindObject, doc: doc}
			}(),
			want: `{"a":"b"}`,
		},
		"empty object": {
			value: Value{kind: KindObject, doc: Document{}},
			want:  "null",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var buf strings.Builder
			err := test.value.iterJSON(json.NewVisitor(&buf), false)
			require.NoError(t, err)
			assert.Equal(t, test.want, buf.String())
		})
	}
}
