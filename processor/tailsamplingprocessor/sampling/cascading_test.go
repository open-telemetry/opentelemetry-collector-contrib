// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	tsconfig "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/config"
)

func fillSpan(span *pdata.Span, durationMicros int64) {
	nowTs := time.Now().UnixNano()
	startTime := nowTs - durationMicros*1000

	span.Attributes().InsertInt("foo", 55)
	span.SetStartTimestamp(pdata.Timestamp(startTime))
	span.SetEndTimestamp(pdata.Timestamp(nowTs))
}

func createTrace(numSpans int, durationMicros int64) *TraceData {
	var traceBatches []pdata.Traces

	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rs := traces.ResourceSpans().At(0)
	rs.InstrumentationLibrarySpans().Resize(1)
	ils := rs.InstrumentationLibrarySpans().At(0)

	ils.Spans().Resize(numSpans)

	for i := 0; i < numSpans; i++ {
		span := ils.Spans().At(i)
		//span.SetTraceID(pdata.NewTraceID([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
		//span.SetSpanID(pdata.NewSpanID([]byte{1, 2, 3, 4, 5, 6, 7, byte(i)}))

		fillSpan(&span, durationMicros)
	}

	traceBatches = append(traceBatches, traces)

	return &TraceData{
		Mutex:           sync.Mutex{},
		Decisions:       nil,
		ArrivalTime:     time.Time{},
		DecisionTime:    time.Time{},
		SpanCount:       int64(numSpans),
		ReceivedBatches: traceBatches,
	}
}

func createCascadingEvaluator() PolicyEvaluator {
	testValue := int64(10000)

	config := tsconfig.PolicyCfg{
		Name:           "test-policy-5",
		Type:           tsconfig.Cascading,
		SpansPerSecond: 1000,
		Rules: []tsconfig.CascadingRuleCfg{
			{
				Name:           "duration",
				SpansPerSecond: 10,
				PropertiesCfg: &tsconfig.PropertiesCfg{
					MinDurationMicros: &testValue,
				},
			},
			{
				Name:           "everything_else",
				SpansPerSecond: -1,
			},
		},
	}

	cascading, _ := NewCascadingFilter(zap.NewNop(), &config)
	return cascading
}

func TestSampling(t *testing.T) {
	cascading := createCascadingEvaluator()

	decision, _ := cascading.Evaluate(pdata.NewTraceID([16]byte{0}), createTrace(8, 1000000))
	require.Equal(t, Sampled, decision)

	decision, _ = cascading.Evaluate(pdata.NewTraceID([16]byte{1}), createTrace(8, 1000000))
	require.Equal(t, NotSampled, decision)
}

func TestSecondChanceEvaluation(t *testing.T) {
	cascading := createCascadingEvaluator()

	decision, _ := cascading.Evaluate(pdata.NewTraceID([16]byte{0}), createTrace(8, 1000))
	require.Equal(t, SecondChance, decision)

	decision, _ = cascading.Evaluate(pdata.NewTraceID([16]byte{1}), createTrace(8, 1000))
	require.Equal(t, SecondChance, decision)

	// This would never fit anyway
	decision, _ = cascading.Evaluate(pdata.NewTraceID([16]byte{1}), createTrace(8000, 1000))
	require.Equal(t, NotSampled, decision)
}

func TestSecondChanceReevaluation(t *testing.T) {
	cascading := createCascadingEvaluator()

	decision, _ := cascading.EvaluateSecondChance(pdata.NewTraceID([16]byte{1}), createTrace(100, 1000))
	require.Equal(t, Sampled, decision)

	// Too much
	decision, _ = cascading.EvaluateSecondChance(pdata.NewTraceID([16]byte{1}), createTrace(1000, 1000))
	require.Equal(t, NotSampled, decision)

	// Just right
	decision, _ = cascading.EvaluateSecondChance(pdata.NewTraceID([16]byte{1}), createTrace(900, 1000))
	require.Equal(t, Sampled, decision)
}
