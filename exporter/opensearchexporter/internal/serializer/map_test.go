// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializer

import (
	"bytes"
	"testing"

	"github.com/elastic/go-structform/json"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestMap(t *testing.T) {
	tests := []struct {
		name     string
		setupMap func() pcommon.Map
		expected string
	}{
		{
			name:     "empty map",
			setupMap: pcommon.NewMap,
			expected: "{}",
		},
		{
			name: "simple string values",
			setupMap: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("key1", "value1")
				m.PutStr("key2", "value2")
				return m
			},
			expected: `{"key1":"value1","key2":"value2"}`,
		},
		{
			name: "mixed value types",
			setupMap: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("string_key", "string_value")
				m.PutInt("int_key", 42)
				m.PutDouble("double_key", 3.14)
				m.PutBool("bool_key", true)
				return m
			},
			expected: `{"string_key":"string_value","int_key":42,"double_key":3.14,"bool_key":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			Map(tt.setupMap(), buf)
			assert.JSONEq(t, tt.expected, buf.String())
		})
	}
}

func TestWriteValue(t *testing.T) {
	tests := []struct {
		name          string
		value         pcommon.Value
		stringifyMaps bool
		expected      string
	}{
		{
			name:     "empty value",
			value:    pcommon.NewValueEmpty(),
			expected: "null",
		},
		{
			name:     "string value",
			value:    pcommon.NewValueStr("test"),
			expected: `"test"`,
		},
		{
			name:     "bool value true",
			value:    pcommon.NewValueBool(true),
			expected: "true",
		},
		{
			name:     "bool value false",
			value:    pcommon.NewValueBool(false),
			expected: "false",
		},
		{
			name:     "int value",
			value:    pcommon.NewValueInt(123),
			expected: "123",
		},
		{
			name:     "double value",
			value:    pcommon.NewValueDouble(3.14),
			expected: "3.14",
		},
		{
			name:     "double value with explicit radix point",
			value:    pcommon.NewValueDouble(1.0),
			expected: "1.0",
		},
		{
			name: "bytes value",
			value: func() pcommon.Value {
				v := pcommon.NewValueBytes()
				v.Bytes().FromRaw([]byte{0xDE, 0xAD, 0xBE, 0xEF})
				return v
			}(),
			expected: `"deadbeef"`,
		},
		{
			name: "map value without stringify",
			value: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("key", "value")
				return v
			}(),
			stringifyMaps: false,
			expected:      `{"key":"value"}`,
		},
		{
			name: "map value with stringify",
			value: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("key", "value")
				return v
			}(),
			stringifyMaps: true,
			expected:      `"{\"key\":\"value\"}"`,
		},
		{
			name: "slice value with mixed types",
			value: func() pcommon.Value {
				v := pcommon.NewValueSlice()
				slice := v.Slice()
				slice.AppendEmpty().SetStr("string")
				slice.AppendEmpty().SetInt(42)
				slice.AppendEmpty().SetBool(true)
				return v
			}(),
			expected: `["string",42,true]`,
		},
		{
			name: "slice value empty",
			value: func() pcommon.Value {
				v := pcommon.NewValueSlice()
				return v
			}(),
			expected: `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			v := json.NewVisitor(buf)
			v.SetExplicitRadixPoint(true)
			WriteValue(v, tt.value, tt.stringifyMaps)
			assert.JSONEq(t, tt.expected, buf.String())
		})
	}
}
