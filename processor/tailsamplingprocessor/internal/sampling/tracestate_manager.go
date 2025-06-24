// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"errors"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

var (
	// ErrTraceStateNotFound indicates no TraceState was found in any spans
	ErrTraceStateNotFound = errors.New("no TraceState found")
	// ErrNoThresholdInTraceState indicates TraceState exists but has no 'th' value
	ErrNoThresholdInTraceState = errors.New("no threshold found in TraceState")
)

// TraceStateManager manages TraceState parsing, validation, and updates
// for OTEP 235 consistent probability sampling.
type TraceStateManager struct{}

// NewTraceStateManager creates a new TraceStateManager instance.
func NewTraceStateManager() *TraceStateManager {
	return &TraceStateManager{}
}

// ParseTraceState extracts TraceState from the first span that has one.
// Returns the parsed OpenTelemetryTraceState or an error if no valid TraceState found.
func (tsm *TraceStateManager) ParseTraceState(trace *TraceData) (*sampling.OpenTelemetryTraceState, error) {
	batches := trace.ReceivedBatches

	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				traceState := span.TraceState()
				if traceState.AsRaw() == "" {
					continue
				}

				// Parse the W3C TraceState
				w3cTS, err := sampling.NewW3CTraceState(traceState.AsRaw())
				if err != nil {
					continue // Skip invalid TraceState, try next span
				}

				// Get OpenTelemetry section
				otelTS := w3cTS.OTelValue()
				return otelTS, nil
			}
		}
	}

	return nil, ErrTraceStateNotFound
}

// ExtractThreshold gets the threshold from a parsed OpenTelemetryTraceState.
// Returns the threshold or an error if no 'th' value is present.
func (tsm *TraceStateManager) ExtractThreshold(otelTS *sampling.OpenTelemetryTraceState) (*sampling.Threshold, error) {
	if otelTS == nil {
		return nil, ErrNoThresholdInTraceState
	}

	threshold, hasThreshold := otelTS.TValueThreshold()
	if !hasThreshold {
		return nil, ErrNoThresholdInTraceState
	}

	return &threshold, nil
}

// ExtractRandomness gets randomness from TraceState rv value or falls back to TraceID.
// Always returns valid randomness value for sampling decisions.
func (tsm *TraceStateManager) ExtractRandomness(otelTS *sampling.OpenTelemetryTraceState, traceID pcommon.TraceID) sampling.Randomness {
	if otelTS != nil {
		// Try to get explicit randomness value from TraceState
		if randomness, hasRandomness := otelTS.RValueRandomness(); hasRandomness {
			return randomness
		}
	}

	// Fall back to TraceID randomness (standard OTEP 235 behavior)
	return sampling.TraceIDToRandomness(traceID)
}

// UpdateTraceState updates all spans in the trace with the final constraint threshold.
// Simplified to apply the same final threshold to all spans uniformly.
func (tsm *TraceStateManager) UpdateTraceState(trace *TraceData, constraintThreshold sampling.Threshold) error {
	// Early return if no TraceState to update
	if !trace.TraceStatePresent {
		return nil
	}

	// TODO: For consistency validation, check that all spans yield the same randomness value.
	// Log warnings if inconsistencies are detected (could indicate upstream sampling issues).

	// Update TraceState in all spans uniformly with the constraint threshold
	batches := trace.ReceivedBatches
	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				// Update TraceState for this span - ignore errors to keep processing simple
				tsm.updateSpanTraceState(span, constraintThreshold)
			}
		}
	}

	// Set the trace-level constraint threshold
	trace.FinalThreshold = &constraintThreshold
	return nil
}

// updateSpanTraceState updates the TraceState for a single span with the constraint threshold.
// Simplified to just apply the constraint threshold directly.
func (tsm *TraceStateManager) updateSpanTraceState(span ptrace.Span, constraintThreshold sampling.Threshold) {
	traceStateRaw := span.TraceState().AsRaw()
	if traceStateRaw == "" {
		return // No TraceState to update
	}

	// Parse the current TraceState
	w3cTS, err := sampling.NewW3CTraceState(traceStateRaw)
	if err != nil {
		return // Skip invalid TraceState
	}

	otelTS := w3cTS.OTelValue()
	
	// Simply apply the constraint threshold (simplified approach)
	err = otelTS.UpdateTValueWithSampling(constraintThreshold)
	if err != nil {
		return // Skip on error
	}

	// Serialize the updated TraceState back to the span
	var w strings.Builder
	if err := w3cTS.Serialize(&w); err != nil {
		return // Skip on error
	}

	span.TraceState().FromRaw(w.String())
}

// HasTraceState checks if any span in the trace has TraceState.
// This is used for optimization to avoid parsing when not needed.
func (tsm *TraceStateManager) HasTraceState(trace *TraceData) bool {
	batches := trace.ReceivedBatches

	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				if span.TraceState().AsRaw() != "" {
					return true
				}
			}
		}
	}

	return false
}

// InitializeTraceData initializes OTEP 235 fields in TraceData based on TraceID and TraceState.
// Uses simplified trace-level threshold management for essential OTEP 235 functionality.
// This should be called when a new trace is first encountered.
func (tsm *TraceStateManager) InitializeTraceData(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) {
	// Check if any spans have TraceState
	trace.TraceStatePresent = tsm.HasTraceState(trace)

	// Extract TraceState if present (use first span with TraceState as representative)
	var globalOtelTS *sampling.OpenTelemetryTraceState
	if trace.TraceStatePresent {
		if parsed, err := tsm.ParseTraceState(trace); err == nil {
			globalOtelTS = parsed
		}
	}

	// Extract randomness (either from TraceState rv or TraceID)
	trace.RandomnessValue = tsm.ExtractRandomness(globalOtelTS, traceID)

	// TODO: For consistency validation, check that all spans have the same randomness value.
	// Inconsistent randomness could indicate upstream sampling inconsistencies.

	// Extract threshold from the first span with TraceState (simplified approach)
	if trace.TraceStatePresent && globalOtelTS != nil {
		if threshold, err := tsm.ExtractThreshold(globalOtelTS); err == nil {
			trace.FinalThreshold = threshold
		}
	}
}

// TODO: For enhanced consistency validation, consider adding methods to:
// 1. ValidateTraceRandomnessConsistency() - check all spans have same randomness
// 2. DetectUpstreamSamplingInconsistencies() - log warnings for inconsistent traces
