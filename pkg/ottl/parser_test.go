// Copyright The OpenTelemetry Authors
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
	"context"
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

// This is not in ottltest because it depends on a type that's a member of OTTL.
func booleanp(b boolean) *boolean {
	return &b
}

func Test_parse(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		expected  *parsedStatement
	}{
		{
			name:      "invocation with string",
			statement: `set("foo")`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							String: ottltest.Strp("foo"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "invocation with float",
			statement: `met(1.2)`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "met",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.2),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "invocation with int",
			statement: `fff(12)`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "fff",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(12),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "complex invocation",
			statement: `set("foo", GetSomething(bear.honey))`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							String: ottltest.Strp("foo"),
						},
						{
							Literal: &mathExprLiteral{
								Converter: &converter{
									Function: "GetSomething",
									Arguments: []value{
										{
											Literal: &mathExprLiteral{
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
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "complex path",
			statement: `set(foo.attributes["bar"].cat, "dog")`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "foo",
										},
										{
											Name: "attributes",
											Keys: []Key{
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
							String: ottltest.Strp("dog"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "complex path",
			statement: `set(foo.bar["x"]["y"].z, Test()[0]["pass"])`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "foo",
										},
										{
											Name: "bar",
											Keys: []Key{
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
							Literal: &mathExprLiteral{
								Converter: &converter{
									Function: "Test",
									Keys: []Key{
										{
											Int: ottltest.Intp(0),
										},
										{
											String: ottltest.Strp("pass"),
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
			name:      "where == clause",
			statement: `set(foo.attributes["bar"].cat, "dog") where name == "fido"`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "foo",
										},
										{
											Name: "attributes",
											Keys: []Key{
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
							String: ottltest.Strp("dog"),
						},
					},
				},
				WhereClause: &booleanExpression{
					Left: &term{
						Left: &booleanValue{
							Comparison: &comparison{
								Left: value{
									Literal: &mathExprLiteral{
										Path: &Path{
											Fields: []Field{
												{
													Name: "name",
												},
											},
										},
									},
								},
								Op: EQ,
								Right: value{
									String: ottltest.Strp("fido"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "where != clause",
			statement: `set(foo.attributes["bar"].cat, "dog") where name != "fido"`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "foo",
										},
										{
											Name: "attributes",
											Keys: []Key{
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
							String: ottltest.Strp("dog"),
						},
					},
				},
				WhereClause: &booleanExpression{
					Left: &term{
						Left: &booleanValue{
							Comparison: &comparison{
								Left: value{
									Literal: &mathExprLiteral{
										Path: &Path{
											Fields: []Field{
												{
													Name: "name",
												},
											},
										},
									},
								},
								Op: NE,
								Right: value{
									String: ottltest.Strp("fido"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "ignore extra spaces",
			statement: `set  ( foo.attributes[ "bar"].cat,   "dog")   where name=="fido"`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "foo",
										},
										{
											Name: "attributes",
											Keys: []Key{
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
							String: ottltest.Strp("dog"),
						},
					},
				},
				WhereClause: &booleanExpression{
					Left: &term{
						Left: &booleanValue{
							Comparison: &comparison{
								Left: value{
									Literal: &mathExprLiteral{
										Path: &Path{
											Fields: []Field{
												{
													Name: "name",
												},
											},
										},
									},
								},
								Op: EQ,
								Right: value{
									String: ottltest.Strp("fido"),
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "handle quotes",
			statement: `set("fo\"o")`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							String: ottltest.Strp("fo\"o"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "invocation with boolean false",
			statement: `convert_gauge_to_sum("cumulative", false)`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "convert_gauge_to_sum",
					Arguments: []value{
						{
							String: ottltest.Strp("cumulative"),
						},
						{
							Bool: (*boolean)(ottltest.Boolp(false)),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "invocation with boolean true",
			statement: `convert_gauge_to_sum("cumulative", true)`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "convert_gauge_to_sum",
					Arguments: []value{
						{
							String: ottltest.Strp("cumulative"),
						},
						{
							Bool: (*boolean)(ottltest.Boolp(true)),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "invocation with bytes",
			statement: `set(attributes["bytes"], 0x0102030405060708)`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "attributes",
											Keys: []Key{
												{
													String: ottltest.Strp("bytes"),
												},
											},
										},
									},
								},
							},
						},
						{
							Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "invocation with nil",
			statement: `set(attributes["test"], nil)`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "attributes",
											Keys: []Key{
												{
													String: ottltest.Strp("test"),
												},
											},
										},
									},
								},
							},
						},
						{
							IsNil: (*isNil)(ottltest.Boolp(true)),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "invocation with Enum",
			statement: `set(attributes["test"], TEST_ENUM)`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "attributes",
											Keys: []Key{
												{
													String: ottltest.Strp("test"),
												},
											},
										},
									},
								},
							},
						},
						{
							Enum: (*EnumSymbol)(ottltest.Strp("TEST_ENUM")),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "Converter with empty list",
			statement: `set(attributes["test"], [])`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "attributes",
											Keys: []Key{
												{
													String: ottltest.Strp("test"),
												},
											},
										},
									},
								},
							},
						},
						{
							List: &list{
								Values: nil,
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "Converter with single-value list",
			statement: `set(attributes["test"], ["value0"])`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "attributes",
											Keys: []Key{
												{
													String: ottltest.Strp("test"),
												},
											},
										},
									},
								},
							},
						},
						{
							List: &list{
								Values: []value{
									{
										String: ottltest.Strp("value0"),
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
			name:      "Converter with multi-value list",
			statement: `set(attributes["test"], ["value1", "value2"])`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "attributes",
											Keys: []Key{
												{
													String: ottltest.Strp("test"),
												},
											},
										},
									},
								},
							},
						},
						{
							List: &list{
								Values: []value{
									{
										String: ottltest.Strp("value1"),
									},
									{
										String: ottltest.Strp("value2"),
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
			name:      "Converter with nested heterogeneous types",
			statement: `set(attributes["test"], [Concat(["a", "b"], "+"), ["1", 2, 3.0], nil, attributes["test"]])`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "attributes",
											Keys: []Key{
												{
													String: ottltest.Strp("test"),
												},
											},
										},
									},
								},
							},
						},
						{
							List: &list{
								Values: []value{
									{
										Literal: &mathExprLiteral{
											Converter: &converter{
												Function: "Concat",
												Arguments: []value{
													{
														List: &list{
															Values: []value{
																{
																	String: ottltest.Strp("a"),
																},
																{
																	String: ottltest.Strp("b"),
																},
															},
														},
													},
													{
														String: ottltest.Strp("+"),
													},
												},
											},
										},
									},
									{
										List: &list{
											Values: []value{
												{
													String: ottltest.Strp("1"),
												},
												{
													Literal: &mathExprLiteral{
														Int: ottltest.Intp(2),
													},
												},
												{
													Literal: &mathExprLiteral{
														Float: ottltest.Floatp(3.0),
													},
												},
											},
										},
									},
									{
										IsNil: (*isNil)(ottltest.Boolp(true)),
									},
									{
										Literal: &mathExprLiteral{
											Path: &Path{
												Fields: []Field{
													{
														Name: "attributes",
														Keys: []Key{
															{
																String: ottltest.Strp("test"),
															},
														},
													},
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
			name:      "Converter math mathExpression",
			statement: `set(attributes["test"], 1000 - 600) where 1 + 1 * 2 == three / One()`,
			expected: &parsedStatement{
				Invocation: invocation{
					Function: "set",
					Arguments: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "attributes",
											Keys: []Key{
												{
													String: ottltest.Strp("test"),
												},
											},
										},
									},
								},
							},
						},
						{
							MathExpression: &mathExpression{
								Left: &addSubTerm{
									Left: &mathValue{
										Literal: &mathExprLiteral{
											Int: ottltest.Intp(1000),
										},
									},
								},
								Right: []*opAddSubTerm{
									{
										Operator: SUB,
										Term: &addSubTerm{
											Left: &mathValue{
												Literal: &mathExprLiteral{
													Int: ottltest.Intp(600),
												},
											},
										},
									},
								},
							},
						},
					},
				},
				WhereClause: &booleanExpression{
					Left: &term{
						Left: &booleanValue{
							Comparison: &comparison{
								Left: value{
									MathExpression: &mathExpression{
										Left: &addSubTerm{
											Left: &mathValue{
												Literal: &mathExprLiteral{
													Int: ottltest.Intp(1),
												},
											},
										},
										Right: []*opAddSubTerm{
											{
												Operator: ADD,
												Term: &addSubTerm{
													Left: &mathValue{
														Literal: &mathExprLiteral{
															Int: ottltest.Intp(1),
														},
													},
													Right: []*opMultDivValue{
														{
															Operator: MULT,
															Value: &mathValue{
																Literal: &mathExprLiteral{
																	Int: ottltest.Intp(2),
																},
															},
														},
													},
												},
											},
										},
									},
								},
								Op: EQ,
								Right: value{
									MathExpression: &mathExpression{
										Left: &addSubTerm{
											Left: &mathValue{
												Literal: &mathExprLiteral{
													Path: &Path{
														Fields: []Field{
															{
																Name: "three",
															},
														},
													},
												},
											},
											Right: []*opMultDivValue{
												{
													Operator: DIV,
													Value: &mathValue{
														Literal: &mathExprLiteral{
															Converter: &converter{
																Function: "One",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			parsed, err := parseStatement(tt.statement)
			assert.NoError(t, err)
			assert.EqualValues(t, tt.expected, parsed)
		})
	}
}

func testParsePath(val *Path) (GetSetter[interface{}], error) {
	if val != nil && len(val.Fields) > 0 && (val.Fields[0].Name == "name" || val.Fields[0].Name == "attributes") {
		return &StandardGetSetter[interface{}]{
			Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
				return tCtx, nil
			},
			Setter: func(ctx context.Context, tCtx interface{}, val interface{}) error {
				reflect.DeepEqual(tCtx, val)
				return nil
			},
		}, nil
	}
	return nil, fmt.Errorf("bad path %v", val)
}

// Helper for test cases where the WHERE clause is all that matters.
// Parse string should start with `set(name, "test") where`...
func setNameTest(b *booleanExpression) *parsedStatement {
	return &parsedStatement{
		Invocation: invocation{
			Function: "set",
			Arguments: []value{
				{
					Literal: &mathExprLiteral{
						Path: &Path{
							Fields: []Field{
								{
									Name: "name",
								},
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
		statement string
		expected  *parsedStatement
	}{
		{
			statement: `true`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Boolean: booleanp(true),
						},
					},
				},
			}),
		},
		{
			statement: `true and false`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Boolean: booleanp(true),
						},
					},
					Right: []*opAndBooleanValue{
						{
							Operator: "and",
							Value: &booleanValue{
								ConstExpr: &constExpr{
									Boolean: booleanp(false),
								},
							},
						},
					},
				},
			}),
		},
		{
			statement: `true and true and false`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Boolean: booleanp(true),
						},
					},
					Right: []*opAndBooleanValue{
						{
							Operator: "and",
							Value: &booleanValue{
								ConstExpr: &constExpr{
									Boolean: booleanp(true),
								},
							},
						},
						{
							Operator: "and",
							Value: &booleanValue{
								ConstExpr: &constExpr{
									Boolean: booleanp(false),
								},
							},
						},
					},
				},
			}),
		},
		{
			statement: `true or false`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Boolean: booleanp(true),
						},
					},
				},
				Right: []*opOrTerm{
					{
						Operator: "or",
						Term: &term{
							Left: &booleanValue{
								ConstExpr: &constExpr{
									Boolean: booleanp(false),
								},
							},
						},
					},
				},
			}),
		},
		{
			statement: `false and true or false`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Boolean: booleanp(false),
						},
					},
					Right: []*opAndBooleanValue{
						{
							Operator: "and",
							Value: &booleanValue{
								ConstExpr: &constExpr{
									Boolean: booleanp(true),
								},
							},
						},
					},
				},
				Right: []*opOrTerm{
					{
						Operator: "or",
						Term: &term{
							Left: &booleanValue{
								ConstExpr: &constExpr{
									Boolean: booleanp(false),
								},
							},
						},
					},
				},
			}),
		},
		{
			statement: `(false and true) or false`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						SubExpr: &booleanExpression{
							Left: &term{
								Left: &booleanValue{
									ConstExpr: &constExpr{
										Boolean: booleanp(false),
									},
								},
								Right: []*opAndBooleanValue{
									{
										Operator: "and",
										Value: &booleanValue{
											ConstExpr: &constExpr{
												Boolean: booleanp(true),
											},
										},
									},
								},
							},
						},
					},
				},
				Right: []*opOrTerm{
					{
						Operator: "or",
						Term: &term{
							Left: &booleanValue{
								ConstExpr: &constExpr{
									Boolean: booleanp(false),
								},
							},
						},
					},
				},
			}),
		},
		{
			statement: `false and (true or false)`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Boolean: booleanp(false),
						},
					},
					Right: []*opAndBooleanValue{
						{
							Operator: "and",
							Value: &booleanValue{
								SubExpr: &booleanExpression{
									Left: &term{
										Left: &booleanValue{
											ConstExpr: &constExpr{
												Boolean: booleanp(true),
											},
										},
									},
									Right: []*opOrTerm{
										{
											Operator: "or",
											Term: &term{
												Left: &booleanValue{
													ConstExpr: &constExpr{
														Boolean: booleanp(false),
													},
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
			statement: `name != "foo" and name != "bar"`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Comparison: &comparison{
							Left: value{
								Literal: &mathExprLiteral{
									Path: &Path{
										Fields: []Field{
											{
												Name: "name",
											},
										},
									},
								},
							},
							Op: NE,
							Right: value{
								String: ottltest.Strp("foo"),
							},
						},
					},
					Right: []*opAndBooleanValue{
						{
							Operator: "and",
							Value: &booleanValue{
								Comparison: &comparison{
									Left: value{
										Literal: &mathExprLiteral{
											Path: &Path{
												Fields: []Field{
													{
														Name: "name",
													},
												},
											},
										},
									},
									Op: NE,
									Right: value{
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
			statement: `name == "foo" or name == "bar"`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Comparison: &comparison{
							Left: value{
								Literal: &mathExprLiteral{
									Path: &Path{
										Fields: []Field{
											{
												Name: "name",
											},
										},
									},
								},
							},
							Op: EQ,
							Right: value{
								String: ottltest.Strp("foo"),
							},
						},
					},
				},
				Right: []*opOrTerm{
					{
						Operator: "or",
						Term: &term{
							Left: &booleanValue{
								Comparison: &comparison{
									Left: value{
										Literal: &mathExprLiteral{
											Path: &Path{
												Fields: []Field{
													{
														Name: "name",
													},
												},
											},
										},
									},
									Op: EQ,
									Right: value{
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
			statement: `true and not false`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Boolean: booleanp(true),
						},
					},
					Right: []*opAndBooleanValue{
						{
							Operator: "and",
							Value: &booleanValue{
								Negation: ottltest.Strp("not"),
								ConstExpr: &constExpr{
									Boolean: booleanp(false),
								},
							},
						},
					},
				},
			}),
		},
		{
			statement: `not name == "bar"`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Negation: ottltest.Strp("not"),
						Comparison: &comparison{
							Left: value{
								Literal: &mathExprLiteral{
									Path: &Path{
										Fields: []Field{
											{
												Name: "name",
											},
										},
									},
								},
							},
							Op: EQ,
							Right: value{
								String: ottltest.Strp("bar"),
							},
						},
					},
				},
			}),
		},
		{
			statement: `not (true or false)`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Negation: ottltest.Strp("not"),
						SubExpr: &booleanExpression{
							Left: &term{
								Left: &booleanValue{
									ConstExpr: &constExpr{
										Boolean: booleanp(true),
									},
								},
							},
							Right: []*opOrTerm{
								{
									Operator: "or",
									Term: &term{
										Left: &booleanValue{
											ConstExpr: &constExpr{
												Boolean: booleanp(false),
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
			statement: `True()`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Converter: &converter{
								Function: "True",
							},
						},
					},
				},
			}),
		},
		{
			statement: `True() and False()`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Converter: &converter{
								Function: "True",
							},
						},
					},
					Right: []*opAndBooleanValue{
						{
							Operator: "and",
							Value: &booleanValue{
								ConstExpr: &constExpr{
									Converter: &converter{
										Function: "False",
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
		name := pat.ReplaceAllString(tt.statement, "_")
		t.Run(name, func(t *testing.T) {
			statement := `set(name, "test") where ` + tt.statement
			parsed, err := parseStatement(statement)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, parsed)
		})
	}
}

var testSymbolTable = map[EnumSymbol]Enum{
	"TEST_ENUM":     0,
	"TEST_ENUM_ONE": 1,
	"TEST_ENUM_TWO": 2,
}

func testParseEnum(val *EnumSymbol) (*Enum, error) {
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
func Test_parseStatement(t *testing.T) {
	tests := []struct {
		statement string
		wantErr   bool
	}{
		{`set(`, true},
		{`set("foo)`, true},
		{`set(name.)`, true},
		{`("foo")`, true},
		{`set("foo") where name =||= "fido"`, true},
		{`set(span_id, SpanIDWrapper{not a hex string})`, true},
		{`set(span_id, SpanIDWrapper{01})`, true},
		{`set(span_id, SpanIDWrapper{010203040506070809})`, true},
		{`set(trace_id, TraceIDWrapper{not a hex string})`, true},
		{`set(trace_id, TraceIDWrapper{0102030405060708090a0b0c0d0e0f})`, true},
		{`set(trace_id, TraceIDWrapper{0102030405060708090a0b0c0d0e0f1011})`, true},
		{`set("foo") where name = "fido"`, true},
		{`set("foo") where name or "fido"`, true},
		{`set("foo") where name and "fido"`, true},
		{`set("foo") where name and`, true},
		{`set("foo") where name or`, true},
		{`set("foo") where (`, true},
		{`set("foo") where )`, true},
		{`set("foo") where (name == "fido"))`, true},
		{`set("foo") where ((name == "fido")`, true},
		{`Set()`, true},
		{`set(int())`, true},
		{`set(1 + int())`, true},
		{`set(int() + 1)`, true},
		{`set(1 * int())`, true},
		{`set(1 * 1 + (2 * int()))`, true},
		{`set() where int() == 1`, true},
		{`set() where 1 == int()`, true},
		{`set() where true and 1 == int() `, true},
		{`set() where false or 1 == int() `, true},
		{`set(foo.attributes["bar"].cat, "dog")`, false},
		{`set(foo.attributes["animal"], "dog") where animal == "cat"`, false},
		{`test() where service == "pinger" or foo.attributes["endpoint"] == "/x/alive"`, false},
		{`test() where service == "pinger" or foo.attributes["verb"] == "GET" and foo.attributes["endpoint"] == "/x/alive"`, false},
		{`test() where animal > "cat"`, false},
		{`test() where animal >= "cat"`, false},
		{`test() where animal <= "cat"`, false},
		{`test() where animal < "cat"`, false},
		{`test() where animal =< "dog"`, true},
		{`test() where animal => "dog"`, true},
		{`test() where animal <> "dog"`, true},
		{`test() where animal = "dog"`, true},
		{`test() where animal`, true},
		{`test() where animal ==`, true},
		{`test() where ==`, true},
		{`test() where == animal`, true},
		{`test() where attributes["path"] == "/healthcheck"`, false},
		{`test() where one() == 1`, true},
		{`test(fail())`, true},
		{`Test()`, true},
	}
	pat := regexp.MustCompile("[^a-zA-Z0-9]+")
	for _, tt := range tests {
		name := pat.ReplaceAllString(tt.statement, "_")
		t.Run(name, func(t *testing.T) {
			_, err := parseStatement(tt.statement)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseStatement(%s) error = %v, wantErr %v", tt.statement, err, tt.wantErr)
				return
			}
		})
	}
}

func Test_Execute(t *testing.T) {
	tests := []struct {
		name              string
		condition         boolExpressionEvaluator[interface{}]
		function          ExprFunc[interface{}]
		expectedCondition bool
		expectedResult    interface{}
	}{
		{
			name:      "Condition matched",
			condition: alwaysTrue[interface{}],
			function: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
				return 1, nil
			},
			expectedCondition: true,
			expectedResult:    1,
		},
		{
			name:      "Condition not matched",
			condition: alwaysFalse[interface{}],
			function: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
				return 1, nil
			},
			expectedCondition: false,
			expectedResult:    nil,
		},
		{
			name:      "No result",
			condition: alwaysTrue[interface{}],
			function: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
				return nil, nil
			},
			expectedCondition: true,
			expectedResult:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement := Statement[interface{}]{
				condition: BoolExpr[any]{tt.condition},
				function:  Expr[any]{exprFunc: tt.function},
			}

			result, condition, err := statement.Execute(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCondition, condition)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_Statements_Execute_Error(t *testing.T) {
	tests := []struct {
		name      string
		condition boolExpressionEvaluator[interface{}]
		function  ExprFunc[interface{}]
		errorMode ErrorMode
	}{
		{
			name: "IgnoreError error from condition",
			condition: func(context.Context, interface{}) (bool, error) {
				return true, fmt.Errorf("test")
			},
			function: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
				return 1, nil
			},
			errorMode: IgnoreError,
		},
		{
			name: "PropagateError error from condition",
			condition: func(context.Context, interface{}) (bool, error) {
				return true, fmt.Errorf("test")
			},
			function: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
				return 1, nil
			},
			errorMode: PropagateError,
		},
		{
			name: "IgnoreError error from function",
			condition: func(context.Context, interface{}) (bool, error) {
				return true, nil
			},
			function: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
				return 1, fmt.Errorf("test")
			},
			errorMode: IgnoreError,
		},
		{
			name: "PropagateError error from function",
			condition: func(context.Context, interface{}) (bool, error) {
				return true, nil
			},
			function: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
				return 1, fmt.Errorf("test")
			},
			errorMode: PropagateError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statements := Statements[interface{}]{
				statements: []*Statement[interface{}]{
					{
						condition: BoolExpr[any]{tt.condition},
						function:  Expr[any]{exprFunc: tt.function},
					},
				},
				errorMode:         tt.errorMode,
				telemetrySettings: componenttest.NewNopTelemetrySettings(),
			}

			err := statements.Execute(context.Background(), nil)
			if tt.errorMode == PropagateError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Statements_Eval(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []boolExpressionEvaluator[interface{}]
		function       ExprFunc[interface{}]
		errorMode      ErrorMode
		expectedResult bool
	}{
		{
			name: "True",
			conditions: []boolExpressionEvaluator[interface{}]{
				alwaysTrue[interface{}],
			},
			errorMode:      IgnoreError,
			expectedResult: true,
		},
		{
			name: "At least one True",
			conditions: []boolExpressionEvaluator[interface{}]{
				alwaysFalse[interface{}],
				alwaysFalse[interface{}],
				alwaysTrue[interface{}],
			},
			errorMode:      IgnoreError,
			expectedResult: true,
		},
		{
			name: "False",
			conditions: []boolExpressionEvaluator[interface{}]{
				alwaysFalse[interface{}],
				alwaysFalse[interface{}],
			},
			errorMode:      IgnoreError,
			expectedResult: false,
		},
		{
			name: "Error is false when using Ignore",
			conditions: []boolExpressionEvaluator[interface{}]{
				alwaysFalse[interface{}],
				func(context.Context, interface{}) (bool, error) {
					return true, fmt.Errorf("test")
				},
				alwaysTrue[interface{}],
			},
			errorMode:      IgnoreError,
			expectedResult: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawStatements []*Statement[interface{}]
			for _, condition := range tt.conditions {
				rawStatements = append(rawStatements, &Statement[interface{}]{
					condition: BoolExpr[any]{condition},
					function: Expr[any]{
						exprFunc: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
							return nil, fmt.Errorf("function should not be called")
						},
					},
				})
			}

			statements := Statements[interface{}]{
				statements:        rawStatements,
				telemetrySettings: componenttest.NewNopTelemetrySettings(),
				errorMode:         tt.errorMode,
			}

			result, err := statements.Eval(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_Statements_Eval_Error(t *testing.T) {
	tests := []struct {
		name       string
		conditions []boolExpressionEvaluator[interface{}]
		function   ExprFunc[interface{}]
		errorMode  ErrorMode
	}{
		{
			name: "Propagate Error from function",
			conditions: []boolExpressionEvaluator[interface{}]{
				func(context.Context, interface{}) (bool, error) {
					return true, fmt.Errorf("test")
				},
			},
			errorMode: PropagateError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawStatements []*Statement[interface{}]
			for _, condition := range tt.conditions {
				rawStatements = append(rawStatements, &Statement[interface{}]{
					condition: BoolExpr[any]{condition},
					function: Expr[any]{
						exprFunc: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
							return nil, fmt.Errorf("function should not be called")
						},
					},
				})
			}

			statements := Statements[interface{}]{
				statements:        rawStatements,
				telemetrySettings: componenttest.NewNopTelemetrySettings(),
				errorMode:         tt.errorMode,
			}

			result, err := statements.Eval(context.Background(), nil)
			assert.Error(t, err)
			assert.False(t, result)
		})
	}
}
