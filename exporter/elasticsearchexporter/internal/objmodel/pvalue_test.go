// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestPValue(t *testing.T) {
	key := "test"
	for _, tc := range []struct {
		name        string
		input       pcommon.Value
		expectedLen int
		expectedDoc Document
	}{
		{
			name:        "empty",
			input:       pcommon.NewValueEmpty(),
			expectedDoc: Document{},
		},
		{
			name:        "int",
			input:       pcommon.NewValueInt(7),
			expectedLen: 1,
			expectedDoc: func() Document {
				var doc Document
				doc.Add(key, IntValue(7))
				return doc
			}(),
		},
		{
			name:        "double",
			input:       pcommon.NewValueDouble(7.1),
			expectedLen: 1,
			expectedDoc: func() Document {
				var doc Document
				doc.Add(key, DoubleValue(7.1))
				return doc
			}(),
		},
		{
			name:        "bool",
			input:       pcommon.NewValueBool(true),
			expectedLen: 1,
			expectedDoc: func() Document {
				var doc Document
				doc.Add(key, BoolValue(true))
				return doc
			}(),
		},
		{
			name: "slice",
			input: func() pcommon.Value {
				s := pcommon.NewValueSlice()
				require.NoError(t, s.FromRaw([]any{
					1,
					2.1,
					map[string]any{
						"str":  "abc",
						"num":  1,
						"bool": true,
					},
				}))
				return s
			}(),
			expectedLen: 1, // Slices are not expanded
			expectedDoc: func() Document {
				var doc Document
				doc.Add(key, ArrValue(
					IntValue(1),
					DoubleValue(2.1),
					DocumentValue(func() Document {
						var d Document
						d.Add("str", StringValue("abc"))
						d.Add("num", IntValue(1))
						d.Add("bool", BoolValue(true))

						return d
					}()),
				))

				doc.Sort()
				return doc
			}(),
		},
		{
			name: "map",
			input: func() pcommon.Value {
				m := pcommon.NewValueMap()
				require.NoError(t, m.FromRaw(map[string]interface{}{
					"str":   "abc",
					"num":   1,
					"bool":  true,
					"slice": []any{1, 2},
					"map": map[string]any{
						"str":   "def",
						"num":   2,
						"bool":  false,
						"slice": []any{3, 4},
					},
				}))
				return m
			}(),
			expectedLen: 8,
			expectedDoc: func() Document {
				var doc Document
				doc.Add(key+".str", StringValue("abc"))
				doc.Add(key+".num", IntValue(1))
				doc.Add(key+".bool", BoolValue(true))
				doc.Add(key+".slice", ArrValue(IntValue(1), IntValue(2)))
				doc.Add(key+".map.str", StringValue("def"))
				doc.Add(key+".map.num", IntValue(2))
				doc.Add(key+".map.bool", BoolValue(false))
				doc.Add(key+".map.slice", ArrValue(IntValue(3), IntValue(4)))

				doc.Sort()
				return doc
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var actual Document
			p := NewPValueProcessor(tc.input)
			p.Process(&actual, key)
			actual.Sort()

			assert.Equal(t, tc.expectedLen, p.Len())
			assert.Equal(t, tc.expectedDoc, actual)
		})
	}
}
