// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cms

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRingbufferEmptyCapacity(t *testing.T) {
	rb, err := NewEmptyRingBufferQueue[int](0)
	assert.Nil(t, rb)
	assert.NotNil(t, err)
}

func TestRingbufferEnqueueFull(t *testing.T) {
	rb, err := NewEmptyRingBufferQueue[int](1)
	assert.Nil(t, err)
	assert.NotNil(t, rb)

	err = rb.Enqueue(1)
	assert.Equal(t, 1, rb.Size())
	assert.True(t, rb.IsFull())

	err = rb.Enqueue(2)
	assert.NotNil(t, err)
	assert.ErrorIs(t, err, errFullBuffer)
	assert.Equal(t, 1, rb.Size())
	assert.True(t, rb.IsFull())

}

func TestRingbufferEnqueueSimple(t *testing.T) {
	rb, err := NewEmptyRingBufferQueue[int](3)
	assert.Nil(t, err)

	var (
		valTail int
		val0    int
		val1    int
		val2    int
	)

	err = rb.Enqueue(3)
	valTail, _ = rb.Tail()
	val0, _ = rb.At(0)
	assert.False(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 1, rb.Size())
	assert.Equal(t, uint32(3), rb.Capacity())
	assert.Equal(t, 3, valTail)
	assert.Equal(t, 3, val0)

	err = rb.Enqueue(2)
	err = rb.Enqueue(1)
	valTail, _ = rb.Tail()
	val0, _ = rb.At(0)
	val1, _ = rb.At(1)
	val2, _ = rb.At(2)
	assert.False(t, rb.IsEmpty())
	assert.True(t, rb.IsFull())
	assert.Equal(t, 3, rb.Size())
	assert.Equal(t, uint32(3), rb.Capacity())
	assert.Equal(t, 1, valTail)
	assert.Equal(t, 3, val0)
	assert.Equal(t, 2, val1)
	assert.Equal(t, 1, val2)
}

func TestRingbufferDequeueSequential(t *testing.T) {
	rb, err := NewEmptyRingBufferQueue[int](3)
	assert.Nil(t, err)

	var (
		valTail int
		val0    int
		val1    int
		val     int
	)

	err = rb.Enqueue(3)
	err = rb.Enqueue(2)
	err = rb.Enqueue(1)

	val, err = rb.Dequeue()
	valTail, _ = rb.Tail()
	val0, _ = rb.At(0)
	val1, _ = rb.At(1)

	assert.Nil(t, err)
	assert.False(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 3, val)
	assert.Equal(t, 1, valTail)
	assert.Equal(t, 2, val0)
	assert.Equal(t, 1, val1)
	assert.Equal(t, 2, rb.Size())

	val, err = rb.Dequeue()
	valTail, _ = rb.Tail()
	val0, _ = rb.At(0)

	assert.Nil(t, err)
	assert.False(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 2, val)
	assert.Equal(t, 1, valTail)
	assert.Equal(t, 1, val0)
	assert.Equal(t, 1, rb.Size())

	val, err = rb.Dequeue()
	assert.Nil(t, err)
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 1, val)
	assert.Equal(t, 0, rb.Size())
}

func TestRingbufferDequeueSimple(t *testing.T) {
	rb, err := NewEmptyRingBufferQueue[int](3)
	assert.Nil(t, err)

	var (
		valTail int
		val0    int
		val     int
	)

	err = rb.Enqueue(3)
	val, err = rb.Dequeue()
	assert.Nil(t, err)
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 3, val)
	assert.Equal(t, 0, rb.Size())

	err = rb.Enqueue(2)
	val, err = rb.Dequeue()
	assert.Nil(t, err)
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 2, val)
	assert.Equal(t, 0, rb.Size())

	err = rb.Enqueue(1)
	val, err = rb.Dequeue()
	valTail, _ = rb.Tail()
	val0, _ = rb.At(0)
	assert.Nil(t, err)
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 1, val)
	assert.Equal(t, 0, valTail)
	assert.Equal(t, 0, val0)
	assert.Equal(t, 0, rb.Size())
}

func TestRingbufferDequeueEmpty(t *testing.T) {
	rb, err := NewEmptyRingBufferQueue[int](3)
	assert.Nil(t, err)

	_, err = rb.Dequeue()
	assert.ErrorIs(t, err, errEmptyBuffer)

	err = rb.Enqueue(1)

	_, err = rb.Dequeue()
	assert.Nil(t, err)

	_, err = rb.Dequeue()
	assert.ErrorIs(t, err, errEmptyBuffer)
}

