// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxcache // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcache"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

func PathExpressionParser[K any](cacheGetter func(K) pcommon.Map) ottl.PathExpressionParser[K] {
	return func(path ottl.Path[K]) (ottl.GetSetter[K], error) {
		if path.Keys() == nil {
			return accessCache(cacheGetter), nil
		}
		return accessCacheKey(cacheGetter, path.Keys()), nil
	}
}

func accessCache[K any](cacheGetter func(K) pcommon.Map) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return cacheGetter(tCtx), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if m, ok := val.(pcommon.Map); ok {
				m.CopyTo(cacheGetter(tCtx))
			}
			return nil
		},
	}
}

func accessCacheKey[K any](cacheGetter func(K) pcommon.Map, key []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			return ctxutil.GetMapValue(ctx, tCtx, cacheGetter(tCtx), key)
		},
		Setter: func(ctx context.Context, tCtx K, val any) error {
			return ctxutil.SetMapValue(ctx, tCtx, cacheGetter(tCtx), key, val)
		},
	}
}
