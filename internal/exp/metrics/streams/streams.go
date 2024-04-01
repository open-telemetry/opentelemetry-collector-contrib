// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"

import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"

// Sequence of streams that can be iterated upon
type Seq[T any] func(yield func(identity.Stream, T) bool) bool

// Map defines a collection of items tracked by a stream-id and the operations
// on it
type Map[T any] interface {
	Load(identity.Stream) (T, bool)
	Store(identity.Stream, T) error
	Delete(identity.Stream)
	Items() func(yield func(identity.Stream, T) bool) bool
	Len() int
	Clear()
}

var _ Map[any] = &HashMap[any]{}

type HashMap[T any] map[identity.Stream]T

func (m *HashMap[T]) Load(id identity.Stream) (T, bool) {
	v, ok := (*m)[id]
	return v, ok
}

func (m *HashMap[T]) Store(id identity.Stream, v T) error {
	(*m)[id] = v
	return nil
}

func (m *HashMap[T]) Delete(id identity.Stream) {
	delete(*m, id)
}

func (m *HashMap[T]) Items() func(yield func(identity.Stream, T) bool) bool {
	return func(yield func(identity.Stream, T) bool) bool {
		for id, v := range *m {
			if !yield(id, v) {
				break
			}
		}
		return false
	}
}

func (m *HashMap[T]) Len() int {
	return len(*m)
}

func (m *HashMap[T]) Clear() {
	*m = map[identity.Stream]T{}
}

// Evictors remove the "least important" stream based on some strategy such as
// the oldest, least active, etc.
type Evictor interface {
	Evict() identity.Stream
}
