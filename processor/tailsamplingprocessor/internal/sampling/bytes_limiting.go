// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"golang.org/x/time/rate"
)

type bytesLimiting struct {
	// Rate limiter using golang.org/x/time/rate for efficient token bucket implementation
	limiter *rate.Limiter
}

var _ PolicyEvaluator = (*bytesLimiting)(nil)

// NewBytesLimiting creates a policy evaluator that samples traces based on byte limit per second using a token bucket algorithm.
// The bucket capacity defaults to 2x the bytes per second to allow for reasonable burst traffic.
func NewBytesLimiting(settings component.TelemetrySettings, bytesPerSecond int64) PolicyEvaluator {
	return NewBytesLimitingWithBurstCapacity(settings, bytesPerSecond, bytesPerSecond*2)
}

// NewBytesLimitingWithBurstCapacity creates a policy evaluator with custom burst capacity.
// Uses golang.org/x/time/rate.Limiter for efficient, thread-safe token bucket implementation.
func NewBytesLimitingWithBurstCapacity(settings component.TelemetrySettings, bytesPerSecond, burstCapacity int64) PolicyEvaluator {
	// Create rate limiter with specified rate and burst capacity
	// rate.Limit is tokens per second (bytes per second in our case)
	// burst capacity is the maximum number of tokens (bytes) that can be consumed in a burst
	limiter := rate.NewLimiter(rate.Limit(bytesPerSecond), int(burstCapacity))

	return &bytesLimiting{
		limiter: limiter,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision based on token bucket algorithm.
// Uses golang.org/x/time/rate.Limiter.AllowN() for efficient, thread-safe token consumption.
func (b *bytesLimiting) Evaluate(ctx context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	// Calculate the size of the trace in bytes
	traceSize := calculateTraceSize(trace)

	// Use AllowN to check if we can consume 'traceSize' tokens
	// AllowN returns true if the limiter allows the event and false otherwise
	// The limiter automatically handles token bucket refill and thread safety
	if b.limiter.AllowN(time.Now(), int(traceSize)) {
		return Sampled, nil
	}

	return NotSampled, nil
}

// calculateTraceSize estimates the size of a trace in bytes
func calculateTraceSize(trace *TraceData) int64 {
	trace.Lock()
	defer trace.Unlock()

	var totalSize int64
	batches := trace.ReceivedBatches

	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)

		// Add resource size
		totalSize += calculateResourceSize(rs.Resource())

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)

			// Add scope size
			totalSize += calculateScopeSize(ss.Scope())

			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				// Add span size
				totalSize += calculateSpanSize(span)
			}
		}
	}

	return totalSize
}

// calculateResourceSize estimates the size of a resource in bytes
func calculateResourceSize(resource pcommon.Resource) int64 {
	// Start with fixed overhead: DroppedAttributesCount (4 bytes for uint32)
	size := int64(4)

	// Resource attributes - inline attribute size calculation for performance
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		size += int64(len(k)) + calculateValueSize(v)
		return true
	})

	return size
}

// calculateScopeSize estimates the size of an instrumentation scope in bytes
func calculateScopeSize(scope pcommon.InstrumentationScope) int64 {
	// Start with fixed overhead: name, version, and DroppedAttributesCount (4 bytes for uint32)
	size := int64(len(scope.Name())) + int64(len(scope.Version())) + 4

	// Scope attributes - inline attribute size calculation for performance
	scope.Attributes().Range(func(k string, v pcommon.Value) bool {
		size += int64(len(k)) + calculateValueSize(v)
		return true
	})

	return size
}

// calculateSpanSize estimates the size of a span in bytes
func calculateSpanSize(span ptrace.Span) int64 {
	// Start with fixed-size fields (batch calculation for performance)
	// TraceID(16) + SpanID(8) + ParentSpanID(8) + StartTime(8) + EndTime(8) + Kind(4) + StatusCode(4) + 3*DroppedCounts(12) = 68 bytes
	size := int64(68)

	// Variable-length string fields
	size += int64(len(span.Name()))
	size += int64(len(span.TraceState().AsRaw()))
	size += int64(len(span.Status().Message()))

	// Span attributes - inline calculation for performance
	span.Attributes().Range(func(k string, v pcommon.Value) bool {
		size += int64(len(k)) + calculateValueSize(v)
		return true
	})

	// Events - pre-calculate length to avoid repeated calls
	eventsLen := span.Events().Len()
	for i := 0; i < eventsLen; i++ {
		event := span.Events().At(i)
		// TimeUnixNano(8) + DroppedAttributesCount(4) = 12 bytes per event
		size += 12 + int64(len(event.Name()))

		// Event attributes - inline calculation
		event.Attributes().Range(func(k string, v pcommon.Value) bool {
			size += int64(len(k)) + calculateValueSize(v)
			return true
		})
	}

	// Links - pre-calculate length to avoid repeated calls
	linksLen := span.Links().Len()
	for i := 0; i < linksLen; i++ {
		link := span.Links().At(i)
		// TraceID(16) + SpanID(8) + DroppedAttributesCount(4) = 28 bytes per link
		size += 28 + int64(len(link.TraceState().AsRaw()))

		// Link attributes - inline calculation
		link.Attributes().Range(func(k string, v pcommon.Value) bool {
			size += int64(len(k)) + calculateValueSize(v)
			return true
		})
	}

	return size
}

// calculateValueSize estimates the size of a pcommon.Value in bytes with optimized inline handling
func calculateValueSize(v pcommon.Value) int64 {
	// Optimize for most common types first (string, int, double, bool)
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return int64(len(v.Str()))
	case pcommon.ValueTypeInt:
		return 8 // int64 is always 8 bytes
	case pcommon.ValueTypeDouble:
		return 8 // float64 is always 8 bytes
	case pcommon.ValueTypeBool:
		return 1 // bool is typically 1 byte
	case pcommon.ValueTypeBytes:
		return int64(len(v.Bytes().AsRaw()))
	case pcommon.ValueTypeMap:
		// For maps, use direct range instead of recursion for performance
		var size int64
		v.Map().Range(func(k string, mapVal pcommon.Value) bool {
			size += int64(len(k)) + calculateValueSize(mapVal)
			return true
		})
		return size
	case pcommon.ValueTypeSlice:
		// For slices, pre-calculate length and use direct indexing
		var size int64
		slice := v.Slice()
		sliceLen := slice.Len()
		for i := 0; i < sliceLen; i++ {
			size += calculateValueSize(slice.At(i))
		}
		return size
	default:
		// Handle unknown/empty types
		return 0
	}
}
