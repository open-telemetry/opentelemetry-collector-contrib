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

func newBooleanValueEvaluator(expr *BooleanValue, functions map[string]interface{}, pathParser PathExpressionParser) (condFunc, error) {
	if expr == nil {
		return alwaysTrue, nil
	}
	switch {
	case expr.Condition != nil:
		condition, err := newConditionEvaluator(expr.Condition, functions, pathParser)
		if err != nil {
			return nil, err
		}
		return condition, nil
	case expr.Expr != nil:
	case expr.SubExpr != nil:
	}

	// left, err := NewGetter(cond.Left, functions, pathParser)
	// if err != nil {
	// 	return nil, err
	// }
	// right, err := NewGetter(cond.Right, functions, pathParser)
	// // TODO(anuraaga): Check if both left and right are literals and const-evaluate
	// if err != nil {
	// 	return nil, err
	// }

	// switch cond.Op {
	// case "==":
	// 	return func(ctx TransformContext) bool {
	// 		a := left.Get(ctx)
	// 		b := right.Get(ctx)
	// 		return a == b
	// 	}, nil
	// case "!=":
	// 	return func(ctx TransformContext) bool {
	// 		a := left.Get(ctx)
	// 		b := right.Get(ctx)
	// 		return a != b
	// 	}, nil
	// }

	return nil, fmt.Errorf("unhandled boolean operation %v", expr)
}
