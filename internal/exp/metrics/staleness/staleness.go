// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package staleness // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

// We override how Now() is returned, so we can have deterministic tests
var NowFunc = time.Now

// Staleness a a wrapper over a map that adds an additional "staleness" value to each entry. Users can
// call ExpireOldEntries() to automatically remove all entries from the map whole staleness value is
// older than the `max`
//
// NOTE: Staleness methods are *not* thread-safe. If the user needs to use Staleness in a multi-threaded
// environment, then it is the user's responsibility to properly serialize calls to Staleness methods
type Staleness[T any] struct {
	max time.Duration

	items Map[T]
	pq    PriorityQueue
}

func NewStaleness[T any](max time.Duration, newMap Map[T]) *Staleness[T] {
	return &Staleness[T]{
		max:   max,
		items: newMap,
		pq:    NewPriorityQueue(),
	}
}

// Load the value at key. If it does not exist, the boolean will be false and the value returned will be the zero value
func (s *Staleness[T]) Load(key identity.Stream) (T, bool) {
	return s.items.Load(key)
}

// Store the given key value pair in the map, and update the pair's staleness value to "now"
func (s *Staleness[T]) Store(id identity.Stream, value T) {
	s.pq.Update(id, NowFunc())
	s.items.Store(id, value)
}

// Items returns an iterator function that in future go version can be used with range
// See: https://go.dev/wiki/RangefuncExperiment
func (s *Staleness[T]) Items() func(yield func(identity.Stream, T) bool) bool {
	return s.items.Items()
}

// ExpireOldEntries will remove all entries whose staleness value is older than `now() - max`
// For example, if an entry has a staleness value of two hours ago, and max == 1 hour, then the entry would
// be removed. But if an entry had a stalness value of 30 minutes, then it *wouldn't* be removed.
func (s *Staleness[T]) ExpireOldEntries() {
	now := NowFunc()

	for {
		_, ts := s.pq.Peek()
		if now.Sub(ts) < s.max {
			break
		}
		id, _ := s.pq.Pop()
		s.items.Delete(id)
	}
}
