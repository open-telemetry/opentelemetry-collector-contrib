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
															Fields: []field{
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
										Fields: []field{
											{
												Name: "foo",
											},
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
								FunctionName: (ottltest.Strp("Sha256")),
							},
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
										Fields: []field{
											{
												Name: "foo",
											},
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
										Fields: []field{
											{
												Name: "foo",
											},
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
										Fields: []field{
											{
												Name: "foo",
											},
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
										Fields: []field{
											{
												Name: "foo",
											},
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
			assert.NoError(t, err)
			assert.EqualValues(t, tt.expected, parsed)
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
			assert.NoError(t, err)
			assert.EqualValues(t, tt.expected, parsed)
		})
	}
}

func testParsePath[K any](p Path[K]) (GetSetter[any], error) {
	if p != nil && (p.Name() == "name" || p.Name() == "attributes") {
		if p.Name() == "attributes" {
			p.Keys()
		}

		return &StandardGetSetter[any]{
			Getter: func(ctx context.Context, tCtx any) (any, error) {
				return tCtx, nil
			},
			Setter: func(ctx context.Context, tCtx any, val any) error {
				reflect.DeepEqual(tCtx, val)
				return nil
			},
		}, nil
	}
	if p != nil && (p.Name() == "dur1" || p.Name() == "dur2") {
		return &StandardGetSetter[any]{
			Getter: func(ctx context.Context, tCtx any) (any, error) {
				m, ok := tCtx.(map[string]time.Duration)
				if !ok {
					return nil, fmt.Errorf("unable to convert transform context to map of strings to times")
				}
				return m[p.Name()], nil
			},
			Setter: func(ctx context.Context, tCtx any, val any) error {
				reflect.DeepEqual(tCtx, val)
				return nil
			},
		}, nil
	}
	if p != nil && (p.Name() == "time1" || p.Name() == "time2") {
		return &StandardGetSetter[any]{
			Getter: func(ctx context.Context, tCtx any) (any, error) {
				m, ok := tCtx.(map[string]time.Time)
				if !ok {
					return nil, fmt.Errorf("unable to convert transform context to map of strings to times")
				}
				return m[p.Name()], nil
			},
			Setter: func(ctx context.Context, tCtx any, val any) error {
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
		{`set(set = foo.attributes["animal"], val = "dog") where animal == "cat"`, false},
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
			ast, err := parseStatement(tt.statement)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseStatement(%s) error = %v, wantErr %v", tt.statement, err, tt.wantErr)
				t.Errorf("AST: %+v", ast)
				return
			}
		})
	}
}

// This test doesn't validate parser results, simply checks whether the parse succeeds or not.
// It's a fast way to check a large range of possible syntaxes.
func Test_parseCondition(t *testing.T) {
	tests := []struct {
		condition string
		wantErr   bool
	}{
		{`set(`, true},
		{`set("foo)`, true},
		{`set(name.)`, true},
		{`("foo")`, true},
		{`name =||= "fido"`, true},
		{`name = "fido"`, true},
		{`name or "fido"`, true},
		{`name and "fido"`, true},
		{`name and`, true},
		{`name or`, true},
		{`(`, true},
		{`)`, true},
		{`(name == "fido"))`, true},
		{`((name == "fido")`, true},
		{`set()`, true},
		{`Int() == 1`, false},
		{`1 == Int()`, false},
		{`true and 1 == Int() `, false},
		{`false or 1 == Int() `, false},
		{`service == "pinger" or foo.attributes["endpoint"] == "/x/alive"`, false},
		{`service == "pinger" or foo.attributes["verb"] == "GET" and foo.attributes["endpoint"] == "/x/alive"`, false},
		{`animal > "cat"`, false},
		{`animal >= "cat"`, false},
		{`animal <= "cat"`, false},
		{`animal < "cat"`, false},
		{`animal =< "dog"`, true},
		{`animal => "dog"`, true},
		{`animal <> "dog"`, true},
		{`animal = "dog"`, true},
		{`animal`, true},
		{`animal ==`, true},
		{`==`, true},
		{`== animal`, true},
		{`attributes["path"] == "/healthcheck"`, false},
		{`One() == 1`, false},
		{`test(fail())`, true},
		{`Test()`, false},
	}
	pat := regexp.MustCompile("[^a-zA-Z0-9]+")
	for _, tt := range tests {
		name := pat.ReplaceAllString(tt.condition, "_")
		t.Run(name, func(t *testing.T) {
			ast, err := parseCondition(tt.condition)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCondition(%s) error = %v, wantErr %v", tt.condition, err, tt.wantErr)
				t.Errorf("AST: %+v", ast)
				return
			}
		})
	}
}

