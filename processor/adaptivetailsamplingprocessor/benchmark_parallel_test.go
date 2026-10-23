// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

// Concurrent benchmarks for ConsumeTraces. The collector calls ConsumeTraces
// from multiple receiver/pipeline goroutines, and the processor serializes
// most of its work behind one mutex, so single-threaded ns/span (see
// benchmark_test.go) does not predict multi-core throughput. These benchmarks
// measure the scaling curve directly. Run with:
//
//	go test -bench=Parallel -benchmem -run='^$' -cpu=1,4,8,16 .
//
// To attribute lock wait time:
//
//	go test -bench=Parallel -run='^$' -cpu=8 -mutexprofile=mutex.out .
//	go tool pprof mutex.out
//
// Interpreting results: ns/op flat (or falling) as -cpu grows means the path
// scales; ns/op rising toward cpu-times-single-threaded cost means the path
// is fully serialized on the mutex.

import (
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// setBatchTraceID rewrites every span's traceID in a prebuilt batch.
func setBatchTraceID(td ptrace.Traces, id pcommon.TraceID) {
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				span.SetTraceID(id)
			}
		}
	}
}

// BenchmarkConsumeTraces_Parallel measures concurrent ingest on the three
// steady-state paths: brand-new traces, appends to pending traces, and
// decision-cache hits (both outcomes). Every goroutine drives its own
// prebuilt batch; traceIDs come from a shared atomic sequence so goroutines
// never collide on the same new trace.
func BenchmarkConsumeTraces_Parallel(b *testing.B) {
	const spansPerBatch = 10
	const window = 8192

	b.Run("new_traces", func(b *testing.B) {
		cfg := benchConfig(benchCatchAllRules())
		cfg.NumTraces = 2_000_000 // no eviction during measurement
		p := newBenchProcessor(b, cfg)
		ctx := b.Context()

		var seq atomic.Uint64
		seq.Store(window) // seeded ids in other cases start below window
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			td := benchTrace(benchTraceID(0), spansPerBatch, "")
			for pb.Next() {
				setBatchTraceID(td, benchTraceID(seq.Add(1)))
				if err := p.ConsumeTraces(ctx, td); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.ReportMetric(float64(b.N*spansPerBatch)/b.Elapsed().Seconds(), "spans/sec")
	})

	b.Run("append_to_pending", func(b *testing.B) {
		cfg := benchConfig(benchCatchAllRules())
		p := newBenchProcessor(b, cfg)
		ctx := b.Context()

		// Seed a window of pending traces; all goroutines append to them.
		seedBatch := benchTrace(benchTraceID(0), spansPerBatch, "")
		for i := range uint64(window) {
			setBatchTraceID(seedBatch, benchTraceID(i))
			if err := p.ConsumeTraces(ctx, seedBatch); err != nil {
				b.Fatal(err)
			}
		}

		// Buffers are cleared in-line (amortized, timed) so retained memory
		// stays bounded; StopTimer is not usable inside RunParallel.
		const resetEvery = 100_000
		resetPending := func() {
			p.mu.Lock()
			for _, pt := range p.traces {
				pt.spans = nil
				pt.spanCount = 0
			}
			p.mu.Unlock()
		}

		var seq atomic.Uint64
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			td := benchTrace(benchTraceID(0), spansPerBatch, "")
			for pb.Next() {
				n := seq.Add(1)
				if n%resetEvery == 0 {
					resetPending()
				}
				setBatchTraceID(td, benchTraceID(n%window))
				if err := p.ConsumeTraces(ctx, td); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.ReportMetric(float64(b.N*spansPerBatch)/b.Elapsed().Seconds(), "spans/sec")
	})

	for _, hit := range []struct {
		name    string
		sampled bool
	}{
		{name: "cache_hit_sampled", sampled: true},
		{name: "cache_hit_dropped", sampled: false},
	} {
		b.Run(hit.name, func(b *testing.B) {
			cfg := benchConfig(benchCatchAllRules())
			cfg.DecisionCache = DecisionCacheConfig{SampledCacheSize: window, NonSampledCacheSize: window}
			p := newBenchProcessor(b, cfg)
			ctx := b.Context()

			for i := range uint64(window) {
				if hit.sampled {
					p.cache.recordSampled(benchTraceID(i), cachedDecision{ruleName: "default", threshold: sampling.AlwaysSampleThreshold})
				} else {
					p.cache.recordNotSampled(benchTraceID(i))
				}
			}

			var seq atomic.Uint64
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				td := benchTrace(benchTraceID(0), spansPerBatch, "")
				for pb.Next() {
					setBatchTraceID(td, benchTraceID(seq.Add(1)%window))
					if err := p.ConsumeTraces(ctx, td); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.ReportMetric(float64(b.N*spansPerBatch)/b.Elapsed().Seconds(), "spans/sec")
		})
	}
}

// BenchmarkConsumeTraces_ParallelMixed measures ingest while decisions race
// with it: every batch carries a root span, so each trace triggers a
// decision_delay timer that fires 1ms later and takes the same mutex from its
// own goroutine (trigger, decide, forward). This is the closest shape to a
// loaded production instance: N ingest goroutines plus a stream of timer
// goroutines contending for the lock.
func BenchmarkConsumeTraces_ParallelMixed(b *testing.B) {
	const spansPerBatch = 10

	cfg := benchConfig(benchCatchAllRules())
	cfg.NumTraces = 2_000_000
	cfg.TraceTimeout = time.Hour
	cfg.DecisionDelay = time.Millisecond
	p := newBenchProcessor(b, cfg)
	ctx := b.Context()

	var seq atomic.Uint64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		td := benchTrace(benchTraceID(0), spansPerBatch, "")
		// Make the first span a root so every trace triggers a decision.
		td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetParentSpanID(pcommon.SpanID{})
		for pb.Next() {
			setBatchTraceID(td, benchTraceID(seq.Add(1)))
			if err := p.ConsumeTraces(ctx, td); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.ReportMetric(float64(b.N*spansPerBatch)/b.Elapsed().Seconds(), "spans/sec")
}
