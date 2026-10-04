// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func BenchmarkSampling(b *testing.B) {
	traceIDs, batches := generateIDsAndBatches(128)
	cfg := Config{
		SamplingStrategy:        samplingStrategyTraceComplete,
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, _ := newTracesProcessor(b.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	tsp := shard0(sp)
	require.NoError(b, tsp.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, tsp.Shutdown(b.Context()))
	}()
	metrics := newPolicyEvaluationMetrics(len(cfg.PolicyCfgs))
	sampleBatches := make([]*samplingpolicy.TraceData, 0, len(batches))

	for _, batch := range batches {
		sampleBatches = append(sampleBatches, &samplingpolicy.TraceData{
			SpanCount:       0,
			ReceivedBatches: batch,
		})
	}

	ctx := b.Context()
	for b.Loop() {
		for i, id := range traceIDs {
			_, _, _ = tsp.makeDecision(ctx, 0, id, sampleBatches[i], metrics)
		}
	}
}

// BenchmarkProcessorThroughput measures concurrent ingest while the decision
// loop ticks as fast as it can, for one and several shards.
func BenchmarkProcessorThroughput(b *testing.B) {
	for _, numShards := range []uint32{1, 4} {
		b.Run(fmt.Sprintf("shards=%d", numShards), func(b *testing.B) {
			benchmarkProcessorThroughput(b, numShards)
		})
	}
}

func benchmarkProcessorThroughput(b *testing.B, numShards uint32) {
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        1024,
		NumShards:        numShards,
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
	sink := &countingSink{}
	p, err := newTracesProcessor(b.Context(), processortest.NewNopSettings(metadata.Type), sink, cfg)
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	m := &ptrace.ProtoMarshaler{}
	b.SetBytes(int64(m.TracesSize(generateBenchBatch(128))))

	var iter atomic.Uint64
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		// ConsumeTraces copies its input before returning (MutatesData is
		// false), so each goroutine can reuse one batch and only give it new
		// trace IDs to avoid hitting the cache. Clone per iteration instead
		// if the processor ever takes ownership of its input.
		batch := generateBenchBatch(128)
		for pb.Next() {
			setIterTraceIDs(batch, iter.Add(1))
			err := p.ConsumeTraces(b.Context(), batch)
			require.NoError(b, err)
		}
	})
	b.ReportMetric(float64(sink.spans.Load())/float64(b.N), "sampled-spans/op")
	b.ReportMetric(float64(sink.calls.Load())/float64(b.N), "consume-calls/op")
}

// BenchmarkProcessorMemory reports the heap retained by traces that wait for
// a decision, which limits the processor in production. Only the retained-*
// metrics are meaningful: the time and allocations include setup and GC.
func BenchmarkProcessorMemory(b *testing.B) {
	// One batch per tick for DecisionWait ticks fills the processor to its
	// steady state just before the first decision.
	const numBatches, tracesPerBatch = 30, 128
	template := generateBenchBatch(tracesPerBatch)
	var retained int64
	for b.Loop() {
		controller := newTestTSPController()
		cfg := Config{
			SamplingStrategy:            samplingStrategyTraceComplete,
			DecisionWait:                numBatches * time.Second,
			NumTraces:                   50000,
			PolicyCfgs:                  testPolicy,
			DropPendingTracesOnShutdown: true,
			Options:                     []Option{withTestController(controller), withIDBatcher()},
		}
		p, err := newTracesProcessor(b.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
		require.NoError(b, err)
		require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))

		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		for i := range numBatches {
			require.NoError(b, p.ConsumeTraces(b.Context(), cloneWithNewTraceIDs(template, uint64(i))))
			controller.waitForTick()
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		retained += int64(after.HeapAlloc) - int64(before.HeapAlloc)
		require.Len(b, shard0(p).idToTrace, numBatches*tracesPerBatch, "no trace must be decided")

		require.NoError(b, p.Shutdown(b.Context()))
	}
	perIter := float64(retained) / float64(b.N)
	b.ReportMetric(perIter/(numBatches*tracesPerBatch), "retained-B/trace")
	b.ReportMetric(perIter/float64(numBatches*template.SpanCount()), "retained-B/span")
}

// withIDBatcher undoes the sync batcher of withTestController, so that a
// trace is decided DecisionWait ticks after it arrives, as in production.
func withIDBatcher() Option {
	return func(tsp *tailSamplingSpanProcessor) { tsp.decisionBatcher = nil }
}

// benchTraceSizes are the span counts of the traces in generateBenchBatch.
var benchTraceSizes = []int{1, 2, 4, 8, 16, 48}

