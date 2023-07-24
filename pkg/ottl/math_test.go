// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
	"go.opentelemetry.io/collector/component/componenttest"
)

func mathParsePath(val *Path) (GetSetter[interface{}], error) {
	if val != nil && len(val.Fields) > 0 && val.Fields[0].Name == "one" {
		return &StandardGetSetter[interface{}]{
			Getter: func(context.Context, interface{}) (interface{}, error) {
				return int64(1), nil
			},
		}, nil
	}
	if val != nil && len(val.Fields) > 0 && val.Fields[0].Name == "two" {
		return &StandardGetSetter[interface{}]{
			Getter: func(context.Context, interface{}) (interface{}, error) {
				return int64(2), nil
			},
		}, nil
	}
	if val != nil && len(val.Fields) > 0 && val.Fields[0].Name == "three" && val.Fields[1].Name == "one" {
		return &StandardGetSetter[interface{}]{
			Getter: func(context.Context, interface{}) (interface{}, error) {
				return 3.1, nil
			},
		}, nil
	}
	return nil, fmt.Errorf("bad path %v", val)
}

func one[K any]() (ExprFunc[K], error) {
	return func(context.Context, K) (interface{}, error) {
		return int64(1), nil
	}, nil
}

func two[K any]() (ExprFunc[K], error) {
	return func(context.Context, K) (interface{}, error) {
		return int64(2), nil
	}, nil
}

func threePointOne[K any]() (ExprFunc[K], error) {
	return func(context.Context, K) (interface{}, error) {
		return 3.1, nil
	}, nil
}

func testTime[K any](time string, format string) (ExprFunc[K], error) {
	loc, err := timeutils.GetLocation(nil, &format)
	if err != nil {
		return nil, err
	}
	return func(_ context.Context, tCtx K) (interface{}, error) {
		timestamp, err := timeutils.ParseStrptime(format, time, loc)
		return timestamp, err
	}, nil
}

func testDuration[K any](duration string) (ExprFunc[K], error) {
	return func(_ context.Context, tCtx K) (interface{}, error) {
		dur, err := time.ParseDuration(duration)
		return dur, err
	}, nil
}

type sumArguments struct {
	Ints []int64 `ottlarg:"0"`
}

//nolint:unparam
func sum[K any](ints []int64) (ExprFunc[K], error) {
	return func(context.Context, K) (interface{}, error) {
		result := int64(0)
		for _, x := range ints {
			result += x
		}
		return result, nil
	}, nil
}

