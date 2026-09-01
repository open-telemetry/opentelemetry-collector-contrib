// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeSubtraceID(trace, span byte) subtraceID {
	return subtraceID{traceID: makeTraceID(trace), spanID: makeSpanID(span)}
}

func TestSubtraceRingBuffer_Capacity(t *testing.T) {
	buf := newSubtraceRingBuffer(3)
	ids := []subtraceID{
		makeSubtraceID(1, 1),
		makeSubtraceID(2, 2),
		makeSubtraceID(3, 3),
		makeSubtraceID(4, 4),
	}
	for _, id := range ids {
		buf.put(id)
	}

	// last 3 should be present
	for _, id := range ids[1:] {
		assert.True(t, buf.contains(id))
	}
	// first evicted
	assert.False(t, buf.contains(ids[0]))
}

func TestSubtraceRingBuffer_PutReturnsEvicted(t *testing.T) {
	buf := newSubtraceRingBuffer(2)
	a := makeSubtraceID(1, 1)
	b := makeSubtraceID(2, 2)
	c := makeSubtraceID(3, 3)

	_, ok := buf.put(a)
	assert.False(t, ok)

	_, ok = buf.put(b)
	assert.False(t, ok)

	evicted, ok := buf.put(c)
	assert.True(t, ok)
	assert.Equal(t, a, evicted)
}

func TestSubtraceRingBuffer_Delete(t *testing.T) {
	buf := newSubtraceRingBuffer(3)
	id := makeSubtraceID(1, 1)
	buf.put(id)

	assert.True(t, buf.delete(id))
	assert.False(t, buf.contains(id))
}

func TestSubtraceRingBuffer_DeleteNonExisting(t *testing.T) {
	buf := newSubtraceRingBuffer(3)
	assert.False(t, buf.delete(makeSubtraceID(99, 99)))
}

func TestSubtraceRingBuffer_Contains(t *testing.T) {
	buf := newSubtraceRingBuffer(3)
	id := makeSubtraceID(5, 5)
	assert.False(t, buf.contains(id))
	buf.put(id)
	assert.True(t, buf.contains(id))
}

// TestSubtraceRingBuffer_SizeZeroPut ensures that a zero-capacity buffer does
// not panic when put() is called. This can happen when NumTraces < NumWorkers
// causes integer division to produce 0.
func TestSubtraceRingBuffer_SizeZeroPut(t *testing.T) {
	buf := newSubtraceRingBuffer(0)
	id := makeSubtraceID(1, 1)
	evicted, ok := buf.put(id)
	assert.False(t, ok)
	assert.Equal(t, subtraceID{}, evicted)
}
