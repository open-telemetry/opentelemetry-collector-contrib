// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/stretchr/testify/require"
)

func Test_getParsedStatementPaths(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		expected  []path
	}{
		{
			name:      "editor with nested map with path",
			statement: `fff({"mapAttr": {"foo": "bar", "get": bear.honey, "arrayAttr":["foo", "bar"]}})`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 38,
						Line:   1,
						Column: 39,
					},
					Context: "bear",
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 43, Line: 1, Column: 44},
							Name: "honey",
						},
					},
				},
			},
		},
		{
			name:      "editor with function path parameter",
			statement: `set("foo", GetSomething(bear.honey))`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 24,
						Line:   1,
						Column: 25,
					},
					Context: "bear",
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 29, Line: 1, Column: 30},
							Name: "honey",
						},
					},
				},
			},
		},
		{
			name:      "path with key",
			statement: `set(foo.attributes["bar"].cat, "dog")`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 4,
						Line:   1,
						Column: 5,
					},
					Context: "foo",
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 8, Line: 1, Column: 9},
							Name: "attributes",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 18, Line: 1, Column: 19},
									String: new("bar"),
								},
							},
						},
						{
							Pos:  lexer.Position{Offset: 26, Line: 1, Column: 27},
							Name: "cat",
						},
					},
				},
			},
		},
		{
			name:      "single path field segment",
			statement: `set(attributes["bar"], "dog")`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 4,
						Line:   1,
						Column: 5,
					},
					Context: "",
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 4, Line: 1, Column: 5},
							Name: "attributes",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 14, Line: 1, Column: 15},
									String: new("bar"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "converter parameters",
			statement: `replace_pattern(attributes["message"], "device=*", attributes["device_name"], SHA256)`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 16,
						Line:   1,
						Column: 17,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 16, Line: 1, Column: 17},
							Name: "attributes",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 26, Line: 1, Column: 27},
									String: new("message"),
								},
							},
						},
					},
				},
				{
					Pos: lexer.Position{
						Offset: 51,
						Line:   1,
						Column: 52,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 51, Line: 1, Column: 52},
							Name: "attributes",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 61, Line: 1, Column: 62},
									String: new("device_name"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "complex path with multiple keys",
			statement: `set(foo.bar["x"]["y"].z, Test()[0]["pass"])`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 4,
						Line:   1,
						Column: 5,
					},
					Context: "foo",
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 8, Line: 1, Column: 9},
							Name: "bar",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 11, Line: 1, Column: 12},
									String: new("x"),
								},
								{
									Pos:    lexer.Position{Offset: 16, Line: 1, Column: 17},
									String: new("y"),
								},
							},
						},
						{
							Pos:  lexer.Position{Offset: 22, Line: 1, Column: 23},
							Name: "z",
						},
					},
				},
			},
		},
		{
			name:      "where clause",
			statement: `set(foo.attributes["bar"].cat, "dog") where name == "fido"`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 4,
						Line:   1,
						Column: 5,
					},
					Context: "foo",
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 8, Line: 1, Column: 9},
							Name: "attributes",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 18, Line: 1, Column: 19},
									String: new("bar"),
								},
							},
						},
						{
							Pos:  lexer.Position{Offset: 26, Line: 1, Column: 27},
							Name: "cat",
						},
					},
				},
				{
					Pos: lexer.Position{
						Offset: 44,
						Line:   1,
						Column: 45,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 44, Line: 1, Column: 45},
							Name: "name",
						},
					},
				},
			},
		},
		{
			name:      "where clause multiple conditions",
			statement: `set(foo.attributes["bar"].cat, "dog") where name == "fido" and surname == "dido" or surname == "DIDO"`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 4,
						Line:   1,
						Column: 5,
					},
					Context: "foo",
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 8, Line: 1, Column: 9},
							Name: "attributes",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 18, Line: 1, Column: 19},
									String: new("bar"),
								},
							},
						},
						{
							Pos:  lexer.Position{Offset: 26, Line: 1, Column: 27},
							Name: "cat",
						},
					},
				},
				{
					Pos: lexer.Position{
						Offset: 44,
						Line:   1,
						Column: 45,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 44, Line: 1, Column: 45},
							Name: "name",
						},
					},
				},
				{
					Pos: lexer.Position{
						Offset: 63,
						Line:   1,
						Column: 64,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 63, Line: 1, Column: 64},
							Name: "surname",
						},
					},
				},
				{
					Pos: lexer.Position{
						Offset: 84,
						Line:   1,
						Column: 85,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 84, Line: 1, Column: 85},
							Name: "surname",
						},
					},
				},
			},
		},
		{
			name:      "where clause sub expression",
			statement: `set(foo.attributes["bar"].cat, "value") where three / (1 + 1) == foo.value`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 4,
						Line:   1,
						Column: 5,
					},
					Context: "foo",
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 8, Line: 1, Column: 9},
							Name: "attributes",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 18, Line: 1, Column: 19},
									String: new("bar"),
								},
							},
						},
						{
							Pos:  lexer.Position{Offset: 26, Line: 1, Column: 27},
							Name: "cat",
						},
					},
				},
				{
					Pos: lexer.Position{
						Offset: 46,
						Line:   1,
						Column: 47,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 46, Line: 1, Column: 47},
							Name: "three",
						},
					},
				},
				{
					Pos: lexer.Position{
						Offset: 65,
						Line:   1,
						Column: 66,
					},
					Context: "foo",
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 69, Line: 1, Column: 70},
							Name: "value",
						},
					},
				},
			},
		},
		{
			name:      "converter with path list",
			statement: `set(attributes["test"], [bear.bear, bear.honey])`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 4,
						Line:   1,
						Column: 5,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 4, Line: 1, Column: 5},
							Name: "attributes",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 14, Line: 1, Column: 15},
									String: new("test"),
								},
							},
						},
					},
				},
				{
					Pos: lexer.Position{
						Offset: 25,
						Line:   1,
						Column: 26,
					},
					Context: "bear",
					Fields:  []field{{Pos: lexer.Position{Offset: 30, Line: 1, Column: 31}, Name: "bear"}},
				},
				{
					Pos: lexer.Position{
						Offset: 36,
						Line:   1,
						Column: 37,
					},
					Context: "bear",
					Fields:  []field{{Pos: lexer.Position{Offset: 41, Line: 1, Column: 42}, Name: "honey"}},
				},
			},
		},
		{
			name:      "converter math math expression",
			statement: `set(attributes["test"], 1000 - 600) where 1 + 1 * 2 == three / One()`,
			expected: []path{
				{
					Pos: lexer.Position{
						Offset: 4,
						Line:   1,
						Column: 5,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 4, Line: 1, Column: 5},
							Name: "attributes",
							Keys: []key{
								{
									Pos:    lexer.Position{Offset: 14, Line: 1, Column: 15},
									String: new("test"),
								},
							},
						},
					},
				},
				{
					Pos: lexer.Position{
						Offset: 55,
						Line:   1,
						Column: 56,
					},
					Fields: []field{
						{
							Pos:  lexer.Position{Offset: 55, Line: 1, Column: 56},
							Name: "three",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, err := parseStatement(tt.statement)
			require.NoError(t, err)

			paths := getParsedStatementPaths(ps)
			require.Equal(t, tt.expected, paths)
		})
	}
}

