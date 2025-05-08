// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAttributeFieldGet(t *testing.T) {
	cases := []struct {
		name       string
		field      Field
		attributes map[string]any
		expected   any
		expectedOK bool
	}{
		{
			"Uninitialized",
			NewAttributeField("nonexistent"),
			nil,
			"",
			false,
		},
		{
			"RootField",
			NewAttributeField(),
			testMap(),
			testMap(),
			true,
		},
		{
			"Simple",
			NewAttributeField("test"),
			map[string]any{
				"test": "val",
			},
			"val",
			true,
		},
		{
			"NonexistentKey",
			NewAttributeField("nonexistent"),
			map[string]any{
				"test": "val",
			},
			nil,
			false,
		},
		{
			"MapField",
			NewAttributeField("map_key"),
			testMap(),
			nestedMap(),
			true,
		},
		{
			"NestedField",
			NewAttributeField("map_key", "nested_key"),
			testMap(),
			"nested_value",
			true,
		},
		{
			"MissingField",
			NewAttributeField("invalid"),
			testMap(),
			nil,
			false,
		},
		{
			"InvalidField",
			NewAttributeField("simple_key", "nested_key"),
			testMap(),
			nil,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Attributes = tc.attributes
			val, ok := entry.Get(tc.field)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expected, val)
		})
	}
}

func TestAttributeFieldDelete(t *testing.T) {
	cases := []struct {
		name               string
		field              Field
		attributes         map[string]any
		expectedAttributes map[string]any
		expectedReturned   any
		expectedOK         bool
	}{
		{
			"Uninitialized",
			NewAttributeField("nonexistent"),
			nil,
			nil,
			"",
			false,
		},
		{
			"SimpleKey",
			NewAttributeField("simple_key"),
			testMap(),
			map[string]any{
				"map_key": nestedMap(),
			},
			"simple_value",
			true,
		},
		{
			"EmptyAttributesAndField",
			NewAttributeField(),
			map[string]any{},
			nil,
			map[string]any{},
			true,
		},
		{
			"EmptyField",
			NewAttributeField(),
			testMap(),
			nil,
			testMap(),
			true,
		},
		{
			"MissingKey",
			NewAttributeField("missing_key"),
			testMap(),
			testMap(),
			nil,
			false,
		},
		{
			"NestedKey",
			NewAttributeField("map_key", "nested_key"),
			testMap(),
			map[string]any{
				"simple_key": "simple_value",
				"map_key":    map[string]any{},
			},
			"nested_value",
			true,
		},
		{
			"MapKey",
			NewAttributeField("map_key"),
			testMap(),
			map[string]any{
				"simple_key": "simple_value",
			},
			nestedMap(),
			true,
		},
		{
			"InvalidNestedKey",
			NewAttributeField("simple_key", "missing"),
			testMap(),
			testMap(),
			nil,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Attributes = tc.attributes
			val, ok := entry.Delete(tc.field)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expectedReturned, val)
			assert.Equal(t, tc.expectedAttributes, entry.Attributes)
		})
	}
}

