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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func testRecord() map[string]interface{} {
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

func TestRecordFieldGet(t *testing.T) {
	cases := []struct {
		name        string
		field       Field
		record      interface{}
		expectedVal interface{}
		expectedOk  bool
	}{
		{
			"EmptyField",
			NewRecordField(),
			testRecord(),
			testRecord(),
			true,
		},
		{
			"SimpleField",
			NewRecordField("simple_key"),
			testRecord(),
			"simple_value",
			true,
		},
		{
			"MapField",
			NewRecordField("map_key"),
			testRecord(),
			nestedMap(),
			true,
		},
		{
			"NestedField",
			NewRecordField("map_key", "nested_key"),
			testRecord(),
			"nested_value",
			true,
		},
		{
			"MissingField",
			NewRecordField("invalid"),
			testRecord(),
			nil,
			false,
		},
		{
			"InvalidField",
			NewRecordField("simple_key", "nested_key"),
			testRecord(),
			nil,
			false,
		},
		{
			"RawField",
			NewRecordField(),
			"raw string",
			"raw string",
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Record = tc.record

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

func TestRecordFieldDelete(t *testing.T) {
	cases := []struct {
		name             string
		field            Field
		record           interface{}
		expectedRecord   interface{}
		expectedReturned interface{}
		expectedOk       bool
	}{
		{
			"SimpleKey",
			NewRecordField("simple_key"),
			testRecord(),
			map[string]interface{}{
				"map_key": nestedMap(),
			},
			"simple_value",
			true,
		},
		{
			"EmptyRecordAndField",
			NewRecordField(),
			map[string]interface{}{},
			nil,
			map[string]interface{}{},
			true,
		},
		{
			"EmptyField",
			NewRecordField(),
			testRecord(),
			nil,
			testRecord(),
			true,
		},
		{
			"MissingKey",
			NewRecordField("missing_key"),
			testRecord(),
			testRecord(),
			nil,
			false,
		},
		{
			"NestedKey",
			NewRecordField("map_key", "nested_key"),
			testRecord(),
			map[string]interface{}{
				"simple_key": "simple_value",
				"map_key":    map[string]interface{}{},
			},
			"nested_value",
			true,
		},
		{
			"MapKey",
			NewRecordField("map_key"),
			testRecord(),
			map[string]interface{}{
				"simple_key": "simple_value",
			},
			nestedMap(),
			true,
		},
		{
			"InvalidNestedKey",
			NewRecordField("simple_key", "missing"),
			testRecord(),
			testRecord(),
			nil,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Record = tc.record

			entry.Delete(tc.field)
			assert.Equal(t, tc.expectedRecord, entry.Record)
		})
	}
}

func TestRecordFieldSet(t *testing.T) {
	cases := []struct {
		name        string
		field       Field
		record      interface{}
		setTo       interface{}
		expectedVal interface{}
	}{
		{
			"OverwriteMap",
			NewRecordField(),
			testRecord(),
			"new_value",
			"new_value",
		},
		{
			"OverwriteRaw",
			NewRecordField(),
			"raw_value",
			"new_value",
			"new_value",
		},
		{
			"OverwriteRawWithMap",
			NewRecordField("embedded", "field"),
			"raw_value",
			"new_value",
			map[string]interface{}{"embedded": map[string]interface{}{"field": "new_value"}},
		},
		{
			"NewMapValue",
			NewRecordField(),
			map[string]interface{}{},
			testRecord(),
			testRecord(),
		},
		{
			"NewRootField",
			NewRecordField("new_key"),
			map[string]interface{}{},
			"new_value",
			map[string]interface{}{"new_key": "new_value"},
		},
		{
			"NewNestedField",
			NewRecordField("new_key", "nested_key"),
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
			NewRecordField("map_key"),
			testRecord(),
			"new_value",
			map[string]interface{}{
				"simple_key": "simple_value",
				"map_key":    "new_value",
			},
		},
		{
			"MergedNestedValue",
			NewRecordField("map_key"),
			testRecord(),
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
			entry.Record = tc.record
			require.NoError(t, entry.Set(tc.field, tc.setTo))
			assert.Equal(t, tc.expectedVal, entry.Record)
		})
	}
}

func TestRecordFieldParent(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		field := RecordField{[]string{"child"}}
		require.Equal(t, RecordField{[]string{}}, field.Parent())
	})

	t.Run("Root", func(t *testing.T) {
		field := RecordField{[]string{}}
		require.Equal(t, RecordField{[]string{}}, field.Parent())
	})
}

func TestRecordFieldChild(t *testing.T) {
	field := RecordField{[]string{"parent"}}
	require.Equal(t, RecordField{[]string{"parent", "child"}}, field.Child("child"))
}

func TestRecordFieldMerge(t *testing.T) {
	entry := &Entry{}
	entry.Record = "raw_value"
	field := RecordField{[]string{"embedded"}}
	values := map[string]interface{}{"new": "values"}
	field.Merge(entry, values)
	expected := map[string]interface{}{"embedded": values}
	require.Equal(t, expected, entry.Record)
}

func TestRecordFieldMarshalJSON(t *testing.T) {
	recordField := RecordField{Keys: []string{"test"}}
	json, err := recordField.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, []byte(`"test"`), json)
}

func TestRecordFieldUnmarshalJSON(t *testing.T) {
	fieldString := []byte(`"test"`)
	var f RecordField
	err := json.Unmarshal(fieldString, &f)
	require.NoError(t, err)
	require.Equal(t, RecordField{Keys: []string{"test"}}, f)
}

func TestRecordFieldUnmarshalJSONFailure(t *testing.T) {
	invalidField := []byte(`{"key":"value"}`)
	var f RecordField
	err := json.Unmarshal(invalidField, &f)
	require.Error(t, err)
	require.Contains(t, err.Error(), "the field is not a string: json")
}

func TestRecordFieldMarshalYAML(t *testing.T) {
	recordField := RecordField{Keys: []string{"test"}}
	yaml, err := recordField.MarshalYAML()
	require.NoError(t, err)
	require.Equal(t, "test", yaml)
}

func TestRecordFieldUnmarshalYAML(t *testing.T) {
	invalidField := []byte("test")
	var f RecordField
	err := yaml.UnmarshalStrict(invalidField, &f)
	require.NoError(t, err)
	require.Equal(t, RecordField{Keys: []string{"test"}}, f)
}

func TestRecordFieldUnmarshalYAMLFailure(t *testing.T) {
	invalidField := []byte(`{"key":"value"}`)
	var f RecordField
	err := yaml.UnmarshalStrict(invalidField, &f)
	require.Error(t, err)
	require.Contains(t, err.Error(), "the field is not a string: yaml")
}

func TestRecordFieldFromJSONDot(t *testing.T) {
	jsonDot := "$.test"
	recordField := fromJSONDot(jsonDot)
	expectedField := RecordField{Keys: []string{"test"}}
	require.Equal(t, expectedField, recordField)
}
