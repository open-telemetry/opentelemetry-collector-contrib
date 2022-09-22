// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ottl

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

// This is not in ottltest because it depends on a type that's a member of OTTL.
func Booleanp(b internal.Boolean) *internal.Boolean {
	return &b
}

func Test_parse(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected *internal.ParsedQuery
	}{
		{
			name:  "invocation with string",
			query: `set("foo")`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							String: ottltest.Strp("foo"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "invocation with float",
			query: `met(1.2)`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "met",
					Arguments: []internal.Value{
						{
							Float: ottltest.Floatp(1.2),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "invocation with int",
			query: `fff(12)`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "fff",
					Arguments: []internal.Value{
						{
							Int: ottltest.Intp(12),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "complex invocation",
			query: `set("foo", getSomething(bear.honey))`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							String: ottltest.Strp("foo"),
						},
						{
							Invocation: &internal.Invocation{
								Function: "getSomething",
								Arguments: []internal.Value{
									{
										Path: &internal.Path{
											Fields: []internal.Field{
												{
													Name: "bear",
												},
												{
													Name: "honey",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "complex path",
			query: `set(foo.attributes["bar"].cat, "dog")`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							Path: &internal.Path{
								Fields: []internal.Field{
									{
										Name: "foo",
									},
									{
										Name:   "attributes",
										MapKey: ottltest.Strp("bar"),
									},
									{
										Name: "cat",
									},
								},
							},
						},
						{
							String: ottltest.Strp("dog"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "where == clause",
			query: `set(foo.attributes["bar"].cat, "dog") where name == "fido"`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							Path: &internal.Path{
								Fields: []internal.Field{
									{
										Name: "foo",
									},
									{
										Name:   "attributes",
										MapKey: ottltest.Strp("bar"),
									},
									{
										Name: "cat",
									},
								},
							},
						},
						{
							String: ottltest.Strp("dog"),
						},
					},
				},
				WhereClause: &internal.BooleanExpression{
					Left: &internal.Term{
						Left: &internal.BooleanValue{
							Comparison: &internal.Comparison{
								Left: internal.Value{
									Path: &internal.Path{
										Fields: []internal.Field{
											{
												Name: "name",
											},
										},
									},
								},
								Op: internal.EQ,
								Right: internal.Value{
									String: ottltest.Strp("fido"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "where != clause",
			query: `set(foo.attributes["bar"].cat, "dog") where name != "fido"`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							Path: &internal.Path{
								Fields: []internal.Field{
									{
										Name: "foo",
									},
									{
										Name:   "attributes",
										MapKey: ottltest.Strp("bar"),
									},
									{
										Name: "cat",
									},
								},
							},
						},
						{
							String: ottltest.Strp("dog"),
						},
					},
				},
				WhereClause: &internal.BooleanExpression{
					Left: &internal.Term{
						Left: &internal.BooleanValue{
							Comparison: &internal.Comparison{
								Left: internal.Value{
									Path: &internal.Path{
										Fields: []internal.Field{
											{
												Name: "name",
											},
										},
									},
								},
								Op: internal.NE,
								Right: internal.Value{
									String: ottltest.Strp("fido"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "ignore extra spaces",
			query: `set  ( foo.attributes[ "bar"].cat,   "dog")   where name=="fido"`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							Path: &internal.Path{
								Fields: []internal.Field{
									{
										Name: "foo",
									},
									{
										Name:   "attributes",
										MapKey: ottltest.Strp("bar"),
									},
									{
										Name: "cat",
									},
								},
							},
						},
						{
							String: ottltest.Strp("dog"),
						},
					},
				},
				WhereClause: &internal.BooleanExpression{
					Left: &internal.Term{
						Left: &internal.BooleanValue{
							Comparison: &internal.Comparison{
								Left: internal.Value{
									Path: &internal.Path{
										Fields: []internal.Field{
											{
												Name: "name",
											},
										},
									},
								},
								Op: internal.EQ,
								Right: internal.Value{
									String: ottltest.Strp("fido"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "handle quotes",
			query: `set("fo\"o")`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							String: ottltest.Strp("fo\"o"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "Invocation with boolean false",
			query: `convert_gauge_to_sum("cumulative", false)`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "convert_gauge_to_sum",
					Arguments: []internal.Value{
						{
							String: ottltest.Strp("cumulative"),
						},
						{
							Bool: (*internal.Boolean)(ottltest.Boolp(false)),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "Invocation with boolean true",
			query: `convert_gauge_to_sum("cumulative", true)`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "convert_gauge_to_sum",
					Arguments: []internal.Value{
						{
							String: ottltest.Strp("cumulative"),
						},
						{
							Bool: (*internal.Boolean)(ottltest.Boolp(true)),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "Invocation with bytes",
			query: `set(attributes["bytes"], 0x0102030405060708)`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							Path: &internal.Path{
								Fields: []internal.Field{
									{
										Name:   "attributes",
										MapKey: ottltest.Strp("bytes"),
									},
								},
							},
						},
						{
							Bytes: (*internal.Bytes)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "Invocation with nil",
			query: `set(attributes["test"], nil)`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							Path: &internal.Path{
								Fields: []internal.Field{
									{
										Name:   "attributes",
										MapKey: ottltest.Strp("test"),
									},
								},
							},
						},
						{
							IsNil: (*internal.IsNil)(ottltest.Boolp(true)),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:  "Invocation with Enum",
			query: `set(attributes["test"], TEST_ENUM)`,
			expected: &internal.ParsedQuery{
				Invocation: internal.Invocation{
					Function: "set",
					Arguments: []internal.Value{
						{
							Path: &internal.Path{
								Fields: []internal.Field{
									{
										Name:   "attributes",
										MapKey: ottltest.Strp("test"),
									},
								},
							},
						},
						{
							Enum: (*internal.EnumSymbol)(ottltest.Strp("TEST_ENUM")),
						},
					},
				},
				WhereClause: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			parsed, err := parseQuery(tt.query)
			assert.NoError(t, err)
			assert.EqualValues(t, tt.expected, parsed)
		})
	}
}

func Test_parse_failure(t *testing.T) {
	tests := []string{
		`set(`,
		`set("foo)`,
		`set(name.)`,
		`("foo")`,
		`set("foo") where name =||= "fido"`,
		`set(span_id, SpanIDWrapper{not a hex string})`,
		`set(span_id, SpanIDWrapper{01})`,
		`set(span_id, SpanIDWrapper{010203040506070809})`,
		`set(trace_id, TraceIDWrapper{not a hex string})`,
		`set(trace_id, TraceIDWrapper{0102030405060708090a0b0c0d0e0f})`,
		`set(trace_id, TraceIDWrapper{0102030405060708090a0b0c0d0e0f1011})`,
		`set("foo") where name = "fido"`,
		`set("foo") where name or "fido"`,
		`set("foo") where name and "fido"`,
		`set("foo") where name and`,
		`set("foo") where name or`,
		`set("foo") where (`,
		`set("foo") where )`,
		`set("foo") where (name == "fido"))`,
		`set("foo") where ((name == "fido")`,
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			_, err := parseQuery(tt)
			assert.Error(t, err)
		})
	}
}

func testParsePath(val *internal.Path) (GetSetter, error) {
	if val != nil && len(val.Fields) > 0 && val.Fields[0].Name == "name" {
		return &testGetSetter{
			getter: func(ctx TransformContext) interface{} {
				return ctx.GetItem()
			},
			setter: func(ctx TransformContext, val interface{}) {
				ctx.GetItem()
			},
		}, nil
	}
	return nil, fmt.Errorf("bad path %v", val)
}

// Helper for test cases where the WHERE clause is all that matters.
// Parse string should start with `set(name, "test") where`...
func setNameTest(b *internal.BooleanExpression) *internal.ParsedQuery {
	return &internal.ParsedQuery{
		Invocation: internal.Invocation{
			Function: "set",
			Arguments: []internal.Value{
				{
					Path: &internal.Path{
						Fields: []internal.Field{
							{
								Name: "name",
							},
						},
					},
				},
				{
					String: ottltest.Strp("test"),
				},
			},
		},
		WhereClause: b,
	}
}

func Test_parseWhere(t *testing.T) {
	tests := []struct {
		query    string
		expected *internal.ParsedQuery
	}{
		{
			query: `true`,
			expected: setNameTest(&internal.BooleanExpression{
				Left: &internal.Term{
					Left: &internal.BooleanValue{
						ConstExpr: Booleanp(true),
					},
				},
			}),
		},
		{
			query: `true and false`,
			expected: setNameTest(&internal.BooleanExpression{
				Left: &internal.Term{
					Left: &internal.BooleanValue{
						ConstExpr: Booleanp(true),
					},
					Right: []*internal.OpAndBooleanValue{
						{
							Operator: "and",
							Value: &internal.BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `true and true and false`,
			expected: setNameTest(&internal.BooleanExpression{
				Left: &internal.Term{
					Left: &internal.BooleanValue{
						ConstExpr: Booleanp(true),
					},
					Right: []*internal.OpAndBooleanValue{
						{
							Operator: "and",
							Value: &internal.BooleanValue{
								ConstExpr: Booleanp(true),
							},
						},
						{
							Operator: "and",
							Value: &internal.BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `true or false`,
			expected: setNameTest(&internal.BooleanExpression{
				Left: &internal.Term{
					Left: &internal.BooleanValue{
						ConstExpr: Booleanp(true),
					},
				},
				Right: []*internal.OpOrTerm{
					{
						Operator: "or",
						Term: &internal.Term{
							Left: &internal.BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `false and true or false`,
			expected: setNameTest(&internal.BooleanExpression{
				Left: &internal.Term{
					Left: &internal.BooleanValue{
						ConstExpr: Booleanp(false),
					},
					Right: []*internal.OpAndBooleanValue{
						{
							Operator: "and",
							Value: &internal.BooleanValue{
								ConstExpr: Booleanp(true),
							},
						},
					},
				},
				Right: []*internal.OpOrTerm{
					{
						Operator: "or",
						Term: &internal.Term{
							Left: &internal.BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `(false and true) or false`,
			expected: setNameTest(&internal.BooleanExpression{
				Left: &internal.Term{
					Left: &internal.BooleanValue{
						SubExpr: &internal.BooleanExpression{
							Left: &internal.Term{
								Left: &internal.BooleanValue{
									ConstExpr: Booleanp(false),
								},
								Right: []*internal.OpAndBooleanValue{
									{
										Operator: "and",
										Value: &internal.BooleanValue{
											ConstExpr: Booleanp(true),
										},
									},
								},
							},
						},
					},
				},
				Right: []*internal.OpOrTerm{
					{
						Operator: "or",
						Term: &internal.Term{
							Left: &internal.BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `false and (true or false)`,
			expected: setNameTest(&internal.BooleanExpression{
				Left: &internal.Term{
					Left: &internal.BooleanValue{
						ConstExpr: Booleanp(false),
					},
					Right: []*internal.OpAndBooleanValue{
						{
							Operator: "and",
							Value: &internal.BooleanValue{
								SubExpr: &internal.BooleanExpression{
									Left: &internal.Term{
										Left: &internal.BooleanValue{
											ConstExpr: Booleanp(true),
										},
									},
									Right: []*internal.OpOrTerm{
										{
											Operator: "or",
											Term: &internal.Term{
												Left: &internal.BooleanValue{
													ConstExpr: Booleanp(false),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}),
		},
		{
			query: `name != "foo" and name != "bar"`,
			expected: setNameTest(&internal.BooleanExpression{
				Left: &internal.Term{
					Left: &internal.BooleanValue{
						Comparison: &internal.Comparison{
							Left: internal.Value{
								Path: &internal.Path{
									Fields: []internal.Field{
										{
											Name: "name",
										},
									},
								},
							},
							Op: internal.NE,
							Right: internal.Value{
								String: ottltest.Strp("foo"),
							},
						},
					},
					Right: []*internal.OpAndBooleanValue{
						{
							Operator: "and",
							Value: &internal.BooleanValue{
								Comparison: &internal.Comparison{
									Left: internal.Value{
										Path: &internal.Path{
											Fields: []internal.Field{
												{
													Name: "name",
												},
											},
										},
									},
									Op: internal.NE,
									Right: internal.Value{
										String: ottltest.Strp("bar"),
									},
								},
							},
						},
					},
				},
			}),
		},
		{
			query: `name == "foo" or name == "bar"`,
			expected: setNameTest(&internal.BooleanExpression{
				Left: &internal.Term{
					Left: &internal.BooleanValue{
						Comparison: &internal.Comparison{
							Left: internal.Value{
								Path: &internal.Path{
									Fields: []internal.Field{
										{
											Name: "name",
										},
									},
								},
							},
							Op: internal.EQ,
							Right: internal.Value{
								String: ottltest.Strp("foo"),
							},
						},
					},
				},
				Right: []*internal.OpOrTerm{
					{
						Operator: "or",
						Term: &internal.Term{
							Left: &internal.BooleanValue{
								Comparison: &internal.Comparison{
									Left: internal.Value{
										Path: &internal.Path{
											Fields: []internal.Field{
												{
													Name: "name",
												},
											},
										},
									},
									Op: internal.EQ,
									Right: internal.Value{
										String: ottltest.Strp("bar"),
									},
								},
							},
						},
					},
				},
			}),
		},
	}

	// create a test name that doesn't confuse vscode so we can rerun tests with one click
	pat := regexp.MustCompile("[^a-zA-Z0-9]+")
	for _, tt := range tests {
		name := pat.ReplaceAllString(tt.query, "_")
		t.Run(name, func(t *testing.T) {
			query := `set(name, "test") where ` + tt.query
			parsed, err := parseQuery(query)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, parsed)
		})
	}
}

var testSymbolTable = map[internal.EnumSymbol]Enum{
	"TEST_ENUM":     0,
	"TEST_ENUM_ONE": 1,
	"TEST_ENUM_TWO": 2,
}

func testParseEnum(val *internal.EnumSymbol) (*Enum, error) {
	if val != nil {
		if enum, ok := testSymbolTable[*val]; ok {
			return &enum, nil
		}
		return nil, fmt.Errorf("enum symbol not found")
	}
	return nil, fmt.Errorf("enum symbol not provided")
}

// This test doesn't validate parser results, simply checks whether the parse succeeds or not.
// It's a fast way to check a large range of possible syntaxes.
func Test_parseQuery(t *testing.T) {
	tests := []struct {
		query   string
		wantErr bool
	}{
		{`set(foo.attributes["bar"].cat, "dog")`, false},
		{`set(foo.attributes["animal"], "dog") where animal == "cat"`, false},
		{`drop() where service == "pinger" or foo.attributes["endpoint"] == "/x/alive"`, false},
		{`drop() where service == "pinger" or foo.attributes["verb"] == "GET" and foo.attributes["endpoint"] == "/x/alive"`, false},
		{`drop() where animal > "cat"`, false},
		{`drop() where animal >= "cat"`, false},
		{`drop() where animal <= "cat"`, false},
		{`drop() where animal < "cat"`, false},
		{`drop() where animal =< "dog"`, true},
		{`drop() where animal => "dog"`, true},
		{`drop() where animal <> "dog"`, true},
		{`drop() where animal = "dog"`, true},
		{`drop() where animal`, true},
		{`drop() where animal ==`, true},
		{`drop() where ==`, true},
		{`drop() where == animal`, true},
		{`drop() where attributes["path"] == "/healthcheck"`, false},
	}
	pat := regexp.MustCompile("[^a-zA-Z0-9]+")
	for _, tt := range tests {
		name := pat.ReplaceAllString(tt.query, "_")
		t.Run(name, func(t *testing.T) {
			_, err := parseQuery(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseQuery(%s) error = %v, wantErr %v", tt.query, err, tt.wantErr)
				return
			}
		})
	}
}
