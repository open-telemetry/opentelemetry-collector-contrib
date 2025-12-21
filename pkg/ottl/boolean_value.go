// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/runtime"
)

func (p *Parser[K]) newComparisonExpr(comparison *comparison) (runtime.BoolExpr[K], error) {
	if comparison == nil {
		return runtime.NewAlwaysTrue[K](), nil
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
	return runtime.NewComparisonExpr[K](left, right, comparison.Op), nil
}

func (p *Parser[K]) newBoolExpr(expr *booleanExpression) (runtime.BoolExpr[K], error) {
	if expr == nil {
		return runtime.NewAlwaysTrue[K](), nil
	}
	f, err := p.newBooleanTermEvaluator(expr.Left)
	if err != nil {
		return nil, err
	}
	funcs := []runtime.BoolExpr[K]{f}
	for _, rhs := range expr.Right {
		f, err = p.newBooleanTermEvaluator(rhs.Term)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return runtime.NewOrExprs(funcs), nil
}

func (p *Parser[K]) newBooleanTermEvaluator(term *term) (runtime.BoolExpr[K], error) {
	if term == nil {
		return runtime.NewAlwaysTrue[K](), nil
	}
	f, err := p.newBooleanValueEvaluator(term.Left)
	if err != nil {
		return nil, err
	}
	funcs := []runtime.BoolExpr[K]{f}
	for _, rhs := range term.Right {
		f, err = p.newBooleanValueEvaluator(rhs.Value)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return runtime.NewAndExprs(funcs), nil
}

func (p *Parser[K]) newBooleanValueEvaluator(value *booleanValue) (runtime.BoolExpr[K], error) {
	if value == nil {
		return runtime.NewAlwaysTrue[K](), nil
	}

	var boolExpr runtime.BoolExpr[K]
	var err error
	switch {
	case value.Comparison != nil:
		boolExpr, err = p.newComparisonExpr(value.Comparison)
		if err != nil {
			return nil, err
		}
	case value.ConstExpr != nil:
		switch {
		case value.ConstExpr.Boolean != nil:
			if *value.ConstExpr.Boolean {
				boolExpr = runtime.NewAlwaysTrue[K]()
			} else {
				boolExpr = runtime.NewAlwaysFalse[K]()
			}
		case value.ConstExpr.Converter != nil:
			boolExpr, err = p.newConverterEvaluator(*value.ConstExpr.Converter)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unhandled boolean operation %v", value)
		}
	case value.SubExpr != nil:
		boolExpr, err = p.newBoolExpr(value.SubExpr)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unhandled boolean operation %v", value)
	}

	if value.Negation != nil {
		return runtime.NewNot(boolExpr), nil
	}
	return boolExpr, nil
}

func (p *Parser[K]) newConverterEvaluator(c converter) (runtime.BoolExpr[K], error) {
	getter, err := p.newGetterFromConverter(c)
	if err != nil {
		return nil, err
	}

	return runtime.NewConverterExpr(getter)
}
