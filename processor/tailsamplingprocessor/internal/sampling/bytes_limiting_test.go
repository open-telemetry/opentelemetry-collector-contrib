// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestBytesLimitingTokenBucket(t *testing.T) {
	// Create a small trace for testing
	trace := newTraceBytesFilter()
	traceSize := calculateTraceSize(trace)

	testCases := []struct {
		name              string
		bytesPerSecond    int64
		burstCapacity     int64
		numTraces         int
		expectedDecisions []samplingpolicy.Decision
		description       string
	}{
		{
			name:              "Under burst capacity - single trace",
			bytesPerSecond:    traceSize / 2, // Low rate
			burstCapacity:     traceSize * 2, // High burst capacity
			numTraces:         1,
			expectedDecisions: []samplingpolicy.Decision{samplingpolicy.Sampled},
			description:       "Should sample when burst capacity allows it",
		},
		{
			name:              "At burst capacity - single trace",
			bytesPerSecond:    traceSize / 2,
			burstCapacity:     traceSize, // Exactly trace size
			numTraces:         1,
			expectedDecisions: []samplingpolicy.Decision{samplingpolicy.Sampled},
			description:       "Should sample when trace fits in burst capacity",
		},
		{
			name:              "Over burst capacity - single large trace",
			bytesPerSecond:    traceSize,
			burstCapacity:     traceSize - 1, // Less than trace size
			numTraces:         1,
			expectedDecisions: []samplingpolicy.Decision{samplingpolicy.NotSampled},
			description:       "Should not sample when trace exceeds burst capacity",
		},
		{
			name:              "Multiple traces within burst",
			bytesPerSecond:    traceSize,
			burstCapacity:     traceSize * 3, // Can hold 3 traces
			numTraces:         2,
			expectedDecisions: []samplingpolicy.Decision{samplingpolicy.Sampled, samplingpolicy.Sampled},
			description:       "Should sample multiple traces that fit in burst capacity",
		},
		{
			name:              "Multiple traces exceeding burst",
			bytesPerSecond:    traceSize / 2,
			burstCapacity:     traceSize + 1, // Can hold just over 1 trace
			numTraces:         2,
			expectedDecisions: []samplingpolicy.Decision{samplingpolicy.Sampled, samplingpolicy.NotSampled},
			description:       "Should reject traces that exceed burst capacity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewBytesLimitingWithBurstCapacity(
				componenttest.NewNopTelemetrySettings(),
				tc.bytesPerSecond,
				tc.burstCapacity,
			)

			for i := 0; i < tc.numTraces; i++ {
				decision, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{byte(i + 1)}), trace)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedDecisions[i], decision,
					"Failed on trace %d: %s", i, tc.description)
			}
		})
	}
}

func TestBytesLimitingDefaultConstructor(t *testing.T) {
	trace := newTraceBytesFilter()
	traceSize := calculateTraceSize(trace)

	// Test default constructor creates bucket with 2x burst capacity
	filter := NewBytesLimiting(componenttest.NewNopTelemetrySettings(), traceSize)

	// Should be able to sample at least 2 traces immediately (2x burst capacity)
	decision1, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{1}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision1)

	decision2, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{2}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision2)

	// Third trace should be rejected (exceeds 2x capacity)
	decision3, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{3}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision3)
}

func TestBytesLimitingTokenRefill(t *testing.T) {
	trace := newTraceBytesFilter()
	traceSize := calculateTraceSize(trace)

	// Create a filter with small burst capacity
	filter := NewBytesLimitingWithBurstCapacity(
		componenttest.NewNopTelemetrySettings(),
		traceSize*2, // 2 traces per second
		traceSize,   // burst capacity for 1 trace
	)

	// First trace should be sampled (using burst capacity)
	decision1, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{1}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision1)

	// Second trace should be rejected (no tokens left)
	decision2, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{2}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision2)

	// Wait for tokens to refill (real time delay)
	// golang.org/x/time/rate handles token refill automatically based on real time
	time.Sleep(600 * time.Millisecond) // Wait for more than half a second to allow refill

	// Third trace should be sampled (tokens refilled)
	decision3, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{3}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision3)
}

func TestBytesLimitingConcurrency(t *testing.T) {
	trace := newTraceBytesFilter()
	traceSize := calculateTraceSize(trace)

	filter := NewBytesLimitingWithBurstCapacity(
		componenttest.NewNopTelemetrySettings(),
		traceSize*10,
		traceSize*2,
	)

	// Test concurrent access doesn't cause race conditions
	results := make(chan samplingpolicy.Decision, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			decision, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{byte(id)}), trace)
			assert.NoError(t, err)
			results <- decision
		}(i)
	}

	// Collect results
	var sampled, notSampled int
	for i := 0; i < 10; i++ {
		decision := <-results
		if decision == samplingpolicy.Sampled {
			sampled++
		} else {
			notSampled++
		}
	}

	// Should have some sampled (at least 2 due to burst) and some not sampled
	assert.Positive(t, sampled, "Should have sampled some traces")
	assert.GreaterOrEqual(t, sampled, 2, "Should sample at least 2 traces due to burst capacity")
}

func TestCalculateTraceSize(t *testing.T) {
	trace := newTraceBytesFilter()
	size := calculateTraceSize(trace)

	// Should return a positive size using ProtoMarshaler.TracesSize()
	assert.Positive(t, size)
}

// newTraceBytesFilter creates a trace for testing bytes limiting
func newTraceBytesFilter() *samplingpolicy.TraceData {
	var trace samplingpolicy.TraceData

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()

	// Add resource attributes
	resource := rs.Resource()
	resource.Attributes().PutStr("service.name", "test-service")
	resource.Attributes().PutStr("service.version", "1.0.0")

	ss := rs.ScopeSpans().AppendEmpty()

	// Add scope info
	scope := ss.Scope()
	scope.SetName("test-scope")
	scope.SetVersion("1.0.0")

	// Add span
	span := ss.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.SetName("test-span")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Millisecond * 100)))

	// Add span attributes
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutStr("http.url", "http://example.com/test")
	span.Attributes().PutInt("http.status_code", 200)

	// Add span event
	event := span.Events().AppendEmpty()
	event.SetName("test-event")
	event.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	event.Attributes().PutStr("event.attr", "value")

	trace.ReceivedBatches = td
	return &trace
}
