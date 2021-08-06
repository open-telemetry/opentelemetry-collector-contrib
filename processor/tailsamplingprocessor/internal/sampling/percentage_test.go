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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func TestPercentageSampling(t *testing.T) {
	var empty = map[string]pdata.AttributeValue{}

	cases := []float32{1, 10, 12.5, 33, 50, 66}

	for _, percentage := range cases {
		t.Run(fmt.Sprintf("sample %f", percentage), func(t *testing.T) {
			trace := newTraceStringAttrs(empty, "example", "value")
			traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

			percentageFilter, err := NewPercentageFilter(zap.NewNop(), percentage)
			assert.NoError(t, err)

			traceCount := 2000
			sampled := 0

			for i := 0; i < traceCount; i++ {
				decision, err := percentageFilter.Evaluate(traceID, trace)
				assert.NoError(t, err)

				if decision == Sampled {
					sampled++
				}
			}

			assert.InDelta(t, (percentage/100)*float32(traceCount), sampled, 0.001, "Amount of sampled traces")
		})
	}
}

func TestPercentageSampling_ignoreAlreadySampledTraces(t *testing.T) {
	var empty = map[string]pdata.AttributeValue{}

	trace := newTraceStringAttrs(empty, "example", "value")
	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	var percentage float32 = 33

	percentageFilter, err := NewPercentageFilter(zap.NewNop(), percentage)
	assert.NoError(t, err)

	traceCount := 100
	sampled := 0

	for i := 0; i < traceCount; i++ {
		trace.Decisions = []Decision{NotSampled, NotSampled}
		decision, err := percentageFilter.Evaluate(traceID, trace)
		assert.NoError(t, err)

		if decision == Sampled {
			sampled++
		}

		// trace has been sampled, should be ignored
		trace.Decisions = []Decision{NotSampled, Sampled}
		decision, err = percentageFilter.Evaluate(traceID, trace)
		assert.NoError(t, err)
		assert.Equal(t, decision, NotSampled)
	}

	assert.EqualValues(t, (percentage/100)*float32(traceCount), sampled)
}

func TestOnLateArrivingSpans_PercentageSampling(t *testing.T) {
	percentageFilter, err := NewPercentageFilter(zap.NewNop(), 0.1)
	assert.Nil(t, err)

	err = percentageFilter.OnLateArrivingSpans(NotSampled, nil)
	assert.Nil(t, err)
}
