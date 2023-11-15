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

func TestResourceFieldGet(t *testing.T) {
	cases := []struct {
		name       string
		field      Field
		resource   map[string]any
		expected   any
		expectedOK bool
	}{
		{
			"Uninitialized",
			NewResourceField("nonexistent"),
			nil,
			"",
			false,
		},
		{
			"RootField",
			NewResourceField(),
			testMap(),
			testMap(),
			true,
		},
		{
			"Simple",
			NewResourceField("test"),
			map[string]any{
				"test": "val",
			},
			"val",
			true,
		},
		{
			"NonexistentKey",
			NewResourceField("nonexistent"),
			map[string]any{
				"test": "val",
			},
			nil,
			false,
		},
		{
			"MapField",
			NewResourceField("map_key"),
			testMap(),
			nestedMap(),
			true,
		},
		{
			"NestedField",
			NewResourceField("map_key", "nested_key"),
			testMap(),
			"nested_value",
			true,
		},
		{
			"MissingField",
			NewResourceField("invalid"),
			testMap(),
			nil,
			false,
		},
		{
			"InvalidField",
			NewResourceField("simple_key", "nested_key"),
			testMap(),
			nil,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Resource = tc.resource
			val, ok := entry.Get(tc.field)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expected, val)
		})
	}
}

func TestResourceFieldDelete(t *testing.T) {
	cases := []struct {
		name             string
		field            Field
		resource         map[string]any
		expectedResource map[string]any
		expectedReturned any
		expectedOK       bool
	}{
		{
			"Uninitialized",
			NewResourceField("nonexistent"),
			nil,
			nil,
			"",
			false,
		},
		{
			"SimpleKey",
			NewResourceField("simple_key"),
			testMap(),
			map[string]any{
				"map_key": nestedMap(),
			},
			"simple_value",
			true,
		},
		{
			"EmptyResourceAndField",
			NewResourceField(),
			map[string]any{},
			nil,
			map[string]any{},
			true,
		},
		{
			"EmptyField",
			NewResourceField(),
			testMap(),
			nil,
			testMap(),
			true,
		},
		{
			"MissingKey",
			NewResourceField("missing_key"),
			testMap(),
			testMap(),
			nil,
			false,
		},
		{
			"NestedKey",
			NewResourceField("map_key", "nested_key"),
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
			NewResourceField("map_key"),
			testMap(),
			map[string]any{
				"simple_key": "simple_value",
			},
			nestedMap(),
			true,
		},
		{
			"InvalidNestedKey",
			NewResourceField("simple_key", "missing"),
			testMap(),
			testMap(),
			nil,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Resource = tc.resource
			val, ok := entry.Delete(tc.field)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expectedReturned, val)
			assert.Equal(t, tc.expectedResource, entry.Resource)
		})
	}
}

func TestResourceFieldSet(t *testing.T) {
	cases := []struct {
		name        string
		field       Field
		resource    map[string]any
		val         any
		expected    map[string]any
		expectedErr bool
	}{
		{
			"Uninitialized",
			NewResourceField("test"),
			nil,
			"val",
			map[string]any{
				"test": "val",
			},
			false,
		},
		{
			"OverwriteRoot",
			NewResourceField(),
			testMap(),
			"val",
			testMap(),
			true,
		},
		{
			"OverwriteRootWithMap",
			NewResourceField(),
			map[string]any{},
			testMap(),
			testMap(),
			false,
		},
		{
			"MergeOverRoot",
			NewResourceField(),
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
			NewResourceField("test"),
			map[string]any{},
			"val",
			map[string]any{
				"test": "val",
			},
			false,
		},
		{
			"OverwriteString",
			NewResourceField("test"),
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
			NewResourceField("test"),
			map[string]any{},
			123,
			map[string]any{
				"test": 123,
			},
			false,
		},
		{
			"Map",
			NewResourceField("test"),
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
			NewResourceField(),
			map[string]any{},
			testMap(),
			testMap(),
			false,
		},
		{
			"NewRootField",
			NewResourceField("new_key"),
			map[string]any{},
			"new_value",
			map[string]any{
				"new_key": "new_value",
			},
			false,
		},
		{
			"NewNestedField",
			NewResourceField("new_key", "nested_key"),
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
			NewResourceField("map_key"),
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
			NewResourceField("map_key"),
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
			entry.Resource = tc.resource
			err := entry.Set(tc.field, tc.val)
			if tc.expectedErr {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.expected, entry.Resource)
		})
	}
}

func TestResourceFieldParent(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		field := ResourceField{[]string{"child"}}
		require.Equal(t, ResourceField{[]string{}}, field.Parent())
	})

	t.Run("Root", func(t *testing.T) {
		field := ResourceField{[]string{}}
		require.Equal(t, ResourceField{[]string{}}, field.Parent())
	})
}

func TestResourceFieldChild(t *testing.T) {
	field := ResourceField{[]string{"parent"}}
	require.Equal(t, ResourceField{[]string{"parent", "child"}}, field.Child("child"))
}

func TestResourceFieldMerge(t *testing.T) {
	entry := &Entry{}
	entry.Resource = map[string]any{"old": "values"}
	field := ResourceField{[]string{"embedded"}}
	values := map[string]any{"new": "values"}
	field.Merge(entry, values)
	expected := map[string]any{"embedded": values, "old": "values"}
	require.Equal(t, expected, entry.Resource)
}

func TestResourceFieldUnmarshal(t *testing.T) {
	cases := []struct {
		name    string
		jsonDot string
		keys    []string
	}{
		{
			"root",
			"resource",
			[]string{},
		},
		{
			"standard",
			"resource.test",
			[]string{"test"},
		},
		{
			"bracketed",
			"resource['test.foo']",
			[]string{"test.foo"},
		},
		{
			"double_bracketed",
			"resource['test.foo']['bar']",
			[]string{"test.foo", "bar"},
		},
		{
			"mixed",
			"resource['test.foo'].bar",
			[]string{"test.foo", "bar"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var fy ResourceField
			err := yaml.UnmarshalStrict([]byte(tc.jsonDot), &fy)
			require.NoError(t, err)
			require.Equal(t, tc.keys, fy.Keys)

			var fj ResourceField
			err = json.Unmarshal([]byte(fmt.Sprintf(`"%s"`, tc.jsonDot)), &fj)
			require.NoError(t, err)
			require.Equal(t, tc.keys, fy.Keys)
		})
	}
}

func TestResourceFieldUnmarshalFailure(t *testing.T) {
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
			"must start with 'resource'",
		},
		{
			"invalid_syntax",
			[]byte(`"test['foo'"`),
			"found unclosed left bracket",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var fy ResourceField
			err := yaml.UnmarshalStrict(tc.invalid, &fy)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)

			var fj ResourceField
			err = json.Unmarshal(tc.invalid, &fj)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}
