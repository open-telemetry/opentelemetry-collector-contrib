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

import "fmt"

func (p *Parser[K]) evaluateMathExpression(expr *expression) (Getter[K], error) {
	mainGetter, err := p.evaluateAddSubTerm(expr.Left)
	if err != nil {
		return nil, err
	}
	for _, rhs := range expr.Right {
		getter, err := p.evaluateAddSubTerm(rhs.Term)
		if err != nil {
			return nil, err
		}
		mainGetter = attemptMathOperation(mainGetter, rhs.Operator, getter)
	}

	return mainGetter, nil
}

func (p *Parser[K]) evaluateAddSubTerm(term *addSubTerm) (Getter[K], error) {
	mainGetter, err := p.evaluateMathValue(term.Left)
	if err != nil {
		return nil, err
	}
	for _, rhs := range term.Right {
		getter, err := p.evaluateMathValue(rhs.Value)
		if err != nil {
			return nil, err
		}
		mainGetter = attemptMathOperation(mainGetter, rhs.Operator, getter)
	}

	return mainGetter, nil
}

func (p *Parser[K]) evaluateMathValue(val *mathValue) (Getter[K], error) {
	switch {
	case val.Literal != nil:
		return p.newGetter(value{Literal: val.Literal})
	case val.SubExpression != nil:
		return p.evaluateMathExpression(val.SubExpression)
	}

	return nil, fmt.Errorf("unhandled mathmatic operation %v", val)
}

func attemptMathOperation[K any](lhs Getter[K], op mathOp, rhs Getter[K]) Getter[K] {
	return exprGetter[K]{
		expr: func(ctx K) (interface{}, error) {
			x, err := lhs.Get(ctx)
			if err != nil {
				return nil, err
			}
			y, err := rhs.Get(ctx)
			if err != nil {
				return nil, err
			}
			switch newX := x.(type) {
			case int64:
				intY, ok := y.(int64)
				if !ok {
					return nil, fmt.Errorf("cannot convert %v to int64", y)
				}
				return performOp[int64](newX, intY, op), nil
			case float64:
				floatY, ok := y.(float64)
				if !ok {
					return nil, fmt.Errorf("cannot convert %v to float64", y)
				}
				return performOp[float64](newX, floatY, op), nil
			default:
				return nil, fmt.Errorf("%v must be int64 or float64", x)
			}
		},
	}
}

func performOp[N int64 | float64](x N, y N, op mathOp) N {
	switch op {
	case ADD:
		return x + y
	case SUB:
		return x - y
	case MULT:
		return x * y
	case DIV:
		return x / y
	}
	return 0
}
