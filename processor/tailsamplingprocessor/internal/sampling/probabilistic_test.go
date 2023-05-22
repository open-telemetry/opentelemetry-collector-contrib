// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
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

				decision, err := probabilisticSampler.Evaluate(context.Background(), traceID, trace)
				assert.NoError(t, err)

				if decision == Sampled {
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

func genRandomTraceIDs(num int) (ids []pcommon.TraceID) {
	r := rand.New(rand.NewSource(1))
	ids = make([]pcommon.TraceID, 0, num)
	for i := 0; i < num; i++ {
		traceID := [16]byte{}
		binary.BigEndian.PutUint64(traceID[:8], r.Uint64())
		binary.BigEndian.PutUint64(traceID[8:], r.Uint64())
		ids = append(ids, pcommon.TraceID(traceID))
	}
	return ids
}
