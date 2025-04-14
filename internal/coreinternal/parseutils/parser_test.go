// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parseutils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SplitString(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		delimiter   string
		expected    []string
		expectedErr error
	}{
		{
			name:      "simple",
			input:     "a b c",
			delimiter: " ",
			expected: []string{
				"a",
				"b",
				"c",
			},
		},
		{
			name:      "single quotes",
			input:     "a 'b c d'",
			delimiter: " ",
			expected: []string{
				"a",
				"b c d",
			},
		},
		{
			name:      "double quotes",
			input:     `a " b c " d`,
			delimiter: " ",
			expected: []string{
				"a",
				" b c ",
				"d",
			},
		},
		{
			name:      "multi-char delimiter",
			input:     "abc!@!  def !@! g",
			delimiter: "!@!",
			expected: []string{
				"abc",
				"  def ",
				" g",
			},
		},
		{
			name:      "leading and trailing delimiters",
			input:     "   name=ottl        func=key_value   hello=world ",
			delimiter: " ",
			expected: []string{
				"name=ottl",
				"func=key_value",
				"hello=world",
			},
		},
		{
			name:      "embedded double quotes in single quoted value",
			input:     `ab c='this is a "co ol" value'`,
			delimiter: " ",
			expected: []string{
				"ab",
				`c=this is a "co ol" value`,
			},
		},
		{
			name:      "embedded double quotes end single quoted value",
			input:     `ab c='this is a "co ol"'`,
			delimiter: " ",
			expected: []string{
				"ab",
				`c=this is a "co ol"`,
			},
		},
		{
			name:      "embedded escaped quotes",
			input:     `ab c="this \"is \"" d='a \'co ol\' value' e="\""`,
			delimiter: " ",
			expected: []string{
				"ab",
				`c=this \"is \"`,
				`d=a \'co ol\' value`,
				`e=\"`,
			},
		},
		{
			name:      "quoted values include whitespace",
			input:     `name="    ottl " func="  key_ value"`,
			delimiter: " ",
			expected: []string{
				"name=    ottl ",
				"func=  key_ value",
			},
		},
		{
			name:      "delimiter longer than input",
			input:     "abc",
			delimiter: "aaaa",
			expected: []string{
				"abc",
			},
		},
		{
			name:      "delimiter not found",
			input:     "a b c",
			delimiter: "!",
			expected: []string{
				"a b c",
			},
		},
		{
			name: "newlines in input",
			input: `a
b
c`,
			delimiter: " ",
			expected: []string{
				"a\nb\nc",
			},
		},
		{
			name: "newline delimiter",
			input: `a b c
d e f
g 
h`,
			delimiter: "\n",
			expected: []string{
				"a b c",
				"d e f",
				"g ",
				"h",
			},
		},
		{
			name:      "empty input",
			input:     "",
			delimiter: " ",
			expected:  nil,
		},
		{
			name:      "equal input and delimiter",
			input:     "abc",
			delimiter: "abc",
			expected:  nil,
		},
		{
			name:        "unclosed quotes",
			input:       "a 'b c",
			delimiter:   " ",
			expectedErr: errors.New("never reached the end of a quoted value"),
		},
		{
			name:        "mismatched quotes",
			input:       `a 'b c' "d '`,
			delimiter:   " ",
			expectedErr: errors.New("never reached the end of a quoted value"),
		},
		{
			name:      "tab delimiters",
			input:     "a	b	c",
			delimiter: "\t",
			expected: []string{
				"a",
				"b",
				"c",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := SplitString(tc.input, tc.delimiter)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			} else {
				assert.EqualError(t, err, tc.expectedErr.Error())
				assert.Nil(t, result)
			}
		})
	}
}

func Test_ParseKeyValuePairs(t *testing.T) {
	testCases := []struct {
		name        string
		pairs       []string
		delimiter   string
		expected    map[string]any
		expectedErr error
	}{
		{
			name:      "multiple delimiters",
			pairs:     []string{"a==b", "c=d=", "e=f"},
			delimiter: "=",
			expected: map[string]any{
				"a": "=b",
				"c": "d=",
				"e": "f",
			},
		},
		{
			name:        "no delimiter found",
			pairs:       []string{"ab"},
			delimiter:   "=",
			expectedErr: errors.New("cannot split \"ab\" into 2 items, got 1 item(s)"),
		},
		{
			name:        "no delimiter found 2x",
			pairs:       []string{"ab", "cd"},
			delimiter:   "=",
			expectedErr: errors.New("cannot split \"ab\" into 2 items, got 1 item(s); cannot split \"cd\" into 2 items, got 1 item(s)"),
		},
		{
			name:      "empty pairs",
			pairs:     []string{},
			delimiter: "=",
			expected:  map[string]any{},
		},
		{
			name:        "empty pair string",
			pairs:       []string{""},
			delimiter:   "=",
			expectedErr: errors.New("cannot split \"\" into 2 items, got 1 item(s)"),
		},
		{
			name:      "empty delimiter",
			pairs:     []string{"a=b", "c=d"},
			delimiter: "",
			expected: map[string]any{
				"a": "=b",
				"c": "=d",
			},
		},
		{
			name:      "empty pairs & delimiter",
			pairs:     []string{},
			delimiter: "",
			expected:  map[string]any{},
		},
		{
			name:      "early delimiter",
			pairs:     []string{"=a=b"},
			delimiter: "=",
			expected: map[string]any{
				"": "a=b",
			},
		},
		{
			name:      "weird spacing",
			pairs:     []string{"        a=   b    ", "     c       =  d "},
			delimiter: "=",
			expected: map[string]any{
				"a": "b",
				"c": "d",
			},
		},
		{
			name:      "escaped quotes",
			pairs:     []string{"key=foobar", `key2="foo bar"`, `key3="foo \"bar\""`, `key4='\'foo\' \'bar\''`},
			delimiter: "=",
			expected: map[string]any{
				"key":  "foobar",
				"key2": `"foo bar"`,
				"key3": `"foo \"bar\""`,
				"key4": `'\'foo\' \'bar\''`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseKeyValuePairs(tc.pairs, tc.delimiter)

			if tc.expectedErr == nil {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			} else {
				assert.EqualError(t, err, tc.expectedErr.Error())
			}
		})
	}
}
