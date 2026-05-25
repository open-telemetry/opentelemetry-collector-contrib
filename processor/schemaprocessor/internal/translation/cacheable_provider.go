// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/metadata"
)

// CacheableProvider is a provider that caches the result of another provider.
// If the provider returns an error, the CacheableProvider will retry the call till limit
// If the provider returns an error multiple times in a row, the CacheableProvider will rate limit the calls.
type CacheableProvider struct {
	provider Provider
	cache    *cache.Cache
	mu       sync.Mutex
	// cooldown is the time to wait before retrying a failed call.
	cooldown time.Duration
	// callcount tracks the number of failed calls in a row.
	callcount int
	// limit is the number of failed calls to allow before setting the cooldown period.
	limit int
	// lastErr is the last error returned by the provider
	lastErr error
	// resetTime is the time when the rate limit will be reset
	resetTime time.Time
	// telemetryBuilder is used to record cache hit/miss metrics
	telemetryBuilder *metadata.TelemetryBuilder
}

// NewCacheableProvider creates a new CacheableProvider.
// The cooldown parameter is the time to wait before retrying a failed call.
// The limit parameter is the number of failed calls to allow before setting the cooldown period.
func NewCacheableProvider(provider Provider, cooldown time.Duration, limit int, telemetryBuilder *metadata.TelemetryBuilder) Provider {
	return &CacheableProvider{
		provider:         provider,
		cache:            cache.New(cache.NoExpiration, cache.NoExpiration),
		cooldown:         cooldown,
		limit:            limit,
		telemetryBuilder: telemetryBuilder,
	}
}

func (p *CacheableProvider) Retrieve(ctx context.Context, key string) (string, error) {
	// Check if the key is in the cache.
	if value, found := p.cache.Get(key); found {
		p.telemetryBuilder.ProcessorSchemaCacheHits.Add(ctx, 1)
		return value.(string), nil
	}

	p.mu.Lock()

	// Check if the key is in the cache again in case it was added while waiting for the lock.
	if value, found := p.cache.Get(key); found {
		p.mu.Unlock()
		p.telemetryBuilder.ProcessorSchemaCacheHits.Add(ctx, 1)
		return value.(string), nil
	}

	// Check if the function is currently rate-limited
	if time.Now().Before(p.resetTime) {
		lastErr := p.lastErr
		p.mu.Unlock()
		return "", fmt.Errorf("rate limited, last error: %w", lastErr)
	}

	// After the cooldown expires, allow one retry but keep the count so the
	// next failure triggers a new cooldown immediately.
	if p.callcount >= p.limit {
		p.callcount = p.limit - 1
	}
	p.callcount++

	// Release the lock before the HTTP call so other goroutines are not blocked
	// for the duration of the network request.
	p.mu.Unlock()

	p.telemetryBuilder.ProcessorSchemaCacheMisses.Add(ctx, 1)
	v, err := p.provider.Retrieve(ctx, key)

	p.mu.Lock()
	defer p.mu.Unlock()
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
