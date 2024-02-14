// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import "go.opentelemetry.io/collector/pdata/pcommon"

// ringBuffer keeps an in-memory bounded buffer with the in-flight trace IDs
type ringBuffer struct {
	index     int
	size      int
	ids       []pcommon.TraceID
	idToIndex map[pcommon.TraceID]int // key is traceID, value is the index on the 'ids' slice
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		index:     -1, // the first span to be received will be placed at position '0'
		size:      size,
		ids:       make([]pcommon.TraceID, size),
		idToIndex: make(map[pcommon.TraceID]int),
	}
}

func (r *ringBuffer) put(traceID pcommon.TraceID) pcommon.TraceID {
	// calculates the item in the ring that we'll store the trace
	r.index = (r.index + 1) % r.size

	// see if the ring has an item already
	evicted := r.ids[r.index]

	if !evicted.IsEmpty() {
		// clear space for the new item
		r.delete(evicted)
	}

	// place the traceID in memory
	r.ids[r.index] = traceID
	r.idToIndex[traceID] = r.index

	return evicted
}

func (r *ringBuffer) contains(traceID pcommon.TraceID) bool {
	_, found := r.idToIndex[traceID]
	return found
}

func (r *ringBuffer) delete(traceID pcommon.TraceID) bool {
	index, found := r.idToIndex[traceID]
	if !found {
		return false
	}

	delete(r.idToIndex, traceID)
	r.ids[index] = pcommon.NewTraceIDEmpty()
	return true
}
