// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func BenchmarkSampling(b *testing.B) {
	traceIDs, batches := generateIDsAndBatches(128)
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, _ := newTracesProcessor(b.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	require.NoError(b, tsp.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, tsp.Shutdown(b.Context()))
	}()
	metrics := newPolicyTickMetrics(len(cfg.PolicyCfgs))
	sampleBatches := make([]*samplingpolicy.TraceData, 0, len(batches))

	for _, batch := range batches {
		sampleBatches = append(sampleBatches, &samplingpolicy.TraceData{
			SpanCount:       0,
			ReceivedBatches: batch,
		})
	}

	for b.Loop() {
		for i, id := range traceIDs {
			_, _ = tsp.makeDecision(id, sampleBatches[i], metrics)
		}
	}
}

func BenchmarkProcessorThroughput(b *testing.B) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    1024,
		// Create a handful of reasonable policies to not only test batching.
		PolicyCfgs: []PolicyCfg{
			{sharedPolicyCfg: sharedPolicyCfg{Name: "always-sample", Type: AlwaysSample}},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name:       "latency",
					Type:       Latency,
					LatencyCfg: LatencyCfg{ThresholdMs: 1},
				},
			},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "ottl",
					Type: OTTLCondition,
					OTTLConditionCfg: OTTLConditionCfg{
						SpanConditions: []string{`attributes["attr_k_1"] == "attr_v_1"`},
					},
				},
			},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name:          "errors",
					Type:          StatusCode,
					StatusCodeCfg: StatusCodeCfg{StatusCodes: []string{"ERROR"}},
				},
			},
		},
		BlockOnOverflow: true,
		DecisionCache: DecisionCacheConfig{
			SampledCacheSize:    8192,
			NonSampledCacheSize: 8192,
		},
		Options: []Option{
			// Tick very frequently to make sure throughput isn't limited by
			// waiting to process the next batch.
			withTickerFrequency(100 * time.Nanosecond),
		},
	}
	p, err := newTracesProcessor(b.Context(), processortest.NewNopSettings(metadata.Type), dropSink{}, cfg)
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	batch := generateRandomizedBatch(128)
	m := &ptrace.ProtoMarshaler{}
	b.SetBytes(int64(m.TracesSize(batch)))

	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Generate new batches to avoid just hitting the cache.
			batch := generateRandomizedBatch(128)
			err := p.ConsumeTraces(b.Context(), batch)
			require.NoError(b, err)
		}
	})
}

// generateRandomizedBatch creates a single batch with randomized ids.
func generateRandomizedBatch(numTraces int) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	scope := rs.ScopeSpans().AppendEmpty()
	for i := range numTraces {
		traceID := uInt64ToTraceID(rand.Uint64())
		for j := 0; j <= i; j++ {
			span := scope.Spans().AppendEmpty()
			span.SetTraceID(traceID)
			span.SetSpanID(uInt64ToSpanID(rand.Uint64()))
		}
	}
	return traces
}

type dropSink struct{}

func (dropSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (dropSink) ConsumeTraces(context.Context, ptrace.Traces) error {
	return nil
}
