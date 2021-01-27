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

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestFieldUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name     string
		input    []byte
		expected Field
	}{
		{
			"SimpleField",
			[]byte(`"test1"`),
			NewRecordField("test1"),
		},
		{
			"ComplexField",
			[]byte(`"test1.test2"`),
			NewRecordField("test1", "test2"),
		},
		{
			"RootField",
			[]byte(`"$"`),
			NewRecordField([]string{}...),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f Field
			err := json.Unmarshal(tc.input, &f)
			require.NoError(t, err)

			require.Equal(t, tc.expected, f)
		})
	}
}

func TestFieldUnmarshalJSONFailure(t *testing.T) {
	invalidField := []byte(`{"key":"value"}`)
	var f Field
	err := json.Unmarshal(invalidField, &f)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot unmarshal object into Go value of type string")
}

func TestFieldMarshalJSON(t *testing.T) {
	cases := []struct {
		name     string
		input    Field
		expected []byte
	}{
		{
			"SimpleField",
			NewRecordField("test1"),
			[]byte(`"test1"`),
		},
		{
			"ComplexField",
			NewRecordField("test1", "test2"),
			[]byte(`"test1.test2"`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := json.Marshal(tc.input)
			require.NoError(t, err)

			require.Equal(t, tc.expected, res)
		})
	}
}

func TestFieldUnmarshalYAML(t *testing.T) {
	cases := []struct {
		name     string
		input    []byte
		expected Field
	}{
		{
			"SimpleField",
			[]byte(`"test1"`),
			NewRecordField("test1"),
		},
		{
			"UnquotedField",
			[]byte(`test1`),
			NewRecordField("test1"),
		},
		{
			"RootField",
			[]byte(`"$"`),
			NewRecordField([]string{}...),
		},
		{
			"ComplexField",
			[]byte(`"test1.test2"`),
			NewRecordField("test1", "test2"),
		},
		{
			"ComplexFieldWithRoot",
			[]byte(`"$.test1.test2"`),
			NewRecordField("test1", "test2"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f Field
			err := yaml.UnmarshalStrict(tc.input, &f)
			require.NoError(t, err)

			require.Equal(t, tc.expected, f)
		})
	}
}

func TestFieldUnmarshalYAMLFailure(t *testing.T) {
	invalidField := []byte(`invalid: field`)
	var f Field
	err := yaml.UnmarshalStrict(invalidField, &f)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot unmarshal !!map into string")
}

func TestFieldMarshalYAML(t *testing.T) {
	cases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			"SimpleField",
			NewRecordField("test1"),
			"test1\n",
		},
		{
			"ComplexField",
			NewRecordField("test1", "test2"),
			"test1.test2\n",
		},
		{
			"EmptyField",
			NewRecordField(),
			"$record\n",
		},
		{
			"FieldWithDots",
			NewRecordField("test.1"),
			"$record['test.1']\n",
		},
		{
			"FieldWithDotsThenNone",
			NewRecordField("test.1", "test2"),
			"$record['test.1']['test2']\n",
		},
		{
			"FieldWithNoDotsThenDots",
			NewRecordField("test1", "test.2"),
			"$record['test1']['test.2']\n",
		},
		{
			"LabelField",
			NewLabelField("test1"),
			"$labels.test1\n",
		},
		{
			"LabelFieldWithDots",
			NewLabelField("test.1"),
			"$labels['test.1']\n",
		},
		{
			"ResourceField",
			NewResourceField("test1"),
			"$resource.test1\n",
		},
		{
			"ResourceFieldWithDots",
			NewResourceField("test.1"),
			"$resource['test.1']\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := yaml.Marshal(tc.input)
			require.NoError(t, err)

			require.Equal(t, tc.expected, string(res))
		})
	}
}

func TestSplitField(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		output    []string
		expectErr bool
	}{
		{"Simple", "test", []string{"test"}, false},
		{"Sub", "test.case", []string{"test", "case"}, false},
		{"Root", "$", []string{"$"}, false},
		{"RootWithSub", "$record.field", []string{"$record", "field"}, false},
		{"RootWithTwoSub", "$record.field1.field2", []string{"$record", "field1", "field2"}, false},
		{"BracketSyntaxSingleQuote", "['test']", []string{"test"}, false},
		{"BracketSyntaxDoubleQuote", `["test"]`, []string{"test"}, false},
		{"RootSubBracketSyntax", `$record["test"]`, []string{"$record", "test"}, false},
		{"BracketThenDot", `$record["test1"].test2`, []string{"$record", "test1", "test2"}, false},
		{"BracketThenBracket", `$record["test1"]["test2"]`, []string{"$record", "test1", "test2"}, false},
		{"DotThenBracket", `$record.test1["test2"]`, []string{"$record", "test1", "test2"}, false},
		{"DotsInBrackets", `$record["test1.test2"]`, []string{"$record", "test1.test2"}, false},
		{"UnclosedBrackets", `$record["test1.test2"`, nil, true},
		{"UnclosedQuotes", `$record["test1.test2]`, nil, true},
		{"UnmatchedQuotes", `$record["test1.test2']`, nil, true},
		{"BracketAtEnd", `$record[`, nil, true},
		{"SingleQuoteAtEnd", `$record['`, nil, true},
		{"DoubleQuoteAtEnd", `$record["`, nil, true},
		{"BracketMissingQuotes", `$record[test]`, nil, true},
		{"CharacterBetweenBracketAndQuote", `$record["test"a]`, nil, true},
		{"CharacterOutsideBracket", `$record["test"]a`, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := splitField(tc.input)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tc.output, s)
		})
	}
}

func TestFieldFromStringInvalidSplit(t *testing.T) {
	_, err := fieldFromString("$resource[test]")
	require.Error(t, err)
	require.Contains(t, err.Error(), "splitting field")
}

func TestFieldFromStringWithResource(t *testing.T) {
	field, err := fieldFromString(`$resource["test"]`)
	require.NoError(t, err)
	require.Equal(t, "$resource.test", field.String())
}

func TestFieldFromStringWithInvalidResource(t *testing.T) {
	_, err := fieldFromString(`$resource["test"]["key"]`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource fields cannot be nested")
}
