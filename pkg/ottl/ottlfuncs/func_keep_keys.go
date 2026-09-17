// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type keepKeysArguments[K any] struct {
	Target ottl.PMapGetSetter[K]
	Keys   ottl.SliceGetter[K, ottl.StringGetter[K]]
}

func NewKeepKeysFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("keep_keys", &keepKeysArguments[K]{}, createKeepKeysFunction[K])
}

func createKeepKeysFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*keepKeysArguments[K])

	if !ok {
		return nil, errors.New("KeepKeysFactory args must be of type *keepKeysArguments[K]")
	}

	return keepKeys(args.Target, &args.Keys), nil
}

func keepKeys[K any](target ottl.PMapGetSetter[K], keys *ottl.SliceGetter[K, ottl.StringGetter[K]]) ottl.ExprFunc[K] {
	// Pre-build the key set when the slice length and all values are known at parse time
	var literalKeySet map[string]struct{}
	length, lengthKnown := keys.Len()
	if lengthKnown {
		if literalValues, allLiteral := ottl.GetLiteralValues[K, string](keys); allLiteral {
			literalKeySet = make(map[string]struct{}, length)
			for _, key := range literalValues {
				literalKeySet[key] = struct{}{}
			}
		}
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		keySet := literalKeySet
		if keySet == nil {
			// Resolve dynamic or runtime-generated keys for the current transform context
			keySet = make(map[string]struct{}, length)
			var keyErr error
			err = keys.Range(ctx, tCtx, func(key ottl.StringGetter[K]) bool {
				k, getErr := key.Get(ctx, tCtx)
				if getErr != nil {
					keyErr = getErr
					return false
				}
				keySet[k] = struct{}{}
				return true
			})
			if err != nil {
				return nil, err
			}
			if keyErr != nil {
				return nil, keyErr
			}
		}

		val.RemoveIf(func(key string, _ pcommon.Value) bool {
			_, ok := keySet[key]
			return !ok
		})
		if val.Len() == 0 {
			val.Clear()
		}
		return nil, target.Set(ctx, tCtx, val)
	}
}
