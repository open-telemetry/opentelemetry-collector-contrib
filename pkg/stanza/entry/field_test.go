// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestFieldUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name                string
		input               []byte
		expected            Field
		expectedErr         string
		expectedErrRootable string
	}{
		{
			name:     "BodyLong",
			input:    []byte(`"body"`),
			expected: NewBodyField(),
		},
		{
			name:     "SimpleField",
			input:    []byte(`"body.test1"`),
			expected: NewBodyField("test1"),
		},
		{
			name:     "ComplexField",
			input:    []byte(`"body.test1.test2"`),
			expected: NewBodyField("test1", "test2"),
		},
		{
			name:     "BracketedField",
			input:    []byte(`"body.test1['file.name']"`),
			expected: NewBodyField("test1", "file.name"),
		},
		{
			name:     "DoubleBracketedField",
			input:    []byte(`"body.test1['file.details']['file.name']"`),
			expected: NewBodyField("test1", "file.details", "file.name"),
		},
		{
			name:     "PostBracketField",
			input:    []byte(`"body.test1['file.details'].name"`),
			expected: NewBodyField("test1", "file.details", "name"),
		},
		{
			name:     "AttributesSimpleField",
			input:    []byte(`"attributes.test1"`),
			expected: NewAttributeField("test1"),
		},
		{
			name:     "AttributesComplexField",
			input:    []byte(`"attributes.test1.test2"`),
			expected: NewAttributeField("test1", "test2"),
		},
		{
			name:     "AttributesBracketedField",
			input:    []byte(`"attributes.test1['file.name']"`),
			expected: NewAttributeField("test1", "file.name"),
		},
		{
			name:     "AttributesDoubleBracketedField",
			input:    []byte(`"attributes.test1['file.details']['file.name']"`),
			expected: NewAttributeField("test1", "file.details", "file.name"),
		},
		{
			name:     "AttributesPostBracketField",
			input:    []byte(`"attributes.test1['file.details'].name"`),
			expected: NewAttributeField("test1", "file.details", "name"),
		},
		{
			name:     "AttributesSimpleField",
			input:    []byte(`"attributes.test1"`),
			expected: NewAttributeField("test1"),
		},
		{
			name:     "ResourceSimpleField",
			input:    []byte(`"resource.test1"`),
			expected: NewResourceField("test1"),
		},
		{
			name:     "ResourceComplexField",
			input:    []byte(`"resource.test1.test2"`),
			expected: NewResourceField("test1", "test2"),
		},
		{
			name:     "ResourceBracketedField",
			input:    []byte(`"resource.test1['file.name']"`),
			expected: NewResourceField("test1", "file.name"),
		},
		{
			name:     "ResourceDoubleBracketedField",
			input:    []byte(`"resource.test1['file.details']['file.name']"`),
			expected: NewResourceField("test1", "file.details", "file.name"),
		},
		{
			name:     "ResourcePostBracketField",
			input:    []byte(`"resource.test1['file.details'].name"`),
			expected: NewResourceField("test1", "file.details", "name"),
		},
		{
			name:     "ResourceSimpleField",
			input:    []byte(`"resource.test1"`),
			expected: NewResourceField("test1"),
		},
		{
			name:        "AttributesRoot",
			input:       []byte(`"attributes"`),
			expectedErr: "attributes cannot be referenced without subfield",
			expected:    NewAttributeField(),
		},
		{
			name:        "ResourceRoot",
			input:       []byte(`"resource"`),
			expectedErr: "resource cannot be referenced without subfield",
			expected:    NewResourceField(),
		},
		{
			name:                "Bool",
			input:               []byte(`"bool"`),
			expectedErrRootable: "unrecognized prefix",
		},
		{
			name:                "Object",
			input:               []byte(`{"key":"value"}`),
			expectedErrRootable: "cannot unmarshal object into Go value of type string",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var field Field
			err := json.Unmarshal(tc.input, &field)

			var rootableField RootableField
			errRootable := json.Unmarshal(tc.input, &rootableField)

			switch {
			case tc.expectedErrRootable != "":
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
				require.Error(t, errRootable)
				require.Contains(t, errRootable.Error(), tc.expectedErrRootable)
			case tc.expectedErr != "":
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
				require.NoError(t, errRootable)
				require.Equal(t, tc.expected, rootableField.Field)
			default:
				require.NoError(t, err)
				require.Equal(t, tc.expected, field)
				require.NoError(t, errRootable)
				require.Equal(t, tc.expected, rootableField.Field)
			}
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
			"BodyRoot",
			[]byte(`"body"`),
			NewBodyField(),
		},
		{
			"SimpleField",
			[]byte(`"body.test1"`),
			NewBodyField("test1"),
		},
		{
			"UnquotedField",
			[]byte(`body.test1`),
			NewBodyField("test1"),
		},
		{
			"ComplexField",
			[]byte(`"body.test1.test2"`),
			NewBodyField("test1", "test2"),
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
	cases := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			"Bool",
			[]byte(`bool`),
			"unrecognized prefix",
		},
		{
			"Map",
			[]byte(`invalid: field`),
			"cannot unmarshal !!map into string",
		},
		{
			"AttributesRoot",
			[]byte(`attributes`),
			"attributes cannot be referenced without subfield",
		},
		{
			"ResourceRoot",
			[]byte(`resource`),
			"resource cannot be referenced without subfield",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f Field
			err := yaml.UnmarshalStrict(tc.input, &f)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expected)
		})
	}
}

func TestFromJSONDot(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		output    []string
		expectErr bool
	}{
		{"Simple", "test", []string{"test"}, false},
		{"Sub", "test.case", []string{"test", "case"}, false},
		{"RootWithSub", "body.field", []string{"body", "field"}, false},
		{"RootWithTwoSub", "body.field1.field2", []string{"body", "field1", "field2"}, false},
		{"BracketSyntaxSingleQuote", "['test']", []string{"test"}, false},
		{"BracketSyntaxDoubleQuote", `["test"]`, []string{"test"}, false},
		{"RootSubBracketSyntax", `body["test"]`, []string{"body", "test"}, false},
		{"BracketThenDot", `body["test1"].test2`, []string{"body", "test1", "test2"}, false},
		{"BracketThenBracket", `body["test1"]["test2"]`, []string{"body", "test1", "test2"}, false},
		{"DotThenBracket", `body.test1["test2"]`, []string{"body", "test1", "test2"}, false},
		{"DotsInBrackets", `body["test1.test2"]`, []string{"body", "test1.test2"}, false},
		{"UnclosedBrackets", `body["test1.test2"`, nil, true},
		{"UnclosedQuotes", `body["test1.test2]`, nil, true},
		{"UnmatchedQuotes", `body["test1.test2']`, nil, true},
		{"BracketAtEnd", `body[`, nil, true},
		{"SingleQuoteAtEnd", `body['`, nil, true},
		{"DoubleQuoteAtEnd", `body["`, nil, true},
		{"BracketMissingQuotes", `body[test]`, nil, true},
		{"CharacterBetweenBracketAndQuote", `body["test"a]`, nil, true},
		{"CharacterOutsideBracket", `body["test"]a`, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := fromJSONDot(tc.input)
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
	_, err := NewField("resource[test]")
	require.Error(t, err)
	require.Contains(t, err.Error(), "splitting field")
}

func TestFieldFromStringWithResource(t *testing.T) {
	field, err := NewField(`resource["test"]`)
	require.NoError(t, err)
	require.Equal(t, "resource.test", field.String())
}
