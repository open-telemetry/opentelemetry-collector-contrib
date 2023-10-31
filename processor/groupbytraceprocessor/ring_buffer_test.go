// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestRingBufferCapacity(t *testing.T) {
	// prepare
	buffer := newRingBuffer(5)

	// test
	traceIDs := []pcommon.TraceID{
		pcommon.TraceID([16]byte{1, 2, 3, 4}),
		pcommon.TraceID([16]byte{2, 3, 4, 5}),
		pcommon.TraceID([16]byte{3, 4, 5, 6}),
		pcommon.TraceID([16]byte{4, 5, 6, 7}),
		pcommon.TraceID([16]byte{5, 6, 7, 8}),
		pcommon.TraceID([16]byte{6, 7, 8, 9}),
	}
	for _, traceID := range traceIDs {
		buffer.put(traceID)
	}

	// verify
	for i := 5; i > 0; i-- { // last 5 traces
		traceID := traceIDs[i]
		assert.True(t, buffer.contains(traceID))
	}

	// the first trace should have been evicted
	assert.False(t, buffer.contains(traceIDs[0]))
}

func TestDeleteFromBuffer(t *testing.T) {
	// prepare
	buffer := newRingBuffer(2)
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	buffer.put(traceID)

	// test
	deleted := buffer.delete(traceID)

	// verify
	assert.True(t, deleted)
	assert.False(t, buffer.contains(traceID))
}

func TestDeleteNonExistingFromBuffer(t *testing.T) {
	// prepare
	buffer := newRingBuffer(2)
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	// test
	deleted := buffer.delete(traceID)

	// verify
	assert.False(t, deleted)
	assert.False(t, buffer.contains(traceID))
}