func TestRingbufferTailEmpty(t *testing.T) {
	rb, err := NewEmptyRingBufferQueue[int](3)
	assert.Nil(t, err)

	val, ok := rb.Tail()
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.False(t, ok)
	assert.Equal(t, 0, val)

	_ = rb.Enqueue(1)
	_, _ = rb.Dequeue()
	val, ok = rb.Tail()
	assert.False(t, ok)
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 0, val)
}

func TestRingbufferAccessByIndexEmpty(t *testing.T) {
	rb, err := NewEmptyRingBufferQueue[int](3)
	assert.Nil(t, err)

	var (
		val0 int
		val1 int
	)

	val0, err = rb.At(0)
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.ErrorIs(t, err, errIncorrectIndex)
	assert.Equal(t, 0, val0)

	val1, err = rb.At(1)
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.ErrorIs(t, err, errIncorrectIndex)
	assert.Equal(t, 0, val1)

	_ = rb.Enqueue(1)
	_, _ = rb.Dequeue()

	val0, err = rb.At(0)
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.ErrorIs(t, err, errIncorrectIndex)
	assert.Equal(t, 0, val0)

	val1, err = rb.At(1)
	assert.True(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.ErrorIs(t, err, errIncorrectIndex)
	assert.Equal(t, 0, val1)
}

func TestRingbufferAccessByIndexSimple(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)
	_ = rb.Enqueue(1)
	_ = rb.Enqueue(2)
	_ = rb.Enqueue(3)

	val0, err0 := rb.At(0)
	val1, err1 := rb.At(1)
	val2, err2 := rb.At(2)

	assert.False(t, rb.IsEmpty())
	assert.True(t, rb.IsFull())
	assert.Nil(t, err0)
	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, 1, val0)
	assert.Equal(t, 2, val1)
	assert.Equal(t, 3, val2)
}

func TestRingbufferAccessByIndexWithShiftSimple(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)
	_ = rb.Enqueue(1)
	_ = rb.Enqueue(2)
	_ = rb.Enqueue(3)
	_, _ = rb.Dequeue()
	_ = rb.Enqueue(4)

	val0, err0 := rb.At(0)
	val1, err1 := rb.At(1)
	val2, err2 := rb.At(2)

	assert.False(t, rb.IsEmpty())
	assert.True(t, rb.IsFull())
	assert.Nil(t, err0)
	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, 2, val0)
	assert.Equal(t, 3, val1)
	assert.Equal(t, 4, val2)
}

func TestRingbufferAccessByIndexOverflow(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)
	_ = rb.Enqueue(1)
	_ = rb.Enqueue(2)
	_ = rb.Enqueue(3)

	val2, err2 := rb.At(2)
	assert.Equal(t, 3, val2)
	assert.Nil(t, err2)

	val3, err3 := rb.At(3)
	assert.Equal(t, 0, val3)
	assert.ErrorIs(t, err3, errIncorrectIndex)

	val6, err6 := rb.At(6)
	assert.Equal(t, 0, val6)
	assert.ErrorIs(t, err6, errIncorrectIndex)
}

func TestRingbufferAccessByNegativeIndex(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)
	val, err := rb.At(-1)
	assert.Equal(t, 0, val)
	assert.ErrorIs(t, err, errIncorrectIndex)

	_ = rb.Enqueue(1)

	val, err = rb.At(-1)
	assert.Equal(t, 0, val)
	assert.ErrorIs(t, err, errIncorrectIndex)
}

func TestRingbufferSizeSimple(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)
	assert.Equal(t, 0, rb.Size())

	_ = rb.Enqueue(1)
	assert.Equal(t, 1, rb.Size())

	_ = rb.Enqueue(2)
	assert.Equal(t, 2, rb.Size())

	_ = rb.Enqueue(3)
	assert.Equal(t, 3, rb.Size())

	_, _ = rb.Dequeue()
	assert.Equal(t, 2, rb.Size())
}

