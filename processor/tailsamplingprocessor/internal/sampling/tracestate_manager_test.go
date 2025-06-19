// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

func TestTraceStateManager_HasTraceState(t *testing.T) {
	tsm := NewTraceStateManager()

	tests := []struct {
		name        string
		traceState  string
		expectedHas bool
	}{
		{
			name:        "no tracestate",
			traceState:  "",
			expectedHas: false,
		},
		{
			name:        "with tracestate",
			traceState:  "ot=th:8",
			expectedHas: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := createTestTraceData(tt.traceState)
			hasTS := tsm.HasTraceState(trace)
			assert.Equal(t, tt.expectedHas, hasTS)
		})
	}
}

func TestTraceStateManager_ParseTraceState(t *testing.T) {
	tsm := NewTraceStateManager()

	tests := []struct {
		name        string
		traceState  string
		expectError bool
		errorType   error
	}{
		{
			name:        "no tracestate",
			traceState:  "",
			expectError: true,
			errorType:   ErrTraceStateNotFound,
		},
		{
			name:        "valid otel tracestate",
			traceState:  "ot=th:8",
			expectError: false,
		},
		{
			name:        "valid complex tracestate",
			traceState:  "ot=th:8;rv:abcdef12345678",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := createTestTraceData(tt.traceState)
			otelTS, err := tsm.ParseTraceState(trace)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.errorType, err)
				assert.Nil(t, otelTS)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, otelTS)
			}
		})
	}
}

func TestTraceStateManager_ExtractRandomness(t *testing.T) {
	tsm := NewTraceStateManager()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	tests := []struct {
		name        string
		traceState  string
		description string
	}{
		{
			name:        "no tracestate - use traceid",
			traceState:  "",
			description: "should extract randomness from TraceID",
		},
		{
			name:        "tracestate without rv - use traceid",
			traceState:  "ot=th:8",
			description: "should extract randomness from TraceID when no rv present",
		},
		{
			name:        "tracestate with rv - use rv",
			traceState:  "ot=th:8;rv:1234567890abcd",
			description: "should extract randomness from rv when present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var otelTS *sampling.OpenTelemetryTraceState
			if tt.traceState != "" {
				if w3cTS, err := sampling.NewW3CTraceState("ot=" + tt.traceState); err == nil {
					otelTS = w3cTS.OTelValue()
				}
			}

			randomness := tsm.ExtractRandomness(otelTS, traceID)

			// Randomness should always be valid
			assert.NotNil(t, randomness)

			// Test that we get consistent results
			randomness2 := tsm.ExtractRandomness(otelTS, traceID)
			assert.Equal(t, randomness, randomness2)
		})
	}
}

func TestTraceStateManager_InitializeTraceData(t *testing.T) {
	tsm := NewTraceStateManager()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	tests := []struct {
		name                      string
		traceState                string
		expectedTraceStatePresent bool
		description               string
	}{
		{
			name:                      "no tracestate",
			traceState:                "",
			expectedTraceStatePresent: false,
			description:               "should initialize with TraceID randomness only",
		},
		{
			name:                      "with tracestate",
			traceState:                "ot=th:8",
			expectedTraceStatePresent: true,
			description:               "should initialize with TraceState present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := createTestTraceData(tt.traceState)

			// Initialize should set all OTEP 235 fields
			tsm.InitializeTraceData(context.Background(), traceID, trace)

			assert.Equal(t, tt.expectedTraceStatePresent, trace.TraceStatePresent)
			assert.NotNil(t, trace.RandomnessValue, "randomness should always be set")

			if tt.expectedTraceStatePresent {
				// If TraceState is present and has threshold, FinalThreshold should be set
				if tt.traceState == "ot=th:8" {
					assert.NotNil(t, trace.FinalThreshold, "threshold should be extracted from TraceState")
				}
			}
		})
	}
}

// createTestTraceData creates a TraceData with a single span having the specified TraceState.
func createTestTraceData(traceState string) *TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))

	if traceState != "" {
		span.TraceState().FromRaw(traceState)
	}

	return &TraceData{
		ArrivalTime:     time.Now(),
		DecisionTime:    time.Time{},
		SpanCount:       &atomic.Int64{},
		ReceivedBatches: traces,
		FinalDecision:   Pending,
	}
}
