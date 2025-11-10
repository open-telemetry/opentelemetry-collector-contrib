// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"
)

type concurrentMetricsBuilderMap[V any] interface {
	Get(string) (V, bool)
	Set(string, V)
	Clear()
	Range(func(string, V))
}

// Implementation with concurrent-map (generic API)
type concurrentMapImpl[V any] struct {
	m cmap.ConcurrentMap[string, V]
}

func newConcurrentMapImpl[V any]() concurrentMetricsBuilderMap[V] {
	return &concurrentMapImpl[V]{m: cmap.New[V]()}
}

func (c *concurrentMapImpl[V]) Get(key string) (V, bool) {
	return c.m.Get(key)
}

func (c *concurrentMapImpl[V]) Set(key string, value V) {
	c.m.Set(key, value)
}

func (c *concurrentMapImpl[V]) Clear() {
	c.m.Clear()
}

func (c *concurrentMapImpl[V]) Range(f func(string, V)) {
	c.m.IterCb(f)
}

// Implementation with sync.Map

type syncMapImpl[V any] struct {
	m sync.Map
}

func newSyncMapImpl[V any]() concurrentMetricsBuilderMap[V] {
	return &syncMapImpl[V]{}
}

func (s *syncMapImpl[V]) Get(key string) (V, bool) {
	v, ok := s.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

func (s *syncMapImpl[V]) Set(key string, value V) {
	s.m.Store(key, value)
}

func (s *syncMapImpl[V]) Clear() {
	s.m.Range(func(k, _ any) bool {
		s.m.Delete(k)
		return true
	})
}

func (s *syncMapImpl[V]) Range(f func(string, V)) {
	s.m.Range(func(k, v any) bool {
		f(k.(string), v.(V))
		return true
	})
}

// Implementation with classic map and mutex

type mutexMapImpl[V any] struct {
	m     map[string]V
	mutex sync.RWMutex
}

func newMutexMapImpl[V any]() concurrentMetricsBuilderMap[V] {
	return &mutexMapImpl[V]{m: make(map[string]V)}
}

func (mm *mutexMapImpl[V]) Get(key string) (V, bool) {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	v, ok := mm.m[key]
	return v, ok
}

func (mm *mutexMapImpl[V]) Set(key string, value V) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	mm.m[key] = value
}

func (mm *mutexMapImpl[V]) Clear() {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	mm.m = make(map[string]V)
}

func (mm *mutexMapImpl[V]) Range(f func(string, V)) {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	for k, v := range mm.m {
		f(k, v)
	}
}
