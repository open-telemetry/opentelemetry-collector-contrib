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

	"github.com/gobwas/glob"
)

type condFunc = func(ctx TransformContext) bool

var alwaysTrue = func(ctx TransformContext) bool {
	return true
}

var likeErrorMessage = "the value to the right of a 'like' condition must be a literal string."

func newConditionEvaluator(cond *Condition, functions map[string]interface{}, pathParser PathExpressionParser) (condFunc, error) {
	if cond == nil {
		return alwaysTrue, nil
	}
	left, err := NewGetter(cond.Left, functions, pathParser)
	if err != nil {
		return nil, err
	}
	right, err := NewGetter(cond.Right, functions, pathParser)
	// TODO(anuraaga): Check if both left and right are literals and const-evaluate
	if err != nil {
		return nil, err
	}

	switch cond.Op {
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
	case "like":

		l, ok := right.(*literal)
		if !ok {
			return nil, fmt.Errorf(likeErrorMessage)
		}
		pattern, ok := l.value.(string)
		if !ok {
			return nil, fmt.Errorf(likeErrorMessage)
		}

		glob, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("the pattern supplied to the right of 'like' is not a valid pattern: %w", err)
		}

		return func(ctx TransformContext) bool {
			a := left.Get(ctx)
			if aStr, ok := a.(string); ok {
				return glob.Match(aStr)
			}
			panic(fmt.Errorf("the value supplied to the left of 'like' was not a string"))
		}, nil
	}

	return nil, fmt.Errorf("unrecognized boolean operation %v", cond.Op)
}