// generateBenchBatch creates a batch shaped like real traffic: a few services
// with k8s resource attributes, two scopes each, spans with attributes and
// traces of mixed sizes spread over the services. About 10% of the traces
// match each of the latency and ottl policies and 5% have an error.
func generateBenchBatch(numTraces int) ptrace.Traces {
	const numServices = 4
	traces := ptrace.NewTraces()
	var spans [numServices][2]ptrace.SpanSlice
	for s := range numServices {
		rs := traces.ResourceSpans().AppendEmpty()
		attrs := rs.Resource().Attributes()
		attrs.PutStr("service.name", fmt.Sprintf("service-%d", s))
		attrs.PutStr("service.namespace", "shop")
		attrs.PutStr("service.version", "1.2.3")
		attrs.PutStr("service.instance.id", fmt.Sprintf("service-%d-7d9f8b6c5-x2k4p", s))
		attrs.PutStr("deployment.environment.name", "production")
		attrs.PutStr("telemetry.sdk.name", "opentelemetry")
		attrs.PutStr("telemetry.sdk.language", "go")
		attrs.PutStr("telemetry.sdk.version", "1.38.0")
		attrs.PutStr("host.name", fmt.Sprintf("ip-10-0-0-%d.eu-west-1.compute.internal", s))
		attrs.PutStr("cloud.region", "eu-west-1")
		attrs.PutStr("k8s.cluster.name", "prod-eu-west-1")
		attrs.PutStr("k8s.namespace.name", "shop")
		attrs.PutStr("k8s.deployment.name", fmt.Sprintf("service-%d", s))
		attrs.PutStr("k8s.pod.name", fmt.Sprintf("service-%d-7d9f8b6c5-x2k4p", s))
		for i, name := range []string{"net/http", "database/sql"} {
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName(name)
			ss.Scope().SetVersion("0.63.0")
			spans[s][i] = ss.Spans()
		}
	}

	start := time.Unix(1700000000, 0)
	var spanID uint64
	for i := range numTraces {
		traceID := uInt64ToTraceID(uint64(i))
		duration := 500 * time.Microsecond
		if i%10 == 0 {
			duration = 5 * time.Millisecond
		}
		size := benchTraceSizes[i%len(benchTraceSizes)]
		rootID := spanID + 1
		for j := range size {
			spanID++
			span := spans[(i+j)%numServices][j%2].AppendEmpty()
			span.SetTraceID(traceID)
			span.SetSpanID(uInt64ToSpanID(spanID))
			if j > 0 {
				span.SetParentSpanID(uInt64ToSpanID(rootID))
			}
			span.SetName("GET /api/v1/items/{id}")
			span.SetKind(ptrace.SpanKindServer)
			span.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
			span.SetEndTimestamp(pcommon.NewTimestampFromTime(start.Add(duration)))
			attrs := span.Attributes()
			attrs.PutStr("http.request.method", "GET")
			attrs.PutStr("http.route", "/api/v1/items/{id}")
			attrs.PutInt("http.response.status_code", 200)
			attrs.PutStr("server.address", "items.shop.svc.cluster.local")
			attrs.PutInt("server.port", 8080)
			attrs.PutStr("url.path", fmt.Sprintf("/api/v1/items/%d", i))
			attrs.PutStr("user_agent.original", "Mozilla/5.0 (X11; Linux x86_64)")
			if i%10 == 3 && j == 0 {
				attrs.PutStr("attr_k_1", "attr_v_1")
			}
			if i%20 == 7 && j == size-1 {
				span.Status().SetCode(ptrace.StatusCodeError)
				attrs.PutInt("http.response.status_code", 500)
			}
		}
	}
	return traces
}

// cloneWithNewTraceIDs returns a copy of template with trace IDs set by
// setIterTraceIDs.
func cloneWithNewTraceIDs(template ptrace.Traces, iter uint64) ptrace.Traces {
	batch := ptrace.NewTraces()
	template.CopyTo(batch)
	setIterTraceIDs(batch, iter)
	return batch
}

// setIterTraceIDs folds iter into the high bits of the second half of every
// trace ID in batch, so each iter yields trace IDs never seen before. The
// second half must stay unique per trace because the decision caches key on
// it.
func setIterTraceIDs(batch ptrace.Traces, iter uint64) {
	for _, rs := range batch.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				id := span.TraceID()
				right := binary.BigEndian.Uint64(id[8:])
				binary.BigEndian.PutUint64(id[8:], iter<<32|right&math.MaxUint32)
				span.SetTraceID(id)
			}
		}
	}
}

// countingSink counts the spans and ConsumeTraces calls it receives, to check
// that the benchmark samples traces.
type countingSink struct {
	spans atomic.Int64
	calls atomic.Int64
}

func (*countingSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (s *countingSink) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	s.spans.Add(int64(td.SpanCount()))
	s.calls.Add(1)
	return nil
}
