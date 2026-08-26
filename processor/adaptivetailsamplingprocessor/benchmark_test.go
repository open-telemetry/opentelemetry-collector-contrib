// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

// Hot-path benchmarks for the adaptive tail sampling processor. These exist to give
// performance work a baseline (tracking issue #49311): span accumulation in
// ConsumeTraces, the decide path (rule evaluation, threshold composition,
// trace assembly), the decision-cache fast path for late spans, and the
// retained memory per pending trace. Run with:
//
//	go test -bench=. -benchmem -run=^$ ./...

import (
	"context"
	"encoding/binary"
	"fmt"
	"runtime"
	"testing"
	"time"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/metadata"
)

func benchConfig(rules []RuleConfig) *Config {
	return &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Hour,
		NumTraces:     100_000,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 1000, NonSampledCacheSize: 1000},
		Rules:         rules,
	}
}

func benchCatchAllRules() []RuleConfig {
	return []RuleConfig{
		{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
	}
}

// benchOTTLRules returns four non-matching OTTL rules followed by a catch-all,
// so evaluation walks every condition against every span before matching.
// This is the worst-case rule-evaluation cost for a five-rule config.
func benchOTTLRules() []RuleConfig {
	rules := make([]RuleConfig, 0, 5)
	for i := range 4 {
		rules = append(rules, RuleConfig{
			Name: fmt.Sprintf("no-match-%d", i),
			Conditions: []string{
				`span.attributes["http.method"] == "DELETE"`,
				fmt.Sprintf(`resource.attributes["service.name"] == "absent-%d"`, i),
			},
			Sampler: SamplerConfig{Type: AlwaysSample},
		})
	}
	rules = append(rules, RuleConfig{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}})
	return rules
}

func newBenchProcessor(b *testing.B, cfg *Config) *adaptiveTailSamplingProcessor {
	b.Helper()
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	if err != nil {
		b.Fatal(err)
	}
	if err := p.Start(b.Context(), nil); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := p.Shutdown(context.WithoutCancel(b.Context())); err != nil {
			b.Fatal(err)
		}
	})
	return p
}

func benchTraceID(n uint64) pcommon.TraceID {
	var id [16]byte
	binary.LittleEndian.PutUint64(id[:8], n)
	id[15] = 1
	return id
}

// benchTrace builds a single-resource trace of nSpans child spans (non-empty
// ParentSpanID, so the default root-span condition does not trigger) with a
// representative attribute shape.
func benchTrace(traceID pcommon.TraceID, nSpans int, traceState string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")
	rs.Resource().Attributes().PutStr("deployment.environment.name", "production")
	ss := rs.ScopeSpans().AppendEmpty()
	for i := range nSpans {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID([8]byte{byte(i + 1), 2, 3, 4, 5, 6, 7, 8})
		span.SetParentSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9})
		span.SetName("GET /api/users/{id}")
		span.SetKind(ptrace.SpanKindServer)
		span.Attributes().PutStr("http.method", "GET")
		span.Attributes().PutStr("http.route", "/api/users/{id}")
		span.Attributes().PutInt("http.response.status_code", 200)
		span.Status().SetCode(ptrace.StatusCodeOk)
		if traceState != "" {
			span.TraceState().FromRaw(traceState)
		}
	}
	return td
}