func Test_evaluateMathExpression(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "simple subtraction",
			input:    "1000 - 600",
			expected: 400,
		},
		{
			name:     "simple division",
			input:    "1 / 1",
			expected: 1,
		},
		{
			name:     "subtraction and addition",
			input:    "1000 - 600 + 1",
			expected: 401,
		},
		{
			name:     "order of operations",
			input:    "10 - 6 * 2 + 2",
			expected: 0,
		},
		{
			name:     "parentheses",
			input:    "30 - 6 * (2 + 2)",
			expected: 6,
		},
		{
			name:     "complex",
			input:    "(4 * 2) + 1 + 1 - 3 / 3 + ( 2 + 1 - (6 / 3))",
			expected: 10,
		},
		{
			name:     "floats",
			input:    ".5 + 2.6",
			expected: 3.1,
		},
		{
			name:     "complex floats",
			input:    "(.5 * 4.0) / .1 + 3.9",
			expected: 23.9,
		},
		{
			name:     "int paths",
			input:    "one + two",
			expected: 3,
		},
		{
			name:     "float paths",
			input:    "three.one + three.one",
			expected: 6.2,
		},
		{
			name:     "int functions",
			input:    "One() + Two()",
			expected: 3,
		},
		{
			name:     "functions",
			input:    "ThreePointOne() + ThreePointOne()",
			expected: 6.2,
		},
		{
			name:     "functions",
			input:    "Sum([1, 2, 3, 4]) / (1 * 10)",
			expected: 1,
		},
		{
			name:     "int division",
			input:    "10 / 3",
			expected: 3,
		},
		{
			name:     "multiply large ints",
			input:    "9223372036854775807 * 9223372036854775807",
			expected: 1,
		},
		{
			name:     "division by large ints",
			input:    "9223372036854775807 / 9223372036854775807",
			expected: 1,
		},
		{
			name:     "add large ints",
			input:    "9223372036854775807 + 9223372036854775807",
			expected: -2,
		},
		{
			name:     "subtraction by large ints",
			input:    "9223372036854775807 - 9223372036854775807",
			expected: 0,
		},
		{
			name:     "multiply large floats",
			input:    "1.79769313486231570814527423731704356798070e+308 * 1.79769313486231570814527423731704356798070e+308",
			expected: math.Inf(0),
		},
		{
			name:     "division by large floats",
			input:    "1.79769313486231570814527423731704356798070e+308 / 1.79769313486231570814527423731704356798070e+308",
			expected: 1,
		},
		{
			name:     "add large numbers",
			input:    "1.79769313486231570814527423731704356798070e+308 + 1.79769313486231570814527423731704356798070e+308",
			expected: math.Inf(0),
		},
		{
			name:     "subtraction by large numbers",
			input:    "1.79769313486231570814527423731704356798070e+308 - 1.79769313486231570814527423731704356798070e+308",
			expected: 0,
		},
		{
			name:     "x is float, y is int",
			input:    "4.0 / 2",
			expected: 2.0,
		},
		{
			name:     "x is int, y is float",
			input:    "4 / 2.0",
			expected: 2.0,
		},
	}

	functions := CreateFactoryMap(
		createFactory("One", &struct{}{}, one[any]),
		createFactory("Two", &struct{}{}, two[any]),
		createFactory("ThreePointOne", &struct{}{}, threePointOne[any]),
		createFactory("Sum", &sumArguments{}, sum[any]),
	)

	p, _ := NewParser[any](
		functions,
		mathParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	mathParser := newParser[value]()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := mathParser.ParseString("", tt.input)
			assert.NoError(t, err)

			getter, err := p.evaluateMathExpression(parsed.MathExpression)
			assert.NoError(t, err)

			result, err := getter.Get(context.Background(), nil)
			assert.NoError(t, err)

			assert.EqualValues(t, tt.expected, result)
		})
	}
}

func Test_evaluateMathExpression_error(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "divide by 0 is gracefully handled",
			input: "1 / 0",
		},
	}

	functions := CreateFactoryMap(
		createFactory("one", &struct{}{}, one[any]),
		createFactory("two", &struct{}{}, two[any]),
		createFactory("threePointOne", &struct{}{}, threePointOne[any]),
		createFactory("sum", &sumArguments{}, sum[any]),
	)

	p, _ := NewParser[any](
		functions,
		mathParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	mathParser := newParser[value]()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := mathParser.ParseString("", tt.input)
			assert.NoError(t, err)

			getter, err := p.evaluateMathExpression(parsed.MathExpression)
			assert.NoError(t, err)

			result, err := getter.Get(context.Background(), nil)
			assert.Nil(t, result)
			assert.Error(t, err)
		})
	}
}

