// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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

func applyOptReplaceFunction[K any](ctx context.Context, tCtx K, compiledPattern *regexp.Regexp, fn ottl.Optional[ottl.FunctionGetter[K]], originalValStr string, replacementVal string) (string, error) {
	var updatedString string
	updatedString = originalValStr
	submatches := compiledPattern.FindAllStringSubmatchIndex(updatedString, -1)
	for _, submatch := range submatches {
		fullMatch := originalValStr[submatch[0]:submatch[1]]
		result := compiledPattern.ExpandString([]byte{}, replacementVal, originalValStr, submatch)
		fnVal := fn.Get()
		replaceValGetter := ottl.StandardStringGetter[K]{
			Getter: func(context.Context, K) (any, error) {
				return string(result), nil
			},
		}
		replacementExpr, errNew := fnVal.Get(&replacePatternFuncArgs[K]{Input: replaceValGetter})
		if errNew != nil {
			return "", errNew
		}
		replacementValRaw, errNew := replacementExpr.Eval(ctx, tCtx)
		if errNew != nil {
			return "", errNew
		}
		replacementValStr, ok := replacementValRaw.(string)
		if !ok {
			return "", fmt.Errorf("the replacement value must be a string")
		}
		updatedString = strings.ReplaceAll(updatedString, fullMatch, replacementValStr)
	}
	return updatedString, nil
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
					var updatedString string
					updatedString, err = applyOptReplaceFunction[K](ctx, tCtx, compiledPattern, fn, originalValStr, replacementVal)
					if err != nil {
						return nil, err
					}
					err = target.Set(ctx, tCtx, updatedString)
					if err != nil {
						return nil, err
					}
				} else {
					updatedStr := compiledPattern.ReplaceAllString(originalValStr, replacementVal)
					err = target.Set(ctx, tCtx, updatedStr)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		return nil, nil
	}, nil
}
