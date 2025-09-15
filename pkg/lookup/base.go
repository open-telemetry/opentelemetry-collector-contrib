// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookup // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// BaseSource provides placeholder behaviors for lookup sources. It exposes the
// helper methods future extensions expect while keeping the skeleton lightweight.
type BaseSource struct {
	sourceType string
	cache      *Cache
}

// NewBaseSource creates a new base source with optional cache support.
func NewBaseSource(
	sourceType string,
	settings component.TelemetrySettings,
	cacheConfig *CacheConfig,
) (*BaseSource, error) {
	_ = settings
	base := &BaseSource{sourceType: sourceType}

	if cacheConfig != nil && cacheConfig.Enabled {
		base.cache = NewCache(cacheConfig.Size, cacheConfig.TTL)
	}

	return base, nil
}

// Type returns the declared source type.
func (b *BaseSource) Type() string {
	return b.sourceType
}

// TypeFunc exposes a functional Type implementation.
func (b *BaseSource) TypeFunc() TypeFunc {
	if b == nil {
		return nil
	}
	return TypeFunc(func() string {
		return b.Type()
	})
}

// WrapLookupWithCache wraps lookup with minimal cache behavior.
func (b *BaseSource) WrapLookupWithCache(
	fetchFunc func(context.Context, string) (any, error),
) LookupFunc {
	return func(ctx context.Context, key string) (any, bool, error) {
		if b != nil && b.cache != nil {
			if val, ok := b.cache.Get(key); ok {
				return val, true, nil
			}
		}

		val, err := fetchFunc(ctx, key)
		if err != nil {
			if errors.Is(err, ErrKeyNotFound) {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("lookup failed for key %s: %w", key, err)
		}

		if b != nil && b.cache != nil {
			b.cache.Set(key, val)
		}

		return val, true, nil
	}
}

// WrapLookupWithMetrics currently forwards to fetchFunc without instrumentation.
func (*BaseSource) WrapLookupWithMetrics(
	fetchFunc func(context.Context, string) (any, error),
) LookupFunc {
	return func(ctx context.Context, key string) (any, bool, error) {
		val, err := fetchFunc(ctx, key)
		if err != nil {
			if errors.Is(err, ErrKeyNotFound) {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("lookup failed for key %s: %w", key, err)
		}
		return val, true, nil
	}
}

// WrapLookup normalises a fetch function into a LookupFunc without extra logic.
func (*BaseSource) WrapLookup(fetch func(context.Context, string) (any, bool, error)) LookupFunc {
	return func(ctx context.Context, key string) (any, bool, error) {
		if fetch == nil {
			return nil, false, nil
		}
		return fetch(ctx, key)
	}
}

func (*BaseSource) Start(context.Context, component.Host) error {
	return nil
}

func (*BaseSource) Shutdown(context.Context) error {
	return nil
}
