// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	testEntry := &Entry{
		Body: map[string]any{
			"string_field": "string_val",
			"byte_field":   []byte(`test`),
			"map_string_interface_field": map[string]any{
				"nested": "interface_val",
			},
			"map_string_interface_nonstring_field": map[string]any{
				"nested": 111,
			},
			"map_string_string_field": map[string]string{
				"nested": "string_val",
			},
			"map_interface_interface_field": map[any]any{
				"nested": "interface_val",
			},
			"map_interface_interface_nonstring_key_field": map[any]any{
				100: "interface_val",
			},
			"map_interface_interface_nonstring_value_field": map[any]any{
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

	t.Run("map[string]any", func(t *testing.T) {
		var m map[string]any
		err := testEntry.Read(NewBodyField("map_string_interface_field"), &m)
		require.NoError(t, err)
		require.Equal(t, map[string]any{"nested": "interface_val"}, m)
	})

	t.Run("map[string]any error", func(t *testing.T) {
		var m map[string]any
		err := testEntry.Read(NewBodyField("string_field"), &m)
		require.Error(t, err)
	})

	t.Run("map[string]string from map[string]any", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_string_interface_field"), &m)
		require.NoError(t, err)
		require.Equal(t, map[string]string{"nested": "interface_val"}, m)
	})

	t.Run("map[string]string from map[string]any err", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_string_interface_nonstring_field"), &m)
		require.Error(t, err)
	})

	t.Run("map[string]string from map[any]any", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_interface_interface_field"), &m)
		require.NoError(t, err)
		require.Equal(t, map[string]string{"nested": "interface_val"}, m)
	})

	t.Run("map[string]string from map[any]any nonstring key error", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_interface_interface_nonstring_key_field"), &m)
		require.Error(t, err)
	})

	t.Run("map[string]string from map[any]any nonstring value error", func(t *testing.T) {
		var m map[string]string
		err := testEntry.Read(NewBodyField("map_interface_interface_nonstring_value_field"), &m)
		require.Error(t, err)
	})

	t.Run("any from any", func(t *testing.T) {
		var i any
		err := testEntry.Read(NewBodyField("map_interface_interface_field"), &i)
		require.NoError(t, err)
		require.Equal(t, map[any]any{"nested": "interface_val"}, i)
	})

	t.Run("string from []byte", func(t *testing.T) {
		var i string
		err := testEntry.Read(NewBodyField("byte_field"), &i)
		require.NoError(t, err)
		require.Equal(t, "test", i)
	})
}

func TestCopy(t *testing.T) {
	now := time.Now()

	entry := New()
	entry.Severity = Severity(0)
	entry.SeverityText = "ok"
	entry.ObservedTimestamp = now
	entry.Timestamp = time.Time{}
	entry.Body = "test"
	entry.Attributes = map[string]any{"label": "value"}
	entry.Resource = map[string]any{"resource": "value"}
	entry.TraceID = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	entry.SpanID = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	entry.TraceFlags = []byte{0x01}
	entry.ScopeName = "my.logger"
	cp := entry.Copy()

	entry.Severity = Severity(1)
	entry.SeverityText = "1"
	entry.Timestamp = time.Now()
	entry.Body = "new"
	entry.Attributes = map[string]any{"label": "new value"}
	entry.Resource = map[string]any{"resource": "new value"}
	entry.TraceID[0] = 0xff
	entry.SpanID[0] = 0xff
	entry.TraceFlags[0] = 0xff
	entry.ScopeName = "foo"

	require.Equal(t, now, cp.ObservedTimestamp)
	require.Equal(t, time.Time{}, cp.Timestamp)
	require.Equal(t, Severity(0), cp.Severity)
	require.Equal(t, "ok", cp.SeverityText)
	require.Equal(t, map[string]any{"label": "value"}, cp.Attributes)
	require.Equal(t, map[string]any{"resource": "value"}, cp.Resource)
	require.Equal(t, "test", cp.Body)
	require.Equal(t, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}, cp.TraceID)
	require.Equal(t, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, cp.SpanID)
	require.Equal(t, []byte{0x01}, cp.TraceFlags)
	require.Equal(t, "my.logger", cp.ScopeName)
}

func TestCopyNil(t *testing.T) {
	now := time.Now()
	entry := New()
	entry.ObservedTimestamp = now
	cp := entry.Copy()

	entry.Severity = Severity(1)
	entry.SeverityText = "1"
	entry.Timestamp = time.Now()
	entry.Body = "new"
	entry.Attributes = map[string]any{"label": "new value"}
	entry.Resource = map[string]any{"resource": "new value"}
	entry.TraceID = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	entry.SpanID = []byte{0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x01, 0x02, 0x03}
	entry.TraceFlags = []byte{0x01}
	entry.ScopeName = "foo"

	require.Equal(t, now, cp.ObservedTimestamp)
	require.Equal(t, time.Time{}, cp.Timestamp)
	require.Equal(t, Severity(0), cp.Severity)
	require.Empty(t, cp.SeverityText)
	require.Equal(t, map[string]any{}, cp.Attributes)
	require.Equal(t, map[string]any{}, cp.Resource)
	require.Nil(t, cp.Body)
	require.Equal(t, []byte{}, cp.TraceID)
	require.Equal(t, []byte{}, cp.SpanID)
	require.Equal(t, []byte{}, cp.TraceFlags)
	require.Empty(t, cp.ScopeName)
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
			"NestedBody",
			"body.test.foo.bar",
			Field{BodyField{[]string{"test", "foo", "bar"}}},
			false,
		},
		{
			"SimpleAttribute",
			"attributes.test",
			Field{AttributeField{[]string{"test"}}},
			false,
		},
		{
			"NestedAttribute",
			"attributes.test.foo.bar",
			Field{AttributeField{[]string{"test", "foo", "bar"}}},
			false,
		},
		{
			"SimpleResource",
			"resource.test",
			Field{ResourceField{[]string{"test"}}},
			false,
		},
		{
			"NestedResource",
			"resource.test.foo.bar",
			Field{ResourceField{[]string{"test", "foo", "bar"}}},
			false,
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
	expected := map[string]any{"label": "value"}
	require.Equal(t, expected, entry.Attributes)
}

func TestAddResourceKey(t *testing.T) {
	entry := Entry{}
	entry.AddResourceKey("key", "value")
	expected := map[string]any{"key": "value"}
	require.Equal(t, expected, entry.Resource)
}

func TestReadToInterfaceMapWithMissingField(t *testing.T) {
	entry := Entry{}
	field := NewAttributeField("label")
	dest := map[string]any{}
	err := entry.readToInterfaceMap(field, &dest)
	require.ErrorContains(t, err, "can not be read as a map[string]any")
}

func TestReadToStringMapWithMissingField(t *testing.T) {
	entry := Entry{}
	field := NewAttributeField("label")
	dest := map[string]string{}
	err := entry.readToStringMap(field, &dest)
	require.ErrorContains(t, err, "can not be read as a map[string]string")
}

func TestReadToInterfaceMissingField(t *testing.T) {
	entry := Entry{}
	field := NewAttributeField("label")
	var dest any
	err := entry.readToInterface(field, &dest)
	require.ErrorContains(t, err, "can not be read as a any")
}

func TestDefaultTimestamps(t *testing.T) {
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	e := New()
	require.Equal(t, now, e.ObservedTimestamp)
	require.True(t, e.Timestamp.IsZero())
}
