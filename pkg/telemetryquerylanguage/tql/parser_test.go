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

package tql

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
)

// This is not in tqltest because it depends on a type that's a member of TQL.
func Booleanp(b Boolean) *Boolean {
	return &b
}

func Test_parse(t *testing.T) {
	tests := []struct {
		query    string
		expected *ParsedQuery
	}{
		{
			query: `set("foo")`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "set",
					Arguments: []Value{
						{
							String: tqltest.Strp("foo"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			query: `met(1.2)`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "met",
					Arguments: []Value{
						{
							Float: tqltest.Floatp(1.2),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			query: `fff(12)`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "fff",
					Arguments: []Value{
						{
							Int: tqltest.Intp(12),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			query: `set("foo", get(bear.honey))`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "set",
					Arguments: []Value{
						{
							String: tqltest.Strp("foo"),
						},
						{
							Invocation: &Invocation{
								Function: "get",
								Arguments: []Value{
									{
										Path: &Path{
											Fields: []Field{
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
			query: `set(foo.attributes["bar"].cat, "dog")`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "set",
					Arguments: []Value{
						{
							Path: &Path{
								Fields: []Field{
									{
										Name: "foo",
									},
									{
										Name:   "attributes",
										MapKey: tqltest.Strp("bar"),
									},
									{
										Name: "cat",
									},
								},
							},
						},
						{
							String: tqltest.Strp("dog"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			query: `set(foo.attributes["bar"].cat, "dog") where name == "fido"`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "set",
					Arguments: []Value{
						{
							Path: &Path{
								Fields: []Field{
									{
										Name: "foo",
									},
									{
										Name:   "attributes",
										MapKey: tqltest.Strp("bar"),
									},
									{
										Name: "cat",
									},
								},
							},
						},
						{
							String: tqltest.Strp("dog"),
						},
					},
				},
				WhereClause: &BooleanExpression{
					Left: &Term{
						Left: &BooleanValue{
							Comparison: &Comparison{
								Left: Value{
									Path: &Path{
										Fields: []Field{
											{
												Name: "name",
											},
										},
									},
								},
								Op: "==",
								Right: Value{
									String: tqltest.Strp("fido"),
								},
							},
						},
					},
				},
			},
		},
		{
			query: `set(foo.attributes["bar"].cat, "dog") where name != "fido"`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "set",
					Arguments: []Value{
						{
							Path: &Path{
								Fields: []Field{
									{
										Name: "foo",
									},
									{
										Name:   "attributes",
										MapKey: tqltest.Strp("bar"),
									},
									{
										Name: "cat",
									},
								},
							},
						},
						{
							String: tqltest.Strp("dog"),
						},
					},
				},
				WhereClause: &BooleanExpression{
					Left: &Term{
						Left: &BooleanValue{
							Comparison: &Comparison{
								Left: Value{
									Path: &Path{
										Fields: []Field{
											{
												Name: "name",
											},
										},
									},
								},
								Op: "!=",
								Right: Value{
									String: tqltest.Strp("fido"),
								},
							},
						},
					},
				},
			},
		},
		{
			query: `set  ( foo.attributes[ "bar"].cat,   "dog")   where name=="fido"`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "set",
					Arguments: []Value{
						{
							Path: &Path{
								Fields: []Field{
									{
										Name: "foo",
									},
									{
										Name:   "attributes",
										MapKey: tqltest.Strp("bar"),
									},
									{
										Name: "cat",
									},
								},
							},
						},
						{
							String: tqltest.Strp("dog"),
						},
					},
				},
				WhereClause: &BooleanExpression{
					Left: &Term{
						Left: &BooleanValue{
							Comparison: &Comparison{
								Left: Value{
									Path: &Path{
										Fields: []Field{
											{
												Name: "name",
											},
										},
									},
								},
								Op: "==",
								Right: Value{
									String: tqltest.Strp("fido"),
								},
							},
						},
					},
				},
			},
		},
		{
			query: `set("fo\"o")`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "set",
					Arguments: []Value{
						{
							String: tqltest.Strp("fo\"o"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			query: `convert_gauge_to_sum("cumulative", false)`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "convert_gauge_to_sum",
					Arguments: []Value{
						{
							String: tqltest.Strp("cumulative"),
						},
						{
							Bool: (*Boolean)(tqltest.Boolp(false)),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			query: `convert_gauge_to_sum("cumulative", true)`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "convert_gauge_to_sum",
					Arguments: []Value{
						{
							String: tqltest.Strp("cumulative"),
						},
						{
							Bool: (*Boolean)(tqltest.Boolp(true)),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			query: `set(attributes["bytes"], 0x0102030405060708)`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "set",
					Arguments: []Value{
						{
							Path: &Path{
								Fields: []Field{
									{
										Name:   "attributes",
										MapKey: tqltest.Strp("bytes"),
									},
								},
							},
						},
						{
							Bytes: (*Bytes)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			query: `set(attributes["test"], nil)`,
			expected: &ParsedQuery{
				Invocation: Invocation{
					Function: "set",
					Arguments: []Value{
						{
							Path: &Path{
								Fields: []Field{
									{
										Name:   "attributes",
										MapKey: tqltest.Strp("test"),
									},
								},
							},
						},
						{
							IsNil: (*IsNil)(tqltest.Boolp(true)),
						},
					},
				},
				WhereClause: nil,
			},
		},
	}

	// create a test name that doesn't confuse vscode so we can rerun tests with one click
	pat := regexp.MustCompile("[^a-zA-Z0-9]+")
	for _, tt := range tests {
		name := pat.ReplaceAllString(tt.query, "_")
		t.Run(name, func(t *testing.T) {
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

func testParsePath(val *Path) (GetSetter, error) {
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
func setNameTest(b *BooleanExpression) *ParsedQuery {
	return &ParsedQuery{
		Invocation: Invocation{
			Function: "set",
			Arguments: []Value{
				{
					Path: &Path{
						Fields: []Field{
							{
								Name: "name",
							},
						},
					},
				},
				{
					String: tqltest.Strp("test"),
				},
			},
		},
		WhereClause: b,
	}
}

func Test_parseWhere(t *testing.T) {
	tests := []struct {
		query    string
		expected *ParsedQuery
	}{
		{
			query: `true`,
			expected: setNameTest(&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(true),
					},
				},
			}),
		},
		{
			query: `true and false`,
			expected: setNameTest(&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(true),
					},
					Right: []*OpAndBooleanValue{
						{
							Operator: "and",
							Value: &BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `true and true and false`,
			expected: setNameTest(&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(true),
					},
					Right: []*OpAndBooleanValue{
						{
							Operator: "and",
							Value: &BooleanValue{
								ConstExpr: Booleanp(true),
							},
						},
						{
							Operator: "and",
							Value: &BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `true or false`,
			expected: setNameTest(&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(true),
					},
				},
				Right: []*OpOrTerm{
					{
						Operator: "or",
						Term: &Term{
							Left: &BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `false and true or false`,
			expected: setNameTest(&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(false),
					},
					Right: []*OpAndBooleanValue{
						{
							Operator: "and",
							Value: &BooleanValue{
								ConstExpr: Booleanp(true),
							},
						},
					},
				},
				Right: []*OpOrTerm{
					{
						Operator: "or",
						Term: &Term{
							Left: &BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `(false and true) or false`,
			expected: setNameTest(&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						SubExpr: &BooleanExpression{
							Left: &Term{
								Left: &BooleanValue{
									ConstExpr: Booleanp(false),
								},
								Right: []*OpAndBooleanValue{
									{
										Operator: "and",
										Value: &BooleanValue{
											ConstExpr: Booleanp(true),
										},
									},
								},
							},
						},
					},
				},
				Right: []*OpOrTerm{
					{
						Operator: "or",
						Term: &Term{
							Left: &BooleanValue{
								ConstExpr: Booleanp(false),
							},
						},
					},
				},
			}),
		},
		{
			query: `false and (true or false)`,
			expected: setNameTest(&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(false),
					},
					Right: []*OpAndBooleanValue{
						{
							Operator: "and",
							Value: &BooleanValue{
								SubExpr: &BooleanExpression{
									Left: &Term{
										Left: &BooleanValue{
											ConstExpr: Booleanp(true),
										},
									},
									Right: []*OpOrTerm{
										{
											Operator: "or",
											Term: &Term{
												Left: &BooleanValue{
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
			expected: setNameTest(&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						Comparison: &Comparison{
							Left: Value{
								Path: &Path{
									Fields: []Field{
										{
											Name: "name",
										},
									},
								},
							},
							Op: "!=",
							Right: Value{
								String: tqltest.Strp("foo"),
							},
						},
					},
					Right: []*OpAndBooleanValue{
						{
							Operator: "and",
							Value: &BooleanValue{
								Comparison: &Comparison{
									Left: Value{
										Path: &Path{
											Fields: []Field{
												{
													Name: "name",
												},
											},
										},
									},
									Op: "!=",
									Right: Value{
										String: tqltest.Strp("bar"),
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
			expected: setNameTest(&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						Comparison: &Comparison{
							Left: Value{
								Path: &Path{
									Fields: []Field{
										{
											Name: "name",
										},
									},
								},
							},
							Op: "==",
							Right: Value{
								String: tqltest.Strp("foo"),
							},
						},
					},
				},
				Right: []*OpOrTerm{
					{
						Operator: "or",
						Term: &Term{
							Left: &BooleanValue{
								Comparison: &Comparison{
									Left: Value{
										Path: &Path{
											Fields: []Field{
												{
													Name: "name",
												},
											},
										},
									},
									Op: "==",
									Right: Value{
										String: tqltest.Strp("bar"),
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
