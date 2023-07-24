// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"

import (
	"sync"
	"time"

	"github.com/jaegertracing/jaeger/thrift-gen/sampling"
)

type serviceStrategyCache interface {
	get(serviceName string) (*sampling.SamplingStrategyResponse, bool)
	put(serviceName string, response *sampling.SamplingStrategyResponse)
	Close() error
}

// serviceStrategyCacheEntry is a timestamped sampling strategy response
type serviceStrategyCacheEntry struct {
	retrievedAt      time.Time
	strategyResponse *sampling.SamplingStrategyResponse
}

// serviceStrategyTTLCache is a naive in-memory TTL serviceStrategyTTLCache of service-specific sampling strategies returned from the remote source.
// Each cached item has its own TTL used to determine whether it is valid for read usage (based on the time of write).
type serviceStrategyTTLCache struct {
	itemTTL time.Duration
	items   map[string]serviceStrategyCacheEntry
	clockFn func() time.Time

	stopCh chan struct{}
	mu     sync.RWMutex
}

// Initial size of cache's underlying map
const initialRemoteResponseCacheSize = 32

// Minimum time between runs of "clean up" of all cache-allocated data.
// Used if the itemTTL is lower than this.
const minCleanupSchedulingPeriod = 5 * time.Minute

func newServiceStrategyCache(itemTTL time.Duration) serviceStrategyCache {
	result := &serviceStrategyTTLCache{
		itemTTL: itemTTL,
		items:   make(map[string]serviceStrategyCacheEntry, initialRemoteResponseCacheSize),
		stopCh:  make(chan struct{}),
		clockFn: time.Now, // Tests can override this, but we use a real clock as the default
	}

	// Launches a "cleaner" goroutine that naively blows away cached data
	cleanerSchedulingPeriod := itemTTL
	if itemTTL < minCleanupSchedulingPeriod {
		cleanerSchedulingPeriod = minCleanupSchedulingPeriod
	}
	go result.periodicallyClearCache(cleanerSchedulingPeriod)
	return result
}

// get returns a cached sampling strategy if one is present and is no older than the serviceStrategyTTLCache's per-item TTL.
func (c *serviceStrategyTTLCache) get(serviceName string) (*sampling.SamplingStrategyResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	found, ok := c.items[serviceName]
	if !ok {
		return nil, false
	}
	if c.clockFn().After(found.retrievedAt.Add(c.itemTTL)) {
		return nil, false
	}
	return found.strategyResponse, true
}

// put unconditionally overwrites the given service's serviceStrategyTTLCache item entry and resets its timestamp used for TTL checks.
func (c *serviceStrategyTTLCache) put(serviceName string, response *sampling.SamplingStrategyResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[serviceName] = serviceStrategyCacheEntry{
		strategyResponse: response,
		retrievedAt:      c.clockFn(),
	}
}

// periodicallyClearCache provides a secondary "cleanup" mechanism motivated by wanting to guarantee
// that the memory usage of this serviceStrategyTTLCache does not grow unbounded over long periods of collector uptime.
// Ideally, this is really unnecessary because the get/put access pattern suffices to maintain a small
// set of services. If that assumption is violated, this async process will periodically flush all cached
// data at a cadence that is proportional to the per-item TTL.
func (c *serviceStrategyTTLCache) periodicallyClearCache(schedulingPeriod time.Duration) {
	ticker := time.NewTicker(schedulingPeriod)
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			c.items = make(map[string]serviceStrategyCacheEntry, initialRemoteResponseCacheSize)
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

func (c *serviceStrategyTTLCache) Close() error {
	close(c.stopCh)
	return nil
}

type noopStrategyCache struct{}

func (n *noopStrategyCache) get(_ string) (*sampling.SamplingStrategyResponse, bool) {
	return nil, false
}

func (n *noopStrategyCache) put(_ string, _ *sampling.SamplingStrategyResponse) {
}

func (n *noopStrategyCache) Close() error {
	return nil
}

func newNoopStrategyCache() serviceStrategyCache {
	return &noopStrategyCache{}
}
