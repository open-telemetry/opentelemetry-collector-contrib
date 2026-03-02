// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"

import (
	"context"
	"time"
)

type CacheConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Default: 1000
	Size int `mapstructure:"size"`

	// Default: 0 (no expiration)
	TTL time.Duration `mapstructure:"ttl"`
}

type Cache struct {
	config CacheConfig
}

func NewCache(cfg CacheConfig) *Cache {
	return &Cache{config: cfg}
}

func (*Cache) Get(_ string) (any, bool) {
	return nil, false
}

func (*Cache) Set(_ string, _ any) {
	// Stub
}

func (*Cache) Clear() {
	// Stub
}

// WrapWithCache wraps a lookup function with caching.
//
// Example:
//
//	cache := lookupsource.NewCache(cfg.Cache)
//	cachedLookup := lookupsource.WrapWithCache(cache, myLookupFunc)
func WrapWithCache(cache *Cache, fn LookupFunc) LookupFunc {
	if cache == nil || !cache.config.Enabled {
		return fn
	}
	return func(ctx context.Context, key string) (any, bool, error) {
		if val, found := cache.Get(key); found {
			return val, true, nil
		}

		val, found, err := fn(ctx, key)
		if err != nil {
			return nil, false, err
		}

		if found {
			cache.Set(key, val)
		}

		return val, found, nil
	}
}
