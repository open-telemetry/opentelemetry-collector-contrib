// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type bytesLimiting struct {
	// Token bucket parameters
	tokens     int64      // current number of tokens in the bucket
	capacity   int64      // maximum bucket capacity (burst size)
	refillRate int64      // tokens added per second (bytes per second)
	lastRefill time.Time  // last time tokens were added to bucket
	mutex      sync.Mutex // protects concurrent access to bucket state
	logger     *zap.Logger
}

var _ PolicyEvaluator = (*bytesLimiting)(nil)

// NewBytesLimiting creates a policy evaluator that samples traces based on byte limit per second using a token bucket algorithm.
// The bucket capacity defaults to 2x the bytes per second to allow for reasonable burst traffic.
func NewBytesLimiting(settings component.TelemetrySettings, bytesPerSecond int64) PolicyEvaluator {
	return NewBytesLimitingWithBurstCapacity(settings, bytesPerSecond, bytesPerSecond*2)
}

// NewBytesLimitingWithBurstCapacity creates a policy evaluator with custom burst capacity.
func NewBytesLimitingWithBurstCapacity(settings component.TelemetrySettings, bytesPerSecond, burstCapacity int64) PolicyEvaluator {
	now := time.Now()
	return &bytesLimiting{
		tokens:     burstCapacity, // start with full bucket
		capacity:   burstCapacity,
		refillRate: bytesPerSecond,
		lastRefill: now,
		logger:     settings.Logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision based on token bucket algorithm.
func (b *bytesLimiting) Evaluate(_ context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	b.logger.Debug("Evaluating bytes in bytes-limiting filter using token bucket")

	// Calculate the size of the trace in bytes
	traceSize := calculateTraceSize(trace)

	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Refill tokens based on time elapsed
	b.refillTokens()

	// Check if we have enough tokens for this trace
	if b.tokens >= traceSize {
		b.tokens -= traceSize
		b.logger.Debug("Trace sampled",
			zap.Int64("trace_size", traceSize),
			zap.Int64("remaining_tokens", b.tokens))
		return Sampled, nil
	}

	b.logger.Debug("Trace not sampled - insufficient tokens",
		zap.Int64("trace_size", traceSize),
		zap.Int64("available_tokens", b.tokens))
	return NotSampled, nil
}

// refillTokens adds tokens to the bucket based on elapsed time since last refill.
// This method assumes the caller holds the mutex.
func (b *bytesLimiting) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)

	if elapsed <= 0 {
		return
	}

	// Calculate tokens to add based on elapsed time
	tokensToAdd := int64(elapsed.Seconds() * float64(b.refillRate))

	if tokensToAdd > 0 {
		b.tokens += tokensToAdd

		// Cap tokens at bucket capacity
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}

		b.lastRefill = now
		b.logger.Debug("Refilled token bucket",
			zap.Int64("tokens_added", tokensToAdd),
			zap.Int64("current_tokens", b.tokens),
			zap.Duration("elapsed", elapsed))
	}
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
