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
type boolExpressionEvaluator = func(ctx TransformContext) bool

var alwaysTrue = func(ctx TransformContext) bool {
	return true
}

var alwaysFalse = func(ctx TransformContext) bool {
	return false
}

// builds a function that returns a short-circuited result of ANDing
// boolExpressionEvaluator funcs
func andFuncs(funcs []boolExpressionEvaluator) boolExpressionEvaluator {
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
// boolExpressionEvaluator funcs
func orFuncs(funcs []boolExpressionEvaluator) boolExpressionEvaluator {
	return func(ctx TransformContext) bool {
		for _, f := range funcs {
			if f(ctx) {
				return true
			}
		}
		return false
	}
}

func (p *Parser) newComparisonEvaluator(comparison *Comparison) (boolExpressionEvaluator, error) {
	if comparison == nil {
		return alwaysTrue, nil
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
	return func(ctx TransformContext) bool {
		a := left.Get(ctx)
		b := right.Get(ctx)
		return compare(a, b, comparison.Op)
	}, nil

}

func (p *Parser) newBooleanExpressionEvaluator(expr *BooleanExpression) (boolExpressionEvaluator, error) {
	if expr == nil {
		return alwaysTrue, nil
	}
	f, err := p.newBooleanTermEvaluator(expr.Left)
	if err != nil {
		return nil, err
	}
	funcs := []boolExpressionEvaluator{f}
	for _, rhs := range expr.Right {
		f, err := p.newBooleanTermEvaluator(rhs.Term)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return orFuncs(funcs), nil
}

func (p *Parser) newBooleanTermEvaluator(term *Term) (boolExpressionEvaluator, error) {
	if term == nil {
		return alwaysTrue, nil
	}
	f, err := p.newBooleanValueEvaluator(term.Left)
	if err != nil {
		return nil, err
	}
	funcs := []boolExpressionEvaluator{f}
	for _, rhs := range term.Right {
		f, err := p.newBooleanValueEvaluator(rhs.Value)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return andFuncs(funcs), nil
}

func (p *Parser) newBooleanValueEvaluator(value *BooleanValue) (boolExpressionEvaluator, error) {
	if value == nil {
		return alwaysTrue, nil
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
			return alwaysTrue, nil
		}
		return alwaysFalse, nil
	case value.SubExpr != nil:
		return p.newBooleanExpressionEvaluator(value.SubExpr)
	}

	return nil, fmt.Errorf("unhandled boolean operation %v", value)
}
