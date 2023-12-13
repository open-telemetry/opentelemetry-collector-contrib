// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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

// function to check if a string is a group number string like "$1", "$2", ...
func IsGroupNumString(s string) bool {
	match, _ := regexp.MatchString(`^\$\d+$`, s)
	return match
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
					var groupNum int
					foundMatch := false
					updatedString = originalValStr
					// check if the string is indeed a group number string
					if !IsGroupNumString(replacementVal) {
						updatedStr := compiledPattern.ReplaceAllString(originalValStr, replacementVal)
						err = target.Set(ctx, tCtx, updatedStr)
						if err != nil {
							return nil, err
						}
						return nil, nil
					}
					// parse the group number string to get actual group number
					captureGroup := strings.TrimPrefix(replacementVal, "$")
					groupNum, err = strconv.Atoi(captureGroup)
					if err != nil {
						return nil, err
					}
					submatches := compiledPattern.FindAllStringSubmatchIndex(updatedString, -1)
					for _, submatch := range submatches {
						groupStart := 2 * groupNum
						groupEnd := 2*groupStart - 1
						if len(submatch) > groupEnd {
							fmt.Println("submatch: ", submatch)
							fnVal := fn.Get()
							old := originalValStr[submatch[groupStart]:submatch[groupEnd]]
							replaceValGetter := ottl.StandardStringGetter[K]{
								Getter: func(context.Context, K) (any, error) {
									return old, nil
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
							updatedString = compiledPattern.ReplaceAllString(updatedString, replacementValStr)
							foundMatch = true
							err = target.Set(ctx, tCtx, updatedString)
							if err != nil {
								return nil, err
							}
						}
						if !foundMatch {
							err = target.Set(ctx, tCtx, updatedString)
							if err != nil {
								return nil, err
							}
						}
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
