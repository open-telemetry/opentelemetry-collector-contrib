// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	testEntry := &Entry{
		Body: map[string]interface{}{
			"string_field": "string_val",
			"byte_field":   []byte(`test`),
			"map_string_interface_field": map[string]interface{}{
				"nested": "interface_val",
			},
			"map_string_interface_nonstring_field": map[string]interface{}{
				"nested": 111,
			},
			"map_string_string_field": map[string]string{
				"nested": "string_val",
			},
			"map_interface_interface_field": map[interface{}]interface{}{
				"nested": "interface_val",
			},
			"map_interface_interface_nonstring_key_field": map[interface{}]interface{}{
				100: "interface_val",
			},
			"map_interface_interface_nonstring_value_field": map[interface{}]interface{}{
				"nested": 100,
			},
		},
	}

	t.Run("field not exist error", func(t *testing.T) {
		var s string
		err := testEntry.Read(NewBodyField("nonexistant_field"), &s)
		require.Error(t, err)
	})

	t.Run("unsupported type error", func(t *testing.T) {
		var s **string
		err := testEntry.Read(NewBodyField("string_field"), &s)
		require.Error(t, err)
	})

	t.Run("string", func(t *testing.T) {
		var s string
		err := testEntry.Read(NewBodyField("string_field"), &s)
		require.NoError(t, err)
		require.Equal(t, "string_val", s)
	})

	t.Run("string error", func(t *testing.T) {
		var s string
		err := testEntry.Read(NewBodyField("map_string_interface_field"), &s)
		require.Error(t, err)
	})

	t.Run("map[string]interface{}", func(t *testing.T) {
		var m map[string]interface{}
		err := testEntry.Read(NewBodyField("map_string_interface_field"), &m)
		require.NoError(t, err)
		require.Equal(t, map[string]interface{}{"nested": "interface_val"}, m)
	})

	t.Run("map[string]interface{} error", func(t *testing.T) {
		var m map[string]interface{}
		err := testEntry.Read(NewBodyField("string_field"), &m)
		require.Error(t, err)
	})

	t.Run("map[string]string from map[string]interface{}", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_string_interface_field"), &m)
		require.NoError(t, err)
		require.Equal(t, map[string]string{"nested": "interface_val"}, m)
	})

	t.Run("map[string]string from map[string]interface{} err", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_string_interface_nonstring_field"), &m)
		require.Error(t, err)
	})

	t.Run("map[string]string from map[interface{}]interface{}", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_interface_interface_field"), &m)
		require.NoError(t, err)
		require.Equal(t, map[string]string{"nested": "interface_val"}, m)
	})

	t.Run("map[string]string from map[interface{}]interface{} nonstring key error", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_interface_interface_nonstring_key_field"), &m)
		require.Error(t, err)
	})

	t.Run("map[string]string from map[interface{}]interface{} nonstring value error", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_interface_interface_nonstring_value_field"), &m)
		require.Error(t, err)
	})

	t.Run("interface{} from any", func(t *testing.T) {
		var i interface{}
		err := testEntry.Read(NewBodyField("map_interface_interface_field"), &i)
		require.NoError(t, err)
		require.Equal(t, map[interface{}]interface{}{"nested": "interface_val"}, i)
	})

	t.Run("string from []byte", func(t *testing.T) {
		var i string
		err := testEntry.Read(NewBodyField("byte_field"), &i)
		require.NoError(t, err)
		require.Equal(t, "test", i)
	})
}

