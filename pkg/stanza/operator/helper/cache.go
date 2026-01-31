// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Cache allows operators to cache a value and look it up later.
type Cache interface {
	Get(key string) any
	Add(key string, data any) bool
	Copy() map[string]any
	MaxSize() uint16
	Stop()
}

// NewMemoryCache takes a cache size and a limiter interval and returns a new memory backed cache.
// This implementation uses a map + RWMutex (original regex cache behavior).
func NewMemoryCache(maxSize uint16, interval uint64) *MemoryCache {
	// start throttling when cache turnover is above 100%
	limit := uint64(maxSize) + 1

	return &MemoryCache{
		cache:   make(map[string]any),
		keys:    make(chan string, maxSize),
		limiter: newStartedAtomicLimiter(limit, interval),
	}
}

// NewSyncMapCache returns a cache backed by sync.Map for lock-free reads
// (original container cache behavior).
func NewSyncMapCache(maxSize uint16, interval uint64) *SyncMapCache {
	limit := uint64(maxSize) + 1
	return &SyncMapCache{
		cache:   sync.Map{},
		keys:    make(chan string, maxSize),
		limiter: newStartedAtomicLimiter(limit, interval),
	}
}

// MemoryCache is an in memory cache of items with a pre defined max size.
// When the cache is full, new items will evict the oldest item using a FIFO style queue.
// Backed by map + RWMutex (regex cache).
type MemoryCache struct {
	cache   map[string]any
	keys    chan string
	mutex   sync.RWMutex
	limiter limiter
}

// SyncMapCache is a cache backed by sync.Map with a FIFO eviction channel (container cache).
type SyncMapCache struct {
	cache   sync.Map
	keys    chan string
	mutex   sync.Mutex
	limiter limiter
}

var (
	_ Cache = (*MemoryCache)(nil)
	_ Cache = (*SyncMapCache)(nil)
)

// Get returns a cached entry, nil if it does not exist.
func (m *MemoryCache) Get(key string) any {
	m.mutex.RLock()
	data := m.cache[key]
	m.mutex.RUnlock()
	return data
}

// Add inserts an item into the cache; if the cache is full, the oldest item is removed.
func (m *MemoryCache) Add(key string, data any) bool {
	if m.limiter.throttled() {
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if len(m.keys) == cap(m.keys) {
		delete(m.cache, <-m.keys)
		m.limiter.increment()
	}

	m.cache[key] = data
	m.keys <- key
	return true
}

// Copy returns a deep copy of the cache.
func (m *MemoryCache) Copy() map[string]any {
	cp := make(map[string]any, cap(m.keys))

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for k, v := range m.cache {
		cp[k] = v
	}
	return cp
}

// MaxSize returns the max size of the cache.
func (m *MemoryCache) MaxSize() uint16 {
	return uint16(cap(m.keys))
}

// Stop stops the limiter goroutine.
func (m *MemoryCache) Stop() {
	m.limiter.stop()
}

// Get returns a cached entry, nil if it does not exist (sync.Map).
func (m *SyncMapCache) Get(key string) any {
	val, ok := m.cache.Load(key)
	if !ok {
		return nil
	}
	return val
}

// Add inserts an item into the cache; if the cache is full, the oldest item is removed (sync.Map).
func (m *SyncMapCache) Add(key string, data any) bool {
	if m.limiter.throttled() {
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if len(m.keys) == cap(m.keys) {
		m.cache.Delete(<-m.keys)
		m.limiter.increment()
	}

	m.cache.Store(key, data)
	m.keys <- key
	return true
}

// Copy returns a deep copy of the cache (sync.Map).
func (m *SyncMapCache) Copy() map[string]any {
	cp := make(map[string]any, cap(m.keys))

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.cache.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			cp[k] = value
		}
		return true
	})
	return cp
}

// MaxSize returns the max size of the cache (sync.Map).
func (m *SyncMapCache) MaxSize() uint16 {
	return uint16(cap(m.keys))
}

// Stop stops the limiter goroutine (sync.Map).
func (m *SyncMapCache) Stop() {
	m.limiter.stop()
}

// limiter provides rate limiting methods for the cache.
type limiter interface {
	init()
	increment()
	currentCount() uint64
	limit() uint64
	resetInterval() time.Duration
	throttled() bool
	stop()
}

// newStartedAtomicLimiter returns a started atomicLimiter.
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

// atomicLimiter enables rate limiting using an atomic counter.
type atomicLimiter struct {
	count    *atomic.Uint64
	max      uint64
	interval time.Duration
	start    sync.Once
	done     chan struct{}
}

var _ limiter = &atomicLimiter{count: &atomic.Uint64{}}

// init initializes the limiter.
func (l *atomicLimiter) init() {
	l.start.Do(func() {
		go func() {
			// During every interval period, reduce the counter by 10%.
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

// increment increments the atomic counter.
func (l *atomicLimiter) increment() {
	if l.count.Load() == l.max {
		return
	}
	l.count.Add(1)
}

// throttled returns true if the cache is currently throttled.
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
