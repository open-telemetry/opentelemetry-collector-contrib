package tailsamplingprocessor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func TestOTEP235Integration(t *testing.T) {
	// Create a TraceStateManager
	tsm := sampling.NewTraceStateManager()

	// Create test trace data with spans containing OTEP 235 tracestate
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	// Create TraceData with a span that has OTEP 235 tracestate
	trace := &sampling.TraceData{
		ArrivalTime:     time.Now(),
		SpanCount:       &atomic.Int64{},
		ReceivedBatches: ptrace.NewTraces(),
	}
	trace.SpanCount.Store(1)

	// Add a span with OTEP 235 tracestate
	rs := trace.ReceivedBatches.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("test-span")
	// Set OTEP 235 tracestate with threshold and r-value
	span.TraceState().FromRaw("ot=th:8;rv:abcdef12345678")

	// Initialize OTEP 235 fields using TraceStateManager
	tsm.InitializeTraceData(context.Background(), traceID, trace)

	// Verify that OTEP 235 fields were populated correctly
	assert.True(t, trace.TraceStatePresent, "TraceStatePresent should be true")
	assert.NotZero(t, trace.RandomnessValue.Unsigned(), "RandomnessValue should be non-zero")
	require.NotNil(t, trace.FinalThreshold, "FinalThreshold should not be nil")

	// Verify threshold was extracted correctly (threshold "8" should be parsed)
	assert.Equal(t, "8", trace.FinalThreshold.TValue(), "Threshold value should match")

	// Test sampling decision
	shouldSample := trace.FinalThreshold.ShouldSample(trace.RandomnessValue)
	t.Logf("Sampling decision: %v (threshold=%s, randomness=%s)",
		shouldSample, trace.FinalThreshold.TValue(), trace.RandomnessValue.RValue())
}

func TestOTEP235Integration_EmptyTraceState(t *testing.T) {
	// Create a TraceStateManager
	tsm := sampling.NewTraceStateManager()

	// Create test trace data with spans containing no tracestate
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	// Create TraceData with a span that has no tracestate
	trace := &sampling.TraceData{
		ArrivalTime:     time.Now(),
		SpanCount:       &atomic.Int64{},
		ReceivedBatches: ptrace.NewTraces(),
	}
	trace.SpanCount.Store(1)

	// Add a span with no tracestate
	rs := trace.ReceivedBatches.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("test-span")
	// No tracestate set

	// Initialize OTEP 235 fields using TraceStateManager
	tsm.InitializeTraceData(context.Background(), traceID, trace)

	// Verify that OTEP 235 fields were populated correctly
	assert.False(t, trace.TraceStatePresent, "TraceStatePresent should be false")
	assert.NotZero(t, trace.RandomnessValue.Unsigned(), "RandomnessValue should be derived from TraceID")
	assert.Nil(t, trace.FinalThreshold, "FinalThreshold should be nil when no tracestate")
}