func TestAttributeFieldSet(t *testing.T) {
	cases := []struct {
		name        string
		field       Field
		attributes  map[string]any
		val         any
		expected    map[string]any
		expectedErr bool
	}{
		{
			"Uninitialized",
			NewAttributeField("test"),
			nil,
			"val",
			map[string]any{
				"test": "val",
			},
			false,
		},
		{
			"OverwriteRoot",
			NewAttributeField(),
			testMap(),
			"val",
			testMap(),
			true,
		},
		{
			"OverwriteRootWithMap",
			NewAttributeField(),
			map[string]any{},
			testMap(),
			testMap(),
			false,
		},
		{
			"MergeOverRoot",
			NewAttributeField(),
			map[string]any{
				"simple_key": "clobbered",
				"hello":      "world",
			},
			testMap(),
			map[string]any{
				"simple_key": "simple_value",
				"map_key":    nestedMap(),
				"hello":      "world",
			},
			false,
		},
		{
			"Simple",
			NewAttributeField("test"),
			map[string]any{},
			"val",
			map[string]any{
				"test": "val",
			},
			false,
		},
		{
			"OverwriteString",
			NewAttributeField("test"),
			map[string]any{
				"test": "original",
			},
			"val",
			map[string]any{
				"test": "val",
			},
			false,
		},
		{
			"NonString",
			NewAttributeField("test"),
			map[string]any{},
			123,
			map[string]any{
				"test": 123,
			},
			false,
		},
		{
			"Map",
			NewAttributeField("test"),
			map[string]any{},
			map[string]any{
				"test": 123,
			},
			map[string]any{
				"test": map[string]any{
					"test": 123,
				},
			},
			false,
		},
		{
			"NewMapValue",
			NewAttributeField(),
			map[string]any{},
			testMap(),
			testMap(),
			false,
		},
		{
			"NewRootField",
			NewAttributeField("new_key"),
			map[string]any{},
			"new_value",
			map[string]any{
				"new_key": "new_value",
			},
			false,
		},
		{
			"NewNestedField",
			NewAttributeField("new_key", "nested_key"),
			map[string]any{},
			"nested_value",
			map[string]any{
				"new_key": map[string]any{
					"nested_key": "nested_value",
				},
			},
			false,
		},
		{
			"OverwriteNestedMap",
			NewAttributeField("map_key"),
			testMap(),
			"new_value",
			map[string]any{
				"simple_key": "simple_value",
				"map_key":    "new_value",
			},
			false,
		},
		{
			"MergedNestedValue",
			NewAttributeField("map_key"),
			testMap(),
			map[string]any{
				"merged_key": "merged_value",
			},
			map[string]any{
				"simple_key": "simple_value",
				"map_key": map[string]any{
					"nested_key": "nested_value",
					"merged_key": "merged_value",
				},
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Attributes = tc.attributes
			err := entry.Set(tc.field, tc.val)
			if tc.expectedErr {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.expected, entry.Attributes)
		})
	}
}

func TestAttributeFieldParent(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		field := AttributeField{[]string{"child"}}
		require.Equal(t, AttributeField{[]string{}}, field.Parent())
	})

	t.Run("Root", func(t *testing.T) {
		field := AttributeField{[]string{}}
		require.Equal(t, AttributeField{[]string{}}, field.Parent())
	})
}

func TestAttributeFieldChild(t *testing.T) {
	field := AttributeField{[]string{"parent"}}
	require.Equal(t, AttributeField{[]string{"parent", "child"}}, field.Child("child"))
}

func TestAttributeFieldMerge(t *testing.T) {
	entry := &Entry{}
	entry.Attributes = map[string]any{"old": "values"}
	field := AttributeField{[]string{"embedded"}}
	values := map[string]any{"new": "values"}
	field.Merge(entry, values)
	expected := map[string]any{"embedded": values, "old": "values"}
	require.Equal(t, expected, entry.Attributes)
}

func TestAttributeFieldUnmarshal(t *testing.T) {
	cases := []struct {
		name    string
		jsonDot string
		keys    []string
	}{
		{
			"root",
			"attributes",
			[]string{},
		},
		{
			"standard",
			"attributes.test",
			[]string{"test"},
		},
		{
			"bracketed",
			"attributes['test.foo']",
			[]string{"test.foo"},
		},
		{
			"double_bracketed",
			"attributes['test.foo']['bar']",
			[]string{"test.foo", "bar"},
		},
		{
			"mixed",
			"attributes['test.foo'].bar",
			[]string{"test.foo", "bar"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var fy AttributeField
			decoder := yaml.NewDecoder(bytes.NewReader([]byte(tc.jsonDot)))
			decoder.KnownFields(true)
			err := decoder.Decode(&fy)
			require.NoError(t, err)
			require.Equal(t, tc.keys, fy.Keys)

			var fj AttributeField
			err = json.Unmarshal([]byte(fmt.Sprintf(`"%s"`, tc.jsonDot)), &fj)
			require.NoError(t, err)
			require.Equal(t, tc.keys, fy.Keys)
		})
	}
}

func TestAttributeFieldUnmarshalFailure(t *testing.T) {
	cases := []struct {
		name        string
		invalid     []byte
		expectedErr string
	}{
		{
			"must_be_string",
			[]byte(`{"key":"value"}`),
			"the field is not a string",
		},
		{
			"must_start_with_prefix",
			[]byte(`"test"`),
			"must start with 'attributes'",
		},
		{
			"invalid_syntax",
			[]byte(`"test['foo'"`),
			"found unclosed left bracket",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var fy AttributeField
			decoder := yaml.NewDecoder(bytes.NewReader(tc.invalid))
			decoder.KnownFields(true)
			err := decoder.Decode(&fy)
			require.ErrorContains(t, err, tc.expectedErr)

			var fj AttributeField
			err = json.Unmarshal(tc.invalid, &fj)
			require.ErrorContains(t, err, tc.expectedErr)
		})
	}
}
