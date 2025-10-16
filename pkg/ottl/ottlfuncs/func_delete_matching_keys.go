// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DeleteMatchingKeysArguments[K any] struct {
	Target  ottl.PMapGetSetter[K]
	Pattern ottl.StringGetter[K]
}

const invalidRegexErrMsg = "the regex pattern supplied to delete_matching_keys '%q' is not a valid pattern: %w"

func NewDeleteMatchingKeysFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("delete_matching_keys", &DeleteMatchingKeysArguments[K]{}, createDeleteMatchingKeysFunction[K])
}

func createDeleteMatchingKeysFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DeleteMatchingKeysArguments[K])

	if !ok {
		return nil, errors.New("DeleteMatchingKeysFactory args must be of type *DeleteMatchingKeysArguments[K]")
	}

	return deleteMatchingKeys(args.Target, args.Pattern)
}

func deleteMatchingKeys[K any](target ottl.PMapGetSetter[K], pattern ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
	literalPattern, ok := ottl.GetLiteralValue(pattern)
	var compiledPattern *regexp.Regexp
	var err error
	if ok {
		compiledPattern, err = regexp.Compile(literalPattern)
		if err != nil {
			return nil, fmt.Errorf(invalidRegexErrMsg, literalPattern, err)
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
				return nil, fmt.Errorf(invalidRegexErrMsg, patternVal, err)
			}
		}
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		val.RemoveIf(func(key string, _ pcommon.Value) bool {
			return cp.MatchString(key)
		})
		return nil, target.Set(ctx, tCtx, val)
	}, nil
}