func Test_getBooleanExpressionPaths(t *testing.T) {
	expected := []path{
		{
			Pos: lexer.Position{
				Offset: 0,
				Line:   1,
				Column: 1,
			},
			Context: "honey",
			Fields:  []field{{Pos: lexer.Position{Offset: 6, Line: 1, Column: 7}, Name: "bear"}},
		},
		{
			Pos: lexer.Position{
				Offset: 21,
				Line:   1,
				Column: 22,
			},
			Context: "foo",
			Fields:  []field{{Pos: lexer.Position{Offset: 25, Line: 1, Column: 26}, Name: "bar"}},
		},
	}

	c, err := parseCondition("honey.bear == 1 and (foo.bar == true or 1 == 1)")
	require.NoError(t, err)

	paths := getBooleanExpressionPaths(c)
	require.Equal(t, expected, paths)
}

func Test_getValuePaths(t *testing.T) {
	expected := []path{
		{
			Pos: lexer.Position{
				Offset: 0,
				Line:   1,
				Column: 1,
			},
			Context: "honey",
			Fields:  []field{{Pos: lexer.Position{Offset: 6, Line: 1, Column: 7}, Name: "bear"}},
		},
		{
			Pos: lexer.Position{
				Offset: 14,
				Line:   1,
				Column: 15,
			},
			Context: "foo",
			Fields:  []field{{Pos: lexer.Position{Offset: 18, Line: 1, Column: 19}, Name: "bar"}},
		},
	}

	c, err := parseValueExpression("honey.bear + (foo.bar * 3)")
	require.NoError(t, err)

	paths := getValuePaths(c)
	require.Equal(t, expected, paths)
}
