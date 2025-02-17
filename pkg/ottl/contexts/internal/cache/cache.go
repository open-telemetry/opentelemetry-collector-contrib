// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/cache"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"
)

// Getter is a function that retrieves the cache from a context
type Getter[K any] func(K) pcommon.Map

// GetSetter returns a GetSetter for accessing a map in a transform context.
// The transform context must have a field that returns the cache.
func GetSetter[K any](m Getter[K], keys []ottl.Key[K]) (ottl.GetSetter[K], error) {
	if keys == nil {
		return accessMap(m), nil
	}
	return accessMapKey(m, keys), nil
}

func accessMap[K any](m Getter[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return m(tCtx), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if newMap, ok := val.(pcommon.Map); ok {
				newMap.CopyTo(m(tCtx))
			}
			return nil
		},
	}
}

func accessMapKey[K any](m Getter[K], keys []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			return internal.GetMapValue[K](ctx, tCtx, m(tCtx), keys)
		},
		Setter: func(ctx context.Context, tCtx K, val any) error {
			return internal.SetMapValue[K](ctx, tCtx, m(tCtx), keys, val)
		},
	}
}
