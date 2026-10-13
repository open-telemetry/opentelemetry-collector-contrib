// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

// subtraceRingBuffer is a fixed-size circular buffer of subtraceID values,
// structurally identical to ringBuffer but keyed on subtraceID.
type subtraceRingBuffer struct {
	index     int
	size      int
	ids       []subtraceID
	idToIndex map[subtraceID]int
}

func newSubtraceRingBuffer(size int) *subtraceRingBuffer {
	return &subtraceRingBuffer{
		index:     -1,
		size:      size,
		ids:       make([]subtraceID, size),
		idToIndex: make(map[subtraceID]int),
	}
}

// put inserts id into the buffer, evicting the oldest entry if the buffer is
// full. It returns the evicted id and true when an eviction occurred.
func (r *subtraceRingBuffer) put(id subtraceID) (evicted subtraceID, evictedOK bool) {
	if r.size == 0 {
		return subtraceID{}, false
	}
	r.index = (r.index + 1) % r.size

	slot := r.ids[r.index]
	var zero subtraceID
	if slot != zero {
		r.delete(slot)
		evicted = slot
		evictedOK = true
	}

	r.ids[r.index] = id
	r.idToIndex[id] = r.index
	return evicted, evictedOK
}

func (r *subtraceRingBuffer) contains(id subtraceID) bool {
	_, found := r.idToIndex[id]
	return found
}

func (r *subtraceRingBuffer) delete(id subtraceID) bool {
	index, found := r.idToIndex[id]
	if !found {
		return false
	}
	delete(r.idToIndex, id)
	var zero subtraceID
	r.ids[index] = zero
	return true
}
