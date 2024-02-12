// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parseutils

import (
	"fmt"
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
			expectedErr: fmt.Errorf("never reached the end of a quoted value"),
		},
		{
			name:        "mismatched quotes",
			input:       `a 'b c' "d '`,
			delimiter:   " ",
			expectedErr: fmt.Errorf("never reached the end of a quoted value"),
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
