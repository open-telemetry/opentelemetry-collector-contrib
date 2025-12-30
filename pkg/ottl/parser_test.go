// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

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
			name:      "editor with string",
			statement: `set("foo")`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								String: ottltest.Strp("foo"),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with float",
			statement: `met(1.2)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "met",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Float: ottltest.Floatp(1.2),
								},
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with int",
			statement: `fff(12)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "fff",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Int: ottltest.Intp(12),
								},
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with map",
			statement: `fff({"stringAttr": "value", "intAttr": 3, "floatAttr": 2.5, "boolAttr": true})`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "fff",
					Arguments: []argument{
						{
							Value: value{
								Map: &mapValue{
									Values: []mapItem{
										{
											Key:   ottltest.Strp("stringAttr"),
											Value: &value{String: ottltest.Strp("value")},
										},
										{
											Key: ottltest.Strp("intAttr"),
											Value: &value{
												Literal: &mathExprLiteral{
													Int: ottltest.Intp(3),
												},
											},
										},
										{
											Key: ottltest.Strp("floatAttr"),
											Value: &value{
												Literal: &mathExprLiteral{
													Float: ottltest.Floatp(2.5),
												},
											},
										},
										{
											Key:   ottltest.Strp("boolAttr"),
											Value: &value{Bool: (*boolean)(ottltest.Boolp(true))},
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
			name:      "editor with empty map",
			statement: `fff({})`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "fff",
					Arguments: []argument{
						{
							Value: value{
								Map: &mapValue{
									Values: nil,
								},
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with converter with a map",
			statement: `fff(GetSomething({"foo":"bar"}))`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "fff",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "GetSomething",
										Arguments: []argument{
											{
												Value: value{
													Map: &mapValue{
														Values: []mapItem{
															{
																Key:   ottltest.Strp("foo"),
																Value: &value{String: ottltest.Strp("bar")},
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
			name:      "editor with nested map",
			statement: `fff({"mapAttr": {"foo": "bar", "get": bear.honey, "arrayAttr":["foo", "bar"]}})`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "fff",
					Arguments: []argument{
						{
							Value: value{
								Map: &mapValue{
									Values: []mapItem{
										{
											Key: ottltest.Strp("mapAttr"),
											Value: &value{
												Map: &mapValue{
													Values: []mapItem{
														{
															Key:   ottltest.Strp("foo"),
															Value: &value{String: ottltest.Strp("bar")},
														},
														{
															Key: ottltest.Strp("get"),
															Value: &value{
																Literal: &mathExprLiteral{
																	Path: &path{
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
														},
														{
															Key: ottltest.Strp("arrayAttr"),
															Value: &value{
																List: &list{
																	Values: []value{
																		{
																			String: ottltest.Strp("foo"),
																		},
																		{
																			String: ottltest.Strp("bar"),
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
				},
				WhereClause: nil,
			},
		},
		{
			name:      "complex editor",
			statement: `set("foo", GetSomething(bear.honey))`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								String: ottltest.Strp("foo"),
							},
						},
						{
							Value: value{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "GetSomething",
										Arguments: []argument{
											{
												Value: value{
													Literal: &mathExprLiteral{
														Path: &path{
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
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
						},
						{
							Value: value{
								String: ottltest.Strp("dog"),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "single field segment",
			statement: `set(attributes["bar"], "dog")`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
						},
						{
							Value: value{
								String: ottltest.Strp("dog"),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "Converter parameters (All Uppercase)",
			statement: `replace_pattern(attributes["message"], "device=*", attributes["device_name"], SHA256)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "replace_pattern",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
								String: ottltest.Strp("device=*"),
							},
						},
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
						},
						{
							Value: value{
								Enum: (*enumSymbol)(ottltest.Strp("SHA256")),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "Converter parameters",
			statement: `replace_pattern(attributes["message"], Sha256)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "replace_pattern",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							FunctionName: ottltest.Strp("Sha256"),
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "Converter parameters (One Uppercase symbol)",
			statement: `replace_pattern(attributes["message"], S)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "replace_pattern",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
								Enum: (*enumSymbol)(ottltest.Strp("S")),
							},
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
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
						},
						{
							Value: value{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Test",
										Keys: []key{
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
				},
				WhereClause: nil,
			},
		},
		{
			name:      "where == clause",
			statement: `set(foo.attributes["bar"].cat, "dog") where name == "fido"`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
						},
						{
							Value: value{
								String: ottltest.Strp("dog"),
							},
						},
					},
				},
				WhereClause: &booleanExpression{
					Left: &term{
						Left: &booleanValue{
							Comparison: &comparison{
								Left: value{
									Literal: &mathExprLiteral{
										Path: &path{
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
								Op: eq,
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
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
						},
						{
							Value: value{
								String: ottltest.Strp("dog"),
							},
						},
					},
				},
				WhereClause: &booleanExpression{
					Left: &term{
						Left: &booleanValue{
							Comparison: &comparison{
								Left: value{
									Literal: &mathExprLiteral{
										Path: &path{
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
								Op: ne,
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
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
										Pos: lexer.Position{
											Offset: 7,
											Line:   1,
											Column: 8,
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
						},
						{
							Value: value{
								String: ottltest.Strp("dog"),
							},
						},
					},
				},
				WhereClause: &booleanExpression{
					Left: &term{
						Left: &booleanValue{
							Comparison: &comparison{
								Left: value{
									Literal: &mathExprLiteral{
										Path: &path{
											Pos: lexer.Position{
												Offset: 52,
												Line:   1,
												Column: 53,
											},
											Fields: []field{
												{
													Name: "name",
												},
											},
										},
									},
								},
								Op: eq,
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
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								String: ottltest.Strp("fo\"o"),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with boolean false",
			statement: `convert_gauge_to_sum("cumulative", false)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "convert_gauge_to_sum",
					Arguments: []argument{
						{
							Value: value{
								String: ottltest.Strp("cumulative"),
							},
						},
						{
							Value: value{
								Bool: (*boolean)(ottltest.Boolp(false)),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with boolean true",
			statement: `convert_gauge_to_sum("cumulative", true)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "convert_gauge_to_sum",
					Arguments: []argument{
						{
							Value: value{
								String: ottltest.Strp("cumulative"),
							},
						},
						{
							Value: value{
								Bool: (*boolean)(ottltest.Boolp(true)),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with bytes",
			statement: `set(attributes["bytes"], 0x0102030405060708)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
														String: ottltest.Strp("bytes"),
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Value: value{
								Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with nil",
			statement: `set(attributes["test"], nil)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
								IsNil: (*isNil)(ottltest.Boolp(true)),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with quoted nil",
			statement: `set(attributes["test"], "nil")`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
								String: ottltest.Strp("nil"),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
		{
			name:      "editor with Enum",
			statement: `set(attributes["test"], TEST_ENUM)`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
								Enum: (*enumSymbol)(ottltest.Strp("TEST_ENUM")),
							},
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
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
								List: &list{
									Values: nil,
								},
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
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
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
				},
				WhereClause: nil,
			},
		},
		{
			name:      "Converter with multi-value list",
			statement: `set(attributes["test"], ["value1", "value2"])`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
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
				},
				WhereClause: nil,
			},
		},
		{
			name:      "Converter with nested heterogeneous types",
			statement: `set(attributes["test"], [Concat(["a", "b"], "+"), ["1", 2, 3.0], nil, attributes["test"]])`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
								List: &list{
									Values: []value{
										{
											Literal: &mathExprLiteral{
												Converter: &converter{
													Function: "Concat",
													Arguments: []argument{
														{
															Value: value{
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
														},
														{
															Value: value{
																String: ottltest.Strp("+"),
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
												Path: &path{
													Pos: lexer.Position{
														Offset: 70,
														Line:   1,
														Column: 71,
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
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Value: value{
								Literal: &mathExprLiteral{
									Path: &path{
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
								},
							},
						},
						{
							Value: value{
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
											Operator: sub,
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
												Operator: add,
												Term: &addSubTerm{
													Left: &mathValue{
														Literal: &mathExprLiteral{
															Int: ottltest.Intp(1),
														},
													},
													Right: []*opMultDivValue{
														{
															Operator: mult,
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
								Op: eq,
								Right: value{
									MathExpression: &mathExpression{
										Left: &addSubTerm{
											Left: &mathValue{
												Literal: &mathExprLiteral{
													Path: &path{
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
											Right: []*opMultDivValue{
												{
													Operator: div,
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
		{
			name:      "editor with named arg",
			statement: `set(name="foo")`,
			expected: &parsedStatement{
				Editor: editor{
					Function: "set",
					Arguments: []argument{
						{
							Name: "name",
							Value: value{
								String: ottltest.Strp("foo"),
							},
						},
					},
				},
				WhereClause: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			parsed, err := parseStatement(tt.statement)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, parsed)
		})
	}
}

func Test_parseCondition_full(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		expected  *booleanExpression
	}{
		{
			name:      "where == clause",
			condition: `name == "fido"`,
			expected: &booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Comparison: &comparison{
							Left: value{
								Literal: &mathExprLiteral{
									Path: &path{
										Pos: lexer.Position{
											Offset: 0,
											Line:   1,
											Column: 1,
										},
										Fields: []field{
											{
												Name: "name",
											},
										},
									},
								},
							},
							Op: eq,
							Right: value{
								String: ottltest.Strp("fido"),
							},
						},
					},
				},
			},
		},
		{
			name:      "where != clause",
			condition: `name != "fido"`,
			expected: &booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Comparison: &comparison{
							Left: value{
								Literal: &mathExprLiteral{
									Path: &path{
										Pos: lexer.Position{
											Offset: 0,
											Line:   1,
											Column: 1,
										},
										Fields: []field{
											{
												Name: "name",
											},
										},
									},
								},
							},
							Op: ne,
							Right: value{
								String: ottltest.Strp("fido"),
							},
						},
					},
				},
			},
		},
		{
			name:      "Converter math mathExpression",
			condition: `1 + 1 * 2 == three / One()`,
			expected: &booleanExpression{
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
											Operator: add,
											Term: &addSubTerm{
												Left: &mathValue{
													Literal: &mathExprLiteral{
														Int: ottltest.Intp(1),
													},
												},
												Right: []*opMultDivValue{
													{
														Operator: mult,
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
							Op: eq,
							Right: value{
								MathExpression: &mathExpression{
									Left: &addSubTerm{
										Left: &mathValue{
											Literal: &mathExprLiteral{
												Path: &path{
													Pos: lexer.Position{
														Offset: 13,
														Line:   1,
														Column: 14,
													},
													Fields: []field{
														{
															Name: "three",
														},
													},
												},
											},
										},
										Right: []*opMultDivValue{
											{
												Operator: div,
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
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			parsed, err := parseCondition(tt.condition)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, parsed)
		})
	}
}

func testParsePath[K any](p Path[K]) (GetSetter[any], error) {
	if p != nil && (p.Name() == "name" || p.Name() == "attributes") {
		if p.Name() == "attributes" {
			p.Keys()
		}

		return &StandardGetSetter[any]{
			Getter: func(_ context.Context, tCtx any) (any, error) {
				return tCtx, nil
			},
			Setter: func(_ context.Context, tCtx, val any) error {
				reflect.DeepEqual(tCtx, val)
				return nil
			},
		}, nil
	}
	if p != nil && (p.Name() == "dur1" || p.Name() == "dur2") {
		return &StandardGetSetter[any]{
			Getter: func(_ context.Context, tCtx any) (any, error) {
				m, ok := tCtx.(map[string]time.Duration)
				if !ok {
					return nil, errors.New("unable to convert transform context to map of strings to times")
				}
				return m[p.Name()], nil
			},
			Setter: func(_ context.Context, tCtx, val any) error {
				reflect.DeepEqual(tCtx, val)
				return nil
			},
		}, nil
	}
	if p != nil && (p.Name() == "time1" || p.Name() == "time2") {
		return &StandardGetSetter[any]{
			Getter: func(_ context.Context, tCtx any) (any, error) {
				m, ok := tCtx.(map[string]time.Time)
				if !ok {
					return nil, errors.New("unable to convert transform context to map of strings to times")
				}
				return m[p.Name()], nil
			},
			Setter: func(_ context.Context, tCtx, val any) error {
				reflect.DeepEqual(tCtx, val)
				return nil
			},
		}, nil
	}
	return nil, fmt.Errorf("bad path %v", p)
}

// Helper for test cases where the WHERE clause is all that matters.
// Parse string should start with `set(name, "test") where`...
func setNameTest(b *booleanExpression) *parsedStatement {
	return &parsedStatement{
		Editor: editor{
			Function: "set",
			Arguments: []argument{
				{
					Value: value{
						Literal: &mathExprLiteral{
							Path: &path{
								Pos: lexer.Position{
									Offset: 4,
									Line:   1,
									Column: 5,
								},
								Fields: []field{
									{
										Name: "name",
									},
								},
							},
						},
					},
				},
				{
					Value: value{
						String: ottltest.Strp("test"),
					},
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
									Path: &path{
										Pos: lexer.Position{
											Offset: 24,
											Line:   1,
											Column: 25,
										},
										Fields: []field{
											{
												Name: "name",
											},
										},
									},
								},
							},
							Op: ne,
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
											Path: &path{
												Pos: lexer.Position{
													Offset: 42,
													Line:   1,
													Column: 43,
												},
												Fields: []field{
													{
														Name: "name",
													},
												},
											},
										},
									},
									Op: ne,
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
									Path: &path{
										Pos: lexer.Position{
											Offset: 24,
											Line:   1,
											Column: 25,
										},
										Fields: []field{
											{
												Name: "name",
											},
										},
									},
								},
							},
							Op: eq,
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
											Path: &path{
												Pos: lexer.Position{
													Offset: 41,
													Line:   1,
													Column: 42,
												},
												Fields: []field{
													{
														Name: "name",
													},
												},
											},
										},
									},
									Op: eq,
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
									Path: &path{
										Pos: lexer.Position{
											Offset: 28,
											Line:   1,
											Column: 29,
										},
										Fields: []field{
											{
												Name: "name",
											},
										},
									},
								},
							},
							Op: eq,
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
		{
			statement: `nil == nil`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Comparison: &comparison{
							Left: value{
								IsNil: (*isNil)(ottltest.Boolp(true)),
							},
							Op: eq,
							Right: value{
								IsNil: (*isNil)(ottltest.Boolp(true)),
							},
						},
					},
				},
			}),
		},
		{
			statement: `nil == "nil"`,
			expected: setNameTest(&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Comparison: &comparison{
							Left: value{
								IsNil: (*isNil)(ottltest.Boolp(true)),
							},
							Op: eq,
							Right: value{
								String: ottltest.Strp("nil"),
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
			require.NoError(t, err)
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
		return nil, errors.New("enum symbol not found")
	}
	return nil, errors.New("enum symbol not provided")
}

func Test_ParseValueExpression_full(t *testing.T) {
	time1 := time.Now()
	time2 := time1.Add(5 * time.Second)
	tests := []struct {
		name            string
		valueExpression string
		tCtx            any
		expected        func() any
	}{
		{
			name:            "string value",
			valueExpression: `"fido"`,
			expected: func() any {
				return "fido"
			},
		},
		{
			name:            "resolve context value",
			valueExpression: `attributes`,
			expected: func() any {
				m := pcommon.NewMap()
				m.PutEmptyMap("attributes").PutStr("foo", "bar")
				return m
			},
			tCtx: map[string]any{
				"attributes": map[string]any{
					"foo": "bar",
				},
			},
		},
		{
			name:            "resolve math expression",
			valueExpression: `time2 - time1`,
			expected: func() any {
				return 5 * time.Second
			},
			tCtx: map[string]time.Time{
				"time1": time1,
				"time2": time2,
			},
		},
		{
			name:            "nil",
			valueExpression: `nil`,
			expected: func() any {
				return nil
			},
		},
		{
			name:            "quoted nil",
			valueExpression: `"nil"`,
			expected: func() any {
				return "nil"
			},
		},
		{
			name:            "string",
			valueExpression: `"string"`,
			expected: func() any {
				return "string"
			},
		},
		{
			name:            "hex values",
			valueExpression: `[0x0000000000000000, 0x0000000000000000]`,
			expected: func() any {
				return []any{
					[]uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
					[]uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
				}
			},
		},
		{
			name:            "boolean",
			valueExpression: `true`,
			expected: func() any {
				return true
			},
		},
		{
			name:            "map",
			valueExpression: `{"map": 1}`,
			expected: func() any {
				m := pcommon.NewMap()
				_ = m.FromRaw(map[string]any{
					"map": int64(1),
				})
				return m
			},
		},
		{
			name:            "string list",
			valueExpression: `["list", "of", "strings"]`,
			expected: func() any {
				return []any{"list", "of", "strings"}
			},
		},
		{
			name:            "nested list",
			valueExpression: `[{"list":[{"foo":"bar"}]}, {"bar":"baz"}]`,
			expected: func() any {
				m1 := pcommon.NewMap()
				m1.PutEmptySlice("list").AppendEmpty().SetEmptyMap().PutStr("foo", "bar")

				m2 := pcommon.NewMap()
				m2.PutStr("bar", "baz")
				return []any{
					m1,
					m2,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.valueExpression, func(t *testing.T) {
			p, _ := NewParser(
				CreateFactoryMap[any](),
				testParsePath[any],
				componenttest.NewNopTelemetrySettings(),
				WithEnumParser[any](testParseEnum),
			)
			parsed, err := p.ParseValueExpression(tt.valueExpression)
			require.NoError(t, err)

			v, err := parsed.Eval(t.Context(), tt.tCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected(), v)
		})
	}
}

func Test_ParseValueExpressions_Error(t *testing.T) {
	expressions := []string{
		`123abc`,
		`1+`,
	}

	p, _ := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
	)

	_, err := p.ParseValueExpressions(expressions)
	assert.Error(t, err)

	var e interface{ Unwrap() []error }
	if errors.As(err, &e) {
		uw := e.Unwrap()
		assert.Len(t, uw, len(expressions), "ParseValueExpressions didn't return an error per expression")

		for i, exprErr := range uw {
			assert.ErrorContains(t, exprErr, fmt.Sprintf("unable to parse OTTL value expression %q", expressions[i]))
		}
	} else {
		assert.Fail(t, "ParseValueExpressions didn't return an error per expression")
	}
}

func Test_ParseStatements_Error(t *testing.T) {
	statements := []string{
		`set(`,
		`set("foo)`,
		`set(name.)`,
	}

	p, _ := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	_, err := p.ParseStatements(statements)
	assert.Error(t, err)

	var e interface{ Unwrap() []error }
	if errors.As(err, &e) {
		uw := e.Unwrap()
		assert.Len(t, uw, len(statements), "ParseStatements didn't return an error per statement")

		for i, statementErr := range uw {
			assert.ErrorContains(t, statementErr, fmt.Sprintf("unable to parse OTTL statement %q", statements[i]))
		}
	} else {
		assert.Fail(t, "ParseStatements didn't return an error per statement")
	}
}

func Test_ParseConditions_Error(t *testing.T) {
	conditions := []string{
		`True(`,
		`"foo == "foo"`,
		`set()`,
	}

	p, _ := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	_, err := p.ParseConditions(conditions)

	assert.Error(t, err)

	var e interface{ Unwrap() []error }
	if errors.As(err, &e) {
		uw := e.Unwrap()
		assert.Len(t, uw, len(conditions), "ParseConditions didn't return an error per condition")

		for i, conditionErr := range uw {
			assert.ErrorContains(t, conditionErr, fmt.Sprintf("unable to parse OTTL condition %q", conditions[i]))
		}
	} else {
		assert.Fail(t, "ParseConditions didn't return an error per condition")
	}
}

// This test doesn't validate parser results, simply checks whether the parse succeeds or not.
// It's a fast way to check a large range of possible syntaxes.
func Test_parseStatement(t *testing.T) {
	converterNameErrorPrefix := "converter names must start with an uppercase letter"
	editorWithIndexErrorPrefix := "only paths and converters may be indexed"

	tests := []struct {
		statement         string
		wantErr           bool
		wantErrContaining string
	}{
		{statement: `set(`, wantErr: true},
		{statement: `set("foo)`, wantErr: true},
		{statement: `set(name.)`, wantErr: true},
		{statement: `("foo")`, wantErr: true},
		{statement: `set("foo") where name =||= "fido"`, wantErr: true},
		{statement: `set(span_id, SpanIDWrapper{not a hex string})`, wantErr: true},
		{statement: `set(span_id, SpanIDWrapper{01})`, wantErr: true},
		{statement: `set(span_id, SpanIDWrapper{010203040506070809})`, wantErr: true},
		{statement: `set(trace_id, TraceIDWrapper{not a hex string})`, wantErr: true},
		{statement: `set(trace_id, TraceIDWrapper{0102030405060708090a0b0c0d0e0f})`, wantErr: true},
		{statement: `set(trace_id, TraceIDWrapper{0102030405060708090a0b0c0d0e0f1011})`, wantErr: true},
		{statement: `set("foo") where name = "fido"`, wantErr: true},
		{statement: `set("foo") where name or "fido"`, wantErr: true},
		{statement: `set("foo") where name and "fido"`, wantErr: true},
		{statement: `set("foo") where name and`, wantErr: true},
		{statement: `set("foo") where name or`, wantErr: true},
		{statement: `set("foo") where (`, wantErr: true},
		{statement: `set("foo") where )`, wantErr: true},
		{statement: `set("foo") where (name == "fido"))`, wantErr: true},
		{statement: `set("foo") where ((name == "fido")`, wantErr: true},
		{statement: `Set()`, wantErr: true},
		{statement: `set(int())`, wantErrContaining: converterNameErrorPrefix},
		{statement: `set(1 + int())`, wantErrContaining: converterNameErrorPrefix},
		{statement: `set(int() + 1)`, wantErrContaining: converterNameErrorPrefix},
		{statement: `set(1 * int())`, wantErrContaining: converterNameErrorPrefix},
		{statement: `set(1 * 1 + (2 * int()))`, wantErrContaining: converterNameErrorPrefix},
		{statement: `set() where int() == 1`, wantErrContaining: converterNameErrorPrefix},
		{statement: `set() where 1 == int()`, wantErrContaining: converterNameErrorPrefix},
		{statement: `set() where true and 1 == int() `, wantErrContaining: converterNameErrorPrefix},
		{statement: `set() where false or 1 == int() `, wantErrContaining: converterNameErrorPrefix},
		{statement: `set(foo.attributes["bar"].cat)["key"]`, wantErrContaining: editorWithIndexErrorPrefix},
		{statement: `set(foo.attributes["bar"].cat, "dog")`},
		{statement: `set(set = foo.attributes["animal"], val = "dog") where animal == "cat"`},
		{statement: `test() where service == "pinger" or foo.attributes["endpoint"] == "/x/alive"`},
		{statement: `test() where service == "pinger" or foo.attributes["verb"] == "GET" and foo.attributes["endpoint"] == "/x/alive"`},
		{statement: `test() where animal > "cat"`},
		{statement: `test() where animal >= "cat"`},
		{statement: `test() where animal <= "cat"`},
		{statement: `test() where animal < "cat"`},
		{statement: `test() where animal =< "dog"`, wantErr: true},
		{statement: `test() where animal => "dog"`, wantErr: true},
		{statement: `test() where animal <> "dog"`, wantErr: true},
		{statement: `test() where animal = "dog"`, wantErr: true},
		{statement: `test() where animal`, wantErr: true},
		{statement: `test() where animal ==`, wantErr: true},
		{statement: `test() where ==`, wantErr: true},
		{statement: `test() where == animal`, wantErr: true},
		{statement: `test() where attributes["path"] == "/healthcheck"`},
		{statement: `test() where one() == 1`, wantErr: true},
		{statement: `test(fail())`, wantErrContaining: converterNameErrorPrefix},
		{statement: `Test()`, wantErr: true},
		{statement: `set() where test(foo)["key"] == "bar"`, wantErrContaining: converterNameErrorPrefix},
		{statement: `set() where test(foo)["key"] == "bar"`, wantErrContaining: editorWithIndexErrorPrefix},
	}
	pat := regexp.MustCompile("[^a-zA-Z0-9]+")
	for _, tt := range tests {
		name := pat.ReplaceAllString(tt.statement, "_")
		t.Run(name, func(t *testing.T) {
			ast, err := parseStatement(tt.statement)
			if (err != nil) != (tt.wantErr || tt.wantErrContaining != "") {
				t.Errorf("parseStatement(%s) error = %v, wantErr %v, wantErrContaining %v", tt.statement, err, tt.wantErr, tt.wantErrContaining)
				t.Errorf("AST: %+v", ast)
				return
			}
			if tt.wantErrContaining != "" {
				require.ErrorContains(t, err, tt.wantErrContaining)
			}
		})
	}
}

// This test doesn't validate parser results, simply checks whether the parse succeeds or not.
// It's a fast way to check a large range of possible syntaxes.
func Test_parseCondition(t *testing.T) {
	converterNameErrorPrefix := "converter names must start with an uppercase letter"
	editorWithIndexErrorPrefix := "only paths and converters may be indexed"

	tests := []struct {
		condition         string
		wantErr           bool
		wantErrContaining string
	}{
		{condition: `set(`, wantErr: true},
		{condition: `set("foo)`, wantErr: true},
		{condition: `set(name.)`, wantErr: true},
		{condition: `("foo")`, wantErr: true},
		{condition: `name =||= "fido"`, wantErr: true},
		{condition: `name = "fido"`, wantErr: true},
		{condition: `name or "fido"`, wantErr: true},
		{condition: `name and "fido"`, wantErr: true},
		{condition: `name and`, wantErr: true},
		{condition: `name or`, wantErr: true},
		{condition: `(`, wantErr: true},
		{condition: `)`, wantErr: true},
		{condition: `(name == "fido"))`, wantErr: true},
		{condition: `((name == "fido")`, wantErr: true},
		{condition: `set()`, wantErr: true},
		{condition: `Int() == 1`},
		{condition: `1 == Int()`},
		{condition: `true and 1 == Int() `},
		{condition: `false or 1 == Int() `},
		{condition: `service == "pinger" or foo.attributes["endpoint"] == "/x/alive"`},
		{condition: `service == "pinger" or foo.attributes["verb"] == "GET" and foo.attributes["endpoint"] == "/x/alive"`},
		{condition: `animal > "cat"`},
		{condition: `animal >= "cat"`},
		{condition: `animal <= "cat"`},
		{condition: `animal < "cat"`},
		{condition: `animal =< "dog"`, wantErr: true},
		{condition: `animal => "dog"`, wantErr: true},
		{condition: `animal <> "dog"`, wantErr: true},
		{condition: `animal = "dog"`, wantErr: true},
		{condition: `animal`, wantErr: true},
		{condition: `animal ==`, wantErr: true},
		{condition: `==`, wantErr: true},
		{condition: `== animal`, wantErr: true},
		{condition: `attributes["path"] == "/healthcheck"`},
		{condition: `One() == 1`},
		{condition: `test(fail())`, wantErr: true},
		{condition: `Test()`},
		{condition: `"test" == Foo`, wantErr: true},
		{condition: `test(animal) == "dog"`, wantErrContaining: converterNameErrorPrefix},
		{condition: `test(animal)["kind"] == "birds"`, wantErrContaining: converterNameErrorPrefix},
		{condition: `test(animal)["kind"] == "birds"`, wantErrContaining: editorWithIndexErrorPrefix},
	}
	pat := regexp.MustCompile("[^a-zA-Z0-9]+")
	for _, tt := range tests {
		name := pat.ReplaceAllString(tt.condition, "_")
		t.Run(name, func(t *testing.T) {
			ast, err := parseCondition(tt.condition)
			if (err != nil) != (tt.wantErr || tt.wantErrContaining != "") {
				t.Errorf("parseCondition(%s) error = %v, wantErr %v", tt.condition, err, tt.wantErr)
				t.Errorf("AST: %+v", ast)
				return
			}
			if tt.wantErrContaining != "" {
				require.ErrorContains(t, err, tt.wantErrContaining)
			}
		})
	}
}

// This test doesn't validate parser results, simply checks whether the parse succeeds or not.
// It's a fast way to check a large range of possible syntaxes.
func Test_parseValueExpression(t *testing.T) {
	converterNameErrorPrefix := "converter names must start with an uppercase letter"
	editorWithIndexErrorPrefix := "only paths and converters may be indexed"

	tests := []struct {
		valueExpression   string
		wantErr           bool
		wantErrContaining string
	}{
		{valueExpression: `time_end - time_end`},
		{valueExpression: `time_end - time_end - attributes["foo"]`},
		{valueExpression: `Test("foo")`},
		{valueExpression: `Test(Test("foo")) - attributes["bar"]`},
		{valueExpression: `Test(Test("foo")) - attributes["bar"]"`, wantErr: true},
		{valueExpression: `test("foo")`, wantErr: true, wantErrContaining: converterNameErrorPrefix},
		{valueExpression: `test(animal)["kind"]`, wantErrContaining: editorWithIndexErrorPrefix},
		{valueExpression: `Test("a"")foo"`, wantErr: true},
		{valueExpression: `Test("a"") == 1"`, wantErr: true},
	}
	pat := regexp.MustCompile("[^a-zA-Z0-9]+")
	for _, tt := range tests {
		name := pat.ReplaceAllString(tt.valueExpression, "_")
		t.Run(name, func(t *testing.T) {
			ast, err := parseValueExpression(tt.valueExpression)
			if (err != nil) != (tt.wantErr || tt.wantErrContaining != "") {
				t.Errorf("parseCondition(%s) error = %v, wantErr %v", tt.valueExpression, err, tt.wantErr)
				t.Errorf("AST: %+v", ast)
				return
			}
			if tt.wantErrContaining != "" {
				require.ErrorContains(t, err, tt.wantErrContaining)
			}
		})
	}
}

func Test_Statement_Execute(t *testing.T) {
	tests := []struct {
		name              string
		condition         boolExpr[any]
		function          ExprFunc[any]
		expectedCondition bool
		expectedResult    any
	}{
		{
			name:      "Condition matched",
			condition: newAlwaysTrue[any](),
			function: func(context.Context, any) (any, error) {
				return 1, nil
			},
			expectedCondition: true,
			expectedResult:    1,
		},
		{
			name:      "Condition not matched",
			condition: newAlwaysFalse[any](),
			function: func(context.Context, any) (any, error) {
				return 1, nil
			},
			expectedCondition: false,
			expectedResult:    nil,
		},
		{
			name:      "No result",
			condition: newAlwaysTrue[any](),
			function: func(context.Context, any) (any, error) {
				return nil, nil
			},
			expectedCondition: true,
			expectedResult:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement := Statement[any]{
				condition:         tt.condition,
				function:          Expr[any]{exprFunc: tt.function},
				telemetrySettings: componenttest.NewNopTelemetrySettings(),
			}

			result, condition, err := statement.Execute(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCondition, condition)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_Condition_Eval(t *testing.T) {
	tests := []struct {
		name           string
		condition      boolExpr[any]
		expectedResult bool
	}{
		{
			name:           "Condition matched",
			condition:      newAlwaysTrue[any](),
			expectedResult: true,
		},
		{
			name:           "Condition not matched",
			condition:      newAlwaysFalse[any](),
			expectedResult: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition := Condition[any]{
				condition: tt.condition,
			}

			result, err := condition.Eval(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_Statements_Execute_Error(t *testing.T) {
	tests := []struct {
		name      string
		condition boolExpr[any]
		function  ExprFunc[any]
		errorMode ErrorMode
	}{
		{
			name:      "IgnoreError error from condition",
			condition: newErrExpr[any](errors.New("test")),
			function: func(context.Context, any) (any, error) {
				return 1, nil
			},
			errorMode: IgnoreError,
		},
		{
			name:      "PropagateError error from condition",
			condition: newErrExpr[any](errors.New("test")),
			function: func(context.Context, any) (any, error) {
				return 1, nil
			},
			errorMode: PropagateError,
		},
		{
			name:      "IgnoreError error from function",
			condition: newAlwaysTrue[any](),
			function: func(context.Context, any) (any, error) {
				return 1, errors.New("test")
			},
			errorMode: IgnoreError,
		},
		{
			name:      "PropagateError error from function",
			condition: newAlwaysTrue[any](),
			function: func(context.Context, any) (any, error) {
				return 1, errors.New("test")
			},
			errorMode: PropagateError,
		},
		{
			name:      "SilentError error from condition",
			condition: newErrExpr[any](errors.New("test")),
			function: func(context.Context, any) (any, error) {
				return 1, nil
			},
			errorMode: SilentError,
		},
		{
			name:      "SilentError error from function",
			condition: newAlwaysTrue[any](),
			function: func(context.Context, any) (any, error) {
				return 1, errors.New("test")
			},
			errorMode: SilentError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statements := StatementSequence[any]{
				statements: []*Statement[any]{
					{
						condition:         tt.condition,
						function:          Expr[any]{exprFunc: tt.function},
						telemetrySettings: componenttest.NewNopTelemetrySettings(),
					},
				},
				errorMode:         tt.errorMode,
				telemetrySettings: componenttest.NewNopTelemetrySettings(),
			}

			err := statements.Execute(t.Context(), nil)
			if tt.errorMode == PropagateError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_ConditionSequence_Eval(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []boolExpr[any]
		function       ExprFunc[any]
		errorMode      ErrorMode
		logicOp        LogicOperation
		expectedResult bool
	}{
		{
			name: "True with OR",
			conditions: []boolExpr[any]{
				newAlwaysTrue[any](),
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: true,
		},
		{
			name: "At least one True with OR",
			conditions: []boolExpr[any]{
				newAlwaysFalse[any](),
				newAlwaysFalse[any](),
				newAlwaysTrue[any](),
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: true,
		},
		{
			name: "False with OR",
			conditions: []boolExpr[any]{
				newAlwaysFalse[any](),
				newAlwaysFalse[any](),
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: false,
		},
		{
			name: "Single erroring condition is treated as false when using Ignore with OR",
			conditions: []boolExpr[any]{
				newErrExpr[any](errors.New("test")),
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: false,
		},
		{
			name: "erroring condition is ignored when using Ignore with OR",
			conditions: []boolExpr[any]{
				newErrExpr[any](errors.New("test")),
				newAlwaysTrue[any](),
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: true,
		},
		{
			name: "True with AND",
			conditions: []boolExpr[any]{
				newAlwaysTrue[any](),
				newAlwaysTrue[any](),
			},
			errorMode:      IgnoreError,
			logicOp:        And,
			expectedResult: true,
		},
		{
			name: "At least one False with AND",
			conditions: []boolExpr[any]{
				newAlwaysFalse[any](),
				newAlwaysTrue[any](),
				newAlwaysTrue[any](),
			},
			errorMode:      IgnoreError,
			logicOp:        And,
			expectedResult: false,
		},
		{
			name: "False with AND",
			conditions: []boolExpr[any]{
				newAlwaysFalse[any](),
			},
			errorMode:      IgnoreError,
			logicOp:        And,
			expectedResult: false,
		},
		{
			name: "Single erroring condition is treated as false when using Ignore with AND",
			conditions: []boolExpr[any]{
				newErrExpr[any](errors.New("test")),
			},
			errorMode:      IgnoreError,
			logicOp:        And,
			expectedResult: false,
		},
		{
			name: "erroring condition is ignored when using Ignore with AND",
			conditions: []boolExpr[any]{
				newErrExpr[any](errors.New("test")),
				newAlwaysTrue[any](),
			},
			errorMode:      IgnoreError,
			logicOp:        And,
			expectedResult: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawStatements []*Condition[any]
			for _, condition := range tt.conditions {
				rawStatements = append(rawStatements, &Condition[any]{
					condition: condition,
				})
			}

			conditions := ConditionSequence[any]{
				conditions:        rawStatements,
				telemetrySettings: componenttest.NewNopTelemetrySettings(),
				errorMode:         tt.errorMode,
				logicOp:           tt.logicOp,
			}

			result, err := conditions.Eval(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_ConditionSequence_Eval_Error(t *testing.T) {
	tests := []struct {
		name       string
		conditions []boolExpr[any]
		function   ExprFunc[any]
		errorMode  ErrorMode
	}{
		{
			name: "Propagate Error from condition",
			conditions: []boolExpr[any]{
				newErrExpr[any](errors.New("test")),
			},
			errorMode: PropagateError,
		},
		{
			name: "Ignore Error from function with IgnoreError",
			conditions: []boolExpr[any]{
				newErrExpr[any](errors.New("test")),
			},
			errorMode: IgnoreError,
		},
		{
			name: "Ignore Error from function with SilentError",
			conditions: []boolExpr[any]{
				newErrExpr[any](errors.New("test")),
			},
			errorMode: SilentError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawConditions []*Condition[any]
			for _, condition := range tt.conditions {
				rawConditions = append(rawConditions, &Condition[any]{
					condition: condition,
				})
			}

			conditions := ConditionSequence[any]{
				conditions:        rawConditions,
				telemetrySettings: componenttest.NewNopTelemetrySettings(),
				errorMode:         tt.errorMode,
			}

			result, err := conditions.Eval(t.Context(), nil)
			assert.False(t, result)
			if tt.errorMode == PropagateError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_prependContextToStatementPaths_InvalidStatement(t *testing.T) {
	ps, err := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
		WithPathContextNames[any]([]string{"foo", "bar"}),
	)
	require.NoError(t, err)
	_, err = ps.prependContextToStatementPaths("foo", "this is invalid")
	require.ErrorContains(t, err, `statement has invalid syntax`)
}

func Test_prependContextToStatementPaths_InvalidContext(t *testing.T) {
	ps, err := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
		WithPathContextNames[any]([]string{"foo", "bar"}),
	)
	require.NoError(t, err)
	_, err = ps.prependContextToStatementPaths("foobar", "set(foo, 1)")
	require.ErrorContains(t, err, `unknown context "foobar" for parser`)
}

func Test_prependContextToStatementPaths_Success(t *testing.T) {
	type mockSetArguments[K any] struct {
		Target Setter[K]
		Value  Getter[K]
	}

	mockSetFactory := NewFactory("set", &mockSetArguments[any]{}, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
		return func(context.Context, any) (any, error) {
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
			name:             "no paths",
			statement:        `set("foo", 1)`,
			context:          "bar",
			pathContextNames: []string{"bar"},
			expected:         `set("foo", 1)`,
		},
		{
			name:             "single path with context",
			statement:        `set(span.value, 1)`,
			context:          "span",
			pathContextNames: []string{"span"},
			expected:         `set(span.value, 1)`,
		},
		{
			name:             "single path without context",
			statement:        "set(value, 1)",
			context:          "span",
			pathContextNames: []string{"span"},
			expected:         "set(span.value, 1)",
		},
		{
			name:             "single path with context - multiple context names",
			statement:        "set(span.value, 1)",
			context:          "spanevent",
			pathContextNames: []string{"spanevent", "span"},
			expected:         "set(span.value, 1)",
		},
		{
			name:             "multiple paths with the same context",
			statement:        `set(span.value, 1) where span.attributes["foo"] == "foo" and span.id == 1`,
			context:          "another",
			pathContextNames: []string{"another", "span"},
			expected:         `set(span.value, 1) where span.attributes["foo"] == "foo" and span.id == 1`,
		},
		{
			name:             "multiple paths with different contexts",
			statement:        `set(another.value, 1) where span.attributes["foo"] == "foo" and another.id == 1`,
			context:          "another",
			pathContextNames: []string{"another", "span"},
			expected:         `set(another.value, 1) where span.attributes["foo"] == "foo" and another.id == 1`,
		},
		{
			name:             "multiple paths with and without contexts",
			statement:        `set(value, 1) where span.attributes["foo"] == "foo" and id == 1`,
			context:          "spanevent",
			pathContextNames: []string{"spanevent", "span"},
			expected:         `set(spanevent.value, 1) where span.attributes["foo"] == "foo" and spanevent.id == 1`,
		},
		{
			name:             "multiple paths without context",
			statement:        `set(value, 1) where name == attributes["foo.name"]`,
			context:          "span",
			pathContextNames: []string{"span"},
			expected:         `set(span.value, 1) where span.name == span.attributes["foo.name"]`,
		},
		{
			name:             "function path parameter without context",
			statement:        `set(attributes["test"], "pass") where IsMatch(name, "operation[AC]")`,
			context:          "log",
			pathContextNames: []string{"log"},
			expected:         `set(log.attributes["test"], "pass") where IsMatch(log.name, "operation[AC]")`,
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
			ps, err := NewParser(
				CreateFactoryMap[any](mockSetFactory),
				testParsePath[any],
				componenttest.NewNopTelemetrySettings(),
				WithEnumParser[any](testParseEnum),
				WithPathContextNames[any](tt.pathContextNames),
			)

			require.NoError(t, err)
			require.NotNil(t, ps)

			result, err := ps.prependContextToStatementPaths(tt.context, tt.statement)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_prependContextToConditionPaths_InvalidCondition(t *testing.T) {
	ps, err := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
		WithPathContextNames[any]([]string{"foo", "bar"}),
	)
	require.NoError(t, err)
	_, err = ps.prependContextToConditionPaths("foo", "this is invalid")
	require.ErrorContains(t, err, `condition has invalid syntax`)
}

func Test_prependContextToConditionPaths_InvalidContext(t *testing.T) {
	ps, err := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
		WithPathContextNames[any]([]string{"foo", "bar"}),
	)
	require.NoError(t, err)
	_, err = ps.prependContextToConditionPaths("foobar", "set(foo, 1)")
	require.ErrorContains(t, err, `unknown context "foobar" for parser`)
}

func Test_prependContextToConditionPaths_Success(t *testing.T) {
	type mockSetArguments[K any] struct {
		Target Setter[K]
		Value  Getter[K]
	}

	mockSetFactory := NewFactory("set", &mockSetArguments[any]{}, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
		return func(context.Context, any) (any, error) {
			return nil, nil
		}, nil
	})

	tests := []struct {
		name             string
		condition        string
		context          string
		pathContextNames []string
		expected         string
	}{
		{
			name:             "no paths",
			condition:        `"foo" == 1`,
			context:          "bar",
			pathContextNames: []string{"bar"},
			expected:         `"foo" == 1`,
		},
		{
			name:             "single path with context",
			condition:        `span.value == 1`,
			context:          "span",
			pathContextNames: []string{"span"},
			expected:         `span.value == 1`,
		},
		{
			name:             "single path without context",
			condition:        "value == 1",
			context:          "span",
			pathContextNames: []string{"span"},
			expected:         "span.value == 1",
		},
		{
			name:             "single path with context - multiple context names",
			condition:        "span.value == 1",
			context:          "spanevent",
			pathContextNames: []string{"spanevent", "span"},
			expected:         "span.value == 1",
		},
		{
			name:             "multiple paths with the same context",
			condition:        `span.attributes["foo"] == "foo" and span.id == 1`,
			context:          "another",
			pathContextNames: []string{"another", "span"},
			expected:         `span.attributes["foo"] == "foo" and span.id == 1`,
		},
		{
			name:             "multiple paths with different contexts",
			condition:        `another.value == 1 and span.attributes["foo"] == "foo" and another.id == 1`,
			context:          "another",
			pathContextNames: []string{"another", "span"},
			expected:         `another.value == 1 and span.attributes["foo"] == "foo" and another.id == 1`,
		},
		{
			name:             "multiple paths with and without contexts",
			condition:        `value == 1 and span.attributes["foo"] == "foo" and id == 1`,
			context:          "spanevent",
			pathContextNames: []string{"spanevent", "span"},
			expected:         `spanevent.value == 1 and span.attributes["foo"] == "foo" and spanevent.id == 1`,
		},
		{
			name:             "multiple paths without context",
			condition:        `value == 1 or name == attributes["foo.name"]`,
			context:          "span",
			pathContextNames: []string{"span"},
			expected:         `span.value == 1 or span.name == span.attributes["foo.name"]`,
		},
		{
			name:             "function path parameter without context",
			condition:        `attributes["test"] == "pass" and IsMatch(name, "operation[AC]")`,
			context:          "log",
			pathContextNames: []string{"log"},
			expected:         `log.attributes["test"] == "pass" and IsMatch(log.name, "operation[AC]")`,
		},
		{
			name:             "function path parameter with context",
			condition:        `attributes["test"] == "pass" and IsMatch(resource.name, "operation[AC]")`,
			context:          "log",
			pathContextNames: []string{"log", "resource"},
			expected:         `log.attributes["test"] == "pass" and IsMatch(resource.name, "operation[AC]")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, err := NewParser(
				CreateFactoryMap[any](mockSetFactory),
				testParsePath[any],
				componenttest.NewNopTelemetrySettings(),
				WithEnumParser[any](testParseEnum),
				WithPathContextNames[any](tt.pathContextNames),
			)

			require.NoError(t, err)
			require.NotNil(t, ps)

			result, err := ps.prependContextToConditionPaths(tt.context, tt.condition)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_prependContextToValueExpressionPaths_InvalidValueExpression(t *testing.T) {
	ps, err := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
		WithPathContextNames[any]([]string{"foo", "bar"}),
	)
	require.NoError(t, err)
	_, err = ps.prependContextToValueExpressionPaths("foo", "this is invalid")
	require.ErrorContains(t, err, `expression has invalid syntax`)
}

func Test_prependContextToValueExpressionPaths_InvalidContext(t *testing.T) {
	ps, err := NewParser(
		CreateFactoryMap[any](),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
		WithPathContextNames[any]([]string{"foo", "bar"}),
	)
	require.NoError(t, err)
	_, err = ps.prependContextToValueExpressionPaths("foobar", "set(foo, 1)")
	require.ErrorContains(t, err, `unknown context "foobar" for parser`)
}

func Test_prependContextToValueExpressionPaths_Success(t *testing.T) {
	type mockSetArguments[K any] struct {
		Target Setter[K]
		Value  Getter[K]
	}

	mockSetFactory := NewFactory("set", &mockSetArguments[any]{}, func(_ FunctionContext, _ Arguments) (ExprFunc[any], error) {
		return func(context.Context, any) (any, error) {
			return nil, nil
		}, nil
	})

	tests := []struct {
		name             string
		expression       string
		context          string
		pathContextNames []string
		expected         string
	}{
		{
			name:             "no paths",
			expression:       `"foo"`,
			context:          "bar",
			pathContextNames: []string{"bar"},
			expected:         `"foo"`,
		},
		{
			name:             "single path with context",
			expression:       `span.value`,
			context:          "span",
			pathContextNames: []string{"span"},
			expected:         `span.value`,
		},
		{
			name:             "single path without context",
			expression:       "value",
			context:          "span",
			pathContextNames: []string{"span"},
			expected:         "span.value",
		},
		{
			name:             "single path with context - multiple context names",
			expression:       "span.value",
			context:          "spanevent",
			pathContextNames: []string{"spanevent", "span"},
			expected:         "span.value",
		},
		{
			name:             "multiple paths with the same context",
			expression:       `span.attributes["foo"] + span.id`,
			context:          "another",
			pathContextNames: []string{"another", "span"},
			expected:         `span.attributes["foo"] + span.id`,
		},
		{
			name:             "multiple paths with different contexts",
			expression:       `another.value + span.attributes["foo"] + another.id`,
			context:          "another",
			pathContextNames: []string{"another", "span"},
			expected:         `another.value + span.attributes["foo"] + another.id`,
		},
		{
			name:             "multiple paths with and without contexts",
			expression:       `value + span.attributes["foo"] + id`,
			context:          "spanevent",
			pathContextNames: []string{"spanevent", "span"},
			expected:         `spanevent.value + span.attributes["foo"] + spanevent.id`,
		},
		{
			name:             "multiple paths without context",
			expression:       `value + name + attributes["foo.name"]`,
			context:          "span",
			pathContextNames: []string{"span"},
			expected:         `span.value + span.name + span.attributes["foo.name"]`,
		},
		{
			name:             "function path parameter without context",
			expression:       `attributes["test"] + IsMatch(name, "operation[AC]")`,
			context:          "log",
			pathContextNames: []string{"log"},
			expected:         `log.attributes["test"] + IsMatch(log.name, "operation[AC]")`,
		},
		{
			name:             "function path parameter with context",
			expression:       `attributes["test"] + IsMatch(resource.name, "operation[AC]")`,
			context:          "log",
			pathContextNames: []string{"log", "resource"},
			expected:         `log.attributes["test"] + IsMatch(resource.name, "operation[AC]")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, err := NewParser(
				CreateFactoryMap[any](mockSetFactory),
				testParsePath[any],
				componenttest.NewNopTelemetrySettings(),
				WithEnumParser[any](testParseEnum),
				WithPathContextNames[any](tt.pathContextNames),
			)

			require.NoError(t, err)
			require.NotNil(t, ps)

			result, err := ps.prependContextToValueExpressionPaths(tt.context, tt.expression)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

type errExpr[K any] struct {
	err error
}

func (e *errExpr[K]) Eval(context.Context, K) (bool, error) {
	return false, e.err
}

func (*errExpr[K]) unexported() {}

func newErrExpr[K any](err error) boolExpr[K] {
	return &errExpr[K]{err: err}
}