func TestRingbufferVisitAllSimple(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)

	arr := make([]int, 0)
	visitor := func(val int) bool {
		arr = append(arr, val)
		return true
	}

	rb.Visit(visitor)
	assert.Equal(t, 0, len(arr))

	_ = rb.Enqueue(1)
	_ = rb.Enqueue(2)
	_ = rb.Enqueue(3)
	rb.Visit(visitor)
	assert.Equal(t, []int{1, 2, 3}, arr)
	arr = arr[:0]

	_, _ = rb.Dequeue()
	rb.Visit(visitor)
	assert.Equal(t, []int{2, 3}, arr)
	arr = arr[:0]

	_ = rb.Enqueue(4)
	rb.Visit(visitor)
	assert.Equal(t, []int{2, 3, 4}, arr)
}

func TestRingbufferPartialVisitAllSimple(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)
	_ = rb.Enqueue(1)
	_ = rb.Enqueue(2)
	_ = rb.Enqueue(3)

	v1 := newPVisitor[int](1)
	rb.Visit(v1.visit)
	assert.Equal(t, 1, len(v1.visited))
	assert.Equal(t, []int{1}, v1.visited)

	v2 := newPVisitor[int](2)
	rb.Visit(v2.visit)
	assert.Equal(t, 2, len(v2.visited))
	assert.Equal(t, []int{1, 2}, v2.visited)
}

func TestRingbufferTailMoveForwardFull(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)
	_ = rb.Enqueue(1)
	_ = rb.Enqueue(2)
	_ = rb.Enqueue(3)

	assert.False(t, rb.IsEmpty())
	assert.True(t, rb.IsFull())

	err := rb.TailMoveForward()

	assert.False(t, rb.IsEmpty())
	assert.True(t, rb.IsFull())
	assert.ErrorIs(t, err, errFullBuffer)
	assert.Equal(t, 3, rb.Size())
}

func TestRingbufferTailMoveForwardFromEmptyToFull(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)
	var err error

	err = rb.TailMoveForward()
	assert.False(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 1, rb.Size())
	assert.Nil(t, err)

	err = rb.TailMoveForward()
	assert.False(t, rb.IsEmpty())
	assert.False(t, rb.IsFull())
	assert.Equal(t, 2, rb.Size())
	assert.Nil(t, err)

	err = rb.TailMoveForward()
	assert.False(t, rb.IsEmpty())
	assert.True(t, rb.IsFull())
	assert.Equal(t, 3, rb.Size())
	assert.Nil(t, err)
}

func TestRingbufferTailMoveForwardAfterDequeue(t *testing.T) {
	rb, _ := NewEmptyRingBufferQueue[int](3)
	var (
		err error
		val int
		ok  bool
	)

	_ = rb.Enqueue(1)
	_ = rb.Enqueue(2)
	_ = rb.Enqueue(3)

	_, _ = rb.Dequeue()
	err = rb.TailMoveForward()
	assert.Nil(t, err)
	assert.True(t, rb.IsFull())

	val, ok = rb.Tail()
	assert.True(t, ok)
	assert.Equal(t, 1, val)
}

func TestRingbufferNewRingBufferQueue(t *testing.T) {
	var (
		val int
		ok  bool
	)

	rb, err := NewRingBufferQueue[int]([]int{3, 2, 1})

	assert.Nil(t, err)
	assert.Equal(t, 0, rb.Size())
	assert.Equal(t, uint32(3), rb.Capacity())
	assert.False(t, rb.IsFull())
	assert.True(t, rb.IsEmpty())

	err = rb.TailMoveForward()
	assert.Nil(t, err)
	assert.Equal(t, 1, rb.Size())

	val, ok = rb.Tail()
	assert.Equal(t, 3, val)
	assert.True(t, ok)

	err = rb.TailMoveForward()
	assert.Nil(t, err)
	assert.Equal(t, 2, rb.Size())

	val, ok = rb.Tail()
	assert.Equal(t, 2, val)
	assert.True(t, ok)

	err = rb.TailMoveForward()
	assert.Nil(t, err)
	assert.Equal(t, 3, rb.Size())

	val, ok = rb.Tail()
	assert.Equal(t, 1, val)
	assert.True(t, ok)
}

func TestRingbufferNewRingBufferQueueEmptyInitData(t *testing.T) {
	rb, err := NewRingBufferQueue[int]([]int{})

	assert.ErrorIs(t, err, errEmptyData)
	assert.Nil(t, rb)
}