func Test_evaluateMathExpressionTimeDuration(t *testing.T) {
	functions := CreateFactoryMap(
		createFactory("Time", &struct {
			Time   string `ottlarg:"0"`
			Format string `ottlarg:"1"`
		}{}, testTime[any]),
		createFactory("Duration", &struct {
			Duration string `ottlarg:"0"`
		}{}, testDuration[any]),
	)

	p, _ := NewParser(
		functions,
		mathParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)
	zeroSecs, err := time.ParseDuration("0s")
	require.NoError(t, err)
	fourtyFiveHourseFourtyTwoMinutesTwetySevenSecs, err := time.ParseDuration("45h42m27s")
	require.NoError(t, err)
	oneHundredOne, err := time.ParseDuration("101h101m101s101ns")
	require.NoError(t, err)
	oneThousandHours, err := time.ParseDuration("1000h")
	require.NoError(t, err)
	threeTwentyEightMins, err := time.ParseDuration("328m")
	require.NoError(t, err)
	elevenTwelve, err := time.ParseDuration("11h12m12s11ns")
	require.NoError(t, err)

	var tests = []struct {
		name string

		mathExpr *mathExpression
		expected any
	}{
		{
			name: "time SUB time, no difference",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Time",
								Arguments: []value{
									{
										String: ottltest.Strp("2023-04-12"),
									},
									{
										String: ottltest.Strp("%Y-%m-%d"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: SUB,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Time",
										Arguments: []value{
											{
												String: ottltest.Strp("2023-04-12"),
											},
											{
												String: ottltest.Strp("%Y-%m-%d"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: zeroSecs,
		},
		{
			name: "time SUB time",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Time",
								Arguments: []value{
									{
										String: ottltest.Strp("1986-10-30T00:17:33 MST"),
									},
									{
										String: ottltest.Strp("%Y-%m-%dT%H:%M:%S %Z"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: SUB,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Time",
										Arguments: []value{
											{
												String: ottltest.Strp("1986-11-01"),
											},
											{
												String: ottltest.Strp("%Y-%m-%d"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: -fourtyFiveHourseFourtyTwoMinutesTwetySevenSecs,
		},
		{
			name: "time ADD dur",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Time",
								Arguments: []value{
									{
										String: ottltest.Strp("Feb 15, 2023"),
									},
									{
										String: ottltest.Strp("%b %d, %Y"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: ADD,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Duration",
										Arguments: []value{
											{
												String: ottltest.Strp("10h"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: time.Date(2023, 2, 15, 10, 0, 0, 0, time.Local),
		},
		{
			name: "time ADD dur, complex dur",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Time",
								Arguments: []value{
									{
										String: ottltest.Strp("02/04/2023"),
									},
									{
										String: ottltest.Strp("%m/%d/%Y"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: ADD,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Duration",
										Arguments: []value{
											{
												String: ottltest.Strp("1h2m3s"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: time.Date(2023, 2, 4, 1, 2, 3, 0, time.Local),
		},
		{
			name: "time SUB dur, complex dur",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Time",
								Arguments: []value{
									{
										String: ottltest.Strp("Mar 14 2023 17:02:59"),
									},
									{
										String: ottltest.Strp("%b %d %Y %H:%M:%S"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: SUB,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Duration",
										Arguments: []value{
											{
												String: ottltest.Strp("11h2m58s"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: time.Date(2023, 3, 14, 6, 0, 1, 0, time.Local),
		},
		{
			name: "time SUB dur, nanosecs",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Time",
								Arguments: []value{
									{
										String: ottltest.Strp("Monday, May 01, 2023"),
									},
									{
										String: ottltest.Strp("%A, %B %d, %Y"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: SUB,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Duration",
										Arguments: []value{
											{
												String: ottltest.Strp("100ns"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: time.Date(2023, 4, 30, 23, 59, 59, 999999900, time.Local),
		},
		{
			name: "dur ADD dur, complex durs",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Duration",
								Arguments: []value{
									{
										String: ottltest.Strp("100h100m100s100ns"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: ADD,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Duration",
										Arguments: []value{
											{
												String: ottltest.Strp("1h1m1s1ns"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: oneHundredOne,
		},
		{
			name: "dur ADD dur, zero dur",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Duration",
								Arguments: []value{
									{
										String: ottltest.Strp("0h"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: ADD,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Duration",
										Arguments: []value{
											{
												String: ottltest.Strp("1000h"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: oneThousandHours,
		},
		{
			name: "dur SUB dur, zero dur",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Duration",
								Arguments: []value{
									{
										String: ottltest.Strp("0h"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: SUB,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Duration",
										Arguments: []value{
											{
												String: ottltest.Strp("328m"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: threeTwentyEightMins,
		},
		{
			name: "dur SUB dur, complex durs",
			mathExpr: &mathExpression{
				Left: &addSubTerm{
					Left: &mathValue{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Duration",
								Arguments: []value{
									{
										String: ottltest.Strp("11h11ns"),
									},
								},
							},
						},
					},
				},
				Right: []*opAddSubTerm{
					{
						Operator: SUB,
						Term: &addSubTerm{
							Left: &mathValue{
								Literal: &mathExprLiteral{
									Converter: &converter{
										Function: "Duration",
										Arguments: []value{
											{
												String: ottltest.Strp("12m12s"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: elevenTwelve,
		},
	}
	for _, tt := range tests {

		getter, err := p.evaluateMathExpression(tt.mathExpr)
		assert.NoError(t, err)

		result, err := getter.Get(context.Background(), nil)
		assert.Equal(t, tt.expected, result)
	}
}
