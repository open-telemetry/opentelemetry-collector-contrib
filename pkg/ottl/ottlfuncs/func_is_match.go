// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsMatchArguments[K any] struct {
	Target  ottl.StringLikeGetter[K]
	Pattern ottl.StringGetter[K]
}

func NewIsMatchFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsMatch", &IsMatchArguments[K]{}, createIsMatchFunction[K])
}

func createIsMatchFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsMatchArguments[K])

	if !ok {
		return nil, errors.New("IsMatchFactory args must be of type *IsMatchArguments[K]")
	}

	return isMatch(args.Target, args.Pattern)
}

func isMatch[K any](target ottl.StringLikeGetter[K], pattern ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
	literalPattern, ok := ottl.GetLiteralValue(pattern)
	var compiledPattern *regexp.Regexp
	var err error
	if ok {
		compiledPattern, err = regexp.Compile(literalPattern)
		if err != nil {
			return nil, fmt.Errorf(ottl.InvalidRegexErrMsg, "IsMatch", literalPattern, err)
		}
	}
	return func(ctx context.Context, tCtx K) (any, error) {
		cp := compiledPattern
		if cp == nil {
			patternVal, err := pattern.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			cp, err = regexp.Compile(patternVal)
			if err != nil {
				return nil, fmt.Errorf(ottl.InvalidRegexErrMsg, "IsMatch", patternVal, err)
			}
		}
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if val == nil {
			return false, nil
		}
		return cp.MatchString(*val), nil
	}, nil
}
