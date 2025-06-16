// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"

import (
	"context"
	"errors"
	"time"

	"github.com/cespare/xxhash"
	lru "github.com/elastic/go-freelru"
	"go.uber.org/zap"
)

// CacheResolver implements DNS resolution caching using LRU caches with TTL
type CacheResolver struct {
	name string
	// nextResolver is the chain resolver to use if the cache misses
	nextResolver Resolver
	hitCache     *lru.ShardedLRU[string, string]
	missCache    *lru.ShardedLRU[string, struct{}]
	logger       *zap.Logger
}

// NewCacheResolver initializes miss cache and hit cache
func NewCacheResolver(
	nextResolver Resolver,
	hitCacheSize int,
	hitCacheTTL time.Duration,
	missCacheSize int,
	missCacheTTL time.Duration,
	logger *zap.Logger,
) (*CacheResolver, error) {
	if nextResolver == nil {
		return nil, errors.New("next resolver must be provided")
	}

	r := &CacheResolver{
		name:         "cache",
		nextResolver: nextResolver,
		logger:       logger,
	}

	// Initialize hit cache
	if hitCacheSize > 0 {
		r.hitCache, _ = lru.NewSharded[string, string](uint32(hitCacheSize), stringHashFn)
		r.hitCache.SetLifetime(hitCacheTTL)
		r.logger.Debug("Initialized hit cache",
			zap.Int("size", hitCacheSize),
			zap.Duration("ttl", hitCacheTTL))
	} else {
		r.logger.Debug("Hit cache disabled")
	}

	// Initialize miss cache
	if missCacheSize > 0 {
		r.missCache, _ = lru.NewSharded[string, struct{}](uint32(missCacheSize), stringHashFn)
		r.missCache.SetLifetime(missCacheTTL)
		r.logger.Debug("Initialized miss cache",
			zap.Int("size", missCacheSize),
			zap.Duration("ttl", missCacheTTL))
	} else {
		r.logger.Debug("Miss cache disabled")
	}

	return r, nil
}

// Resolve performs a forward DNS lookup (hostname to IP) using the cache and the underlying chain resolver
func (r *CacheResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	return r.resolveWithCache(ctx, hostname, LogKeyHostname, r.nextResolver.Resolve)
}

// Reverse performs a reverse DNS lookup (IP to hostname) using the cache and the underlying chain resolver
func (r *CacheResolver) Reverse(ctx context.Context, ip string) (string, error) {
	return r.resolveWithCache(ctx, ip, LogKeyIP, r.nextResolver.Reverse)
}

func (r *CacheResolver) Name() string {
	return r.name
}

// Close releases resources used by the cache and the underlying chain resolver
func (r *CacheResolver) Close() error {
	if r.hitCache != nil {
		r.hitCache.Purge()
	}
	if r.missCache != nil {
		r.missCache.Purge()
	}

	if r.nextResolver != nil {
		return r.nextResolver.Close()
	}

	return nil
}

// resolveWithCache searches target in miss cache and hit cache.
// If not found, it falls back to the underlying chain resolver. Stores the result in cache.
func (r *CacheResolver) resolveWithCache(
	ctx context.Context,
	target string,
	logKey string,
	resolveFunc func(ctx context.Context, target string) (string, error),
) (string, error) {
	if r.missCache != nil {
		if _, found := r.missCache.Get(target); found {
			r.logger.Debug("DNS lookup from miss cache",
				zap.String(logKey, target))
			return "", nil
		}
	}

	if r.hitCache != nil {
		if result, found := r.hitCache.Get(target); found {
			r.logger.Debug("DNS lookup from hit cache",
				zap.String(logKey, target),
				zap.String(Flip(logKey), result))
			return result, nil
		}
	}

	// Call the underlying chain resolver
	result, err := resolveFunc(ctx, target)

	switch {
	case errors.Is(err, ErrNoResolution) ||
		errors.Is(err, ErrNotInHostFiles) ||
		errors.Is(err, ErrNSPermanentFailure): // No resolution or NS permanent failure
		if r.missCache != nil {
			r.missCache.Add(target, struct{}{})

			r.logger.Debug("Add miss cache",
				zap.String(logKey, target),
				zap.Error(err))
		}
		return "", nil
	case err == nil: // Successful resolution
		if r.hitCache != nil {
			r.hitCache.Add(target, result)

			r.logger.Debug("Add hit cache",
				zap.String(logKey, target),
				zap.String(Flip(logKey), result))
		}
		return result, nil
	default: // retryable errors eg. timeout
		return "", err
	}
}

// stringHashFn calculates a hash value from the keys for the LRU cache.
func stringHashFn(s string) uint32 {
	return uint32(xxhash.Sum64String(s))
}
