// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"

import (
	"sync"
	"sync/atomic"
)

// Concurrent describes operations of a hashmap-like datastructure that is safe
// for concurrent access, as implemented by e.g. [sync.Map]
type Concurrent[K comparable, V any] interface {
	Load(K) (V, bool)
	LoadOrStore(K, V) (V, bool)
	LoadAndDelete(K) (V, bool)
}

var _ Concurrent[any, any] = &sync.Map{}

// Limit applies the limit to m, such that no more the item count never exceeds
// it
//
// try is used to track the limit. its value may temporarliy exceed limit. For
// accurate size measurements, use [Count] instead. try may be shared across
// multiple maps, to enforce a common limit.
func Limit[M Concurrent[K, V], K comparable, V any](m M, limit int64, try *atomic.Int64) *Limited[M, K, V] {
	if try == nil {
		try = new(atomic.Int64)
	}
	return &Limited[M, K, V]{
		elems: m,
		try:   try,
		limit: limit,
	}
}

type Limited[M Concurrent[K, V], K comparable, V any] struct {
	elems M
	try   *atomic.Int64
	limit int64
}

var _ Concurrent[any, any] = (*Limited[*sync.Map, any, any])(nil)

func (m *Limited[M, K, V]) Load(k K) (V, bool) {
	return m.elems.Load(k)
}

func (m *Limited[M, K, V]) LoadOrStore(k K, def V) (V, bool) {
	v, ok := m.elems.Load(k)
	if ok {
		return v, true
	}

	sz := m.try.Add(1)
	if sz > m.limit {
		m.try.Add(-1)
		return m.elems.Load(k) // possibly created by now, or *new(T),false
	}

	v, loaded := m.elems.LoadOrStore(k, def)
	if loaded { // already created
		m.try.Add(-1)
	}
	return v, loaded
}

func (m *Limited[M, K, V]) LoadAndDelete(k K) (V, bool) {
	v, ok := m.elems.LoadAndDelete(k)
	if ok {
		m.try.Add(-1)
	}
	return v, ok
}

// Exceeded reports whether a [Limited.LoadOrStore] failed due to the limit being exceeded.
func Exceeded[T comparable](v T, loaded bool) bool {
	return !loaded && v == *new(T)
}

// Count store / delete operations towards given size.
// Size can be shared across multiple maps.
func Count[M Concurrent[K, V], K comparable, V any](m M, size *atomic.Int64) Sized[M, K, V] {
	return Sized[M, K, V]{elems: m, size: size}
}

type Sized[M Concurrent[K, V], K comparable, V any] struct {
	elems M
	size  *atomic.Int64
}

func (s Sized[M, K, V]) Load(k K) (V, bool) {
	return s.elems.Load(k)
}

func (s Sized[M, K, V]) LoadOrStore(k K, def V) (V, bool) {
	v, loaded := s.elems.LoadOrStore(k, def)
	if !loaded {
		s.size.Add(1)
	}
	return v, loaded
}

func (s Sized[M, K, V]) LoadAndDelete(k K) (V, bool) {
	v, loaded := s.elems.LoadAndDelete(k)
	if loaded {
		s.size.Add(-1)
	}
	return v, loaded
}
