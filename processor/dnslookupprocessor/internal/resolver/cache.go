// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"go.uber.org/zap"
)

// CacheResolver implements DNS resolution caching using LRU caches with TTL
type CacheResolver struct {
	name string
	// nextResolver is the chain resolver to use if the cache misses
	nextResolver Resolver
	hitCache     *lru.LRU[string, string]
	missCache    *lru.LRU[string, struct{}]
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
		r.hitCache = lru.NewLRU[string, string](hitCacheSize, nil, hitCacheTTL)
		r.logger.Debug("Initialized hit cache",
			zap.Int("size", hitCacheSize),
			zap.Duration("ttl", hitCacheTTL))
	} else {
		r.logger.Debug("Hit cache disabled")
	}

	// Initialize miss cache
	if missCacheSize > 0 {
		r.missCache = lru.NewLRU[string, struct{}](missCacheSize, nil, missCacheTTL)
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
	// Add failure to miss cache
	if err != nil {
		if r.missCache != nil {
			r.missCache.Add(target, struct{}{})
			r.logger.Debug("Add miss cache",
				zap.String(logKey, target),
				zap.Error(err))
		}
		return "", err
	}

	// Add success including no resolution to hit cache
	if r.hitCache != nil {
		r.hitCache.Add(target, result)
		r.logger.Debug("Add hit cache",
			zap.String(logKey, target),
			zap.String(Flip(logKey), result))
	}

	return result, nil
}
