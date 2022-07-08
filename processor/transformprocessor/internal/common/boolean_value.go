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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"fmt"
)

func newBooleanExpressionEvaluator(expr *BooleanExpression, functions map[string]interface{}, pathParser PathExpressionParser) (condFunc, error) {
	if expr == nil {
		return alwaysTrue, nil
	}
	f, err := newBooleanTermEvaluator(expr.Left, functions, pathParser)
	if err != nil {
		return nil, err
	}
	funcs := []condFunc{f}
	for _, rhs := range expr.Right {
		f, err := newBooleanTermEvaluator(rhs.Term, functions, pathParser)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return orFuncs(funcs), nil
}

func newBooleanTermEvaluator(term *Term, functions map[string]interface{}, pathParser PathExpressionParser) (condFunc, error) {
	if term == nil {
		return alwaysTrue, nil
	}
	f, err := newBooleanValueEvaluator(term.Left, functions, pathParser)
	if err != nil {
		return nil, err
	}
	funcs := []condFunc{f}
	for _, rhs := range term.Right {
		f, err := newBooleanValueEvaluator(rhs.Value, functions, pathParser)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, f)
	}

	return andFuncs(funcs), nil
}

func newBooleanValueEvaluator(value *BooleanValue, functions map[string]interface{}, pathParser PathExpressionParser) (condFunc, error) {
	if value == nil {
		return alwaysTrue, nil
	}
	switch {
	case value.Condition != nil:
		condition, err := newConditionEvaluator(value.Condition, functions, pathParser)
		if err != nil {
			return nil, err
		}
		return condition, nil
	case value.ConstExpr != nil:
		return func(ctx TransformContext) bool {
			return bool(*value.ConstExpr)
		}, nil
	case value.SubExpr != nil:
		return newBooleanExpressionEvaluator(value.SubExpr, functions, pathParser)
	}

	return nil, fmt.Errorf("unhandled boolean operation %v", value)
}
