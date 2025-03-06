// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xidutils

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestUInt64ToTraceIDConversion(t *testing.T) {
	assert.Equal(t,
		pcommon.TraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),
		UInt64ToTraceID(0, 0),
		"Failed 0 conversion:")
	assert.Equal(t,
		pcommon.TraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01}),
		UInt64ToTraceID(256*256+256+1, 256+1),
		"Failed simple conversion:")
	assert.Equal(t,
		pcommon.TraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05}),
		UInt64ToTraceID(0, 5),
		"Failed to convert 0 high:")
	assert.Equal(t,
		UInt64ToTraceID(5, 0),
		pcommon.TraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),
		UInt64ToTraceID(5, 0),
		"Failed to convert 0 low:")
	assert.Equal(t,
		pcommon.TraceID([16]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05}),
		UInt64ToTraceID(math.MaxUint64, 5),
		"Failed to convert MaxUint64:")
}

func TestUInt64ToSpanIDConversion(t *testing.T) {
	assert.Equal(t,
		pcommon.SpanID([8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),
		UInt64ToSpanID(0),
		"Failed 0 conversion:")
	assert.Equal(t,
		pcommon.SpanID([8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01}),
		UInt64ToSpanID(256*256+256+1),
		"Failed simple conversion:")
	assert.Equal(t,
		pcommon.SpanID([8]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}),
		UInt64ToSpanID(math.MaxUint64),
		"Failed to convert MaxUint64:")
}

func TestTraceIDUInt64RoundTrip(t *testing.T) {
	wh := uint64(0x70605040302010FF)
	wl := uint64(0x0001020304050607)
	gh, gl := TraceIDToUInt64Pair(UInt64ToTraceID(wh, wl))
	assert.Equal(t, wl, gl)
	assert.Equal(t, wh, gh)
}

func TestSpanIdUInt64RoundTrip(t *testing.T) {
	w := uint64(0x0001020304050607)
	assert.Equal(t, w, SpanIDToUInt64(UInt64ToSpanID(w)))
}
