// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
)

// BenchmarkShouldDrop_HighThroughput measures the per-operation cost of the
// shouldDrop hot path under parallel load. b.ReportAllocs() is used to assert
// that the steady-state path (tracker already exists) is allocation-free.
//
// Each parallel goroutine is assigned its own unique pre-warmed tracker key so
// that no two goroutines contend on the same per-tracker sync.Mutex. Without
// this, all goroutines fight over one lock and the Go 1.25 mutex implementation
// (which uses an internal HashTrieMap for its wait queue) produces spurious
// allocations from goroutine-parking bookkeeping — allocations that belong to
// the runtime scheduler, not to our hot path. By giving each goroutine its own
// tracker, we measure the true uncontended steady-state cost.
//
// Run with: go test -bench=BenchmarkShouldDrop_HighThroughput -benchmem ./cardinalityprocessor/.
func BenchmarkShouldDrop_HighThroughput(b *testing.B) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 10_000,
		EpochDurationSeconds:        300,
	}

	// Create the mock OTel settings (which includes the mock MeterProvider and Logger)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(b.Context(), cfg, set, new(consumertest.MetricsSink))
	require.NoError(b, err)

	p := proc.(*cardinalityProcessor)

	// Pre-build a stable pcommon.Value to feed into shouldDrop. Building it
	// once outside the timed loop ensures the benchmark measures the hot path,
	// not the value-construction cost.
	val := pcommon.NewValueStr("bench_val")

	// Pre-warm 64 unique trackers (well above any realistic GOMAXPROCS value).
	// Each goroutine in RunParallel will claim one of these keys so that all
	// trackers exist before the timed loop begins.
	const numKeys = 64
	keys := make([]string, numKeys)
	for i := range keys {
		keys[i] = fmt.Sprintf("bench_key_%d", i)
		p.shouldDrop("bench_metric", keys[i], val)
	}

	b.ReportAllocs()
	b.ResetTimer()

	// Each goroutine picks its own slot from the pre-warmed key array.
	// The assignment is done once per goroutine, outside the pb.Next() loop,
	// so the fmt.Sprintf and atomic.Add are not included in the per-op cost.
	var counter atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		key := keys[int(counter.Add(1))%numKeys]
		for pb.Next() {
			p.shouldDrop("bench_metric", key, val)
		}
	})
}

// BenchmarkConsumeMetrics_Passthrough measures the full ConsumeMetrics pipeline
// cost when all data points are within the cardinality limit (zero drops).
// This represents the "happy path" overhead that every metric pays.
func BenchmarkConsumeMetrics_Passthrough(b *testing.B) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100_000, // High limit — nothing gets dropped
		EpochDurationSeconds:        300,
	}
	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(b.Context(), cfg, set, next)
	require.NoError(b, err)

	// Build a static payload with 10 data points, each with 3 low-cardinality labels.
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("http.server.duration")
	m.SetEmptyGauge()
	for i := range 10 {
		dp := m.Gauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("region", "us-east-1")
		dp.Attributes().PutStr("method", "GET")
		dp.Attributes().PutStr("status", "200")
	}

	b.ReportAllocs()
	for b.Loop() {
		next.Reset()
		require.NoError(b, proc.ConsumeMetrics(b.Context(), md))
	}
}

// BenchmarkConsumeMetrics_HighCardinality measures the full ConsumeMetrics
// pipeline cost when every data point triggers a cardinality-limit drop.
// This exercises the most expensive path: HLL estimation + attribute removal.
func BenchmarkConsumeMetrics_HighCardinality(b *testing.B) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 5, // Tiny limit — almost everything gets dropped
		EpochDurationSeconds:        300,
	}
	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(b.Context(), cfg, set, next)
	require.NoError(b, err)

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		// Build a fresh payload each iteration with unique label values
		// to ensure we always exceed the cardinality limit.
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("bench.high_card")
		m.SetEmptyGauge()
		for j := range 10 {
			dp := m.Gauge().DataPoints().AppendEmpty()
			dp.SetIntValue(1)
			dp.Attributes().PutStr("request_id", fmt.Sprintf("req-%d-%d", i, j))
		}
		next.Reset()
		require.NoError(b, proc.ConsumeMetrics(b.Context(), md))
		i++
	}
}

// BenchmarkConsumeMetrics_MixedMetricTypes measures the pipeline cost when
// processing a batch containing all 5 OTel metric types simultaneously.
// This proves type dispatch in processMetric adds no measurable overhead.
func BenchmarkConsumeMetrics_MixedMetricTypes(b *testing.B) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100_000,
		EpochDurationSeconds:        300,
	}
	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(b.Context(), cfg, set, next)
	require.NoError(b, err)

	// Build a static payload with one data point per metric type.
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Gauge
	mGauge := sm.Metrics().AppendEmpty()
	mGauge.SetName("bench.gauge")
	mGauge.SetEmptyGauge()
	mGauge.Gauge().DataPoints().AppendEmpty().Attributes().PutStr("k", "v")

	// Sum
	mSum := sm.Metrics().AppendEmpty()
	mSum.SetName("bench.sum")
	mSum.SetEmptySum()
	mSum.Sum().DataPoints().AppendEmpty().Attributes().PutStr("k", "v")

	// Histogram
	mHisto := sm.Metrics().AppendEmpty()
	mHisto.SetName("bench.histogram")
	mHisto.SetEmptyHistogram()
	mHisto.Histogram().DataPoints().AppendEmpty().Attributes().PutStr("k", "v")

	// ExponentialHistogram
	mExp := sm.Metrics().AppendEmpty()
	mExp.SetName("bench.exp_histogram")
	mExp.SetEmptyExponentialHistogram()
	mExp.ExponentialHistogram().DataPoints().AppendEmpty().Attributes().PutStr("k", "v")

	// Summary
	mSummary := sm.Metrics().AppendEmpty()
	mSummary.SetName("bench.summary")
	mSummary.SetEmptySummary()
	mSummary.Summary().DataPoints().AppendEmpty().Attributes().PutStr("k", "v")

	b.ReportAllocs()
	for b.Loop() {
		next.Reset()
		require.NoError(b, proc.ConsumeMetrics(b.Context(), md))
	}
}

// BenchmarkConsumeMetrics_LargeBatch measures amortized per-datapoint cost
// when processing a single pmetric.Metrics payload with 1,000 data points.
// This proves the processor handles production-size batches efficiently.
func BenchmarkConsumeMetrics_LargeBatch(b *testing.B) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100_000,
		EpochDurationSeconds:        300,
	}
	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(b.Context(), cfg, set, next)
	require.NoError(b, err)

	// Build a single large batch with 1000 data points across 10 metrics.
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for m := range 10 {
		met := sm.Metrics().AppendEmpty()
		met.SetName(fmt.Sprintf("bench.batch.metric_%d", m))
		met.SetEmptyGauge()
		for d := range 100 {
			dp := met.Gauge().DataPoints().AppendEmpty()
			dp.SetIntValue(int64(d))
			dp.Attributes().PutStr("region", "us-east-1")
			dp.Attributes().PutStr("instance", fmt.Sprintf("i-%d", d%10))
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		next.Reset()
		require.NoError(b, proc.ConsumeMetrics(b.Context(), md))
	}
}
