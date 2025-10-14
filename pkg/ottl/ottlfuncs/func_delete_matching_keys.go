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
	Pattern ottl.Getter[K]
}

func NewDeleteMatchingKeysFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("delete_matching_keys", &DeleteMatchingKeysArguments[K]{}, createDeleteMatchingKeysFunction[K])
}

func createDeleteMatchingKeysFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DeleteMatchingKeysArguments[K])

	if !ok {
		return nil, errors.New("DeleteMatchingKeysFactory args must be of type *DeleteMatchingKeysArguments[K]")
	}

	return deleteMatchingKeys(args.Target, args.Pattern), nil
}

func deleteMatchingKeys[K any](target ottl.PMapGetSetter[K], pattern ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		patternVal, err := pattern.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		patternString, ok := patternVal.(string)
		if !ok {
			return nil, fmt.Errorf("pattern for delete_matching_keys must be a string but got '%T'", patternVal)
		}
		compiledPattern, err := regexp.Compile(patternString)
		if err != nil {
			return nil, fmt.Errorf("the regex pattern supplied to delete_matching_keys is not a valid pattern: %w", err)
		}
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		val.RemoveIf(func(key string, _ pcommon.Value) bool {
			return compiledPattern.MatchString(key)
		})
		return nil, target.Set(ctx, tCtx, val)
	}
}
