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

package tql // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"

import (
	"fmt"
)

// BoolExpressionEvaluator is a function that returns the result.
type BoolExpressionEvaluator = func(ctx TransformContext) bool

var alwaysTrue = func(ctx TransformContext) bool {
	return true
}

var alwaysFalse = func(ctx TransformContext) bool {
	return false
}

// builds a function that returns a short-circuited result of ANDing
// BoolExpressionEvaluator funcs
func andFuncs(funcs []BoolExpressionEvaluator) BoolExpressionEvaluator {
	return func(ctx TransformContext) bool {
		for _, f := range funcs {
			if !f(ctx) {
				return false
			}
		}
		return true
	}
}

// builds a function that returns a short-circuited result of ORing
// BoolExpressionEvaluator funcs
func orFuncs(funcs []BoolExpressionEvaluator) BoolExpressionEvaluator {
	return func(ctx TransformContext) bool {
		for _, f := range funcs {
			if f(ctx) {
				return true
			}
		}
		return false
	}
}

func newComparisonEvaluator(comparison *Comparison, functions map[string]interface{}, pathParser PathExpressionParser, enumParser EnumParser) (BoolExpressionEvaluator, error) {
	if comparison == nil {
		return alwaysTrue, nil
	}
	left, err := NewGetter(comparison.Left, functions, pathParser, enumParser)
	if err != nil {
		return nil, err
	}
	right, err := NewGetter(comparison.Right, functions, pathParser, enumParser)
	// TODO(anuraaga): Check if both left and right are literals and const-evaluate
	if err != nil {
		return nil, err
	}

	switch comparison.Op {
	case "==":
		return func(ctx TransformContext) bool {
			a := left.Get(ctx)
			b := right.Get(ctx)
			return a == b
		}, nil
	case "!=":
		return func(ctx TransformContext) bool {
			a := left.Get(ctx)
			b := right.Get(ctx)
			return a != b
		}, nil
	}

	return nil, fmt.Errorf("unrecognized boolean operation %v", comparison.Op)
}

func newBooleanExpressionEvaluator(expr *BooleanExpression, functions map[string]interface{}, pathParser PathExpressionParser, enumParser EnumParser) (BoolExpressionEvaluator, error) {
	if expr == nil {
		return alwaysTrue, nil
	}
	f, err := newBooleanTermEvaluator(expr.Left, functions, pathParser, enumParser)
	if err != nil {
		return nil, err
	}
	funcs := []BoolExpressionEvaluator{f}
	for _, rhs := range expr.Right {
		f, err := newBooleanTermEvaluator(rhs.Term, functions, pathParser, enumParser)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return orFuncs(funcs), nil
}

func newBooleanTermEvaluator(term *Term, functions map[string]interface{}, pathParser PathExpressionParser, enumParser EnumParser) (BoolExpressionEvaluator, error) {
	if term == nil {
		return alwaysTrue, nil
	}
	f, err := newBooleanValueEvaluator(term.Left, functions, pathParser, enumParser)
	if err != nil {
		return nil, err
	}
	funcs := []BoolExpressionEvaluator{f}
	for _, rhs := range term.Right {
		f, err := newBooleanValueEvaluator(rhs.Value, functions, pathParser, enumParser)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return andFuncs(funcs), nil
}

func newBooleanValueEvaluator(value *BooleanValue, functions map[string]interface{}, pathParser PathExpressionParser, enumParser EnumParser) (BoolExpressionEvaluator, error) {
	if value == nil {
		return alwaysTrue, nil
	}
	switch {
	case value.Comparison != nil:
		comparison, err := newComparisonEvaluator(value.Comparison, functions, pathParser, enumParser)
		if err != nil {
			return nil, err
		}
		return comparison, nil
	case value.ConstExpr != nil:
		if *value.ConstExpr {
			return alwaysTrue, nil
		}
		return alwaysFalse, nil
	case value.SubExpr != nil:
		return newBooleanExpressionEvaluator(value.SubExpr, functions, pathParser, enumParser)
	}

	return nil, fmt.Errorf("unhandled boolean operation %v", value)
}
