// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package heap // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor/internal/heap"

import "container/heap"

type Iter[T any] func(yield func(int, T) bool)

var _ heap.Interface = (*impl[any])(nil)

type Heap[T any] struct {
	Elems []T
	less  func(T, T) bool
}

func New[T any](less func(T, T) bool) *Heap[T] {
	return &Heap[T]{less: less}
}

func (h *Heap[T]) Push(v T) {
	heap.Push(h.as(), v)
}

func (h *Heap[T]) Pop() T {
	return heap.Pop(h.as()).(T)
}

func (h *Heap[T]) Peek() T {
	return h.Elems[0]
}

func (h *Heap[T]) Len() int {
	return len(h.Elems)
}

func (h *Heap[T]) as() *impl[T] {
	return (*impl[T])(h)
}

type impl[T any] Heap[T]

func (h *impl[T]) Len() int {
	return len(h.Elems)
}

func (h *impl[T]) Less(i int, j int) bool {
	return h.less(h.Elems[i], h.Elems[j])
}

func (h *impl[T]) Swap(i int, j int) {
	h.Elems[i], h.Elems[j] = h.Elems[j], h.Elems[i]
}

func (h *impl[T]) Push(x any) {
	h.Elems = append(h.Elems, x.(T))
}

func (h *impl[T]) Pop() any {
	v := h.Elems[h.Len()-1]
	h.Elems = h.Elems[:h.Len()-1]
	return v
}
