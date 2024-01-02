// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadFieldsWithDefaults(t *testing.T) {
	defaults := map[string]any{
		"one":           "1",
		"two":           int64(2),
		"three":         uint64(3),
		"four":          true,
		"duration":      "42ns",
		"name":          "squashed",
		"person_ptr":    "foo",
		"person_struct": "bar",
	}
	s := testStruct{
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
	testReadFields(t, s, defaults)
}

func TestReadFieldsWithoutDefaults(t *testing.T) {
	testReadFields(t, testStruct{}, map[string]any{
		"one":           "",
		"three":         uint64(0),
		"four":          false,
		"name":          "",
		"person_ptr":    "",
		"person_struct": "",
	})
}

func testReadFields(t *testing.T, s testStruct, defaults map[string]any) {
	root, _ := ReadFields(
		reflect.ValueOf(s),
		testDR(),
	)

	assert.Equal(t, "testStruct comment\n", root.Doc)

	assert.Equal(t, "configschema.testStruct", root.Type)

	assert.Equal(t, 11, len(root.Fields))

	assert.Equal(t, &Field{
		Name:    "one",
		Kind:    "string",
		Default: defaults["one"],
	}, getFieldByName(root.Fields, "one"))

	assert.Equal(t, &Field{
		Name:    "two",
		Kind:    "int",
		Default: defaults["two"],
	}, getFieldByName(root.Fields, "two"))

	assert.Equal(t, &Field{
		Name:    "three",
		Kind:    "uint",
		Default: defaults["three"],
	}, getFieldByName(root.Fields, "three"))

	assert.Equal(t, &Field{
		Name:    "four",
		Kind:    "bool",
		Default: defaults["four"],
	}, getFieldByName(root.Fields, "four"))

	assert.Equal(t, &Field{
		Name:    "duration",
		Type:    "time.Duration",
		Kind:    "int64",
		Default: defaults["duration"],
		Doc:     "embedded, package qualified comment\n",
	}, getFieldByName(root.Fields, "duration"))

	assert.Equal(t, &Field{
		Name:    "name",
		Kind:    "string",
		Default: defaults["name"],
	}, getFieldByName(root.Fields, "name"))

	personPtr := getFieldByName(root.Fields, "person_ptr")
	assert.Equal(t, "*configschema.testPerson", personPtr.Type)
	assert.Equal(t, "ptr", personPtr.Kind)
	assert.Equal(t, 1, len(personPtr.Fields))
	assert.Equal(t, &Field{
		Name:    "name",
		Kind:    "string",
		Default: defaults["person_ptr"],
	}, getFieldByName(personPtr.Fields, "name"))

	personStruct := getFieldByName(root.Fields, "person_struct")
	assert.Equal(t, "configschema.testPerson", personStruct.Type)
	assert.Equal(t, "struct", personStruct.Kind)
	assert.Equal(t, 1, len(personStruct.Fields))
	assert.Equal(t, &Field{
		Name:    "name",
		Kind:    "string",
		Default: defaults["person_struct"],
	}, getFieldByName(personStruct.Fields, "name"))

	persons := getFieldByName(root.Fields, "persons")
	assert.Equal(t, "[]configschema.testPerson", persons.Type)
	assert.Equal(t, "slice", persons.Kind)
	assert.Equal(t, 1, len(persons.Fields))
	assert.Equal(t, &Field{
		Name:    "name",
		Kind:    "string",
		Default: "",
	}, getFieldByName(persons.Fields, "name"))

	personPtrs := getFieldByName(root.Fields, "person_ptrs")
	assert.Equal(t, "[]*configschema.testPerson", personPtrs.Type)
	assert.Equal(t, "slice", personPtrs.Kind)
	assert.Equal(t, 1, len(personPtrs.Fields))
	assert.Equal(t, &Field{
		Name:    "name",
		Kind:    "string",
		Default: "",
	}, getFieldByName(personPtrs.Fields, "name"))
}

func getFieldByName(fields []*Field, name string) *Field {
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}
