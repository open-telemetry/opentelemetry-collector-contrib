// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"time"

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
			want: Document{fields: []field{{"i", IntValue(42)}, {"str", StringValue("test")}}},
		},
		"ignores nil values": {
			build: func() Document {
				m := pcommon.NewMap()
				m.PutEmpty("null")
				m.PutStr("str", "test")
				return DocumentFromAttributes(m)
			},
			want: Document{fields: []field{{"str", StringValue("test")}}},
		},
		"from map with prefix": {
			build: func() Document {
				m := pcommon.NewMap()
				m.PutInt("i", 42)
				m.PutStr("str", "test")
				return DocumentFromAttributesWithPath("prefix", m)
			},
			want: Document{fields: []field{{"prefix.i", IntValue(42)}, {"prefix.str", StringValue("test")}}},
		},
		"add attributes with key": {
			build: func() (doc Document) {
				m := pcommon.NewMap()
				m.PutInt("i", 42)
				m.PutStr("str", "test")
				doc.AddAttributes("prefix", m)
				return doc
			},
			want: Document{fields: []field{{"prefix.i", IntValue(42)}, {"prefix.str", StringValue("test")}}},
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
			want: Document{fields: []field{{"prefix.i", IntValue(42)}, {"prefix.str", StringValue("test")}}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := test.build()
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
			want: Document{fields: []field{{"a", IntValue(1)}, {"c", IntValue(3)}}},
		},
		"duplicate keys": {
			build: func() (doc Document) {
				doc.AddInt("a", 1)
				doc.AddInt("c", 3)
				doc.AddInt("a", 2)
				return doc
			},
			want: Document{fields: []field{{"a", ignoreValue}, {"a", IntValue(2)}, {"c", IntValue(3)}}},
		},
		"duplicate after flattening from map: namespace object at end": {
			build: func() Document {
				am := pcommon.NewMap()
				am.PutInt("namespace.a", 42)
				am.PutStr("toplevel", "test")
				am.PutEmptyMap("namespace").PutInt("a", 23)
				return DocumentFromAttributes(am)
			},
			want: Document{fields: []field{{"namespace.a", ignoreValue}, {"namespace.a", IntValue(23)}, {"toplevel", StringValue("test")}}},
		},
		"duplicate after flattening from map: namespace object at beginning": {
			build: func() Document {
				am := pcommon.NewMap()
				am.PutEmptyMap("namespace").PutInt("a", 23)
				am.PutInt("namespace.a", 42)
				am.PutStr("toplevel", "test")
				return DocumentFromAttributes(am)
			},
			want: Document{fields: []field{{"namespace.a", ignoreValue}, {"namespace.a", IntValue(42)}, {"toplevel", StringValue("test")}}},
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
			want: Document{fields: []field{{"arr", ArrValue(Value{kind: KindObject, doc: Document{fields: []field{
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
			want: Document{fields: []field{{"namespace.a", IntValue(2)}, {"namespace.value", IntValue(1)}}},
		},
		"dedup removes primitive if value exists": {
			build: func() (doc Document) {
				doc.AddInt("namespace", 1)
				doc.AddInt("namespace.a", 2)
				doc.AddInt("namespace.value", 3)
				return doc
			},
			want: Document{fields: []field{{"namespace.a", IntValue(2)}, {"namespace.value", ignoreValue}, {"namespace.value", IntValue(3)}}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := test.build()
			doc.Dedup(nil)
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
			want: Value{kind: KindObject, doc: Document{fields: []field{{"a", IntValue(1)}}}},
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
		attrs map[string]any
		want  string
	}{
		"no nesting with multiple fields": {
			attrs: map[string]any{
				"a": "test",
				"b": 1,
			},
			want: `{"a":"test","b":1}`,
		},
		"shared prefix": {
			attrs: map[string]any{
				"a.str": "test",
				"a.i":   1,
			},
			want: `{"a.i":1,"a.str":"test"}`,
		},
		"multiple namespaces with dot": {
			attrs: map[string]any{
				"a.str": "test",
				"b.i":   1,
			},
			want: `{"a.str":"test","b.i":1}`,
		},
		"nested maps": {
			attrs: map[string]any{
				"a": map[string]any{
					"str": "test",
					"i":   1,
				},
			},
			want: `{"a.i":1,"a.str":"test"}`,
		},
		"multi-level nested namespace maps": {
			attrs: map[string]any{
				"a": map[string]any{
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
			doc.Dedup(nil)
			err := doc.Serialize(&buf, false, nil)
			require.NoError(t, err)

			assert.Equal(t, test.want, buf.String())
		})
	}
}

func TestDocument_Serialize_Dedot(t *testing.T) {
	tests := map[string]struct {
		attrs map[string]any
		want  string
	}{
		"no nesting with multiple fields": {
			attrs: map[string]any{
				"a": "test",
				"b": 1,
			},
			want: `{"a":"test","b":1}`,
		},
		"shared prefix": {
			attrs: map[string]any{
				"a.str": "test",
				"a.i":   1,
			},
			want: `{"a":{"i":1,"str":"test"}}`,
		},
		"multiple namespaces": {
			attrs: map[string]any{
				"a.str": "test",
				"b.i":   1,
			},
			want: `{"a":{"str":"test"},"b":{"i":1}}`,
		},
		"nested maps": {
			attrs: map[string]any{
				"a": map[string]any{
					"str": "test",
					"i":   1,
				},
			},
			want: `{"a":{"i":1,"str":"test"}}`,
		},
		"multi-level nested namespace maps": {
			attrs: map[string]any{
				"a": map[string]any{
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
			doc.Dedup(nil)
			err := doc.Serialize(&buf, true, nil)
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
		"nil value":          {value: nilValue, want: "null"},
		"bool value: true":   {value: BoolValue(true), want: "true"},
		"bool value: false":  {value: BoolValue(false), want: "false"},
		"int value":          {value: IntValue(42), want: "42"},
		"uint value":         {value: UIntValue(42), want: "42"},
		"double value: 3.14": {value: DoubleValue(3.14), want: "3.14"},
		"double value: 1.0":  {value: DoubleValue(1.0), want: "1.0"},
		"NaN is undefined":   {value: DoubleValue(math.NaN()), want: "null"},
		"Inf is undefined":   {value: DoubleValue(math.Inf(0)), want: "null"},
		"string value":       {value: StringValue("Hello World!"), want: `"Hello World!"`},
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
			err := test.value.iterJSON(newJSONVisitor(&buf), false)
			require.NoError(t, err)
			assert.Equal(t, test.want, buf.String())
		})
	}
}

func TestCloneDocument_NestedValuesAreIndependent(t *testing.T) {
	cases := []struct {
		name  string
		build func() Document
		key   string
	}{
		{
			name: "nested object",
			build: func() Document {
				var nested Document
				nested.AddInt("a", 1)
				nested.AddString("a.b", "nested")
				var src Document
				src.Add("obj", Value{kind: KindObject, doc: nested})
				return src
			},
			key: "obj",
		},
		{
			name: "array of objects",
			build: func() Document {
				var nested Document
				nested.AddInt("a", 1)
				nested.AddString("a.b", "nested")
				var src Document
				src.Add("arr", ArrValue(Value{kind: KindObject, doc: nested}))
				return src
			},
			key: "arr",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.build()
			cloned := cloneDocument(src)
			cloned.Dedup(nil)

			require.Equal(t, "a", nestedFirstKey(t, src, tc.key))
		})
	}
}

func TestAssembleFromSortedBase_MatchesConcatDedup(t *testing.T) {
	cases := []struct {
		name      string
		base      func() Document
		extra     func() Document
		protected map[string]struct{}
	}{
		{
			name: "prefix conflict keeps resource and record value",
			base: func() Document {
				var doc Document
				doc.AddString("x", "resource")
				doc.AddString("x.a", "scope")
				return doc
			},
			extra: func() Document {
				var doc Document
				doc.AddString("x", "record")
				return doc
			},
		},
		{
			name: "equal key last-wins keeps extra",
			base: func() Document {
				var doc Document
				doc.AddString("service.name", "from-resource")
				return doc
			},
			extra: func() Document {
				var doc Document
				doc.AddString("service.name", "from-record")
				return doc
			},
		},
		{
			name: "duplicate keys inside extra",
			base: func() Document {
				var doc Document
				doc.AddInt("a", 1)
				return doc
			},
			extra: func() Document {
				var doc Document
				doc.AddInt("b", 2)
				doc.AddInt("b", 3)
				return doc
			},
		},
		{
			name: "protected prefix ignores nested extra",
			base: func() Document {
				var doc Document
				doc.AddString("host", "from-resource")
				return doc
			},
			extra: func() Document {
				var doc Document
				doc.AddString("host.name", "from-record")
				return doc
			},
			protected: map[string]struct{}{"host": {}},
		},
		{
			name: "empty extra",
			base: func() Document {
				var doc Document
				doc.AddInt("a", 1)
				doc.AddInt("c", 3)
				return doc
			},
			extra: func() Document {
				return Document{}
			},
		},
		{
			name: "empty base",
			base: func() Document {
				return Document{}
			},
			extra: func() Document {
				var doc Document
				doc.AddInt("a", 1)
				doc.AddInt("c", 3)
				return doc
			},
		},
		{
			name: "dynamic templates copy from base",
			base: func() Document {
				var doc Document
				doc.AddInt("a", 1)
				doc.AddDynamicTemplate("a", "histogram")
				return doc
			},
			extra: func() Document {
				var doc Document
				doc.AddInt("b", 2)
				return doc
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertAssembleMatchesConcat(t, tc.base(), tc.extra(), tc.protected)
		})
	}
}

func TestAssembleFromSortedBase_RandomKeys(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 50 {
		assertAssembleMatchesConcat(t, randomDoc(rng, 20), randomDoc(rng, 8), nil)
	}
}

func TestAssembleFromSortedBase_DoesNotMutateBase(t *testing.T) {
	var nested Document
	nested.AddInt("a", 1)
	nested.AddString("a.b", "nested")
	var base Document
	base.Add("obj", Value{kind: KindObject, doc: nested})
	base.Sort()

	var extra Document
	extra.AddInt("n", 1)
	_ = assembleFromSortedBase(base, extra, nil)

	require.Equal(t, "a", nestedFirstKey(t, base, "obj"))
}

func assertAssembleMatchesConcat(t *testing.T, base, extra Document, protected map[string]struct{}) {
	t.Helper()

	concat := cloneDocument(base)
	concat.fields = append(concat.fields, cloneDocument(extra).fields...)
	var want strings.Builder
	require.NoError(t, concat.Serialize(&want, true, protected))

	sorted := cloneDocument(base)
	sorted.Sort()
	gotDoc := assembleFromSortedBase(sorted, cloneDocument(extra), protected)
	var got strings.Builder
	require.NoError(t, gotDoc.writeJSON(&got, true))

	require.Equal(t, want.String(), got.String())
	require.Equal(t, concat.fields, gotDoc.fields)
	require.Equal(t, concat.dynamicTemplates, gotDoc.dynamicTemplates)
}

func randomDoc(rng *rand.Rand, n int) Document {
	var doc Document
	for i := range n {
		key := "k" + strconv.Itoa(rng.IntN(12))
		if rng.IntN(4) == 0 {
			key += ".a"
		}
		doc.AddString(key, "v"+strconv.Itoa(i))
	}
	return doc
}

func nestedFirstKey(t *testing.T, doc Document, key string) string {
	t.Helper()
	for i := range doc.fields {
		if doc.fields[i].key != key {
			continue
		}
		v := doc.fields[i].value
		switch v.kind {
		case KindObject:
			require.NotEmpty(t, v.doc.fields)
			return v.doc.fields[0].key
		case KindArr:
			require.NotEmpty(t, v.arr)
			require.Equal(t, KindObject, v.arr[0].kind)
			require.NotEmpty(t, v.arr[0].doc.fields)
			return v.arr[0].doc.fields[0].key
		}
	}
	t.Fatalf("key %q not found", key)
	return ""
}
