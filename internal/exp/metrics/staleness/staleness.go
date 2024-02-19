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
}

type Staleness[T any] struct {
	max time.Duration

	Map[T]
	pq PriorityQueue
}

func NewStaleness[T any](max time.Duration, newMap Map[T]) *Staleness[T] {
	return &Staleness[T]{
		max: max,
		Map: newMap,
		pq:  NewPriorityQueue(),
	}
}

func (s *Staleness[T]) Store(id identity.Stream, value T) {
	s.pq.Update(id, nowFunc())
	s.Map.Store(id, value)
}

func (s *Staleness[T]) ExpireOldEntries() {
	now := nowFunc()

	for {
		_, ts := s.pq.Peek()
		if now.Sub(ts) < s.max {
			break
		}
		id, _ := s.pq.Pop()
		s.Map.Delete(id)
	}
}
