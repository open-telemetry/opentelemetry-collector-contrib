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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
)

func Test_newComparisonEvaluator(t *testing.T) {
	tests := []struct {
		name       string
		comparison *Comparison
		item       interface{}
	}{
		{
			name: "literals match",
			comparison: &Comparison{
				Left: Value{
					String: tqltest.Strp("hello"),
				},
				Right: Value{
					String: tqltest.Strp("hello"),
				},
				Op: "==",
			},
		},
		{
			name: "literals don't match",
			comparison: &Comparison{
				Left: Value{
					String: tqltest.Strp("hello"),
				},
				Right: Value{
					String: tqltest.Strp("goodbye"),
				},
				Op: "!=",
			},
		},
		{
			name: "path expression matches",
			comparison: &Comparison{
				Left: Value{
					Path: &Path{
						Fields: []Field{
							{
								Name: "name",
							},
						},
					},
				},
				Right: Value{
					String: tqltest.Strp("bear"),
				},
				Op: "==",
			},
			item: "bear",
		},
		{
			name: "path expression not matches",
			comparison: &Comparison{
				Left: Value{
					Path: &Path{
						Fields: []Field{
							{
								Name: "name",
							},
						},
					},
				},
				Right: Value{
					String: tqltest.Strp("cat"),
				},
				Op: "!=",
			},
			item: "bear",
		},
		{
			name:       "no condition",
			comparison: nil,
		},

		{
			name: "compare Enum to int",
			comparison: &Comparison{
				Left: Value{
					Enum: (*EnumSymbol)(tqltest.Strp("TEST_ENUM")),
				},
				Right: Value{
					Int: tqltest.Intp(0),
				},
				Op: "==",
			},
		},
		{
			name: "compare int to Enum",
			comparison: &Comparison{
				Left: Value{
					Int: tqltest.Intp(2),
				},
				Op: "==",
				Right: Value{
					Enum: (*EnumSymbol)(tqltest.Strp("TEST_ENUM_TWO")),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := newComparisonEvaluator(tt.comparison, DefaultFunctionsForTests(), testParsePath, testParseEnum)
			assert.NoError(t, err)
			assert.True(t, evaluate(tqltest.TestTransformContext{
				Item: tt.item,
			}))
		})
	}
}

func Test_newConditionEvaluator_invalid(t *testing.T) {
	tests := []struct {
		name       string
		comparison *Comparison
	}{
		{
			name: "unknown operation",
			comparison: &Comparison{
				Left: Value{
					String: tqltest.Strp("bear"),
				},
				Op: "<>",
				Right: Value{
					String: tqltest.Strp("cat"),
				},
			},
		},
		{
			name: "unknown Path",
			comparison: &Comparison{
				Left: Value{
					Enum: (*EnumSymbol)(tqltest.Strp("SYMBOL_NOT_FOUND")),
				},
				Op: "==",
				Right: Value{
					String: tqltest.Strp("trash"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newComparisonEvaluator(tt.comparison, DefaultFunctionsForTests(), testParsePath, testParseEnum)
			assert.Error(t, err)
		})
	}
}

func Test_newBooleanExpressionEvaluator(t *testing.T) {
	tests := []struct {
		name string
		want bool
		expr *BooleanExpression
	}{
		{"a", false,
			&BooleanExpression{
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
			},
		},
		{"b", true,
			&BooleanExpression{
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
					},
				},
			},
		},
		{"c", false,
			&BooleanExpression{
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
			},
		},
		{"d", true,
			&BooleanExpression{
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
		{"e", true,
			&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(false),
					},
				},
				Right: []*OpOrTerm{
					{
						Operator: "or",
						Term: &Term{
							Left: &BooleanValue{
								ConstExpr: Booleanp(true),
							},
						},
					},
				},
			},
		},
		{"f", false,
			&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(false),
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
		{"g", true,
			&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(false),
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
				Right: []*OpOrTerm{
					{
						Operator: "or",
						Term: &Term{
							Left: &BooleanValue{
								ConstExpr: Booleanp(true),
							},
						},
					},
				},
			},
		},
		{"h", true,
			&BooleanExpression{
				Left: &Term{
					Left: &BooleanValue{
						ConstExpr: Booleanp(true),
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := newBooleanExpressionEvaluator(tt.expr, DefaultFunctionsForTests(), testParsePath, testParseEnum)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, evaluate(tqltest.TestTransformContext{
				Item: nil,
			}))
		})
	}
}