// BenchmarkConsumeTraces_Accumulate measures the buffering path: per-span
// bucketing, the per-span root-span OTTL evaluation, span copy into the
// pending buffer, and the trace_timeout timer arm for new traces. Timers use
// a 1h timeout so none fire during measurement. The eviction-pressure cases
// cap NumTraces low so every new trace also evicts and decides the oldest,
// exercising the saturated-deployment path for both eviction policies.
func BenchmarkConsumeTraces_Accumulate(b *testing.B) {
	for _, bc := range []struct {
		name          string
		spansPerTrace int
		rootCondition string
		numTraces     int
		eviction      EvictionConfig
	}{
		// Pure accumulation: NumTraces is set high enough that no eviction
		// happens during measurement (evictions now decide traces, which
		// would fold decision cost into these numbers).
		{name: "10spans_default_root_condition", spansPerTrace: 10, numTraces: 2_000_000},
		{name: "1span_default_root_condition", spansPerTrace: 1, numTraces: 2_000_000},
		{
			name: "10spans_custom_root_condition", spansPerTrace: 10,
			rootCondition: `span.attributes["otelcol.adaptive_tail_sampling.root_span"] == true`,
			numTraces:     2_000_000,
		},
		// Sustained buffer pressure: every new trace evicts (and decides) the
		// oldest one. The two policies bound the cost of overload differently;
		// this makes the delta visible in the baseline.
		{
			name: "10spans_eviction_pressure_evaluate", spansPerTrace: 10,
			numTraces: 64,
		},
		{
			name: "10spans_eviction_pressure_probabilistic", spansPerTrace: 10,
			numTraces: 64,
			eviction:  EvictionConfig{Policy: EvictionProbabilistic, SamplingPercentage: 10},
		},
	} {
		b.Run(bc.name, func(b *testing.B) {
			cfg := benchConfig(benchCatchAllRules())
			cfg.RootSpanCondition = bc.rootCondition
			if bc.numTraces > 0 {
				cfg.NumTraces = bc.numTraces
			}
			cfg.Eviction = bc.eviction
			p := newBenchProcessor(b, cfg)
			ctx := b.Context()

			// Pre-build a rotating window of batches so input construction is
			// outside the timed loop and traceIDs stay unique per iteration.
			const window = 512
			batches := make([]ptrace.Traces, window)
			for i := range batches {
				batches[i] = benchTrace(benchTraceID(uint64(i)), bc.spansPerTrace, "")
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				td := batches[i%window]
				// Rewrite the traceID so every iteration is a new trace.
				id := benchTraceID(uint64(i) + window)
				for _, rs := range td.ResourceSpans().All() {
					for _, ss := range rs.ScopeSpans().All() {
						for _, span := range ss.Spans().All() {
							span.SetTraceID(id)
						}
					}
				}
				if err := p.ConsumeTraces(ctx, td); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(b.N*bc.spansPerTrace)/b.Elapsed().Seconds(), "spans/sec")
		})
	}
}

// BenchmarkConsumeTraces_AppendToPending measures the known-traceID path:
// spans arriving for a trace that already has a pendingTrace entry. Unlike
// the accumulate benchmark this skips pendingTrace creation, timer arming,
// and eviction, so it isolates the per-batch bucketing and span-append cost.
// The pending buffers are reset (untimed) periodically so retained memory
// stays bounded without touching the timer machinery.
func BenchmarkConsumeTraces_AppendToPending(b *testing.B) {
	cfg := benchConfig(benchCatchAllRules())
	p := newBenchProcessor(b, cfg)
	ctx := b.Context()

	const spansPerBatch = 10
	const window = 512
	// Seed a window of pending traces so every iteration hits an existing
	// entry. Child spans only, so nothing triggers a decision.
	batches := make([]ptrace.Traces, window)
	for i := range batches {
		batches[i] = benchTrace(benchTraceID(uint64(i)), spansPerBatch, "")
		if err := p.ConsumeTraces(ctx, batches[i]); err != nil {
			b.Fatal(err)
		}
	}

	// resetPending clears the accumulated span buffers without touching the
	// traces map or timers, keeping the benchmark's memory bounded.
	resetPending := func() {
		p.mu.Lock()
		for _, pt := range p.traces {
			pt.spans = nil
			pt.spanCount = 0
		}
		p.mu.Unlock()
	}
	const resetEvery = 50_000

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i > 0 && i%resetEvery == 0 {
			b.StopTimer()
			resetPending()
			b.StartTimer()
		}
		if err := p.ConsumeTraces(ctx, batches[i%window]); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N*spansPerBatch)/b.Elapsed().Seconds(), "spans/sec")
}

// BenchmarkDecide measures the decision path: rule evaluation (OTTL), incoming
// tracestate scan, threshold composition, decision-cache record, trace
// assembly (full span copy + tracestate/attribute stamping), and forwarding to
// a nop consumer. The pending trace is re-inserted each iteration; the map
// insert is <1% of the measured cost.
func BenchmarkDecide(b *testing.B) {
	for _, bc := range []struct {
		name          string
		spansPerTrace int
		rules         []RuleConfig
		traceState    string
	}{
		{name: "1span_catchall", spansPerTrace: 1, rules: benchCatchAllRules()},
		{name: "10spans_catchall", spansPerTrace: 10, rules: benchCatchAllRules()},
		{name: "100spans_catchall", spansPerTrace: 100, rules: benchCatchAllRules()},
		{name: "1000spans_catchall", spansPerTrace: 1000, rules: benchCatchAllRules()},
		{name: "10000spans_catchall", spansPerTrace: 10_000, rules: benchCatchAllRules()},
		{name: "10spans_5ottl_rules", spansPerTrace: 10, rules: benchOTTLRules()},
		{name: "100spans_5ottl_rules", spansPerTrace: 100, rules: benchOTTLRules()},
		{name: "10000spans_5ottl_rules", spansPerTrace: 10_000, rules: benchOTTLRules()},
		{name: "10spans_catchall_incoming_tracestate", spansPerTrace: 10, rules: benchCatchAllRules(), traceState: "ot=th:8;rv:ab8befca837da2"},
	} {
		b.Run(bc.name, func(b *testing.B) {
			p := newBenchProcessor(b, benchConfig(bc.rules))

			id := benchTraceID(1)
			template := benchTrace(id, bc.spansPerTrace, bc.traceState)

			// The pending trace is rebuilt (untimed) every iteration: the
			// decide path consumes pt.spans, and reusing one pendingTrace
			// would also understate steady-state behavior.
			newPT := func() *pendingTrace {
				td := ptrace.NewTraces()
				template.CopyTo(td)
				spans := make([]ptrace.ResourceSpans, 0, td.ResourceSpans().Len())
				for _, rs := range td.ResourceSpans().All() {
					spans = append(spans, rs)
				}
				return &pendingTrace{traceID: id, spans: spans, spanCount: bc.spansPerTrace}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				pt := newPT()
				b.StartTimer()
				p.mu.Lock()
				p.traces[id] = pt
				p.mu.Unlock()
				p.decide(id)
			}
			b.ReportMetric(float64(b.N*bc.spansPerTrace)/b.Elapsed().Seconds(), "spans/sec")
		})
	}
}

