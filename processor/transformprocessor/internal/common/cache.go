// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type cacheContextKey struct{}

func WithCache(ctx context.Context, cache *pcommon.Map) context.Context {
	return context.WithValue(ctx, cacheContextKey{}, cache)
}

func newCacheFrom(ctx context.Context) pcommon.Map {
	cache := ctx.Value(cacheContextKey{}).(*pcommon.Map)
	if cache != nil {
		return *cache
	}
	return pcommon.NewMap()
}
