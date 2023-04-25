// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configschema

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFields_Config(t *testing.T) {
	fields, err := readFields(
		reflect.ValueOf(testConfig{Name: "foo"}),
		nopCommentReader,
	)
	require.NoError(t, err)
	assert.Equal(t, "configschema.testConfig", fields.Type)
	nameField := fields.CfgFields[0]
	assert.Equal(t, "name", nameField.Name)
	assert.Equal(t, "string", nameField.Kind)
	assert.Equal(t, "foo", nameField.Default)
}

func TestReadFields_TestStructWithDefaults(t *testing.T) {
	ts := testStruct{
		One:      "1",
		Two:      2,
		Three:    3,
		Four:     true,
		Duration: 42,
		Squashed: testPerson{"squashed"},
		PersonPtr: &testPerson{
			Name: "foo",
		},
		PersonStruct: testPerson{
			Name: "bar",
		},
	}
	testReadFields(t, ts, map[string]any{
		"one":           "1",
		"two":           int64(2),
		"three":         uint64(3),
		"four":          true,
		"duration":      "42ns",
		"name":          "squashed",
		"person_ptr":    "foo",
		"person_struct": "bar",
	})
}

func TestReadFields_TestStructWithoutDefaults(t *testing.T) {
	testReadFields(t, testStruct{}, map[string]interface{}{
		"one":           "",
		"three":         uint64(0),
		"four":          false,
		"name":          "",
		"person_ptr":    "",
		"person_struct": "",
	})
}

func testReadFields(t *testing.T, ts testStruct, expectedDefaults map[string]any) {
	root, err := readFields(reflect.ValueOf(ts), testCommentReader())
	require.NoError(t, err)
	assert.Equal(t, "testStruct comment\n", root.Doc)
	assert.Equal(t, "configschema.testStruct", root.Type)

	assert.Equal(t, 11, len(root.CfgFields))

	assert.Equal(t, &cfgField{
		Name:    "one",
		Kind:    "string",
		Default: expectedDefaults["one"],
	}, getFieldByName(root.CfgFields, "one"))

	/*
		assert.Equal(t, &cfgField{
			Name:    "two",
			Kind:    "int",
			Default: defaults["two"],
		}, getFieldByName(root.CfgFields, "two"))

		assert.Equal(t, &cfgField{
			Name:    "three",
			Kind:    "uint",
			Default: defaults["three"],
		}, getFieldByName(root.CfgFields, "three"))

		assert.Equal(t, &cfgField{
			Name:    "four",
			Kind:    "bool",
			Default: defaults["four"],
		}, getFieldByName(root.CfgFields, "four"))

		assert.Equal(t, &cfgField{
			Name:    "duration",
			Type:    "time.Duration",
			Kind:    "int64",
			Default: defaults["duration"],
			Doc:     "embedded, package qualified comment\n",
		}, getFieldByName(root.CfgFields, "duration"))

		assert.Equal(t, &cfgField{
			Name:    "name",
			Kind:    "string",
			Default: defaults["name"],
		}, getFieldByName(root.CfgFields, "name"))

		personPtr := getFieldByName(root.CfgFields, "person_ptr")
		assert.Equal(t, "*configschema.testPerson", personPtr.Type)
		assert.Equal(t, "ptr", personPtr.Kind)
		assert.Equal(t, 1, len(personPtr.CfgFields))
		assert.Equal(t, &cfgField{
			Name:    "name",
			Kind:    "string",
			Default: defaults["person_ptr"],
		}, getFieldByName(personPtr.CfgFields, "name"))

		personStruct := getFieldByName(root.CfgFields, "person_struct")
		assert.Equal(t, "configschema.testPerson", personStruct.Type)
		assert.Equal(t, "struct", personStruct.Kind)
		assert.Equal(t, 1, len(personStruct.CfgFields))
		assert.Equal(t, &cfgField{
			Name:    "name",
			Kind:    "string",
			Default: defaults["person_struct"],
		}, getFieldByName(personStruct.CfgFields, "name"))

		persons := getFieldByName(root.CfgFields, "persons")
		assert.Equal(t, "[]configschema.testPerson", persons.Type)
		assert.Equal(t, "slice", persons.Kind)
		assert.Equal(t, 1, len(persons.CfgFields))
		assert.Equal(t, &cfgField{
			Name:    "name",
			Kind:    "string",
			Default: "",
		}, getFieldByName(persons.CfgFields, "name"))

		personPtrs := getFieldByName(root.CfgFields, "person_ptrs")
		assert.Equal(t, "[]*configschema.testPerson", personPtrs.Type)
		assert.Equal(t, "slice", personPtrs.Kind)
		assert.Equal(t, 1, len(personPtrs.CfgFields))
		assert.Equal(t, &cfgField{
			Name:    "name",
			Kind:    "string",
			Default: "",
		}, getFieldByName(personPtrs.CfgFields, "name"))

		tls := getFieldByName(root.CfgFields, "tls")
		assert.NotEmpty(t, tls.Doc)
		caFile := getFieldByName(tls.CfgFields, "ca_file")
		assert.NotEmpty(t, caFile.Doc)*/
}

func nopCommentReader(reflect.Value) (map[string]string, error) {
	return nil, nil
}

func testCommentReader() commentReaderFunc {
	return commentReader{
		dr: testDR(),
	}.commentsForStruct
}

func getFieldByName(fields []*cfgField, name string) *cfgField {
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}
