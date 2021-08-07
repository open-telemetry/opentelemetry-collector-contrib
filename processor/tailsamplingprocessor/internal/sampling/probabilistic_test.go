// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func TestProbabilisticSampling(t *testing.T) {
	tests := []struct {
		name                       string
		samplingPercentage         float32
		includedAlreadySampled     bool // if set 50% of the traces will have a Sampled decision
		expectedSamplingPercentage float32
	}{
		{
			"sampling percentage 100",
			100,
			false,
			100,
		},
		{
			"sampling percentage 0",
			0,
			false,
			0,
		},
		{
			"sampling percentage 33",
			33,
			false,
			33,
		},
		{
			"sampling percentage -50",
			-50,
			false,
			0,
		},
		{
			"sampling percentage 150",
			150,
			false,
			100,
		},
		{
			"sampling percentage 50 with already sampled traces",
			50,
			true,
			25,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceCount := 200

			var emptyAttrs = map[string]pdata.AttributeValue{}

			probabilisticSampler := NewProbabilisticSampler(zap.NewNop(), tt.samplingPercentage)

			sampled := 0
			for i := 0; i < traceCount; i++ {
				trace := newTraceStringAttrs(emptyAttrs, "example", "value")
				if tt.includedAlreadySampled && i%2 == 0 {
					trace.Decisions = []Decision{Sampled}
				}

				traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

				decision, err := probabilisticSampler.Evaluate(traceID, trace)
				assert.NoError(t, err)

				if decision == Sampled {
					sampled++
				}
			}

			assert.InDelta(t, tt.expectedSamplingPercentage/100, float32(sampled)/float32(traceCount), 0.01)
		})
	}
}

func TestOnLateArrivingSpans_PercentageSampling(t *testing.T) {
	probabilisticSampler := NewProbabilisticSampler(zap.NewNop(), 10)

	err := probabilisticSampler.OnLateArrivingSpans(NotSampled, nil)
	assert.Nil(t, err)
}
