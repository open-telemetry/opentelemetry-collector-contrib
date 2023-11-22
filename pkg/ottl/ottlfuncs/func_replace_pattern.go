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

type replacePatternFuncArgs[K any] struct {
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
	compiledPattern, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("the regex pattern supplied to replace_pattern is not a valid pattern: %w", err)
	}
	return func(ctx context.Context, tCtx K) (any, error) {
		originalVal, err := target.Get(ctx, tCtx)
		var replacementVal string
		if err != nil {
			return nil, err
		}
		if originalVal == nil {
			return nil, nil
		}
		replacementVal, err = replacement.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if originalValStr, ok := originalVal.(string); ok {
			if compiledPattern.MatchString(originalValStr) {
				if !fn.IsEmpty() {
					result := []byte{}
					submatches := compiledPattern.FindStringSubmatchIndex(originalValStr)
					result = compiledPattern.ExpandString(result, replacementVal, originalValStr, submatches)
					replacementVal = string(result)
					fnVal := fn.Get()
					replaceValGetter := ottl.StandardStringGetter[K]{
						Getter: func(context.Context, K) (any, error) {
							return replacementVal, nil
						},
					}
					replacementExpr, errNew := fnVal.Get(&replacePatternFuncArgs[K]{Input: replaceValGetter})
					if errNew != nil {
						return nil, errNew
					}
					replacementValRaw, errNew := replacementExpr.Eval(ctx, tCtx)
					if errNew != nil {
						return nil, errNew
					}
					replacementValStr, ok := replacementValRaw.(string)
					if !ok {
						return nil, fmt.Errorf("replacement value is not a string")
					}
					replacementVal = replacementValStr
				}
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
