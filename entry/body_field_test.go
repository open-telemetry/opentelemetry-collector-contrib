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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func testBody() map[string]interface{} {
	return map[string]interface{}{
		"simple_key": "simple_value",
		"map_key":    nestedMap(),
	}
}

func nestedMap() map[string]interface{} {
	return map[string]interface{}{
		"nested_key": "nested_value",
	}
}

func TestNewBodyFieldGet(t *testing.T) {
	cases := []struct {
		name        string
		field       Field
		body        interface{}
		expectedVal interface{}
		expectedOk  bool
	}{
		{
			"EmptyField",
			NewBodyField(),
			testBody(),
			testBody(),
			true,
		},
		{
			"SimpleField",
			NewBodyField("simple_key"),
			testBody(),
			"simple_value",
			true,
		},
		{
			"MapField",
			NewBodyField("map_key"),
			testBody(),
			nestedMap(),
			true,
		},
		{
			"NestedField",
			NewBodyField("map_key", "nested_key"),
			testBody(),
			"nested_value",
			true,
		},
		{
			"MissingField",
			NewBodyField("invalid"),
			testBody(),
			nil,
			false,
		},
		{
			"InvalidField",
			NewBodyField("simple_key", "nested_key"),
			testBody(),
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
		body             interface{}
		expectedBody     interface{}
		expectedReturned interface{}
		expectedOk       bool
	}{
		{
			"SimpleKey",
			NewBodyField("simple_key"),
			testBody(),
			map[string]interface{}{
				"map_key": nestedMap(),
			},
			"simple_value",
			true,
		},
		{
			"EmptyBodyAndField",
			NewBodyField(),
			map[string]interface{}{},
			nil,
			map[string]interface{}{},
			true,
		},
		{
			"EmptyField",
			NewBodyField(),
			testBody(),
			nil,
			testBody(),
			true,
		},
		{
			"MissingKey",
			NewBodyField("missing_key"),
			testBody(),
			testBody(),
			nil,
			false,
		},
		{
			"NestedKey",
			NewBodyField("map_key", "nested_key"),
			testBody(),
			map[string]interface{}{
				"simple_key": "simple_value",
				"map_key":    map[string]interface{}{},
			},
			"nested_value",
			true,
		},
		{
			"MapKey",
			NewBodyField("map_key"),
			testBody(),
			map[string]interface{}{
				"simple_key": "simple_value",
			},
			nestedMap(),
			true,
		},
		{
			"InvalidNestedKey",
			NewBodyField("simple_key", "missing"),
			testBody(),
			testBody(),
			nil,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Body = tc.body
			entry.Delete(tc.field)
			assert.Equal(t, tc.expectedBody, entry.Body)
		})
	}
}

func TestBodyFieldSet(t *testing.T) {
	cases := []struct {
		name        string
		field       Field
		body        interface{}
		setTo       interface{}
		expectedVal interface{}
	}{
		{
			"OverwriteMap",
			NewBodyField(),
			testBody(),
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
			map[string]interface{}{"embedded": map[string]interface{}{"field": "new_value"}},
		},
		{
			"NewMapValue",
			NewBodyField(),
			map[string]interface{}{},
			testBody(),
			testBody(),
		},
		{
			"NewRootField",
			NewBodyField("new_key"),
			map[string]interface{}{},
			"new_value",
			map[string]interface{}{"new_key": "new_value"},
		},
		{
			"NewNestedField",
			NewBodyField("new_key", "nested_key"),
			map[string]interface{}{},
			"nested_value",
			map[string]interface{}{
				"new_key": map[string]interface{}{
					"nested_key": "nested_value",
				},
			},
		},
		{
			"OverwriteNestedMap",
			NewBodyField("map_key"),
			testBody(),
			"new_value",
			map[string]interface{}{
				"simple_key": "simple_value",
				"map_key":    "new_value",
			},
		},
		{
			"MergedNestedValue",
			NewBodyField("map_key"),
			testBody(),
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
	values := map[string]interface{}{"new": "values"}
	field.Merge(entry, values)
	expected := map[string]interface{}{"embedded": values}
	require.Equal(t, expected, entry.Body)
}

// TODO add more test cases
// 1. fields with dots `body["file.name"]`
// 2. fields with deprecated "$body" or "$." prefix
func TestBodyFieldMarshal(t *testing.T) {
	cases := []struct {
		name    string
		keys    []string
		jsonDot string
	}{
		{
			"standard",
			[]string{"test"},
			"body.test",
		},
		{
			"bracketed",
			[]string{"test.foo"},
			"body['test.foo']",
		},
		{
			"double_bracketed",
			[]string{"test.foo", "bar"},
			"body['test.foo']['bar']",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field := BodyField{Keys: tc.keys}
			yaml, err := field.MarshalYAML()
			require.NoError(t, err)
			require.Equal(t, tc.jsonDot, yaml)

			json, err := field.MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, []byte(fmt.Sprintf(`"%s"`, tc.jsonDot)), json)
		})
	}
}

func TestBodyFieldUnmarshal(t *testing.T) {
	cases := []struct {
		name    string
		jsonDot string
		keys    []string
	}{
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
		{
			"deprecated_prefix",
			"$body.test",
			[]string{"test"},
		},
		{
			"deprecated_prefix_bracketed",
			"$body['test.foo']",
			[]string{"test.foo"},
		},
		{
			"deprecated_shorthand",
			"$.test",
			[]string{"test"},
		},
		{
			"deprecated_shorthand_bracketed",
			"$['test.foo']",
			[]string{"test.foo"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var fy BodyField
			err := yaml.UnmarshalStrict([]byte(tc.jsonDot), &fy)
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
			err := yaml.UnmarshalStrict(tc.invalid, &fy)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)

			var fj BodyField
			err = json.Unmarshal(tc.invalid, &fj)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}
