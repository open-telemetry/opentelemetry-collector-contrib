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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

// valueFor is a test helper to eliminate a lot of tedium in writing tests of Comparisons.
func valueFor(x any) Value {
	val := Value{}
	switch v := x.(type) {
	case []byte:
		var b Bytes = v
		val.Bytes = &b
	case string:
		switch {
		case v == "NAME":
			// if the string is NAME construct a path of "name".
			val.Path = &Path{
				Fields: []Field{
					{
						Name: "name",
					},
				},
			}
		case strings.Contains(v, "ENUM"):
			// if the string contains ENUM construct an EnumSymbol from it.
			val.Enum = (*EnumSymbol)(ottltest.Strp(v))
		default:
			val.String = ottltest.Strp(v)
		}
	case float64:
		val.Float = ottltest.Floatp(v)
	case *float64:
		val.Float = v
	case int:
		val.Int = ottltest.Intp(int64(v))
	case *int64:
		val.Int = v
	case bool:
		val.Bool = Booleanp(Boolean(v))
	case nil:
		var n IsNil = true
		val.IsNil = &n
	default:
		panic("test error!")
	}
	return val
}

// comparison is a test helper that constructs a Comparison object using valueFor
func comparison(left any, right any, op string) *Comparison {
	return &Comparison{
		Left:  valueFor(left),
		Right: valueFor(right),
		Op:    compareOpTable[op],
	}
}

func Test_newComparisonEvaluator(t *testing.T) {
	p := NewParser(
		defaultFunctionsForTests(),
		testParsePath,
		testParseEnum,
		component.TelemetrySettings{},
	)

	tests := []struct {
		name string
		l    any
		r    any
		op   string
		item interface{}
		want bool
	}{
		{"literals match", "hello", "hello", "==", nil, true},
		{"literals don't match", "hello", "goodbye", "!=", nil, true},
		{"path expression matches", "NAME", "bear", "==", "bear", true},
		{"path expression not matches", "NAME", "cat", "!=", "bear", true},
		{"compare Enum to int", "TEST_ENUM", int(0), "==", nil, true},
		{"compare int to Enum", int(2), "TEST_ENUM_TWO", "==", nil, true},
		{"2 > Enum 0", int(2), "TEST_ENUM", ">", nil, true},
		{"not 2 < Enum 0", int(2), "TEST_ENUM", "<", nil, false},
		{"not 6 == 3.14", 6, 3.14, "==", nil, false},
		{"6 != 3.14", 6, 3.14, "!=", nil, true},
		{"6 > 3.14", 6, 3.14, ">", nil, true},
		{"6 >= 3.14", 6, 3.14, ">=", nil, true},
		{"not 6 < 3.14", 6, 3.14, "<", nil, false},
		{"not 6 <= 3.14", 6, 3.14, "<=", nil, false},
		{"'foo' > 'bar'", "foo", "bar", ">", nil, true},
		{"'foo' > bear", "foo", "NAME", ">", "bear", true},
		{"true > false", true, false, ">", nil, true},
		{"not true > 0", true, 0, ">", nil, false},
		{"not 'true' == true", "true", true, "==", nil, false},
		{"[]byte('a') < []byte('b')", []byte("a"), []byte("b"), "<", nil, true},
		{"nil == nil", nil, nil, "==", nil, true},
		{"nil == []byte(nil)", nil, []byte(nil), "==", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp := comparison(tt.l, tt.r, tt.op)
			evaluate, err := p.newComparisonEvaluator(comp)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, evaluate(ottltest.TestTransformContext{
				Item: tt.item,
			}))
		})
	}
}

func Test_newConditionEvaluator_invalid(t *testing.T) {
	p := NewParser(
		defaultFunctionsForTests(),
		testParsePath,
		testParseEnum,
		component.TelemetrySettings{},
	)

	tests := []struct {
		name       string
		comparison *Comparison
	}{
		{
			name: "unknown Path",
			comparison: &Comparison{
				Left: Value{
					Enum: (*EnumSymbol)(ottltest.Strp("SYMBOL_NOT_FOUND")),
				},
				Op: EQ,
				Right: Value{
					String: ottltest.Strp("trash"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.newComparisonEvaluator(tt.comparison)
			assert.Error(t, err)
		})
	}
}

func Test_newBooleanExpressionEvaluator(t *testing.T) {
	p := NewParser(
		defaultFunctionsForTests(),
		testParsePath,
		testParseEnum,
		component.TelemetrySettings{},
	)

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
			evaluate, err := p.newBooleanExpressionEvaluator(tt.expr)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, evaluate(ottltest.TestTransformContext{
				Item: nil,
			}))
		})
	}
}
