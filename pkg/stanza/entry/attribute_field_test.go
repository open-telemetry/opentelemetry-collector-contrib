// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestAttributeFieldGet(t *testing.T) {
	cases := []struct {
		name       string
		field      Field
		attributes map[string]interface{}
		expected   interface{}
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
			map[string]interface{}{
				"test": "val",
			},
			"val",
			true,
		},
		{
			"NonexistentKey",
			NewAttributeField("nonexistent"),
			map[string]interface{}{
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
		attributes         map[string]interface{}
		expectedAttributes map[string]interface{}
		expectedReturned   interface{}
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
			map[string]interface{}{
				"map_key": nestedMap(),
			},
			"simple_value",
			true,
		},
		{
			"EmptyAttributesAndField",
			NewAttributeField(),
			map[string]interface{}{},
			nil,
			map[string]interface{}{},
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
			map[string]interface{}{
				"simple_key": "simple_value",
				"map_key":    map[string]interface{}{},
			},
			"nested_value",
			true,
		},
		{
			"MapKey",
			NewAttributeField("map_key"),
			testMap(),
			map[string]interface{}{
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
		attributes  map[string]interface{}
		val         interface{}
		expected    map[string]interface{}
		expectedErr bool
	}{
		{
			"Uninitialized",
			NewAttributeField("test"),
			nil,
			"val",
			map[string]interface{}{
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
			map[string]interface{}{},
			testMap(),
			testMap(),
			false,
		},
		{
			"MergeOverRoot",
			NewAttributeField(),
			map[string]interface{}{
				"simple_key": "clobbered",
				"hello":      "world",
			},
			testMap(),
			map[string]interface{}{
				"simple_key": "simple_value",
				"map_key":    nestedMap(),
				"hello":      "world",
			},
			false,
		},
		{
			"Simple",
			NewAttributeField("test"),
			map[string]interface{}{},
			"val",
			map[string]interface{}{
				"test": "val",
			},
			false,
		},
		{
			"OverwriteString",
			NewAttributeField("test"),
			map[string]interface{}{
				"test": "original",
			},
			"val",
			map[string]interface{}{
				"test": "val",
			},
			false,
		},
		{
			"NonString",
			NewAttributeField("test"),
			map[string]interface{}{},
			123,
			map[string]interface{}{
				"test": 123,
			},
			false,
		},
		{
			"Map",
			NewAttributeField("test"),
			map[string]interface{}{},
			map[string]interface{}{
				"test": 123,
			},
			map[string]interface{}{
				"test": map[string]interface{}{
					"test": 123,
				},
			},
			false,
		},
		{
			"NewMapValue",
			NewAttributeField(),
			map[string]interface{}{},
			testMap(),
			testMap(),
			false,
		},
		{
			"NewRootField",
			NewAttributeField("new_key"),
			map[string]interface{}{},
			"new_value",
			map[string]interface{}{
				"new_key": "new_value",
			},
			false,
		},
		{
			"NewNestedField",
			NewAttributeField("new_key", "nested_key"),
			map[string]interface{}{},
			"nested_value",
			map[string]interface{}{
				"new_key": map[string]interface{}{
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
			map[string]interface{}{
				"simple_key": "simple_value",
				"map_key":    "new_value",
			},
			false,
		},
		{
			"MergedNestedValue",
			NewAttributeField("map_key"),
			testMap(),
			map[string]interface{}{
				"merged_key": "merged_value",
			},
			map[string]interface{}{
				"simple_key": "simple_value",
				"map_key": map[string]interface{}{
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
	entry.Attributes = map[string]interface{}{"old": "values"}
	field := AttributeField{[]string{"embedded"}}
	values := map[string]interface{}{"new": "values"}
	field.Merge(entry, values)
	expected := map[string]interface{}{"embedded": values, "old": "values"}
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
			err := yaml.UnmarshalStrict([]byte(tc.jsonDot), &fy)
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
			err := yaml.UnmarshalStrict(tc.invalid, &fy)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)

			var fj AttributeField
			err = json.Unmarshal(tc.invalid, &fj)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}
