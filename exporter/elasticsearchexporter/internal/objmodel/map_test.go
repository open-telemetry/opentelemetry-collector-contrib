// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestMap(t *testing.T) {
	key := "test"
	for _, tc := range []struct {
		name        string
		m           pcommon.Map
		keyRemapper func(string) string
		expectedLen int
		expectedDoc Document
	}{
		{
			name:        "empty",
			m:           pcommon.NewMap(),
			expectedDoc: Document{},
		},
		{
			name: "map",
			m: func() pcommon.Map {
				m := pcommon.NewMap()
				m.FromRaw(map[string]interface{}{
					"str":   "abc",
					"num":   1.1,
					"bool":  true,
					"slice": []any{1, 2.1},
					"map": map[string]any{
						"str":   "def",
						"num":   2,
						"bool":  false,
						"slice": []any{3, 4},
					},
				})
				return m
			}(),
			expectedLen: 8,
			expectedDoc: func() Document {
				var doc Document
				doc.Add(key+".str", StringValue("abc"))
				doc.Add(key+".num", DoubleValue(1.1))
				doc.Add(key+".bool", BoolValue(true))
				doc.Add(key+".slice", ArrValue(IntValue(1), DoubleValue(2.1)))
				doc.Add(key+".map.str", StringValue("def"))
				doc.Add(key+".map.num", IntValue(2))
				doc.Add(key+".map.bool", BoolValue(false))
				doc.Add(key+".map.slice", ArrValue(IntValue(3), IntValue(4)))

				doc.Sort()
				return doc
			}(),
		},
		{
			name: "map_with_remapper",
			m: func() pcommon.Map {
				m := pcommon.NewMap()
				m.FromRaw(map[string]interface{}{
					"str":   "abc",
					"num":   1.1,
					"bool":  true,
					"slice": []any{1, 2.1},
					"map": map[string]any{
						"str":   "def",
						"num":   2,
						"bool":  false,
						"slice": []any{3, 4},
					},
				})
				return m
			}(),
			keyRemapper: func(k string) string {
				switch k {
				case "test.str":
					return "" // should be ignored
				case "test.map.num":
					return "k.map.num"
				}
				return k
			},
			// expected len is approximate and doesn't accout for entries
			// removed via the key remapper
			expectedLen: 8,
			expectedDoc: func() Document {
				var doc Document
				doc.Add(key+".num", DoubleValue(1.1))
				doc.Add(key+".bool", BoolValue(true))
				doc.Add(key+".slice", ArrValue(IntValue(1), DoubleValue(2.1)))
				doc.Add(key+".map.str", StringValue("def"))
				doc.Add("k.map.num", IntValue(2))
				doc.Add(key+".map.bool", BoolValue(false))
				doc.Add(key+".map.slice", ArrValue(IntValue(3), IntValue(4)))

				doc.Sort()
				return doc
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var actual Document
			p := NewMapProcessor(tc.m, tc.keyRemapper)
			p.Process(&actual, key)
			actual.Sort()

			assert.Equal(t, tc.expectedLen, p.Len())
			assert.Equal(t, tc.expectedDoc, actual)
		})
	}
}