func TestCopy(t *testing.T) {
	entry := New()
	entry.Severity = Severity(0)
	entry.SeverityText = "ok"
	entry.Timestamp = time.Time{}
	entry.Body = "test"
	entry.Attributes = map[string]interface{}{"label": "value"}
	entry.Resource = map[string]interface{}{"resource": "value"}
	entry.TraceId = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	entry.SpanId = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	entry.TraceFlags = []byte{0x01}
	copy := entry.Copy()

	entry.Severity = Severity(1)
	entry.SeverityText = "1"
	entry.Timestamp = time.Now()
	entry.Body = "new"
	entry.Attributes = map[string]interface{}{"label": "new value"}
	entry.Resource = map[string]interface{}{"resource": "new value"}
	entry.TraceId[0] = 0xff
	entry.SpanId[0] = 0xff
	entry.TraceFlags[0] = 0xff

	require.Equal(t, time.Time{}, copy.Timestamp)
	require.Equal(t, Severity(0), copy.Severity)
	require.Equal(t, "ok", copy.SeverityText)
	require.Equal(t, map[string]interface{}{"label": "value"}, copy.Attributes)
	require.Equal(t, map[string]interface{}{"resource": "value"}, copy.Resource)
	require.Equal(t, "test", copy.Body)
	require.Equal(t, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}, copy.TraceId)
	require.Equal(t, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, copy.SpanId)
	require.Equal(t, []byte{0x01}, copy.TraceFlags)
}

func TestCopyNil(t *testing.T) {
	entry := New()
	entry.Timestamp = time.Time{}
	copy := entry.Copy()

	entry.Severity = Severity(1)
	entry.SeverityText = "1"
	entry.Timestamp = time.Now()
	entry.Body = "new"
	entry.Attributes = map[string]interface{}{"label": "new value"}
	entry.Resource = map[string]interface{}{"resource": "new value"}
	entry.TraceId = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	entry.SpanId = []byte{0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x01, 0x02, 0x03}
	entry.TraceFlags = []byte{0x01}

	require.Equal(t, time.Time{}, copy.Timestamp)
	require.Equal(t, Severity(0), copy.Severity)
	require.Equal(t, "", copy.SeverityText)
	require.Equal(t, map[string]interface{}{}, copy.Attributes)
	require.Equal(t, map[string]interface{}{}, copy.Resource)
	require.Equal(t, nil, copy.Body)
	require.Equal(t, []byte{}, copy.TraceId)
	require.Equal(t, []byte{}, copy.SpanId)
	require.Equal(t, []byte{}, copy.TraceFlags)
}

func TestFieldFromString(t *testing.T) {
	cases := []struct {
		name          string
		input         string
		output        Field
		expectedError bool
	}{
		{
			"Body",
			"body",
			Field{BodyField{[]string{}}},
			false,
		},
		{
			"PrefixedBody",
			"body.test",
			Field{BodyField{[]string{"test"}}},
			false,
		},
		{
			"FullPrefixedBody",
			"body.test",
			Field{BodyField{[]string{"test"}}},
			false,
		},
		{
			"SimpleAttribute",
			"attributes.test",
			Field{AttributeField{"test"}},
			false,
		},
		{
			"AttributesTooManyFields",
			"attributes.test.bar",
			Field{},
			true,
		},
		{
			"SimpleResource",
			"resource.test",
			Field{ResourceField{"test"}},
			false,
		},
		{
			"ResourceTooManyFields",
			"resource.test.bar",
			Field{},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := NewField(tc.input)
			if tc.expectedError {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.output, f)
		})
	}
}

func TestAddAttribute(t *testing.T) {
	entry := Entry{}
	entry.AddAttribute("label", "value")
	expected := map[string]interface{}{"label": "value"}
	require.Equal(t, expected, entry.Attributes)
}

func TestAddResourceKey(t *testing.T) {
	entry := Entry{}
	entry.AddResourceKey("key", "value")
	expected := map[string]interface{}{"key": "value"}
	require.Equal(t, expected, entry.Resource)
}

func TestReadToInterfaceMapWithMissingField(t *testing.T) {
	entry := Entry{}
	field := NewAttributeField("label")
	dest := map[string]interface{}{}
	err := entry.readToInterfaceMap(field, &dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "can not be read as a map[string]interface{}")
}

func TestReadToStringMapWithMissingField(t *testing.T) {
	entry := Entry{}
	field := NewAttributeField("label")
	dest := map[string]string{}
	err := entry.readToStringMap(field, &dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "can not be read as a map[string]string")
}

func TestReadToInterfaceMissingField(t *testing.T) {
	entry := Entry{}
	field := NewAttributeField("label")
	var dest interface{}
	err := entry.readToInterface(field, &dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "can not be read as a interface{}")
}
