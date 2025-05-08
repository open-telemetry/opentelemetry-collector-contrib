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

func testMap() map[string]any {
	return map[string]any{
		"simple_key": "simple_value",
		"map_key":    nestedMap(),
	}
}

func nestedMap() map[string]any {
	return map[string]any{
		"nested_key": "nested_value",
	}
}

func TestNewBodyFieldGet(t *testing.T) {
	cases := []struct {
		name        string
		field       Field
		body        any
		expectedVal any
		expectedOk  bool
	}{
		{
			"EmptyField",
			NewBodyField(),
			testMap(),
			testMap(),
			true,
		},
		{
			"SimpleField",
			NewBodyField("simple_key"),
			testMap(),
			"simple_value",
			true,
		},
		{
			"MapField",
			NewBodyField("map_key"),
			testMap(),
			nestedMap(),
			true,
		},
		{
			"NestedField",
			NewBodyField("map_key", "nested_key"),
			testMap(),
			"nested_value",
			true,
		},
		{
			"MissingField",
			NewBodyField("invalid"),
			testMap(),
			nil,
			false,
		},
		{
			"InvalidField",
			NewBodyField("simple_key", "nested_key"),
			testMap(),
			nil,
			false,
		},
		{
			"RawField",
			NewBodyField(),
			"raw string",
			"raw string",
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Body = tc.body

			val, ok := entry.Get(tc.field)
			if !assert.Equal(t, tc.expectedOk, ok) {
				return
			}
			if !assert.Equal(t, tc.expectedVal, val) {
				return
			}
		})
	}
}

func TestBodyFieldDelete(t *testing.T) {
	cases := []struct {
		name             string
		field            Field
		body             any
		expectedBody     any
		expectedReturned any
		expectedOk       bool
	}{
		{
			"SimpleKey",
			NewBodyField("simple_key"),
			testMap(),
			map[string]any{
				"map_key": nestedMap(),
			},
			"simple_value",
			true,
		},
		{
			"EmptyBodyAndField",
			NewBodyField(),
			map[string]any{},
			nil,
			map[string]any{},
			true,
		},
		{
			"EmptyField",
			NewBodyField(),
			testMap(),
			nil,
			testMap(),
			true,
		},
		{
			"MissingKey",
			NewBodyField("missing_key"),
			testMap(),
			testMap(),
			nil,
			false,
		},
		{
			"NestedKey",
			NewBodyField("map_key", "nested_key"),
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
			NewBodyField("map_key"),
			testMap(),
			map[string]any{
				"simple_key": "simple_value",
			},
			nestedMap(),
			true,
		},
		{
			"InvalidNestedKey",
			NewBodyField("simple_key", "missing"),
			testMap(),
			testMap(),
			nil,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Body = tc.body
			val, ok := entry.Delete(tc.field)
			require.Equal(t, tc.expectedOk, ok)
			require.Equal(t, tc.expectedReturned, val)
			assert.Equal(t, tc.expectedBody, entry.Body)
		})
	}
}

func TestBodyFieldSet(t *testing.T) {
	cases := []struct {
		name        string
		field       Field
		body        any
		setTo       any
		expectedVal any
	}{
		{
			"OverwriteMap",
			NewBodyField(),
			testMap(),
			"new_value",
			"new_value",
		},
		{
			"OverwriteRaw",
			NewBodyField(),
			"raw_value",
			"new_value",
			"new_value",
		},
		{
			"OverwriteRawWithMap",
			NewBodyField("embedded", "field"),
			"raw_value",
			"new_value",
			map[string]any{
				"embedded": map[string]any{
					"field": "new_value",
				},
			},
		},
		{
			"NewMapValue",
			NewBodyField(),
			map[string]any{},
			testMap(),
			testMap(),
		},
		{
			"NewRootField",
			NewBodyField("new_key"),
			map[string]any{},
			"new_value",
			map[string]any{
				"new_key": "new_value",
			},
		},
		{
			"NewNestedField",
			NewBodyField("new_key", "nested_key"),
			map[string]any{},
			"nested_value",
			map[string]any{
				"new_key": map[string]any{
					"nested_key": "nested_value",
				},
			},
		},
		{
			"OverwriteNestedMap",
			NewBodyField("map_key"),
			testMap(),
			"new_value",
			map[string]any{
				"simple_key": "simple_value",
				"map_key":    "new_value",
			},
		},
		{
			"MergedNestedValue",
			NewBodyField("map_key"),
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
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Body = tc.body
			require.NoError(t, entry.Set(tc.field, tc.setTo))
			assert.Equal(t, tc.expectedVal, entry.Body)
		})
	}
}

func TestBodyFieldParent(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		field := BodyField{[]string{"child"}}
		require.Equal(t, BodyField{[]string{}}, field.Parent())
	})

	t.Run("Root", func(t *testing.T) {
		field := BodyField{[]string{}}
		require.Equal(t, BodyField{[]string{}}, field.Parent())
	})
}

func TestBodyFieldChild(t *testing.T) {
	field := BodyField{[]string{"parent"}}
	require.Equal(t, BodyField{[]string{"parent", "child"}}, field.Child("child"))
}

func TestBodyFieldMerge(t *testing.T) {
	entry := &Entry{}
	entry.Body = "raw_value"
	field := BodyField{[]string{"embedded"}}
	values := map[string]any{"new": "values"}
	field.Merge(entry, values)
	expected := map[string]any{"embedded": values}
	require.Equal(t, expected, entry.Body)
}

func TestBodyFieldUnmarshal(t *testing.T) {
	cases := []struct {
		name    string
		jsonDot string
		keys    []string
	}{
		{
			"root",
			"body",
			[]string{},
		},
		{
			"standard",
			"body.test",
			[]string{"test"},
		},
		{
			"bracketed",
			"body['test.foo']",
			[]string{"test.foo"},
		},
		{
			"double_bracketed",
			"body['test.foo']['bar']",
			[]string{"test.foo", "bar"},
		},
		{
			"mixed",
			"body['test.foo'].bar",
			[]string{"test.foo", "bar"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var fy BodyField
			decoder := yaml.NewDecoder(bytes.NewReader([]byte(tc.jsonDot)))
			decoder.KnownFields(true)
			err := decoder.Decode(&fy)
			require.NoError(t, err)
			require.Equal(t, tc.keys, fy.Keys)

			var fj BodyField
			err = json.Unmarshal([]byte(fmt.Sprintf(`"%s"`, tc.jsonDot)), &fj)
			require.NoError(t, err)
			require.Equal(t, tc.keys, fy.Keys)
		})
	}
}

func TestBodyFieldUnmarshalFailure(t *testing.T) {
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
			"must start with 'body'",
		},
		{
			"invalid_syntax",
			[]byte(`"test['foo'"`),
			"found unclosed left bracket",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var fy BodyField
			decoder := yaml.NewDecoder(bytes.NewReader(tc.invalid))
			decoder.KnownFields(true)
			err := decoder.Decode(&fy)
			require.ErrorContains(t, err, tc.expectedErr)

			var fj BodyField
			err = json.Unmarshal(tc.invalid, &fj)
			require.ErrorContains(t, err, tc.expectedErr)
		})
	}
}
