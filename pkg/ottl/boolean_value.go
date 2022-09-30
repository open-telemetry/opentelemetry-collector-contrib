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
	"fmt"
)

// boolExpressionEvaluator is a function that returns the result.
type boolExpressionEvaluator[K any] func(ctx K) bool

func alwaysTrue[K any](K) bool {
	return true
}

func alwaysFalse[K any](K) bool {
	return false
}

// builds a function that returns a short-circuited result of ANDing
// boolExpressionEvaluator funcs
func andFuncs[K any](funcs []boolExpressionEvaluator[K]) boolExpressionEvaluator[K] {
	return func(ctx K) bool {
		for _, f := range funcs {
			if !f(ctx) {
				return false
			}
		}
		return true
	}
}

// builds a function that returns a short-circuited result of ORing
// boolExpressionEvaluator funcs
func orFuncs[K any](funcs []boolExpressionEvaluator[K]) boolExpressionEvaluator[K] {
	return func(ctx K) bool {
		for _, f := range funcs {
			if f(ctx) {
				return true
			}
		}
		return false
	}
}

func (p *Parser[K]) newComparisonEvaluator(comparison *comparison) (boolExpressionEvaluator[K], error) {
	if comparison == nil {
		return alwaysTrue[K], nil
	}
	left, err := p.newGetter(comparison.Left)
	if err != nil {
		return nil, err
	}
	right, err := p.newGetter(comparison.Right)
	if err != nil {
		return nil, err
	}

	// The parser ensures that we'll never get an invalid comparison.Op, so we don't have to check that case.
	return func(ctx K) bool {
		a := left.Get(ctx)
		b := right.Get(ctx)
		return p.compare(a, b, comparison.Op)
	}, nil

}

func (p *Parser[K]) newBooleanExpressionEvaluator(expr *booleanExpression) (boolExpressionEvaluator[K], error) {
	if expr == nil {
		return alwaysTrue[K], nil
	}
	f, err := p.newBooleanTermEvaluator(expr.Left)
	if err != nil {
		return nil, err
	}
	funcs := []boolExpressionEvaluator[K]{f}
	for _, rhs := range expr.Right {
		f, err := p.newBooleanTermEvaluator(rhs.Term)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return orFuncs(funcs), nil
}

func (p *Parser[K]) newBooleanTermEvaluator(term *term) (boolExpressionEvaluator[K], error) {
	if term == nil {
		return alwaysTrue[K], nil
	}
	f, err := p.newBooleanValueEvaluator(term.Left)
	if err != nil {
		return nil, err
	}
	funcs := []boolExpressionEvaluator[K]{f}
	for _, rhs := range term.Right {
		f, err := p.newBooleanValueEvaluator(rhs.Value)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return andFuncs(funcs), nil
}

func (p *Parser[K]) newBooleanValueEvaluator(value *booleanValue) (boolExpressionEvaluator[K], error) {
	if value == nil {
		return alwaysTrue[K], nil
	}
	switch {
	case value.Comparison != nil:
		comparison, err := p.newComparisonEvaluator(value.Comparison)
		if err != nil {
			return nil, err
		}
		return comparison, nil
	case value.ConstExpr != nil:
		if *value.ConstExpr {
			return alwaysTrue[K], nil
		}
		return alwaysFalse[K], nil
	case value.SubExpr != nil:
		return p.newBooleanExpressionEvaluator(value.SubExpr)
	}

	return nil, fmt.Errorf("unhandled boolean operation %v", value)
}
