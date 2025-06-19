package tailsamplingprocessor

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestTraceStateManager_ParseTraceState(t *testing.T) {
	manager := NewTraceStateManager()

	tests := []struct {
		name          string
		traceState    string
		expectError   bool
		hasThreshold  bool
		hasRandomness bool
	}{
		{
			name:          "empty tracestate",
			traceState:    "",
			expectError:   false,
			hasThreshold:  false,
			hasRandomness: true, // Should derive from TraceID
		},
		{
			name:          "valid otep 235 fields",
			traceState:    "ot=th:8;rv:abcdef12345678",
			expectError:   false,
			hasThreshold:  true,
			hasRandomness: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
			span.TraceState().FromRaw(tt.traceState)

			threshold, randomness, hasThreshold, hasRandomness := manager.ParseTraceState(span)

			assert.Equal(t, tt.hasThreshold, hasThreshold)
			// We always expect randomness (either from tracestate or TraceID)
			assert.True(t, hasRandomness)

			// Verify basic types
			assert.IsType(t, sampling.Threshold{}, threshold)
			assert.IsType(t, sampling.Randomness{}, randomness)
		})
	}
}

func TestTraceStateManager_ShouldSample(t *testing.T) {
	manager := NewTraceStateManager()

	// Create a very low threshold (high sampling probability)
	lowThreshold := sampling.AlwaysSampleThreshold

	// Create a medium randomness value
	mediumRandomness, err := sampling.UnsignedToRandomness(sampling.MaxAdjustedCount / 2)
	require.NoError(t, err)

	// Should sample with always-sample threshold
	shouldSample := manager.ShouldSample(lowThreshold, mediumRandomness)
	assert.True(t, shouldSample)

	// Should not sample with never-sample threshold
	shouldNotSample := manager.ShouldSample(sampling.NeverSampleThreshold, mediumRandomness)
	assert.False(t, shouldNotSample)
}
