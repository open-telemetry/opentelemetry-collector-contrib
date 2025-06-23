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

// UpdateTraceState updates all spans in the trace with the constraint threshold.
// Phase 6: Uses pkg/sampling equalizing pattern - each span gets the more restrictive of
// its original threshold vs the constraint threshold. This enables proper per-span adjusted counts.
func (tsm *TraceStateManager) UpdateTraceState(trace *TraceData, constraintThreshold sampling.Threshold) error {
	if !trace.TraceStatePresent {
		return nil // No TraceState to update
	}

	batches := trace.ReceivedBatches
	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				spanID := span.SpanID()

				// Update TraceState for this specific span using pkg/sampling API
				finalThreshold, err := tsm.updateSpanTraceStateAndGetFinal(span, constraintThreshold)
				if err != nil {
					// Log but continue processing other spans
					continue
				}

				// Update the SpanThresholds map with the final threshold for this span
				if spanInfo, exists := trace.SpanThresholds[spanID]; exists {
					spanInfo.FinalThreshold = &finalThreshold
				}
			}
		}
	}

	// Set the trace-level constraint threshold
	trace.FinalThreshold = &constraintThreshold
	return nil
}

// updateSpanTraceStateAndGetFinal updates the TraceState for a single span with the constraint threshold.
// Uses pkg/sampling equalizing pattern: choose more restrictive of original vs constraint.
// Returns the final effective threshold that was applied.
func (tsm *TraceStateManager) updateSpanTraceStateAndGetFinal(span ptrace.Span, constraintThreshold sampling.Threshold) (sampling.Threshold, error) {
	traceStateRaw := span.TraceState().AsRaw()
	if traceStateRaw == "" {
		return constraintThreshold, nil // No TraceState to update, return constraint as final
	}

	// Parse the current TraceState
	w3cTS, err := sampling.NewW3CTraceState(traceStateRaw)
	if err != nil {
		return constraintThreshold, err
	}

	otelTS := w3cTS.OTelValue()
	// Determine the effective threshold using pkg/sampling equalizing pattern
	var effectiveThreshold sampling.Threshold
	if originalThreshold, hasThreshold := otelTS.TValueThreshold(); hasThreshold {
		// If original is more restrictive than constraint, use original
		// Otherwise use constraint (this prevents inconsistent sampling)
		if sampling.ThresholdLessThan(constraintThreshold, originalThreshold) {
			effectiveThreshold = originalThreshold
		} else {
			effectiveThreshold = constraintThreshold
		}
	} else {
		// No original threshold, use constraint
		effectiveThreshold = constraintThreshold
	}

	// Update the threshold using pkg/sampling API
	err = otelTS.UpdateTValueWithSampling(effectiveThreshold)
	if err != nil {
		return effectiveThreshold, err
	}

	// Serialize the updated TraceState back to the span
	var w strings.Builder
	if err := w3cTS.Serialize(&w); err != nil {
		return effectiveThreshold, err
	}

	span.TraceState().FromRaw(w.String())
	return effectiveThreshold, nil
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
// Phase 6: Enhanced to support per-span threshold tracking for accurate adjusted counts.
// This should be called when a new trace is first encountered.
func (tsm *TraceStateManager) InitializeTraceData(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) {
	// Check if any spans have TraceState
	trace.TraceStatePresent = tsm.HasTraceState(trace)

	// Initialize per-span threshold tracking
	trace.SpanThresholds = make(map[pcommon.SpanID]*SpanThresholdInfo)

	// Extract TraceState if present and process each span individually
	var globalOtelTS *sampling.OpenTelemetryTraceState
	if trace.TraceStatePresent {
		if parsed, err := tsm.ParseTraceState(trace); err == nil {
			globalOtelTS = parsed
		}
	}

	// Extract randomness (either from TraceState rv or TraceID)
	trace.RandomnessValue = tsm.ExtractRandomness(globalOtelTS, traceID)

	// Process each span to extract individual threshold information
	var mostRestrictiveThreshold *sampling.Threshold

	batches := trace.ReceivedBatches
	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				spanID := span.SpanID()

				// Parse individual span's TraceState
				var spanThreshold *sampling.Threshold
				if span.TraceState().AsRaw() != "" {
					if w3cTS, err := sampling.NewW3CTraceState(span.TraceState().AsRaw()); err == nil {
						otelTS := w3cTS.OTelValue()
						if threshold, err := tsm.ExtractThreshold(otelTS); err == nil {
							spanThreshold = threshold
						}
					}
				}

				// Track per-span threshold information
				spanInfo := &SpanThresholdInfo{
					SpanID:            spanID,
					OriginalThreshold: spanThreshold,
					FinalThreshold:    nil, // Will be set when sampling decision is made
				}
				trace.SpanThresholds[spanID] = spanInfo

				// Track most restrictive threshold across all spans
				if spanThreshold != nil {
					if mostRestrictiveThreshold == nil {
						mostRestrictiveThreshold = spanThreshold
					} else {
						// Use the more restrictive (higher) threshold
						if sampling.ThresholdGreater(*spanThreshold, *mostRestrictiveThreshold) {
							mostRestrictiveThreshold = spanThreshold
						}
					}
				}
			}
		}
	}

	// Set trace-level threshold to most restrictive found
	if mostRestrictiveThreshold != nil {
		trace.FinalThreshold = mostRestrictiveThreshold
	}
}