// BenchmarkLateSpan_CacheHit measures the decision-cache fast path: spans
// arriving for a trace that already has a recorded decision. Sampled hits are
// stamped and forwarded; non-sampled hits are discarded.
func BenchmarkLateSpan_CacheHit(b *testing.B) {
	for _, bc := range []struct {
		name    string
		sampled bool
	}{
		{name: "sampled_stamp_and_forward", sampled: true},
		{name: "non_sampled_discard", sampled: false},
	} {
		b.Run(bc.name, func(b *testing.B) {
			p := newBenchProcessor(b, benchConfig(benchCatchAllRules()))
			ctx := b.Context()

			id := benchTraceID(1)
			if bc.sampled {
				p.cache.recordSampled(id, cachedDecision{ruleName: "default", threshold: sampling.AlwaysSampleThreshold})
			} else {
				p.cache.recordNotSampled(id)
			}
			const spansPerBatch = 10
			td := benchTrace(id, spansPerBatch, "")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := p.ConsumeTraces(ctx, td); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(b.N*spansPerBatch)/b.Elapsed().Seconds(), "spans/sec")
		})
	}
}

// BenchmarkMemory_PendingTraces reports the retained heap per buffered pending
// trace (10 spans each), the number capacity planning cares about when sizing
// num_traces. The ns/op number is not meaningful here; read retained_B/trace.
func BenchmarkMemory_PendingTraces(b *testing.B) {
	cfg := benchConfig(benchCatchAllRules())
	cfg.NumTraces = b.N + 1 // never evict during measurement
	p := newBenchProcessor(b, cfg)
	ctx := b.Context()

	const spansPerTrace = 10
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		td := benchTrace(benchTraceID(uint64(i)), spansPerTrace, "")
		if err := p.ConsumeTraces(ctx, td); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if after.HeapAlloc > before.HeapAlloc {
		b.ReportMetric(float64(after.HeapAlloc-before.HeapAlloc)/float64(b.N), "retained_B/trace")
	}
}

// BenchmarkDecide_RecordFingerprint measures the added decide-path cost of
// recording the fingerprint attribute: one PutStr per span when enabled, plus
// one SHA-256 per decision in hash mode.
func BenchmarkDecide_RecordFingerprint(b *testing.B) {
	rules := []RuleConfig{{Name: "default", Sampler: SamplerConfig{
		Type:                  AdaptivePercentage,
		GoalPercentage:        100,
		FingerprintAttributes: []string{`resource.attributes["service.name"]`},
	}}}
	for _, mode := range []RecordFingerprint{RecordFingerprintNone, RecordFingerprintValue, RecordFingerprintHash} {
		for _, spansPerTrace := range []int{10, 1000} {
			b.Run(fmt.Sprintf("%s/%dspans", mode, spansPerTrace), func(b *testing.B) {
				cfg := benchConfig(rules)
				cfg.RecordFingerprint = mode
				p := newBenchProcessor(b, cfg)

				id := benchTraceID(1)
				template := benchTrace(id, spansPerTrace, "")
				template.ResourceSpans().At(0).Resource().Attributes().PutStr("service.name", "svc")

				newPT := func() *pendingTrace {
					td := ptrace.NewTraces()
					template.CopyTo(td)
					spans := make([]ptrace.ResourceSpans, 0, td.ResourceSpans().Len())
					for _, rs := range td.ResourceSpans().All() {
						spans = append(spans, rs)
					}
					return &pendingTrace{traceID: id, spans: spans, spanCount: spansPerTrace}
				}

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					pt := newPT()
					b.StartTimer()
					p.mu.Lock()
					p.traces[id] = pt
					p.mu.Unlock()
					p.decide(id)
				}
			})
		}
	}
}
