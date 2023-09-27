// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ReplacePatternArguments[K any] struct {
	Target       ottl.GetSetter[K]
	RegexPattern string
	Replacement  ottl.StringGetter[K]
	Function     ottl.Optional[ottl.FunctionGetter[K]]
}

type FuncArgs[K any] struct {
	Input ottl.StringGetter[K]
}

func NewReplacePatternFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("replace_pattern", &ReplacePatternArguments[K]{}, createReplacePatternFunction[K])
}

func createReplacePatternFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ReplacePatternArguments[K])

	if !ok {
		return nil, fmt.Errorf("ReplacePatternFactory args must be of type *ReplacePatternArguments[K]")
	}

	return replacePattern(args.Target, args.RegexPattern, args.Replacement, args.Function)
}

func replacePattern[K any](target ottl.GetSetter[K], regexPattern string, replacement ottl.StringGetter[K], fn ottl.Optional[ottl.FunctionGetter[K]]) (ottl.ExprFunc[K], error) {
	var replacementVal string
	compiledPattern, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("the regex pattern supplied to replace_pattern is not a valid pattern: %w", err)
	}
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		originalVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if fn.IsEmpty() {
			replacementVal, err = replacement.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
		} else {
			fnVal := fn.Get()
			replacementExpr, err := fnVal.Get(&FuncArgs[K]{Input: replacement})
			if err != nil {
				return nil, err
			}
			replacementValRaw, err := replacementExpr.Eval(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			replacementVal = replacementValRaw.(string)
		}
		if originalVal == nil {
			return nil, nil
		}
		if originalValStr, ok := originalVal.(string); ok {
			if compiledPattern.MatchString(originalValStr) {
				updatedStr := compiledPattern.ReplaceAllString(originalValStr, replacementVal)
				err = target.Set(ctx, tCtx, updatedStr)
				if err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	}, nil
}
