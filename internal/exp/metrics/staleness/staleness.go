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
	Load(key identity.Stream) (T, bool)
	Store(key identity.Stream, value T)
	Delete(key identity.Stream)
	// Range calls f sequentially for each key and value present in the map.
	// If f returns false, range stops the iteration.
	Range(f func(key identity.Stream, value T) bool)
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
