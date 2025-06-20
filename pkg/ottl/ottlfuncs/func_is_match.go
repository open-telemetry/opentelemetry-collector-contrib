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
	Pattern ottl.LiteralGetter[K, string, ottl.StringGetter[K]]
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

func isMatch[K any](target ottl.StringLikeGetter[K], patternGetter ottl.LiteralGetter[K, string, ottl.StringGetter[K]]) (ottl.ExprFunc[K], error) {
	var patternRegex *regexp.Regexp
	if patternGetter.IsLiteral() {
		var err error
		patternRegex, err = compileIsMatchPattern(patternGetter.GetLiteral)
		if err != nil {
			return nil, err
		}
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		var regex *regexp.Regexp
		var err error
		if patternRegex != nil {
			regex = patternRegex
		} else {
			regex, err = compileIsMatchPattern(func() (string, error) {
				return patternGetter.Get(ctx, tCtx)
			})
			if err != nil {
				return nil, err
			}
		}

		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if val == nil {
			return false, nil
		}
		return regex.MatchString(*val), nil
	}, nil
}

func compileIsMatchPattern(patternGetter func() (string, error)) (*regexp.Regexp, error) {
	pattern, err := patternGetter()
	if err != nil {
		return nil, fmt.Errorf("failed to get pattern for IsMatch: %w", err)
	}
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("the pattern supplied to IsMatch is not a valid regexp pattern: %w", err)
	}
	return regex, err
}
