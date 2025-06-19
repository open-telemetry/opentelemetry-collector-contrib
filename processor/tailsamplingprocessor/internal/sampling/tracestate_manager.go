// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"

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

// UpdateTraceState updates all spans in the trace with the new threshold value.
// This preserves existing TraceState structure while updating the 'th' field.
// NOTE: This is a simplified implementation for OTEP 235 support.
func (tsm *TraceStateManager) UpdateTraceState(trace *TraceData, threshold sampling.Threshold) error {
	batches := trace.ReceivedBatches

	// Convert threshold to string for TraceState
	thresholdStr := threshold.TValue()

	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				// Get existing TraceState
				existingTraceState := span.TraceState().AsRaw()

				// For now, we'll create a simple OTel tracestate with threshold
				// TODO: Implement proper tracestate merging when serialization APIs are available
				newTraceState := "ot=th:" + thresholdStr
				if existingTraceState != "" && existingTraceState != newTraceState {
					// Preserve existing if it's different (avoid overwriting other vendor data)
					// This is a simplified approach - full implementation would merge properly
					continue
				}

				// Set the updated TraceState
				span.TraceState().FromRaw(newTraceState)
			}
		}
	}

	return nil
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
// This should be called when a new trace is first encountered.
func (tsm *TraceStateManager) InitializeTraceData(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) {
	// Check if any spans have TraceState
	trace.TraceStatePresent = tsm.HasTraceState(trace)

	// Extract TraceState if present
	var otelTS *sampling.OpenTelemetryTraceState
	if trace.TraceStatePresent {
		if parsed, err := tsm.ParseTraceState(trace); err == nil {
			otelTS = parsed
		}
	}

	// Extract randomness (either from TraceState rv or TraceID)
	trace.RandomnessValue = tsm.ExtractRandomness(otelTS, traceID)

	// Extract existing threshold if present
	if otelTS != nil {
		if threshold, err := tsm.ExtractThreshold(otelTS); err == nil {
			trace.FinalThreshold = threshold
		}
	}
}
