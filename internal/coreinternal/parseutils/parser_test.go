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

func BenchmarkSplitString(b *testing.B) {
	benchCases := []struct {
		name      string
		input     string
		delimiter string
	}{
		{
			name:      "simple_short",
			input:     "a b c",
			delimiter: " ",
		},
		{
			name:      "quoted_values",
			input:     `name="John Doe" age=30 city="New York"`,
			delimiter: " ",
		},
		{
			name:      "multi_char_delimiter",
			input:     "key1=val1!@!key2=val2!@!key3=val3!@!key4=val4",
			delimiter: "!@!",
		},
		{
			name:      "many_fields",
			input:     `a=1 b=2 c=3 d=4 e=5 f=6 g=7 h=8 i=9 j=10 k=11 l=12 m=13 n=14 o=15 p=16`,
			delimiter: " ",
		},
		{
			name:      "long_quoted_value",
			input:     `key1="this is a very long quoted value that contains many words and spaces" key2="another long value here"`,
			delimiter: " ",
		},
		{
			name:      "escaped_quotes",
			input:     `a="hello \"world\"" b='it\'s cool' c="test \"value\""`,
			delimiter: " ",
		},
		{
			name:      "leading_trailing_delimiters",
			input:     "   name=ottl        func=key_value   hello=world   foo=bar   ",
			delimiter: " ",
		},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = SplitString(bc.input, bc.delimiter)
			}
		})
	}
}
