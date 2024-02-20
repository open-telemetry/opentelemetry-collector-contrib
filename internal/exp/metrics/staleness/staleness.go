// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package staleness

import (
	"context"
	"sync"
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

	items Map[T]
	pq    PriorityQueue

	sig chan struct{}
	mtx sync.RWMutex
}

func NewStaleness[T any](max time.Duration, newMap Map[T]) *Staleness[T] {
	return &Staleness[T]{
		max: max,

		items: newMap,
		pq:    NewPriorityQueue(),

		sig: make(chan struct{}),
		mtx: sync.RWMutex{},
	}
}

func (s *Staleness[T]) Load(id identity.Stream) (T, bool) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.items.Load(id)
}

func (s *Staleness[T]) Store(id identity.Stream, v T) {
	s.mtx.Lock()
	s.pq.Update(id, time.Now())
	s.items.Store(id, v)
	s.mtx.Unlock()

	// "try-send" to notify possibly sleeping expiry routine of write
	select {
	case s.sig <- struct{}{}:
	default:
	}
}

func (s *Staleness[T]) Delete(id identity.Stream) {
	s.mtx.Lock()
	s.items.Delete(id)
	s.mtx.Unlock()
}

func (s *Staleness[T]) Items() func(yield func(identity.Stream, T) bool) bool {
	return func(yield func(identity.Stream, T) bool) bool {
		s.mtx.RLock()
		defer s.mtx.RUnlock()
		return s.items.Items()(yield)
	}
}

func (s *Staleness[T]) expireOldItems() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	now := nowFunc()
	for {
		_, ts := s.pq.Peek()
		if now.Sub(ts) < s.max {
			break
		}
		id, _ := s.pq.Pop()
		s.items.Delete(id)
	}
}

func (s *Staleness[T]) Start(ctx context.Context) {
	for {
		s.expireOldItems()

		n := s.pq.Len()
		switch {
		case n == 0:
			// no more items: sleep until next write
			select {
			case <-s.sig:
			case <-ctx.Done():
				return
			}
		case n > 0:
			// sleep until earliest possible next expiry time
			_, ts := s.pq.Peek()
			at := time.Until(ts.Add(s.max))

			select {
			case <-time.After(at):
			case <-ctx.Done():
				return
			}
		}
	}
}
