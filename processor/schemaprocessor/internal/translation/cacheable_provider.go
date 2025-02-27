// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// CacheableProvider is a provider that caches the result of another provider.
type CacheableProvider struct {
	provider  Provider
	cache     *cache.Cache
	mu        sync.Mutex
	cooldown  time.Duration
	callcount int
	limit     int
	lastErr   error
	resetTime time.Time
}

// NewCacheableProvider creates a new CacheableProvider.
func NewCacheableProvider(provider Provider, cooldown time.Duration, limit int) Provider {
	return &CacheableProvider{
		provider: provider,
		// TODO make cache configurable
		cache:    cache.New(cache.NoExpiration, cache.NoExpiration),
		cooldown: cooldown,
		limit:    limit,
	}
}

func (p *CacheableProvider) Retrieve(ctx context.Context, key string) (string, error) {
	// Check if the key is in the cache.
	if value, found := p.cache.Get(key); found {
		return value.(string), nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// Check if the function is currently rate-limited
	if time.Now().Before(p.resetTime) {
		return "", fmt.Errorf("rate limited, last error: %w", p.lastErr)
	}

	// Reset count if past cooldown period
	if p.callcount >= p.limit {
		p.callcount = 0
	}
	p.callcount++

	v, err := p.provider.Retrieve(ctx, key)
	if err != nil {
		p.lastErr = err
		// If the call limit is reached, set the cooldown period
		if p.callcount >= p.limit {
			p.resetTime = time.Now().Add(p.cooldown)
		}
		return "", err
	}
	p.callcount = 0
	p.cache.Set(key, v, cache.NoExpiration)
	return v, nil
}
