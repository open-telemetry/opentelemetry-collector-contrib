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

func TestPercentageSampling(t *testing.T) {
	tests := []struct {
		name                       string
		samplingPercentage         float32
		includedAlreadySampled     bool // if set 50% of the traces will have a Sampled decision
		expectedSamplingPercentage float32
		total                      int
	}{
		{
			"sampling percentage 100",
			100,
			false,
			100,
			2000,
		},
		{
			"sampling percentage 0",
			0,
			false,
			0,
			2000,
		},
		{
			"sampling percentage 33",
			33,
			false,
			33,
			2000,
		},
		{
			"sampling percentage -50",
			-50,
			false,
			0,
			2000,
		},
		{
			"sampling percentage 150",
			150,
			false,
			100,
			2000,
		},
		{
			"sampling percentage 50 with already sampled traces",
			50,
			true,
			25,
			2000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var empty = map[string]pdata.AttributeValue{}

			percentageFilter, err := NewPercentageFilter(zap.NewNop(), tt.samplingPercentage)
			assert.NoError(t, err)

			sampled := 0
			for i := 0; i < tt.total; i++ {
				trace := newTraceStringAttrs(empty, "example", "value")
				if tt.includedAlreadySampled && i%2 == 0 {
					trace.Decisions = []Decision{Sampled}
				}

				traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

				decision, err := percentageFilter.Evaluate(traceID, trace)
				assert.NoError(t, err)

				if decision == Sampled {
					sampled++
				}
			}

			assert.InDelta(t, tt.expectedSamplingPercentage/100, float32(sampled)/float32(tt.total), 0.01)
		})
	}
}

func TestOnLateArrivingSpans_PercentageSampling(t *testing.T) {
	percentageFilter, err := NewPercentageFilter(zap.NewNop(), 0.1)
	assert.Nil(t, err)

	err = percentageFilter.OnLateArrivingSpans(NotSampled, nil)
	assert.Nil(t, err)
}
