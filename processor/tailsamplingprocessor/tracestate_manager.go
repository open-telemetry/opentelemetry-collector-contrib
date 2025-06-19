package tailsamplingprocessor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TraceStateManager handles parsing and updating of W3C tracestate headers
// for OTEP 235 sampling fields (t-value and r-value)
type TraceStateManager struct{}

// NewTraceStateManager creates a new TraceStateManager
func NewTraceStateManager() *TraceStateManager {
	return &TraceStateManager{}
}

// ParseTraceState extracts sampling-related information from a span's tracestate
func (tsm *TraceStateManager) ParseTraceState(span ptrace.Span) (threshold sampling.Threshold, randomness sampling.Randomness, hasThreshold, hasRandomness bool) {
	traceState := span.TraceState()

	// Default values
	threshold = sampling.AlwaysSampleThreshold
	hasThreshold = false
	hasRandomness = false

	if traceState.AsRaw() != "" {
		// Parse W3C tracestate
		w3cTraceState, err := sampling.NewW3CTraceState(traceState.AsRaw())
		if err == nil {
			// Get OpenTelemetry tracestate section
			otelTraceState := w3cTraceState.OTelValue()

			// Extract threshold (t-value)
			threshold, hasThreshold = otelTraceState.TValueThreshold()

			// Extract randomness (r-value)
			randomness, hasRandomness = otelTraceState.RValueRandomness()
		}
	}

	// If no r-value in tracestate, derive from TraceID
	if !hasRandomness {
		randomness = sampling.TraceIDToRandomness(span.TraceID())
		hasRandomness = true
	}

	return threshold, randomness, hasThreshold, hasRandomness
}

// ShouldSample determines if a span should be sampled based on threshold and randomness
func (tsm *TraceStateManager) ShouldSample(threshold sampling.Threshold, randomness sampling.Randomness) bool {
	return threshold.ShouldSample(randomness)
}
