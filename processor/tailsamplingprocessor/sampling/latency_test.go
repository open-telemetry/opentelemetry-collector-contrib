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
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestEvaluate_Latency(t *testing.T) {
	filter := NewLatency(zap.NewNop(), 5000)

	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	decision, err := filter.Evaluate(traceID, newTraceWithDuration(4500*time.Millisecond))
	assert.Nil(t, err)
	assert.Equal(t, decision, NotSampled)

	decision, err = filter.Evaluate(traceID, newTraceWithDuration(5500*time.Millisecond))
	assert.Nil(t, err)
	assert.Equal(t, decision, Sampled)
}

func TestOnLateArrivingSpans_Latency(t *testing.T) {
	filter := NewLatency(zap.NewNop(), 5000)
	err := filter.OnLateArrivingSpans(NotSampled, nil)
	assert.Nil(t, err)
}

func newTraceWithDuration(duration time.Duration) *TraceData {
	now := time.Now()

	var traceBatches []pdata.Traces
	traces := pdata.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetStartTimestamp(pdata.TimestampFromTime(now))
	span.SetEndTimestamp(pdata.TimestampFromTime(now.Add(duration)))
	traceBatches = append(traceBatches, traces)
	return &TraceData{
		ReceivedBatches: traceBatches,
	}
}
