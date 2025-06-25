// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"encoding/binary"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

func TestProbabilisticSampling(t *testing.T) {
	tests := []struct {
		name                       string
		samplingPercentage         float64
		hashSalt                   string
		expectedSamplingPercentage float64
	}{
		{
			"100%",
			100,
			"",
			100,
		},
		{
			"0%",
			0,
			"",
			0,
		},
		{
			"25%",
			25,
			"",
			25,
		},
		{
			"33%",
			33,
			"",
			33,
		},
		{
			"33% - custom salt",
			33,
			"test-salt",
			33,
		},
		{
			"-%50",
			-50,
			"",
			0,
		},
		{
			"150%",
			150,
			"",
			100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceCount := 100_000

			probabilisticSampler := NewProbabilisticSampler(componenttest.NewNopTelemetrySettings(), tt.hashSalt, tt.samplingPercentage)

			sampled := 0
			for _, traceID := range genRandomTraceIDs(traceCount) {
				trace := newTraceStringAttrs(nil, "example", "value")

				// Initialize OTEP 235 fields as would be done by the processor
				tsm := NewTraceStateManager()
				tsm.InitializeTraceData(context.Background(), traceID, trace)

				decision, err := probabilisticSampler.Evaluate(context.Background(), traceID, trace)
				assert.NoError(t, err)

				// OTEP 235: Use the threshold decision with the trace's randomness
				if decision.ShouldSample(trace.RandomnessValue) {
					sampled++
				}
			}

			effectiveSamplingPercentage := float32(sampled) / float32(traceCount) * 100
			assert.InDelta(t, tt.expectedSamplingPercentage, effectiveSamplingPercentage, 0.2,
				"Effective sampling percentage is %f, expected %f", effectiveSamplingPercentage, tt.expectedSamplingPercentage,
			)
		})
	}
}

// TestProbabilisticThresholds tests that the probabilistic sampler returns correct OTEP 235 thresholds
func TestProbabilisticThresholds(t *testing.T) {
	tests := []struct {
		name               string
		samplingPercentage float64
		expectedThreshold  sampling.Threshold
	}{
		{
			"0% should be NeverSampleThreshold",
			0,
			sampling.NeverSampleThreshold,
		},
		{
			"100% should be AlwaysSampleThreshold",
			100,
			sampling.AlwaysSampleThreshold,
		},
		{
			"negative should be NeverSampleThreshold",
			-10,
			sampling.NeverSampleThreshold,
		},
		{
			"over 100% should be AlwaysSampleThreshold",
			150,
			sampling.AlwaysSampleThreshold,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := NewProbabilisticSampler(componenttest.NewNopTelemetrySettings(), "", tt.samplingPercentage)

			// Create a dummy trace for evaluation
			trace := newTraceStringAttrs(nil, "example", "value")
			traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

			decision, err := sampler.Evaluate(context.Background(), traceID, trace)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedThreshold, decision.Threshold, "Threshold should match expected value for %f%%", tt.samplingPercentage)
		})
	}
}

func genRandomTraceIDs(num int) (ids []pcommon.TraceID) {
	// NOTE: using a fixed seed is intentional here,
	// as otherwise the delta in the tests above will
	// be unpredictable.
	r := rand.New(rand.NewPCG(123, 456))
	ids = make([]pcommon.TraceID, 0, num)
	for i := 0; i < num; i++ {
		traceID := [16]byte{}
		binary.BigEndian.PutUint64(traceID[:8], r.Uint64())
		binary.BigEndian.PutUint64(traceID[8:], r.Uint64())
		ids = append(ids, pcommon.TraceID(traceID))
	}
	return ids
}