func Test_Statement_Execute(t *testing.T) {
	tests := []struct {
		name              string
		condition         boolExpressionEvaluator[any]
		function          ExprFunc[any]
		expectedCondition bool
		expectedResult    any
	}{
		{
			name:      "Condition matched",
			condition: alwaysTrue[any],
			function: func(ctx context.Context, tCtx any) (any, error) {
				return 1, nil
			},
			expectedCondition: true,
			expectedResult:    1,
		},
		{
			name:      "Condition not matched",
			condition: alwaysFalse[any],
			function: func(ctx context.Context, tCtx any) (any, error) {
				return 1, nil
			},
			expectedCondition: false,
			expectedResult:    nil,
		},
		{
			name:      "No result",
			condition: alwaysTrue[any],
			function: func(ctx context.Context, tCtx any) (any, error) {
				return nil, nil
			},
			expectedCondition: true,
			expectedResult:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement := Statement[any]{
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

func Test_Condition_Eval(t *testing.T) {
	tests := []struct {
		name           string
		condition      boolExpressionEvaluator[any]
		expectedResult bool
	}{
		{
			name:           "Condition matched",
			condition:      alwaysTrue[any],
			expectedResult: true,
		},
		{
			name:           "Condition not matched",
			condition:      alwaysFalse[any],
			expectedResult: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition := Condition[any]{
				condition: BoolExpr[any]{tt.condition},
			}

			result, err := condition.Eval(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_Statements_Execute_Error(t *testing.T) {
	tests := []struct {
		name      string
		condition boolExpressionEvaluator[any]
		function  ExprFunc[any]
		errorMode ErrorMode
	}{
		{
			name: "IgnoreError error from condition",
			condition: func(context.Context, any) (bool, error) {
				return true, fmt.Errorf("test")
			},
			function: func(ctx context.Context, tCtx any) (any, error) {
				return 1, nil
			},
			errorMode: IgnoreError,
		},
		{
			name: "PropagateError error from condition",
			condition: func(context.Context, any) (bool, error) {
				return true, fmt.Errorf("test")
			},
			function: func(ctx context.Context, tCtx any) (any, error) {
				return 1, nil
			},
			errorMode: PropagateError,
		},
		{
			name: "IgnoreError error from function",
			condition: func(context.Context, any) (bool, error) {
				return true, nil
			},
			function: func(ctx context.Context, tCtx any) (any, error) {
				return 1, fmt.Errorf("test")
			},
			errorMode: IgnoreError,
		},
		{
			name: "PropagateError error from function",
			condition: func(context.Context, any) (bool, error) {
				return true, nil
			},
			function: func(ctx context.Context, tCtx any) (any, error) {
				return 1, fmt.Errorf("test")
			},
			errorMode: PropagateError,
		},
		{
			name: "SilentError error from condition",
			condition: func(context.Context, any) (bool, error) {
				return true, fmt.Errorf("test")
			},
			function: func(ctx context.Context, tCtx any) (any, error) {
				return 1, nil
			},
			errorMode: SilentError,
		},
		{
			name: "SilentError error from function",
			condition: func(context.Context, any) (bool, error) {
				return true, nil
			},
			function: func(ctx context.Context, tCtx any) (any, error) {
				return 1, fmt.Errorf("test")
			},
			errorMode: SilentError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statements := StatementSequence[any]{
				statements: []*Statement[any]{
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

func Test_ConditionSequence_Eval(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []boolExpressionEvaluator[any]
		function       ExprFunc[any]
		errorMode      ErrorMode
		logicOp        LogicOperation
		expectedResult bool
	}{
		{
			name: "True with OR",
			conditions: []boolExpressionEvaluator[any]{
				alwaysTrue[any],
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: true,
		},
		{
			name: "At least one True with OR",
			conditions: []boolExpressionEvaluator[any]{
				alwaysFalse[any],
				alwaysFalse[any],
				alwaysTrue[any],
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: true,
		},
		{
			name: "False with OR",
			conditions: []boolExpressionEvaluator[any]{
				alwaysFalse[any],
				alwaysFalse[any],
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: false,
		},
		{
			name: "Single erroring condition is treated as false when using Ignore with OR",
			conditions: []boolExpressionEvaluator[any]{
				func(context.Context, any) (bool, error) {
					return true, fmt.Errorf("test")
				},
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: false,
		},
		{
			name: "erroring condition is ignored when using Ignore with OR",
			conditions: []boolExpressionEvaluator[any]{
				func(context.Context, any) (bool, error) {
					return true, fmt.Errorf("test")
				},
				alwaysTrue[any],
			},
			errorMode:      IgnoreError,
			logicOp:        Or,
			expectedResult: true,
		},
		{
			name: "True with AND",
			conditions: []boolExpressionEvaluator[any]{
				alwaysTrue[any],
				alwaysTrue[any],
			},
			errorMode:      IgnoreError,
			logicOp:        And,
			expectedResult: true,
		},
		{
			name: "At least one False with AND",
			conditions: []boolExpressionEvaluator[any]{
				alwaysFalse[any],
				alwaysTrue[any],
				alwaysTrue[any],
			},
			errorMode:      IgnoreError,
			logicOp:        And,
			expectedResult: false,
		},
		{
			name: "False with AND",
			conditions: []boolExpressionEvaluator[any]{
				alwaysFalse[any],
			},
			errorMode:      IgnoreError,
			logicOp:        And,
			expectedResult: false,
		},
		{
			name: "Single erroring condition is treated as false when using Ignore with AND",
			conditions: []boolExpressionEvaluator[any]{
				func(context.Context, any) (bool, error) {
					return true, fmt.Errorf("test")
				},
			},
			errorMode:      IgnoreError,
			logicOp:        And,
			expectedResult: false,
		},
		{
			name: "erroring condition is ignored when using Ignore with AND",
			conditions: []boolExpressionEvaluator[any]{
				func(context.Context, any) (bool, error) {
					return true, fmt.Errorf("test")
				},
				alwaysTrue[any],
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
					condition: BoolExpr[any]{condition},
				})
			}

			conditions := ConditionSequence[any]{
				conditions:        rawStatements,
				telemetrySettings: componenttest.NewNopTelemetrySettings(),
				errorMode:         tt.errorMode,
				logicOp:           tt.logicOp,
			}

			result, err := conditions.Eval(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_ConditionSequence_Eval_Error(t *testing.T) {
	tests := []struct {
		name       string
		conditions []boolExpressionEvaluator[any]
		function   ExprFunc[any]
		errorMode  ErrorMode
	}{
		{
			name: "Propagate Error from condition",
			conditions: []boolExpressionEvaluator[any]{
				func(context.Context, any) (bool, error) {
					return true, fmt.Errorf("test")
				},
			},
			errorMode: PropagateError,
		},
		{
			name: "Ignore Error from function with IgnoreError",
			conditions: []boolExpressionEvaluator[any]{
				func(context.Context, any) (bool, error) {
					return true, fmt.Errorf("test")
				},
			},
			errorMode: IgnoreError,
		},
		{
			name: "Ignore Error from function with SilentError",
			conditions: []boolExpressionEvaluator[any]{
				func(context.Context, any) (bool, error) {
					return true, fmt.Errorf("test")
				},
			},
			errorMode: SilentError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawConditions []*Condition[any]
			for _, condition := range tt.conditions {
				rawConditions = append(rawConditions, &Condition[any]{
					condition: BoolExpr[any]{condition},
				})
			}

			conditions := ConditionSequence[any]{
				conditions:        rawConditions,
				telemetrySettings: componenttest.NewNopTelemetrySettings(),
				errorMode:         tt.errorMode,
			}

			result, err := conditions.Eval(context.Background(), nil)
			assert.False(t, result)
			if tt.errorMode == PropagateError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
