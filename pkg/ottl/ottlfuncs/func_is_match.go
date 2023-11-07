// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// Cache compiled patterns
var compiledPatterns = make(map[string]*regexp.Regexp)
var mutex = &sync.RWMutex{}

type IsMatchArguments[K any] struct {
	Target  ottl.StringLikeGetter[K]
	Pattern ottl.StringLikeGetter[K]
}

func GetCompiledPattern(pattern *string) (*regexp.Regexp, error) {
	mutex.RLock()
	compiledPattern, ok := compiledPatterns[*pattern]
	mutex.RUnlock()
	if !ok {
		var err error
		compiledPattern, err = regexp.Compile(*pattern)
		if err != nil {
			return nil, fmt.Errorf("the pattern supplied to IsMatch is not a valid regexp pattern: %w", err)
		}
		mutex.Lock()
		compiledPatterns[*pattern] = compiledPattern
		mutex.Unlock()
	}
	return compiledPattern, nil
}

func NewIsMatchFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsMatch", &IsMatchArguments[K]{}, createIsMatchFunction[K])
}

func createIsMatchFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsMatchArguments[K])

	if !ok {
		return nil, fmt.Errorf("IsMatchFactory args must be of type *IsMatchArguments[K]")
	}

	return isMatch(args.Target, args.Pattern)
}

func isMatch[K any](target ottl.StringLikeGetter[K], pattern ottl.StringLikeGetter[K]) (ottl.ExprFunc[K], error) {

	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := pattern.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if val == nil {
			return false, nil
		}
		compiledPattern, err := GetCompiledPattern(val)
		if err != nil {
			return nil, err
		}
		if compiledPattern == nil {
			return false, nil
		}
		val2, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if val2 == nil {
			return false, nil
		}
		return compiledPattern.MatchString(*val2), nil
	}, nil
}
