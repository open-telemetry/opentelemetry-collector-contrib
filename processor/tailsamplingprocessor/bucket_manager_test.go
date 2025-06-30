// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	internalsampling "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

// Helper types and functions

type testTrace struct {
	traceID    pcommon.TraceID
	randomness uint64
	traceData  *internalsampling.TraceData
}

type fixedTimeSource struct {
	t time.Time
}

func (f *fixedTimeSource) Now() time.Time {
	return f.t
}

// generateUniformRandomnessTraces creates traces with evenly distributed randomness values
func generateUniformRandomnessTraces(count int, maxRandomness uint64) []testTrace {
	traces := make([]testTrace, count)

	for i := 0; i < count; i++ {
		// Calculate uniformly distributed randomness value
		randomness := uint64(float64(i) / float64(count-1) * float64(maxRandomness))

		// Create a trace ID that encodes this randomness value
		traceID := pcommon.NewTraceIDEmpty()

		// Encode the randomness in the lower 8 bytes of the trace ID
		// This matches how extractRandomnessValue works
		for j := 0; j < 8; j++ {
			traceID[8+j] = byte(randomness >> (8 * j))
		}

		// Create trace data with TraceState containing the randomness value
		traceData := &internalsampling.TraceData{
			ReceivedBatches: createTraceWithRandomness(traceID, randomness),
		}

		traces[i] = testTrace{
			traceID:    traceID,
			randomness: randomness,
			traceData:  traceData,
		}
	}

	return traces
}

// createTraceWithRandomness creates a trace with explicit randomness in TraceState
func createTraceWithRandomness(traceID pcommon.TraceID, randomness uint64) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanIDEmpty())
	span.SetName("test-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Millisecond)))

	// Set TraceState with explicit randomness value (ot=rv:xxxxxxxxxxxxx)
	// Format the randomness as a 14-character hex string (56 bits)
	randomnessHex := fmt.Sprintf("%014x", randomness)
	traceState := fmt.Sprintf("ot=rv:%s", randomnessHex)
	span.TraceState().FromRaw(traceState)

	return traces
}

// TestVaropSamplingBehavior tests that Varopt sampling maintains statistical properties
func TestVaropSamplingBehavior(t *testing.T) {
	logger := zaptest.NewLogger(t)

	bm := newBucketManagerWithTimeSource(
		logger,
		1, // single bucket
		time.Minute,
		10, // keep only 10 traces
		1.0,
		&fixedTimeSource{time.Now()},
	)

	bucket := bm.buckets[0]

	// Add 100 traces to test Varopt behavior with overflow
	numTraces := 100
	capacity := 10

	// Add all traces
	for i := 0; i < numTraces; i++ {
		traceID := pcommon.NewTraceIDEmpty()
		traceID[8] = byte(i) // Just to make them unique

		bucketTrace := &bucketTrace{
			traceID:         traceID,
			randomnessValue: uint64(i),
			trace:           &internalsampling.TraceData{},
		}

		bucket.mutex.Lock()
		// Use Varopt to add the trace with uniform weight
		_, err := bucket.sampler.Add(bucketTrace, 1.0)
		require.NoError(t, err, "Failed to add trace to Varopt sampler")
		bucket.mutex.Unlock()
	}

	// Verify we kept exactly the capacity (10 traces)
	keptTraces := bucket.getTracesWithTailAdjustments()
	require.Equal(t, capacity, len(keptTraces), "Should keep exactly the capacity number of traces")

	// Verify each trace has a valid tail sampling adjustment
	for _, trace := range keptTraces {
		adjustment := trace.getTailSamplingAdjustment()
		assert.GreaterOrEqual(t, adjustment, 1.0, "Tail sampling adjustment should be >= 1.0")
	}
}
