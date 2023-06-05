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

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
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