// CalculateSpanAdjustedCount calculates the adjusted count for a specific span
// based on its original threshold and the final threshold applied.
// Phase 6: This implements per-span adjusted count calculation for accurate observability.
func (tsm *TraceStateManager) CalculateSpanAdjustedCount(spanID pcommon.SpanID, trace *TraceData) float64 {
	spanInfo, exists := trace.SpanThresholds[spanID]
	if !exists {
		// No threshold information available for this span
		return 0.0
	}

	// If no final threshold was applied, return 0 (span was not sampled)
	if spanInfo.FinalThreshold == nil {
		return 0.0
	}

	// If no original threshold, use the final threshold directly
	if spanInfo.OriginalThreshold == nil {
		return spanInfo.FinalThreshold.AdjustedCount()
	}

	// For OTEP 235 per-span adjusted count calculation:
	// Adjusted count = original sampling probability / final sampling probability
	// This represents how many original spans this sampled span represents

	originalProb := spanInfo.OriginalThreshold.Probability()
	finalProb := spanInfo.FinalThreshold.Probability()

	if finalProb == 0.0 {
		return 0.0 // Should not have been sampled
	}

	return originalProb / finalProb
}

// GetSpanAdjustedCount calculates the adjusted count for a span based on its final threshold.
// Phase 6: This is primarily used for testing and validation to verify that per-span
// threshold adjustments produce the correct adjusted counts. In production, the
// observability backend calculates adjusted counts from the updated TraceState.
func (tsm *TraceStateManager) GetSpanAdjustedCount(span ptrace.Span) float64 {
	traceStateRaw := span.TraceState().AsRaw()
	if traceStateRaw == "" {
		return 1.0 // No sampling information means count = 1
	}

	// Parse the TraceState to get the threshold
	w3cTS, err := sampling.NewW3CTraceState(traceStateRaw)
	if err != nil {
		return 1.0 // Parse error, default to 1
	}

	otelTS := w3cTS.OTelValue()
	if threshold, ok := otelTS.TValueThreshold(); ok {
		// Use the pkg/sampling API to calculate adjusted count
		return threshold.AdjustedCount()
	}

	return 1.0 // No threshold, default to 1
}

// GetSpanThresholds returns a copy of the span threshold information for external access
// Phase 6: Allows other components to access per-span threshold data
func (tsm *TraceStateManager) GetSpanThresholds(trace *TraceData) map[pcommon.SpanID]*SpanThresholdInfo {
	result := make(map[pcommon.SpanID]*SpanThresholdInfo)
	for spanID, info := range trace.SpanThresholds {
		// Create a copy to avoid external modification
		result[spanID] = &SpanThresholdInfo{
			SpanID:            info.SpanID,
			OriginalThreshold: info.OriginalThreshold,
			FinalThreshold:    info.FinalThreshold,
		}
	}
	return result
}
