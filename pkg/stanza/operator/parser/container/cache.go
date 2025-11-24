// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/container"

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// cache allows operators to cache a value and look it up later
type cache interface {
	get(key string) any
	add(key string, data any) bool
	copy() map[string]any
	maxSize() uint16
	stop()
}

// newMemoryCache takes a cache size and a limiter interval and
// returns a new memory backed cache
func newMemoryCache(maxSize uint16, interval uint64) *memoryCache {
	// start throttling when cache turnover is above 100%
	limit := uint64(maxSize) + 1

	return &memoryCache{
		cache:   sync.Map{},
		keys:    make(chan string, maxSize),
		limiter: newStartedAtomicLimiter(limit, interval),
	}
}

// memoryCache is an in memory cache of items with a pre defined
// max size. Memory's underlying storage is a sync.Map for lock-free
// reads, optimized for high-throughput scenarios (1M+ ops/sec).
// When the cache is full, new items will evict the oldest
// item using a FIFO style queue.
type memoryCache struct {
	// Key / Value pairs of cached items - using sync.Map for lock-free reads
	cache sync.Map

	// When the cache is full, the oldest entry's key is
	// read from the channel and used to index into the
	// cache during cleanup
	keys chan string

	// Mutex protects the keys channel and eviction logic
	// (reads are lock-free via sync.Map)
	mu sync.Mutex

	// Limiter rate limits the cache
	limiter limiter
}

var _ cache = (&memoryCache{})

// get returns a cached entry, nil if it does not exist
// Lock-free read using sync.Map - optimized for high throughput
func (m *memoryCache) get(key string) any {
	val, ok := m.cache.Load(key)
	if !ok {
		return nil
	}
	return val
}

// add inserts an item into the cache, if the cache is full, the
// oldest item is removed
func (m *memoryCache) add(key string, data any) bool {
	if m.limiter.throttled() {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.keys) == cap(m.keys) {
		// Pop the oldest key from the channel
		// and remove it from the cache
		evictKey := <-m.keys
		m.cache.Delete(evictKey)

		// notify the rate limiter that an entry
		// was evicted
		m.limiter.increment()
	}

	// Write the cached entry and add the key
	// to the channel
	m.cache.Store(key, data)
	m.keys <- key
	return true
}

// copy returns a deep copy of the cache
func (m *memoryCache) copy() map[string]any {
	cp := make(map[string]any, cap(m.keys))

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			cp[k] = value
		}
		return true
	})
	return cp
}

// maxSize returns the max size of the cache
func (m *memoryCache) maxSize() uint16 {
	return uint16(cap(m.keys))
}

func (m *memoryCache) stop() {
	m.limiter.stop()
}

// limiter provides rate limiting methods for
// the cache
type limiter interface {
	init()
	increment()
	currentCount() uint64
	limit() uint64
	resetInterval() time.Duration
	throttled() bool
	stop()
}

// newStartedAtomicLimiter returns a started atomicLimiter
func newStartedAtomicLimiter(maxVal, interval uint64) *atomicLimiter {
	if interval == 0 {
		interval = 5
	}

	a := &atomicLimiter{
		count:    &atomic.Uint64{},
		max:      maxVal,
		interval: time.Second * time.Duration(interval),
		done:     make(chan struct{}),
	}

	a.init()
	return a
}

// atomicLimiter enables rate limiting using an atomic
// counter. When count is >= max, throttled will return
// true. The count is reset on an interval.
type atomicLimiter struct {
	count    *atomic.Uint64
	max      uint64
	interval time.Duration
	start    sync.Once
	stopOnce sync.Once
	done     chan struct{}
}

var _ limiter = &atomicLimiter{count: &atomic.Uint64{}}

// init initializes the limiter
func (l *atomicLimiter) init() {
	// start the reset go routine once
	l.start.Do(func() {
		go func() {
			// During every interval period, reduce the counter
			// by 10%
			x := math.Round(-0.10 * float64(l.max))
			ticker := time.NewTicker(l.interval)
			for {
				select {
				case <-l.done:
					ticker.Stop()
					return
				case <-ticker.C:
					if l.currentCount() > 0 {
						l.count.Add(^uint64(x))
					}
				}
			}
		}()
	})
}

// increment increments the atomic counter
func (l *atomicLimiter) increment() {
	if l.count.Load() == l.max {
		return
	}
	l.count.Add(1)
}

// Returns true if the cache is currently throttled, meaning a high
// number of evictions have recently occurred due to the cache being
// full. When the cache is constantly locked, reads and writes are
// blocked, causing the parser to be slower than if it was
// not caching at all.
func (l *atomicLimiter) throttled() bool {
	return l.currentCount() >= l.max
}

func (l *atomicLimiter) currentCount() uint64 {
	return l.count.Load()
}

func (l *atomicLimiter) limit() uint64 {
	return l.max
}

func (l *atomicLimiter) resetInterval() time.Duration {
	return l.interval
}

func (l *atomicLimiter) stop() {
	close(l.done)
}
