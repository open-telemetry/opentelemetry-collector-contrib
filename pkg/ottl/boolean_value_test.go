// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

// valueFor is a test helper to eliminate a lot of tedium in writing tests of Comparisons.
func valueFor(x any) value {
	val := value{}
	switch v := x.(type) {
	case []byte:
		var b byteSlice = v
		val.Bytes = &b
	case string:
		switch {
		case v == "NAME":
			// if the string is NAME construct a path of "name".
			val.Literal = &mathExprLiteral{
				Path: &Path{
					Fields: []Field{
						{
							Name: "name",
						},
					},
				},
			}
		case strings.Contains(v, "ENUM"):
			// if the string contains ENUM construct an EnumSymbol from it.
			val.Enum = (*EnumSymbol)(ottltest.Strp(v))
		case v == "dur1" || v == "dur2":
			val.Literal = &mathExprLiteral{
				Path: &Path{
					Fields: []Field{
						{
							Name: v,
						},
					},
				},
			}
		case v == "time1" || v == "time2":
			val.Literal = &mathExprLiteral{
				Path: &Path{
					Fields: []Field{
						{
							Name: v,
						},
					},
				},
			}
		default:
			val.String = ottltest.Strp(v)
		}
	case float64:
		val.Literal = &mathExprLiteral{Float: ottltest.Floatp(v)}
	case *float64:
		val.Literal = &mathExprLiteral{Float: v}
	case int:
		val.Literal = &mathExprLiteral{Int: ottltest.Intp(int64(v))}
	case *int64:
		val.Literal = &mathExprLiteral{Int: v}
	case bool:
		val.Bool = booleanp(boolean(v))
	case nil:
		var n isNil = true
		val.IsNil = &n
	default:
		panic("test error!")
	}
	return val
}

// comparison is a test helper that constructs a comparison object using valueFor
func comparisonHelper(left any, right any, op string) *comparison {
	return &comparison{
		Left:  valueFor(left),
		Right: valueFor(right),
		Op:    compareOpTable[op],
	}
}

