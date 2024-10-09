// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
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
							Name: "attributes",
							Keys: []key{
								{
									String: ottltest.Strp("bar"),
								},
							},
						},
						{
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
							Name: "attributes",
							Keys: []key{
								{
									String: ottltest.Strp("bar"),
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
							Name: "attributes",
							Keys: []key{
								{
									String: ottltest.Strp("message"),
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
							Name: "attributes",
							Keys: []key{
								{
									String: ottltest.Strp("device_name"),
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
							Name: "bar",
							Keys: []key{
								{
									String: ottltest.Strp("x"),
								},
								{
									String: ottltest.Strp("y"),
								},
							},
						},
						{
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
							Name: "attributes",
							Keys: []key{
								{
									String: ottltest.Strp("bar"),
								},
							},
						},
						{
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
							Name: "attributes",
							Keys: []key{
								{
									String: ottltest.Strp("bar"),
								},
							},
						},
						{
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
							Name: "attributes",
							Keys: []key{
								{
									String: ottltest.Strp("bar"),
								},
							},
						},
						{
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
							Name: "attributes",
							Keys: []key{
								{
									String: ottltest.Strp("test"),
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
					Fields:  []field{{Name: "bear"}},
				},
				{
					Pos: lexer.Position{
						Offset: 36,
						Line:   1,
						Column: 37,
					},
					Context: "bear",
					Fields:  []field{{Name: "honey"}},
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
							Name: "attributes",
							Keys: []key{
								{
									String: ottltest.Strp("test"),
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
			Fields:  []field{{Name: "bear"}},
		},
		{
			Pos: lexer.Position{
				Offset: 21,
				Line:   1,
				Column: 22,
			},
			Context: "foo",
			Fields:  []field{{Name: "bar"}},
		},
	}

	c, err := parseCondition("honey.bear == 1 and (foo.bar == true or 1 == 1)")
	require.NoError(t, err)

	paths := getBooleanExpressionPaths(c)
	require.Equal(t, expected, paths)
}

func Test_AppendStatementPathsContext_InvalidStatement(t *testing.T) {
	ps, err := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
		WithPathContextNames[any]([]string{"foo", "bar"}),
	)
	require.NoError(t, err)
	_, err = ps.AppendStatementPathsContext("foo", "this is invalid")
	require.ErrorContains(t, err, `statement has invalid syntax`)
}

func Test_AppendStatementPathsContext_InvalidContext(t *testing.T) {
	ps, err := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
		WithPathContextNames[any]([]string{"foo", "bar"}),
	)
	require.NoError(t, err)
	_, err = ps.AppendStatementPathsContext("foobar", "set(foo, 1)")
	require.ErrorContains(t, err, `unknown context "foobar" for parser`)
}

func Test_AppendStatementPathsContext_Success(t *testing.T) {
	type mockSetArguments[K any] struct {
		Target Setter[K]
		Value  Getter[K]
	}

	mockSetFactory := NewFactory("set", &mockSetArguments[any]{}, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
		return func(_ context.Context, _ any) (any, error) {
			return nil, nil
		}, nil
	})

	tests := []struct {
		name             string
		statement        string
		context          string
		pathContextNames []string
		expected         string
	}{
		{
			name:      "no paths",
			statement: `set("foo", 1)`,
			context:   "bar",
		},
		{
			name:      "single path with context",
			statement: `set(span.value, 1)`,
			context:   "span",
		},
		{
			name:      "single path without context",
			statement: "set(value, 1)",
			expected:  "set(span.value, 1)",
			context:   "span",
		},
		{
			name:             "single path with context - multiple context names",
			statement:        "set(span.value, 1)",
			pathContextNames: []string{"spanevent", "span"},
			context:          "spanevent",
		},
		{
			name:             "multiple paths with the same context",
			statement:        `set(span.value, 1) where span.attributes["foo"] == "foo" and span.id == 1`,
			pathContextNames: []string{"another", "span"},
			context:          "another",
		},
		{
			name:             "multiple paths with different contexts",
			statement:        `set(another.value, 1) where span.attributes["foo"] == "foo" and another.id == 1`,
			pathContextNames: []string{"another", "span"},
			context:          "another",
		},
		{
			name:             "multiple paths with and without contexts",
			statement:        `set(value, 1) where span.attributes["foo"] == "foo" and id == 1`,
			expected:         `set(spanevent.value, 1) where span.attributes["foo"] == "foo" and spanevent.id == 1`,
			pathContextNames: []string{"spanevent", "span"},
			context:          "spanevent",
		},
		{
			name:      "multiple paths without context",
			statement: `set(value, 1) where name == attributes["foo.name"]`,
			expected:  `set(span.value, 1) where span.name == span.attributes["foo.name"]`,
			context:   "span",
		},
		{
			name:      "function path parameter without context",
			statement: `set(attributes["test"], "pass") where IsMatch(name, "operation[AC]")`,
			context:   "log",
			expected:  `set(log.attributes["test"], "pass") where IsMatch(log.name, "operation[AC]")`,
		},
		{
			name:             "function path parameter with context",
			statement:        `set(attributes["test"], "pass") where IsMatch(resource.name, "operation[AC]")`,
			context:          "log",
			pathContextNames: []string{"log", "resource"},
			expected:         `set(log.attributes["test"], "pass") where IsMatch(resource.name, "operation[AC]")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.pathContextNames) == 0 {
				tt.pathContextNames = append(tt.pathContextNames, tt.context)
			}

			ps, err := NewParser(
				CreateFactoryMap[any](mockSetFactory),
				testParsePath[any],
				componenttest.NewNopTelemetrySettings(),
				WithEnumParser[any](testParseEnum),
				WithPathContextNames[any](tt.pathContextNames),
			)

			require.NoError(t, err)
			require.NotNil(t, ps)

			var expected string
			if tt.expected != "" {
				expected = tt.expected
			} else {
				expected = tt.statement
			}

			result, err := ps.AppendStatementPathsContext(tt.context, tt.statement)
			require.NoError(t, err)
			assert.Equal(t, expected, result)
		})
	}
}
