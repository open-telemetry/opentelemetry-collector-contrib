// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cms // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cms"

import (
	"errors"
)

var (
	errFullBuffer     = errors.New("buffer is full")
	errEmptyBuffer    = errors.New("buffer is empty")
	errIncorrectIndex = errors.New("element index is incorrect")
	errEmptyData      = errors.New("data is empty")
)

// RingBufferQueue is the ring buffer data structure implementation
type RingBufferQueue[T any] struct {
	capacity uint32
	head     uint32
	tail     uint32
	full     bool
	data     []T
}

func (r *RingBufferQueue[T]) Capacity() uint32 {
	return r.capacity
}

func (r *RingBufferQueue[T]) IsEmpty() bool {
	return r.head == r.tail && !r.full
}

func (r *RingBufferQueue[T]) IsFull() bool {
	return r.full
}

// Size returns the number of elements that are currently in the buffer.
func (r *RingBufferQueue[T]) Size() int {
	if r.IsFull() {
		return int(r.capacity)
	}

	if r.IsEmpty() {
		return 0
	}

	if r.tail > r.head {
		return int(r.tail) - int(r.head)
	}

	return int(r.capacity) - (int(r.head) - int(r.tail))
}

// Enqueue adds element in the buffer.
func (r *RingBufferQueue[T]) Enqueue(val T) error {
	if r.IsFull() {
		return errFullBuffer
	}

	r.data[r.tail] = val
	r.tail = (r.tail + 1) % r.capacity
	r.full = r.head == r.tail

	return nil
}

// TailMoveForward moves the tail pointer to the next position in the buffer.
// If the buffer is full an error is returned.
func (r *RingBufferQueue[T]) TailMoveForward() error {
	if r.IsFull() {
		return errFullBuffer
	}

	r.tail = (r.tail + 1) % r.capacity
	r.full = r.head == r.tail

	return nil
}

// Visit iterates through the buffer, starting with the head element and ending
// with the tail element. For each of the iterable elements, the visit function
// is called. The iteration ends either when the tail is reached or when the
// visitor returns false.
func (r *RingBufferQueue[T]) Visit(visitor func(T) bool) {

	start := int(r.head)
	for i := 0; i < r.Size(); i++ {
		if !visitor(r.data[(start+i)%int(r.capacity)]) {
			return
		}
	}
}

// At returns an element by its serial (head-first) index. If the buffer is
// empty or the index is out of range [0:len(buffer)-1] then an error is
// returned.
func (r *RingBufferQueue[T]) At(idx int) (T, error) {
	if r.IsEmpty() || idx < 0 || idx >= r.Size() {
		var t T
		return t, errIncorrectIndex
	}

	start := int(r.head)
	return r.data[(start+idx)%int(r.capacity)], nil
}

// Tail returns the last element from the ring buffer. If the buffer is empty
// the method returns the `false` flag.
func (r *RingBufferQueue[T]) Tail() (T, bool) {
	if r.IsEmpty() {
		var t T
		return t, false
	}

	if r.tail == 0 {
		return r.data[r.capacity-1], true
	}
	return r.data[r.tail-1], true
}

// Dequeue removes and returns the first (head) element from the buffer. If the
// buffer is empty an error is returned.
func (r *RingBufferQueue[T]) Dequeue() (T, error) {
	if r.IsEmpty() {
		var t T
		return t, errEmptyBuffer
	}

	element := r.data[r.head]
	r.full = false
	r.head = (r.head + 1) % r.capacity

	return element, nil
}

func NewEmptyRingBufferQueue[T any](capacity uint32) (*RingBufferQueue[T], error) {

	if capacity == 0 {
		return nil, errors.New("empty capacity value passed")
	}

	return &RingBufferQueue[T]{
		capacity: capacity,
		head:     0,
		tail:     0,
		full:     false,
		data:     make([]T, capacity),
	}, nil
}

func NewRingBufferQueue[T any](initData []T) (*RingBufferQueue[T], error) {

	if len(initData) == 0 {
		return nil, errEmptyData
	}

	return &RingBufferQueue[T]{
		capacity: uint32(len(initData)),
		full:     false,
		data:     initData,
	}, nil
}
