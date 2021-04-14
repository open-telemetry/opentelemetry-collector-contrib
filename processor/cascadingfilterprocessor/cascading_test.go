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

package cascadingfilterprocessor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	cfconfig "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/sampling"
)

var testValue = 10 * time.Millisecond
var cfg = cfconfig.Config{
	ProcessorSettings:       &config.ProcessorSettings{},
	DecisionWait:            2 * time.Second,
	NumTraces:               100,
	ExpectedNewTracesPerSec: 100,
	SpansPerSecond:          1000,
	PolicyCfgs: []cfconfig.PolicyCfg{
		{
			Name:           "duration",
			SpansPerSecond: 10,
			PropertiesCfg: cfconfig.PropertiesCfg{
				MinDuration: &testValue,
			},
		},
		{
			Name:           "everything else",
			SpansPerSecond: -1,
		},
	},
}

func fillSpan(span *pdata.Span, durationMicros int64) {
	nowTs := time.Now().UnixNano()
	startTime := nowTs - durationMicros*1000

	span.Attributes().InsertInt("foo", 55)
	span.SetStartTimestamp(pdata.Timestamp(startTime))
	span.SetEndTimestamp(pdata.Timestamp(nowTs))
}

func createTrace(fsp *cascadingFilterSpanProcessor, numSpans int, durationMicros int64) *sampling.TraceData {
	var traceBatches []pdata.Traces

	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rs := traces.ResourceSpans().At(0)
	rs.InstrumentationLibrarySpans().Resize(1)
	ils := rs.InstrumentationLibrarySpans().At(0)

	ils.Spans().Resize(numSpans)

	for i := 0; i < numSpans; i++ {
		span := ils.Spans().At(i)

		fillSpan(&span, durationMicros)
	}

	traceBatches = append(traceBatches, traces)

	return &sampling.TraceData{
		Mutex:           sync.Mutex{},
		Decisions:       make([]sampling.Decision, len(fsp.policies)),
		ArrivalTime:     time.Time{},
		DecisionTime:    time.Time{},
		SpanCount:       int64(numSpans),
		ReceivedBatches: traceBatches,
	}
}

func createCascadingEvaluator(t *testing.T) *cascadingFilterSpanProcessor {
	cascading, err := newCascadingFilterSpanProcessor(zap.NewNop(), nil, cfg)
	assert.NoError(t, err)
	return cascading
}

func TestSampling(t *testing.T) {
	cascading := createCascadingEvaluator(t)

	decision, policy := cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{0}), createTrace(cascading, 8, 1000000))
	require.NotNil(t, policy)
	require.Equal(t, sampling.Sampled, decision)

	decision, _ = cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{1}), createTrace(cascading, 1000, 1000))
	require.Equal(t, sampling.SecondChance, decision)
}

func TestSecondChanceEvaluation(t *testing.T) {
	cascading := createCascadingEvaluator(t)

	decision, _ := cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{0}), createTrace(cascading, 8, 1000))
	require.Equal(t, sampling.SecondChance, decision)

	decision, _ = cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{1}), createTrace(cascading, 8, 1000))
	require.Equal(t, sampling.SecondChance, decision)

	// TODO: This could me optimized to make a decision within cascadingfilter processor, as such span would never fit anyway
	//decision, _ = cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{1}), createTrace(8000, 1000), metrics)
	//require.Equal(t, sampling.NotSampled, decision)
}

func TestProbabilisticFilter(t *testing.T) {
	ratio := float32(0.5)
	cfg.ProbabilisticFilteringRatio = &ratio
	cascading := createCascadingEvaluator(t)

	trace1 := createTrace(cascading, 8, 1000000)
	decision, _ := cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{0}), trace1)
	require.Equal(t, sampling.Sampled, decision)
	require.True(t, trace1.SelectedByProbabilisticFilter)

	trace2 := createTrace(cascading, 800, 1000000)
	decision, _ = cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{1}), trace2)
	require.Equal(t, sampling.SecondChance, decision)
	require.False(t, trace2.SelectedByProbabilisticFilter)

	ratio = float32(0.0)
	cfg.ProbabilisticFilteringRatio = &ratio
}

//func TestSecondChanceReevaluation(t *testing.T) {
//	cascading := createCascadingEvaluator()
//
//	decision, _ := cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{1}), createTrace(100, 1000), metrics)
//	require.Equal(t, sampling.Sampled, decision)
//
//	// Too much
//	decision, _ = cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{1}), createTrace(1000, 1000), metrics)
//	require.Equal(t, sampling.NotSampled, decision)
//
//	// Just right
//	decision, _ = cascading.makeProvisionalDecision(pdata.NewTraceID([16]byte{1}), createTrace(900, 1000), metrics)
//	require.Equal(t, sampling.Sampled, decision)
//}