func Test_newComparisonEvaluator(t *testing.T) {
	p, _ := NewParser(
		defaultFunctionsForTests(),
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	twelveNanoseconds, err := time.ParseDuration("12ns")
	require.NoError(t, err)

	oneMillisecond, err := time.ParseDuration("1ms")
	require.NoError(t, err)

	threeSeconds, err := time.ParseDuration("3s")
	require.NoError(t, err)

	twentyTwoMinutes, err := time.ParseDuration("22m")
	require.NoError(t, err)

	oneHundredThirtyFiveHours, err := time.ParseDuration("135h")
	require.NoError(t, err)

	var tests = []struct {
		name string
		l    any
		r    any
		op   string
		item any
		want bool
	}{
		{name: "literals match", l: "hello", r: "hello", op: "==", want: true},
		{name: "literals don't match", l: "hello", r: "goodbye", op: "!=", want: true},
		{name: "path expression matches", l: "NAME", r: "bear", op: "==", item: "bear", want: true},
		{name: "path expression not matches", l: "NAME", r: "cat", op: "!=", item: "bear", want: true},
		{name: "compare Enum to int", l: "TEST_ENUM", r: 0, op: "==", want: true},
		{name: "compare int to Enum", l: 2, r: "TEST_ENUM_TWO", op: "==", want: true},
		{name: "2 > Enum 0", l: 2, r: "TEST_ENUM", op: ">", want: true},
		{name: "not 2 < Enum 0", l: 2, r: "TEST_ENUM", op: "<"},
		{name: "not 6 == 3.14", l: 6, r: 3.14, op: "=="},
		{name: "6 != 3.14", l: 6, r: 3.14, op: "!=", want: true},
		{name: "6 > 3.14", l: 6, r: 3.14, op: ">", want: true},
		{name: "6 >= 3.14", l: 6, r: 3.14, op: ">=", want: true},
		{name: "not 6 < 3.14", l: 6, r: 3.14, op: "<"},
		{name: "not 6 <= 3.14", l: 6, r: 3.14, op: "<="},
		{name: "'foo' > 'bar'", l: "foo", r: "bar", op: ">", want: true},
		{name: "'foo' > bear", l: "foo", r: "NAME", op: ">", item: "bear", want: true},
		{name: "true > false", l: true, r: false, op: ">", want: true},
		{name: "not true > 0", l: true, r: 0, op: ">"},
		{name: "not 'true' == true", l: "true", r: true, op: "=="},
		{name: "[]byte('a') < []byte('b')", l: []byte("a"), r: []byte("b"), op: "<", want: true},
		{name: "nil == nil", op: "==", want: true},
		{name: "nil == []byte(nil)", r: []byte(nil), op: "==", want: true},
		{name: "compare equal durations", l: "dur1", r: "dur2", op: "==", want: true, item: map[string]time.Duration{"dur1": oneMillisecond, "dur2": oneMillisecond}},
		{name: "compare unequal durations", l: "dur1", r: "dur2", op: "==", want: false, item: map[string]time.Duration{"dur1": oneMillisecond, "dur2": threeSeconds}},
		{name: "compare not equal durations", l: "dur1", r: "dur2", op: "!=", want: true, item: map[string]time.Duration{"dur1": oneMillisecond, "dur2": threeSeconds}},
		{name: "compare not equal durations", l: "dur1", r: "dur2", op: "!=", want: false, item: map[string]time.Duration{"dur1": threeSeconds, "dur2": threeSeconds}},
		{name: "compare less than durations", l: "dur1", r: "dur2", op: "<", want: true, item: map[string]time.Duration{"dur1": oneMillisecond, "dur2": twentyTwoMinutes}},
		{name: "compare not less than durations", l: "dur1", r: "dur2", op: "<", want: false, item: map[string]time.Duration{"dur1": twentyTwoMinutes, "dur2": twentyTwoMinutes}},
		{name: "compare less than equal to durations", l: "dur1", r: "dur2", op: "<=", want: true, item: map[string]time.Duration{"dur1": threeSeconds, "dur2": threeSeconds}},
		{name: "compare not less than equal to durations", l: "dur1", r: "dur2", op: "<=", want: false, item: map[string]time.Duration{"dur1": oneHundredThirtyFiveHours, "dur2": threeSeconds}},
		{name: "compare greater than durations", l: "dur1", r: "dur2", op: ">", want: true, item: map[string]time.Duration{"dur1": oneMillisecond, "dur2": twelveNanoseconds}},
		{name: "compare not greater than durations", l: "dur1", r: "dur2", op: ">", want: false, item: map[string]time.Duration{"dur1": twelveNanoseconds, "dur2": twentyTwoMinutes}},
		{name: "compare greater than equal to durations", l: "dur1", r: "dur2", op: ">=", want: true, item: map[string]time.Duration{"dur1": oneHundredThirtyFiveHours, "dur2": threeSeconds}},
		{name: "compare not greater than equal to durations", l: "dur1", r: "dur2", op: ">=", want: false, item: map[string]time.Duration{"dur1": oneMillisecond, "dur2": threeSeconds}},
		{name: "compare equal times", l: "time1", r: "time2", op: "==", want: true, item: map[string]time.Time{"time1": JanFirst2023, "time2": JanFirst2023}},
		{name: "compare unequal times", l: "time1", r: "time2", op: "==", want: false, item: map[string]time.Time{"time1": JanFirst2023, "time2": time.Date(2023, 1, 2, 0, 0, 0, 0, time.Local)}},
		{name: "compare for not equal times", l: "time1", r: "time2", op: "!=", want: true, item: map[string]time.Time{"time1": JanFirst2023, "time2": time.Date(2002, 11, 2, 01, 01, 01, 01, time.Local)}},
		{name: "compare for equal times using not equal", l: "time1", r: "time2", op: "!=", want: false, item: map[string]time.Time{"time1": time.Date(2002, 11, 2, 01, 01, 01, 01, time.Local), "time2": time.Date(2002, 11, 2, 01, 01, 01, 01, time.Local)}},
		{name: "compare less than times", l: "time1", r: "time2", op: "<", want: true, item: map[string]time.Time{"time1": JanFirst2023, "time2": time.Date(2023, 5, 2, 01, 01, 01, 01, time.Local)}},
		{name: "compare not less than times", l: "time1", r: "time2", op: "<", want: false, item: map[string]time.Time{"time1": time.Date(2023, 6, 2, 01, 01, 01, 01, time.Local), "time2": time.Date(2023, 5, 2, 01, 01, 01, 01, time.Local)}},
		{name: "compare less than equal to times", l: "time1", r: "time2", op: "<=", want: true, item: map[string]time.Time{"time1": time.Date(2003, 5, 2, 01, 01, 01, 01, time.Local), "time2": time.Date(2003, 5, 2, 01, 01, 01, 01, time.Local)}},
		{name: "compare not less than equal to times", l: "time1", r: "time2", op: "<=", want: false, item: map[string]time.Time{"time1": time.Date(2002, 5, 2, 01, 01, 01, 01, time.Local), "time2": time.Date(1999, 5, 2, 01, 01, 01, 01, time.Local)}},
		{name: "compare not greater than equal to w/ times", l: "time1", r: "time2", op: ">=", want: false, item: map[string]time.Time{"time1": time.Date(2002, 5, 2, 01, 01, 01, 01, time.Local), "time2": time.Date(2003, 5, 2, 01, 01, 01, 01, time.Local)}},
		{name: "compare greater than equal to w/ times", l: "time1", r: "time2", op: ">=", want: true, item: map[string]time.Time{"time1": time.Date(2022, 5, 2, 01, 01, 01, 01, time.Local), "time2": time.Date(2003, 5, 2, 01, 01, 01, 01, time.Local)}},
		{name: "compare greater than w/ times", l: "time1", r: "time2", op: ">", want: true, item: map[string]time.Time{"time1": time.Date(2022, 5, 2, 01, 01, 01, 01, time.Local), "time2": time.Date(2003, 5, 2, 01, 01, 01, 01, time.Local)}},
		{name: "compare not greater than w/ times", l: "time1", r: "time2", op: ">", want: false, item: map[string]time.Time{"time1": time.Date(2002, 3, 2, 01, 01, 01, 01, time.Local), "time2": time.Date(2003, 5, 2, 01, 01, 01, 01, time.Local)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp := comparisonHelper(tt.l, tt.r, tt.op)
			evaluator, err := p.newComparisonEvaluator(comp)
			assert.NoError(t, err)
			result, err := evaluator.Eval(context.Background(), tt.item)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_newConditionEvaluator_invalid(t *testing.T) {
	p, _ := NewParser(
		defaultFunctionsForTests(),
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	tests := []struct {
		name       string
		comparison *comparison
	}{
		{
			name: "unknown Path",
			comparison: &comparison{
				Left: value{
					Enum: (*EnumSymbol)(ottltest.Strp("SYMBOL_NOT_FOUND")),
				},
				Op: EQ,
				Right: value{
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

func True() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		return true, nil
	}, nil
}
func False() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		return false, nil
	}, nil
}

func Test_newBooleanExpressionEvaluator(t *testing.T) {
	functions := defaultFunctionsForTests()
	functions["True"] = createFactory("True", &struct{}{}, True)
	functions["False"] = createFactory("False", &struct{}{}, False)

	p, _ := NewParser(
		functions,
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	tests := []struct {
		name string
		want bool
		expr *booleanExpression
	}{
		{"a", false,
			&booleanExpression{
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
			},
		},
		{"b", true,
			&booleanExpression{
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
					},
				},
			},
		},
		{"c", false,
			&booleanExpression{
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
			},
		},
		{"d", true,
			&booleanExpression{
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
		{"e", true,
			&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Boolean: booleanp(false),
						},
					},
				},
				Right: []*opOrTerm{
					{
						Operator: "or",
						Term: &term{
							Left: &booleanValue{
								ConstExpr: &constExpr{
									Boolean: booleanp(true),
								},
							},
						},
					},
				},
			},
		},
		{"f", false,
			&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Boolean: booleanp(false),
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
		{"g", true,
			&booleanExpression{
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
									Boolean: booleanp(false),
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
									Boolean: booleanp(true),
								},
							},
						},
					},
				},
			},
		},
		{"h", true,
			&booleanExpression{
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
			},
		},
		{"i", true,
			&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Negation: ottltest.Strp("not"),
						ConstExpr: &constExpr{
							Boolean: booleanp(false),
						},
					},
				},
			},
		},
		{"j", false,
			&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Negation: ottltest.Strp("not"),
						ConstExpr: &constExpr{
							Boolean: booleanp(true),
						},
					},
				},
			},
		},
		{"k", true,
			&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Negation: ottltest.Strp("not"),
						Comparison: &comparison{
							Left: value{
								String: ottltest.Strp("test"),
							},
							Op: EQ,
							Right: value{
								String: ottltest.Strp("not test"),
							},
						},
					},
				},
			},
		},
		{"l", false,
			&booleanExpression{
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
			},
		},
		{"m", false,
			&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						Negation: ottltest.Strp("not"),
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
				Right: []*opOrTerm{
					{
						Operator: "or",
						Term: &term{
							Left: &booleanValue{
								Negation: ottltest.Strp("not"),
								ConstExpr: &constExpr{
									Boolean: booleanp(true),
								},
							},
						},
					},
				},
			},
		},
		{"n", true,
			&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Converter: &converter{
								Function: "True",
							},
						},
					},
				},
			},
		},
		{"o", false,
			&booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Converter: &converter{
								Function: "False",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator, err := p.newBoolExpr(tt.expr)
			assert.NoError(t, err)
			result, err := evaluator.Eval(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_newBooleanExpressionEvaluator_invalid(t *testing.T) {
	functions := map[string]Factory[any]{"Hello": createFactory("Hello", &struct{}{}, hello)}

	p, _ := NewParser(
		functions,
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	tests := []struct {
		name string
		expr *booleanExpression
	}{
		{
			name: "Converter doesn't return bool",
			expr: &booleanExpression{
				Left: &term{
					Left: &booleanValue{
						ConstExpr: &constExpr{
							Converter: &converter{
								Function: "Hello",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator, err := p.newBoolExpr(tt.expr)
			assert.NoError(t, err)
			_, err = evaluator.Eval(context.Background(), nil)
			assert.Error(t, err)
		})
	}
}
