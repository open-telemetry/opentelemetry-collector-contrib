// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

// TestPhase5_TraceStateProcessing tests the core Phase 5 functionality:
// TraceState integration with tail sampling decisions
func TestPhase5_TraceStateProcessing(t *testing.T) {
	sink := &consumertest.TracesSink{}
	idb := newSyncIDBatcher()

	// Configure with AlwaysSample for deterministic behavior
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               defaultNumTraces,
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "always_sample",
					Type: AlwaysSample,
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(idb),
		},
	}

	processor, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), sink, cfg)
	require.NoError(t, err)

	tsp := processor.(*tailSamplingSpanProcessor)

	err = tsp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err := tsp.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	// Test 1: Trace with OTEP 235 TraceState
	traceID1 := pcommon.NewTraceIDEmpty()
	copy(traceID1[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	traces1 := createSimpleTraceWithTraceState(traceID1, "ot=th:8;rv:abcdef12345678")
	err = processor.ConsumeTraces(context.Background(), traces1)
	require.NoError(t, err)

	// Test 2: Trace without TraceState (should still work)
	traceID2 := pcommon.NewTraceIDEmpty()
	copy(traceID2[:], []byte{2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	traces2 := createSimpleTraceWithTraceState(traceID2, "")
	err = processor.ConsumeTraces(context.Background(), traces2)
	require.NoError(t, err)

	// Trigger decision making
	tsp.policyTicker.OnTick() // First tick builds batches
	tsp.policyTicker.OnTick() // Second tick makes decisions

	// Verify both traces were processed
	consumed := sink.AllTraces()
	require.Equal(t, 2, len(consumed), "Both traces should be sampled with AlwaysSample policy")

	// Verify TraceState handling
	for i, trace := range consumed {
		spans := getAllSpansFromTrace(trace)
		require.NotEmpty(t, spans, "Trace %d should have spans", i)

		for _, span := range spans {
			// TraceState should be preserved or appropriately updated
			traceState := span.TraceState().AsRaw()
			t.Logf("Trace %d span TraceState: %s", i, traceState)

			// For the first trace with OTEP 235 TraceState, verify it's handled
			if i == 0 && traceState != "" {
				// Should contain threshold information
				assert.Contains(t, traceState, "th:", "TraceState should contain threshold information")
			}
		}
	}
}

// TestPhase5_ProbabilisticPolicy tests TraceState with probabilistic sampling
func TestPhase5_ProbabilisticPolicy(t *testing.T) {
	sink := &consumertest.TracesSink{}
	idb := newSyncIDBatcher()

	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               defaultNumTraces,
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "probabilistic_50",
					Type: Probabilistic,
					ProbabilisticCfg: ProbabilisticCfg{
						SamplingPercentage: 50.0,
					},
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(idb),
		},
	}

	processor, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), sink, cfg)
	require.NoError(t, err)

	tsp := processor.(*tailSamplingSpanProcessor)

	err = tsp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err := tsp.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	// Create trace with OTEP 235 TraceState
	traceID := pcommon.NewTraceIDEmpty()
	copy(traceID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	traces := createSimpleTraceWithTraceState(traceID, "ot=th:8;rv:2")
	err = processor.ConsumeTraces(context.Background(), traces)
	require.NoError(t, err)

	// Trigger decision making
	tsp.policyTicker.OnTick()
	tsp.policyTicker.OnTick()

	// Verify processing completed without errors
	consumed := sink.AllTraces()
	t.Logf("Probabilistic policy result: %d traces sampled", len(consumed))

	// The test passes if no errors occurred during processing
	// The actual sampling decision depends on the probabilistic algorithm
}

// Helper functions
func createSimpleTraceWithTraceState(traceID pcommon.TraceID, traceState string) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanIDEmpty())
	span.SetName("test-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Millisecond)))

	if traceState != "" {
		span.TraceState().FromRaw(traceState)
	}

	return traces
}

func getAllSpansFromTrace(traces ptrace.Traces) []ptrace.Span {
	var spans []ptrace.Span
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				spans = append(spans, ss.Spans().At(k))
			}
		}
	}
	return spans
}
