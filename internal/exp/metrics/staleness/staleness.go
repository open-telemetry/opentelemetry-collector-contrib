// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package staleness

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

// We override how Now() is returned, so we can have deterministic tests
var nowFunc = time.Now

type Map[T any] interface {
	// Load the value at key. If it does not exist, the boolean will be false and the value returned will be the zero value
	Load(key identity.Stream) (T, bool)
	// Store the given key value pair in the map
	Store(key identity.Stream, value T)
	// LoadOrStore will either load the value from the map and return it and the boolean `true`
	// or if it doesn't exist in the Map yet, the value passed in will be stored and then returned with the boolean `false`
	// LoadOrStore(key identity.Stream, value T) (T, bool)
	// Remove the value at key from the map
	Delete(key identity.Stream)
	// Items returns an iterator function that in future go version can be used with range
	// See: https://go.dev/wiki/RangefuncExperiment
	Items() func(yield func(identity.Stream, T) bool) bool
	// Number of items
	Len() int
}

type Staleness[T any] struct {
	Max time.Duration

	items Map[T]
	pq    PriorityQueue
}

func NewStaleness[T any](max time.Duration, items Map[T]) *Staleness[T] {
	return &Staleness[T]{
		Max: max,

		items: items,
		pq:    NewPriorityQueue(),
	}
}

func (s *Staleness[T]) Load(id identity.Stream) (T, bool) {
	return s.items.Load(id)
}

func (s *Staleness[T]) Store(id identity.Stream, v T) {
	s.pq.Update(id, time.Now())
	s.items.Store(id, v)
}

func (s *Staleness[T]) Delete(id identity.Stream) {
	s.items.Delete(id)
}

func (s *Staleness[T]) Items() func(yield func(identity.Stream, T) bool) bool {
	return s.items.Items()
}

func (s *Staleness[T]) ExpireOldItems() {
	now := nowFunc()
	for {
		_, ts := s.pq.Peek()
		if now.Sub(ts) < s.Max {
			break
		}
		id, _ := s.pq.Pop()
		s.items.Delete(id)
	}
}

func (s *Staleness[T]) Len() int {
	return s.items.Len()
}

func (s *Staleness[T]) Next() (identity.Stream, time.Time) {
	return s.pq.Peek()
}
