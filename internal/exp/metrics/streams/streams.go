package streams

import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"

// Sequence of streams that can be iterated upon
type Seq[T any] func(yield func(identity.Stream, T) bool) bool

// Map defines a collection of items tracked by a stream-id and the operations
// on it
type Map[T any] interface {
	Load(identity.Stream) (T, bool)
	Store(identity.Stream, T)
	Delete(identity.Stream)
	Items() func(yield func(identity.Stream, T) bool) bool
	Len() int
}

func EmptyMap[T any]() HashMap[T] {
	return HashMap[T]{items: make(map[identity.Stream]T)}
}

type HashMap[T any] struct {
	items map[identity.Stream]T
}

func (m HashMap[T]) Load(id identity.Stream) (T, bool) {
	v, ok := m.items[id]
	return v, ok
}

func (m HashMap[T]) Store(id identity.Stream, v T) {
	m.items[id] = v
}

func (m HashMap[T]) Delete(id identity.Stream) {
	delete(m.items, id)
}

func (m HashMap[T]) Items() func(yield func(identity.Stream, T) bool) bool {
	return func(yield func(identity.Stream, T) bool) bool {
		for id, v := range m.items {
			if !yield(id, v) {
				break
			}
		}
		return false
	}
}

func (m HashMap[T]) Len() int {
	return len(m.items)
}
