// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"

import (
	"context"
	"sync"
	"time"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/tilinna/clock"
)

type serviceStrategyCache interface {
	get(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, bool)
	put(ctx context.Context, serviceName string, response *api_v2.SamplingStrategyResponse)
	Close() error
}

// serviceStrategyCacheEntry is a timestamped sampling strategy response
type serviceStrategyCacheEntry struct {
	retrievedAt      time.Time
	strategyResponse *api_v2.SamplingStrategyResponse
}

// serviceStrategyTTLCache is a naive in-memory TTL serviceStrategyTTLCache of service-specific sampling strategies
// returned from the remote source. Each cached item has its own TTL used to determine whether it is valid for read
// usage (based on the time of write).
type serviceStrategyTTLCache struct {
	itemTTL time.Duration

	stopCh chan struct{}
	rw     sync.RWMutex
	items  map[string]serviceStrategyCacheEntry
}

// Initial size of cache's underlying map
const initialRemoteResponseCacheSize = 32

func newServiceStrategyCache(itemTTL time.Duration) serviceStrategyCache {
	result := &serviceStrategyTTLCache{
		itemTTL: itemTTL,
		items:   make(map[string]serviceStrategyCacheEntry, initialRemoteResponseCacheSize),
		stopCh:  make(chan struct{}),
	}

	// Launches a "cleaner" goroutine that naively blows away stale items with a frequency equal to the item TTL.
	// Note that this is for memory usage and not for correctness (the get() function checks item validity).
	go result.periodicallyClearCache(context.Background(), itemTTL)
	return result
}

// get returns a cached sampling strategy if one is present and is no older than the serviceStrategyTTLCache's per-item TTL.
func (c *serviceStrategyTTLCache) get(
	ctx context.Context,
	serviceName string,
) (*api_v2.SamplingStrategyResponse, bool) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	found, ok := c.items[serviceName]
	if !ok {
		return nil, false
	}
	if c.staleItem(ctx, found) {
		return nil, false
	}
	return found.strategyResponse, true
}

// put unconditionally overwrites the given service's serviceStrategyTTLCache item entry and resets its timestamp used for TTL checks.
func (c *serviceStrategyTTLCache) put(
	ctx context.Context,
	serviceName string,
	response *api_v2.SamplingStrategyResponse,
) {
	c.rw.Lock()
	defer c.rw.Unlock()
	c.items[serviceName] = serviceStrategyCacheEntry{
		strategyResponse: response,
		retrievedAt:      clock.Now(ctx),
	}
}

// periodicallyClearCache periodically clears expired items from the cache and replaces the backing map with only
// valid (fresh) items. Note that this is not necessary for correctness, just preferred for memory usage hygiene.
// Client request activity drives the replacement of stale items with fresh items upon cache misses for any service.
func (c *serviceStrategyTTLCache) periodicallyClearCache(
	ctx context.Context,
	schedulingPeriod time.Duration,
) {
	ticker := clock.NewTicker(ctx, schedulingPeriod)
	for {
		select {
		case <-ticker.C:
			c.rw.Lock()
			newItems := make(map[string]serviceStrategyCacheEntry, initialRemoteResponseCacheSize)
			for serviceName, item := range c.items {
				if !c.staleItem(ctx, item) {
					newItems[serviceName] = item
				}
			}
			// Notice that we swap the map rather than using map's delete (which doesn't reduce its allocated size).
			c.items = newItems
			c.rw.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

func (c *serviceStrategyTTLCache) Close() error {
	close(c.stopCh)
	return nil
}

func (c *serviceStrategyTTLCache) staleItem(ctx context.Context, item serviceStrategyCacheEntry) bool {
	return clock.Now(ctx).After(item.retrievedAt.Add(c.itemTTL))
}

type noopStrategyCache struct{}

func (n *noopStrategyCache) get(_ context.Context, _ string) (*api_v2.SamplingStrategyResponse, bool) {
	return nil, false
}

func (n *noopStrategyCache) put(_ context.Context, _ string, _ *api_v2.SamplingStrategyResponse) {
}

func (n *noopStrategyCache) Close() error {
	return nil
}

func newNoopStrategyCache() serviceStrategyCache {
	return &noopStrategyCache{}
}
