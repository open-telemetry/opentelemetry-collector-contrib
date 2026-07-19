// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestCardinalityProcessor_ConsumeMetrics(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100,
		EpochDurationSeconds:        30,
		NeverDropLabels:             []string{"region", "status_code"},
	}

	next := new(consumertest.MetricsSink)

	// Create the mock OTel settings (which includes the mock MeterProvider and Logger)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))

	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("http.server.duration")
	m.SetEmptyGauge()
	dp := m.Gauge().DataPoints().AppendEmpty()

	attrs := dp.Attributes()
	attrs.PutStr("region", "us-east-1")     // Protected
	attrs.PutStr("user_id", "user_123")     // To be intercepted
	attrs.PutStr("temp_label", "delete_me") // To be intercepted

	err = proc.ConsumeMetrics(t.Context(), md)
	assert.NoError(t, err)

	require.GreaterOrEqual(t, len(next.AllMetrics()), 1, "sink should have received at least one metrics batch")

	resultMetrics := next.AllMetrics()[0]

	require.Equal(t, 1, resultMetrics.ResourceMetrics().Len(), "expected 1 ResourceMetrics")
	resultRM := resultMetrics.ResourceMetrics().At(0)

	require.Equal(t, 1, resultRM.ScopeMetrics().Len(), "expected 1 ScopeMetrics")
	resultSM := resultRM.ScopeMetrics().At(0)

	require.Equal(t, 1, resultSM.Metrics().Len(), "expected 1 Metric")
	resultMetric := resultSM.Metrics().At(0)

	require.Equal(t, 1, resultMetric.Gauge().DataPoints().Len(), "expected 1 DataPoint")
	resultAttrs := resultMetric.Gauge().DataPoints().At(0).Attributes()

	_, regionExists := resultAttrs.Get("region")
	assert.True(t, regionExists, "protected label 'region' must survive")

	assert.Equal(t, 3, resultAttrs.Len(), "Should have all 3 attributes in Phase 1 setup")
}

func TestCardinalityProcessor_HighCardinalityLimit(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 50,
		EpochDurationSeconds:        30,
		NeverDropLabels:             []string{"region"},
		EnforcementMode:             EnforcementStripAndReaggregate,
	}

	next := new(consumertest.MetricsSink)
	// Create the mock OTel settings (which includes the mock MeterProvider and Logger)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	p, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	err = p.Start(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = p.Shutdown(t.Context()) }()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("http.requests")
	gauge := m.SetEmptyGauge()

	for i := range 100 {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetIntValue(1)
		dp.Attributes().PutStr("user_id", fmt.Sprintf("user-%d", i))
		dp.Attributes().PutStr("region", "us-east")
	}

	err = p.ConsumeMetrics(t.Context(), md)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(next.AllMetrics()), 1, "sink should have received at least one metrics batch")

	outMd := next.AllMetrics()[0]

	require.Equal(t, 1, outMd.ResourceMetrics().Len(), "expected 1 ResourceMetrics")
	outRM := outMd.ResourceMetrics().At(0)

	require.Equal(t, 1, outRM.ScopeMetrics().Len(), "expected 1 ScopeMetrics")
	outSM := outRM.ScopeMetrics().At(0)

	require.Equal(t, 1, outSM.Metrics().Len(), "expected 1 Metric")
	outDps := outSM.Metrics().At(0).Gauge().DataPoints()

	// With reaggregation (default enforcement mode = strip_and_reaggregate),
	// stripped data points with identical remaining attributes ({region="us-east"})
	// are merged into a single data point via last-value-wins. So the output
	// contains ~50 kept data points (with user_id) + 1 merged data point
	// (the reaggregated stripped ones).
	keptCount := 0
	mergedCount := 0

	for i := 0; i < outDps.Len(); i++ {
		dp := outDps.At(i)

		_, hasRegion := dp.Attributes().Get("region")
		require.True(t, hasRegion, "Protected label 'region' should never be dropped")

		if _, hasUser := dp.Attributes().Get("user_id"); hasUser {
			keptCount++
		} else {
			mergedCount++
		}
	}

	// The ~50 data points within the limit keep their user_id.
	assert.InDelta(t, 50, keptCount, 10,
		"kept count must be within 10 of the 50-label limit (got %d)", keptCount)

	// All stripped data points are merged into exactly 1 via reaggregation
	// (they all share {region="us-east"} after user_id is removed).
	assert.Equal(t, 1, mergedCount,
		"all stripped data points should be reaggregated into exactly 1")

	// Total = kept + 1 merged
	assert.Equal(t, keptCount+1, outDps.Len(),
		"total data points must equal kept + 1 reaggregated")

	t.Logf("SUCCESS: Kept %d user_ids, Reaggregated %d stripped into 1 merged point", keptCount, mergedCount)
}

// TestShardDistribution verifies that the maphash routing function spreads
// 10,000 unique metric names across all 256 shards with no shard left empty.
// An empty shard would indicate a degenerate hash distribution that would
// concentrate lock contention onto the remaining populated shards.
func TestShardDistribution(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1000,
		EpochDurationSeconds:        30,
	}

	// Create the mock OTel settings (which includes the mock MeterProvider and Logger)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))

	proc, err := newCardinalityProcessor(t.Context(), cfg, set, new(consumertest.MetricsSink))
	require.NoError(t, err)

	// Type-assert to the concrete type so we can access getShard directly.
	p := proc.(*cardinalityProcessor)

	const totalMetrics = 10_000
	hitCount := make([]int, numShards)

	for i := range totalMetrics {
		shard := p.getShard(fmt.Sprintf("metric_%d", i))

		// Identify which shard index was returned by comparing pointers.
		for idx, s := range p.shards {
			if s == shard {
				hitCount[idx]++
				break
			}
		}
	}

	emptyShards := 0
	for idx, count := range hitCount {
		if count == 0 {
			t.Logf("Shard %d received 0 metrics", idx)
			emptyShards++
		}
	}

	assert.Equal(t, 0, emptyShards,
		"Every shard must receive at least one metric across 10,000 unique names; "+
			"%d shard(s) were empty", emptyShards)

	// Log distribution stats for manual inspection.
	minHits, maxHits := hitCount[0], hitCount[0]
	for _, c := range hitCount {
		if c < minHits {
			minHits = c
		}
		if c > maxHits {
			maxHits = c
		}
	}
	t.Logf("Shard distribution across %d metrics: min=%d, max=%d, expected≈%d",
		totalMetrics, minHits, maxHits, totalMetrics/numShards)
}

// TestConcurrency_Sharded verifies that 100 goroutines can call shouldDrop
// concurrently without deadlocking, panicking, or triggering the race detector.
// Each goroutine uses a distinct value space so that high-cardinality pressure
// is applied from multiple directions simultaneously.
func TestConcurrency_Sharded(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 500,
		EpochDurationSeconds:        30,
	}
	// Create the mock OTel settings (which includes the mock MeterProvider and Logger)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))

	proc, err := newCardinalityProcessor(t.Context(), cfg, set, new(consumertest.MetricsSink))
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	const (
		goroutines = 50
		iterations = 200
	)

	var wg sync.WaitGroup
	for id := range goroutines {
		wg.Go(func() {
			for i := range iterations {
				// Each goroutine produces a unique value space to exercise
				// concurrent tracker creation and the double-check lock path.
				val := fmt.Sprintf("value_%d_%d", id, i)
				p.shouldDrop("concurrent_metric", "key", pcommon.NewValueStr(val))
			}
		})
	}

	wg.Wait()
	// If we reach here without a deadlock, race condition, or panic, the test passes.
}

func TestCardinalityProcessor_TagOnlyMode(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 50,
		EpochDurationSeconds:        300,
		EnforcementMode:             EnforcementTagOnly,
	}

	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	// Fire 100 metrics with unique user_ids
	for i := range 100 {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("api.request")
		m.SetEmptyGauge()
		dp := m.Gauge().DataPoints().AppendEmpty()

		dp.Attributes().PutStr("user_id", fmt.Sprintf("user_%d", i))

		err = proc.ConsumeMetrics(t.Context(), md)
		require.NoError(t, err)
	}

	outMetrics := next.AllMetrics()
	require.Len(t, outMetrics, 100)

	taggedCount := 0
	droppedCount := 0

	// Inspect the resulting metrics
	for _, md := range outMetrics {
		attrs := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes()

		// 1. Verify the attribute was NEVER dropped
		if _, hasUserID := attrs.Get("user_id"); !hasUserID {
			droppedCount++
		}

		// 2. Count how many got the special routing tag
		if _, hasTag := attrs.Get("otel.metric.overflow"); hasTag {
			taggedCount++
		}
	}

	// Assertions
	assert.Equal(t, 0, droppedCount, "0 attributes should be dropped in TagOnly mode")
	assert.Positive(t, taggedCount, "Some attributes should have been tagged for routing")
	assert.Less(t, taggedCount, 100, "Not all attributes should be tagged (first 50 are allowed)")

	t.Logf("SUCCESS: Kept all 100 user_ids. Tagged %d user_ids for cold storage.", taggedCount)
}

// TestEnforcementResolvesIdentityCollisions verifies that inline spatial
// reaggregation preserves the Single-Writer invariant when enforcement strips
// attributes. When enforcement triggers on a Delta Sum, stripped data points
// that share the same identity are merged: their values are summed into a
// single data point.
func TestEnforcementResolvesIdentityCollisions(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1, // Tiny limit to trigger enforcement immediately
		EpochDurationSeconds:        300,
		EnforcementMode:             EnforcementStripAndReaggregate,
	}

	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	// Send 3 data points for the same metric with different user_ids.
	// Using Delta Sum so that reaggregation can sum the values.
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("api.request")
	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	for i := 1; i <= 3; i++ {
		dp := sum.DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i * 10)) // 10, 20, 30
		dp.Attributes().PutStr("region", "us-east")
		dp.Attributes().PutStr("user_id", fmt.Sprintf("user_%d", i))
	}

	err = proc.ConsumeMetrics(t.Context(), md)
	require.NoError(t, err)

	outMetrics := next.AllMetrics()
	require.Len(t, outMetrics, 1)

	outDpList := outMetrics[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()

	// Verify: no duplicate identities exist in the output.
	// The first data point (user_1) should keep user_id (within limit).
	// Data points 2 and 3 get stripped and then reaggregated into one.
	// So we expect 2 data points total: one with user_id, one without (merged).
	require.Equal(t, 2, outDpList.Len(),
		"expected 2 data points: 1 kept + 1 reaggregated")

	// Verify no duplicate identities.
	seen := make(map[uint64]bool)
	for i := 0; i < outDpList.Len(); i++ {
		h := hashAttributes(outDpList.At(i).Attributes())
		assert.False(t, seen[h],
			"duplicate identity found at index %d — Single-Writer violation!", i)
		seen[h] = true
	}

	// Find the reaggregated data point (the one without user_id).
	for i := 0; i < outDpList.Len(); i++ {
		dp := outDpList.At(i)
		if _, hasUser := dp.Attributes().Get("user_id"); !hasUser {
			// This is the merged point. Values 20+30=50 (user_2 and user_3).
			assert.Equal(t, int64(50), dp.IntValue(),
				"merged data point should sum the stripped values (20+30=50)")
		}
	}

	t.Log("SUCCESS: Reaggregation resolved identity collision — no Single-Writer violation.")
}

// TestInternalTelemetry wires the processor to a real OTel SDK ManualReader so
// that internal instruments (counter, gauge) can be collected and asserted
// deterministically without relying on a background export interval.
//
// Phase 1 creates exactly 5 trackers by sending 5 data points, each carrying a
// distinct label key against the same metric name. The delta limit is not
// exceeded, so no labels are stripped.
//
// Phase 2 pushes key_0 past the cardinality limit by contributing 5 more
// unique values (v1-v5). Together with the initial v0 from Phase 1, key_0 now
// has 6 unique values. The 6th insertion produces a delta of 6 > 5, which
// triggers exactly one drop: one attribute strip and one savings increment.
func TestInternalTelemetry(t *testing.T) {
	const (
		costPerMetric    = 0.10
		cardinalityLimit = 5
	)

	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: cardinalityLimit,
		EpochDurationSeconds:        300,
		EstimatedCostPerMetricMonth: costPerMetric,
		EnforcementMode:             EnforcementStripAndReaggregate,
	}

	// ManualReader captures all SDK metric data on demand via Collect().
	// NewMeterProvider registers it as the single export pipeline.
	reader := sdkmetric.NewManualReader()
	sdkProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() {
		if err := sdkProvider.Shutdown(t.Context()); err != nil {
			t.Errorf("sdk provider shutdown: %v", err)
		}
	}()

	// Replace the nop MeterProvider with the real SDK provider so the
	// processor registers its instruments against our ManualReader.
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	set.MeterProvider = sdkProvider

	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	// --- Phase 1: create exactly 5 trackers, 0 drops --------------------------
	//
	// Each data point carries one label key (key_0 through key_4) with the
	// shared value "v0". The trackerKey is (metricName, attrKey), so 5 unique
	// attrKeys create 5 independent trackers. Each tracker records 1 unique
	// value, well below the limit of 5, so shouldDrop returns false for all.
	batch1 := pmetric.NewMetrics()
	rm1 := batch1.ResourceMetrics().AppendEmpty()
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	m1 := sm1.Metrics().AppendEmpty()
	m1.SetName("telemetry_test_metric")
	m1.SetEmptyGauge()

	for i := range 5 {
		dp := m1.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr(fmt.Sprintf("key_%d", i), "v0")
	}

	err = proc.ConsumeMetrics(t.Context(), batch1)
	require.NoError(t, err)

	// --- Phase 2: exceed the limit on key_0, triggering exactly 1 drop --------
	//
	// Values v1-v5 are new for key_0. After inserting v5 the sketch estimate
	// reaches 6 (v0 from Phase 1 + v1-v5 here). Delta = 6 > 5 → drop.
	// The attribute is removed from the data point, labelsStripped increments
	// by 1, and estimatedSavings increments by costPerMetric.
	batch2 := pmetric.NewMetrics()
	rm2 := batch2.ResourceMetrics().AppendEmpty()
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	m2 := sm2.Metrics().AppendEmpty()
	m2.SetName("telemetry_test_metric")
	m2.SetEmptyGauge()

	for i := 1; i <= 5; i++ {
		dp := m2.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("key_0", fmt.Sprintf("v%d", i))
	}

	err = proc.ConsumeMetrics(t.Context(), batch2)
	require.NoError(t, err)

	// --- Collect: snapshot all SDK metrics at this exact instant ---------------
	var collected metricdata.ResourceMetrics
	err = reader.Collect(t.Context(), &collected)
	require.NoError(t, err)

	// --- Assert: otelcol_processor_cardinality_trackers.active = 5 ---------------------------------
	//
	// The observable gauge callback sums len(shard.trackers) across all 256
	// shards when Collect() fires. Exactly 5 trackers were created in Phase 1
	// and no new trackers were added in Phase 2 (key_0 already existed).
	activeMetric := findMetricByName(t, collected, "otelcol_processor_cardinality_trackers.active")
	gaugeData, ok := activeMetric.Data.(metricdata.Gauge[int64])
	require.True(t, ok, "otelcol_processor_cardinality_trackers.active must be a Gauge[int64]")

	var totalActive int64
	for _, dp := range gaugeData.DataPoints {
		totalActive += dp.Value
	}
	assert.Equal(t, int64(5), totalActive,
		"otelcol_processor_cardinality_trackers.active must report exactly 5 (one per unique label key)")

	// --- Assert: otelcol_processor_cardinality_labels.stripped = 1 ---------------------------
	//
	// Only the insertion of v5 into key_0's tracker produced a delta > limit.
	// Every earlier insertion was within bounds.
	strippedMetric := findMetricByName(t, collected, "otelcol_processor_cardinality_labels.stripped")
	strippedSum, ok := strippedMetric.Data.(metricdata.Sum[int64])
	require.True(t, ok, "otelcol_processor_cardinality_labels.stripped must be a Sum[int64]")

	var totalStripped int64
	for _, dp := range strippedSum.DataPoints {
		totalStripped += dp.Value
	}
	assert.Equal(t, int64(1), totalStripped,
		"otelcol_processor_cardinality_labels.stripped must be 1 (only the 6th unique value for key_0 was dropped)")

	// --- Assert: otelcol_processor_cardinality_savings.estimated = costPerMetric ---------------
	//
	// One drop × EstimatedCostPerMetricMonth = $0.10.
	savingsMetric := findMetricByName(t, collected, "otelcol_processor_cardinality_savings.estimated")
	savingsSum, ok := savingsMetric.Data.(metricdata.Sum[float64])
	require.True(t, ok, "otelcol_processor_cardinality_savings.estimated must be a Sum[float64]")

	var totalSavings float64
	for _, dp := range savingsSum.DataPoints {
		totalSavings += dp.Value
	}
	assert.InDelta(t, costPerMetric, totalSavings, 1e-9,
		"otelcol_processor_cardinality_savings.estimated must equal %v (1 drop × cost-per-metric)", costPerMetric)
}

// findMetricByName searches a collected ResourceMetrics snapshot for a metric
// with the given name. It fails the test immediately if the metric is absent,
// printing all available metric names to aid diagnosis.
func findMetricByName(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}

	t.Fatalf("metric %q not found in collected telemetry; available: %v",
		name, allMetricNames(rm))

	return metricdata.Metrics{}
}

// allMetricNames returns the names of every metric present in a collected
// ResourceMetrics snapshot. Used exclusively in test failure messages.
func allMetricNames(rm metricdata.ResourceMetrics) []string {
	var names []string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names = append(names, m.Name)
		}
	}

	return names
}

func TestCardinalityProcessor_AlternateMetricTypes(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 50,
		EpochDurationSeconds:        300,
	}

	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// 1. Histogram
	mHisto := sm.Metrics().AppendEmpty()
	mHisto.SetName("test.histogram")
	mHisto.SetEmptyHistogram()
	hdp := mHisto.Histogram().DataPoints().AppendEmpty()
	hdp.Attributes().PutStr("user_id", "u1")

	// 2. ExponentialHistogram
	mExp := sm.Metrics().AppendEmpty()
	mExp.SetName("test.exp_histogram")
	mExp.SetEmptyExponentialHistogram()
	edp := mExp.ExponentialHistogram().DataPoints().AppendEmpty()
	edp.Attributes().PutStr("user_id", "u2")

	// 3. Summary
	mSum := sm.Metrics().AppendEmpty()
	mSum.SetName("test.summary")
	mSum.SetEmptySummary()
	sdp := mSum.Summary().DataPoints().AppendEmpty()
	sdp.Attributes().PutStr("user_id", "u3")

	err = proc.ConsumeMetrics(t.Context(), md)
	require.NoError(t, err)

	require.Len(t, next.AllMetrics(), 1)
	outSm := next.AllMetrics()[0].ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 3, outSm.Metrics().Len())
}

func TestCardinalityProcessor_EpochRotation(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 50,
		EpochDurationSeconds:        300,
	}

	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	// Insert data to initialize a sketch
	p.shouldDrop("metric", "key", pcommon.NewValueStr("val"))

	shard := p.getShard("metric")
	shard.mu.RLock()
	tracker := shard.trackers[trackerKey{metricName: "metric", attrKey: "key"}]
	shard.mu.RUnlock()

	require.NotNil(t, tracker)
	require.NotNil(t, tracker.current)

	// Manually trigger a rotation to hit the coverage branch
	p.rotate()

	// Re-lock to check the flip occurred safely
	shard.mu.RLock()
	require.NotNil(t, tracker.previous)
	shard.mu.RUnlock()
}

// TestTopOffenders verifies that the otelcol_processor_cardinality_top.offenders gauge correctly
// reports the highest-delta (metric, label) pairs after an epoch rotation.
//
// Setup:
//   - TopOffendersCount = 3
//   - 5 different label keys are populated with varying unique-value counts:
//     key_0: 20 values, key_1: 15 values, key_2: 10 values,
//     key_3: 5 values, key_4: 1 value
//
// After rotate(), the gauge should emit exactly 3 data points (top 3 by delta),
// ordered by descending delta, with metric_name and label_key attributes.
func TestTopOffenders(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100, // High limit so nothing is dropped
		EpochDurationSeconds:        300,
		TopOffendersCount:           3,
	}

	reader := sdkmetric.NewManualReader()
	sdkProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() {
		if err := sdkProvider.Shutdown(t.Context()); err != nil {
			t.Errorf("sdk provider shutdown: %v", err)
		}
	}()

	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	set.MeterProvider = sdkProvider

	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	// Populate 5 label keys with varying unique-value counts.
	// key_0: 20 unique values → delta 20
	// key_1: 15 unique values → delta 15
	// key_2: 10 unique values → delta 10
	// key_3:  5 unique values → delta  5
	// key_4:  1 unique value  → delta  1
	valueCounts := []int{20, 15, 10, 5, 1}
	for keyIdx, count := range valueCounts {
		key := fmt.Sprintf("key_%d", keyIdx)
		for v := range count {
			p.shouldDrop("offender_metric", key, pcommon.NewValueStr(fmt.Sprintf("val_%d", v)))
		}
	}

	// Trigger rotation to compute the top-N snapshot.
	p.rotate()

	// Collect the telemetry.
	var collected metricdata.ResourceMetrics
	err = reader.Collect(t.Context(), &collected)
	require.NoError(t, err)

	// Find the otelcol_processor_cardinality_top.offenders gauge.
	topMetric := findMetricByName(t, collected, "otelcol_processor_cardinality_top.offenders")
	gaugeData, ok := topMetric.Data.(metricdata.Gauge[int64])
	require.True(t, ok, "otelcol_processor_cardinality_top.offenders must be a Gauge[int64]")

	// Assert we get exactly TopOffendersCount data points.
	require.Len(t, gaugeData.DataPoints, 3,
		"expected exactly 3 top offender data points (TopOffendersCount=3)")

	// Verify that data points are the top 3 by delta (key_0=20, key_1=15, key_2=10)
	// and that each carries the correct attributes.
	expectedKeys := []string{"key_0", "key_1", "key_2"}
	expectedDeltas := []int64{20, 15, 10}

	// Build a map of label_key → delta from the collected data points.
	type dpInfo struct {
		metricName string
		labelKey   string
		delta      int64
	}
	var dps []dpInfo
	for _, dp := range gaugeData.DataPoints {
		mn, mnOk := dp.Attributes.Value(attribute.Key("metric_name"))
		lk, lkOk := dp.Attributes.Value(attribute.Key("label_key"))
		require.True(t, mnOk, "data point must have metric_name attribute")
		require.True(t, lkOk, "data point must have label_key attribute")
		dps = append(dps, dpInfo{
			metricName: mn.AsString(),
			labelKey:   lk.AsString(),
			delta:      dp.Value,
		})
	}

	// Sort by descending delta for deterministic assertion.
	sort.Slice(dps, func(i, j int) bool { return dps[i].delta > dps[j].delta })

	for i, dp := range dps {
		assert.Equal(t, "offender_metric", dp.metricName,
			"data point %d: metric_name must be 'offender_metric'", i)
		assert.Equal(t, expectedKeys[i], dp.labelKey,
			"data point %d: label_key must be %q", i, expectedKeys[i])
		assert.Equal(t, expectedDeltas[i], dp.delta,
			"data point %d: delta must be %d", i, expectedDeltas[i])
	}

	t.Logf("SUCCESS: Top 3 offenders reported correctly: %v", dps)
}

// TestSortOffenders exercises the sortOffenders insertion sort with edge cases:
// empty slice, single element, reverse-sorted input, and already-sorted input.
func TestSortOffenders(t *testing.T) {
	tests := []struct {
		name   string
		input  []offenderEntry
		expect []uint64 // expected deltas in descending order
	}{
		{
			name:   "empty",
			input:  nil,
			expect: nil,
		},
		{
			name:   "single element",
			input:  []offenderEntry{{delta: 42}},
			expect: []uint64{42},
		},
		{
			name: "reverse sorted (ascending → must become descending)",
			input: []offenderEntry{
				{delta: 1}, {delta: 5}, {delta: 10}, {delta: 50},
			},
			expect: []uint64{50, 10, 5, 1},
		},
		{
			name: "already sorted descending (no-op)",
			input: []offenderEntry{
				{delta: 100}, {delta: 50}, {delta: 10}, {delta: 1},
			},
			expect: []uint64{100, 50, 10, 1},
		},
		{
			name: "mixed order",
			input: []offenderEntry{
				{delta: 7}, {delta: 3}, {delta: 99}, {delta: 15}, {delta: 1},
			},
			expect: []uint64{99, 15, 7, 3, 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sortOffenders(tc.input)
			var got []uint64
			for _, e := range tc.input {
				got = append(got, e.delta)
			}
			assert.Equal(t, tc.expect, got)
		})
	}
}

// TestCollectShardDeltas_Disabled verifies that collectShardDeltas is a no-op
// when topN is 0 (feature disabled).
func TestCollectShardDeltas_Disabled(t *testing.T) {
	entries := []trackerEntry{
		{key: trackerKey{metricName: "m", attrKey: "k"}, t: newTracker()},
	}
	// Simulate some growth on the tracker.
	entries[0].t.cachedCurr = 100
	entries[0].t.cachedPrev = 0

	result := collectShardDeltas(entries, nil, 0)
	assert.Nil(t, result, "collectShardDeltas must return nil when topN=0")
}

// TestTopOffenders_Disabled verifies that no gauge data points are emitted
// when TopOffendersCount is set to 0.
func TestTopOffenders_Disabled(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100,
		EpochDurationSeconds:        300,
		TopOffendersCount:           0, // Feature disabled
	}

	reader := sdkmetric.NewManualReader()
	sdkProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() {
		if err := sdkProvider.Shutdown(t.Context()); err != nil {
			t.Errorf("sdk provider shutdown: %v", err)
		}
	}()

	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	set.MeterProvider = sdkProvider

	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	// Add some data.
	for i := range 10 {
		p.shouldDrop("metric_a", "label_a", pcommon.NewValueStr(fmt.Sprintf("val_%d", i)))
	}

	p.rotate()

	var collected metricdata.ResourceMetrics
	err = reader.Collect(t.Context(), &collected)
	require.NoError(t, err)

	// otelcol_processor_cardinality_top.offenders should either not be present at all (OTel SDK
	// omits empty gauges) or present with 0 data points.
	var found bool
	for _, sm := range collected.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "otelcol_processor_cardinality_top.offenders" {
				found = true
				gaugeData, ok := m.Data.(metricdata.Gauge[int64])
				require.True(t, ok)
				assert.Empty(t, gaugeData.DataPoints,
					"when TopOffendersCount=0, no data points should be emitted")
			}
		}
	}
	if !found {
		t.Log("otelcol_processor_cardinality_top.offenders gauge not emitted (expected when disabled)")
	}
}

// TestTopOffenders_EvictionAndReplacement verifies that the bounded min-scan
// correctly evicts the smallest entry when a larger candidate arrives, and
// that candidates smaller than the minimum are rejected.
func TestTopOffenders_EvictionAndReplacement(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 10000,
		EpochDurationSeconds:        300,
		TopOffendersCount:           2, // Only keep top 2
	}

	reader := sdkmetric.NewManualReader()
	sdkProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() {
		if err := sdkProvider.Shutdown(t.Context()); err != nil {
			t.Errorf("sdk provider shutdown: %v", err)
		}
	}()

	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	set.MeterProvider = sdkProvider

	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	// key_small: 5 unique values (will be evicted)
	for i := range 5 {
		p.shouldDrop("m", "key_small", pcommon.NewValueStr(fmt.Sprintf("v%d", i)))
	}
	// key_medium: 50 unique values
	for i := range 50 {
		p.shouldDrop("m", "key_medium", pcommon.NewValueStr(fmt.Sprintf("v%d", i)))
	}
	// key_large: 200 unique values (should evict key_small)
	for i := range 200 {
		p.shouldDrop("m", "key_large", pcommon.NewValueStr(fmt.Sprintf("v%d", i)))
	}
	// key_tiny: 2 unique values (should be rejected — smaller than both survivors)
	for i := range 2 {
		p.shouldDrop("m", "key_tiny", pcommon.NewValueStr(fmt.Sprintf("v%d", i)))
	}

	p.rotate()

	var collected metricdata.ResourceMetrics
	err = reader.Collect(t.Context(), &collected)
	require.NoError(t, err)

	topMetric := findMetricByName(t, collected, "otelcol_processor_cardinality_top.offenders")
	gaugeData, ok := topMetric.Data.(metricdata.Gauge[int64])
	require.True(t, ok)
	require.Len(t, gaugeData.DataPoints, 2)

	// Collect the reported labels.
	var labels []string
	for _, dp := range gaugeData.DataPoints {
		lk, _ := dp.Attributes.Value(attribute.Key("label_key"))
		labels = append(labels, lk.AsString())
	}

	assert.Contains(t, labels, "key_large", "key_large (200) must survive")
	assert.Contains(t, labels, "key_medium", "key_medium (50) must survive")
	assert.NotContains(t, labels, "key_small", "key_small (5) must be evicted")
	assert.NotContains(t, labels, "key_tiny", "key_tiny (2) must be rejected")

	t.Logf("SUCCESS: Eviction works — survivors: %v", labels)
}

// TestMaxTrackerCount_Disabled verifies that when MaxTrackerCount is 0,
// an unlimited number of trackers can be created.
func TestMaxTrackerCount_Disabled(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1,
		EpochDurationSeconds:        300,
		MaxTrackerCount:             0,
	}

	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	for i := range 1000 {
		p.shouldDrop(fmt.Sprintf("metric_%d", i), "lbl", pcommon.NewValueStr("val_1"))
	}

	require.Equal(t, int64(1000), p.trackerCount.Load(), "should allow all trackers when disabled")
}

// TestMaxTrackerCount_Rejection verifies that when MaxTrackerCount is reached,
// no new trackers are created, and they simply pass through silently.
func TestMaxTrackerCount_Rejection(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1,
		EpochDurationSeconds:        300,
		MaxTrackerCount:             50,
	}

	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	// Attempt to create 100 unique trackers
	for i := range 100 {
		// shouldDrop returns true when limit exceeded, but we just verify the count
		p.shouldDrop(fmt.Sprintf("metric_%d", i), "lbl", pcommon.NewValueStr("val_1"))
	}

	require.Equal(t, int64(50), p.trackerCount.Load(), "should stop creating trackers at 50")
	require.Equal(t, int64(50), p.trackersRejected.Load(), "should have rejected 50 trackers")
}

// TestMaxTrackerCount_EvictionFreesSlots verifies that stale tracker eviction
// properly decrements trackerCount, allowing new trackers to be accepted again.
func TestMaxTrackerCount_EvictionFreesSlots(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1,
		EpochDurationSeconds:        300,
		MaxTrackerCount:             10,
	}

	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	// Fill to capacity
	for i := range 10 {
		p.shouldDrop(fmt.Sprintf("metric_%d", i), "lbl", pcommon.NewValueStr("val_1"))
	}

	require.Equal(t, int64(10), p.trackerCount.Load(), "filled to capacity")

	// Attempt 5 more, they should be rejected
	for i := 10; i < 15; i++ {
		p.shouldDrop(fmt.Sprintf("metric_%d", i), "lbl", pcommon.NewValueStr("val_1"))
	}
	require.Equal(t, int64(10), p.trackerCount.Load(), "still at capacity")
	require.Equal(t, int64(5), p.trackersRejected.Load(), "rejected 5")

	// Rotate enough times to evict all stale trackers.
	// 1st rotation resets the cached counts, 2nd rotation sets staleEpochs=1,
	// 3rd rotation sets staleEpochs=2 (which triggers eviction limit).
	p.rotate()
	p.rotate()
	p.rotate()

	require.Equal(t, int64(0), p.trackerCount.Load(), "eviction should clear trackerCount")

	// Verify we can now accept new trackers
	for i := 15; i < 20; i++ {
		p.shouldDrop(fmt.Sprintf("metric_%d", i), "lbl", pcommon.NewValueStr("val_1"))
	}

	require.Equal(t, int64(5), p.trackerCount.Load(), "new trackers accepted after eviction")
	require.Equal(t, int64(5), p.trackersRejected.Load(), "refusals count remains unchanged from before")
}

// TestMetricOverrides_SpecificLimit verifies that a metric with an override
// uses its specific limit instead of the global default.
func TestMetricOverrides_SpecificLimit(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 10,
		EpochDurationSeconds:        300,
		MetricOverrides: map[string]int{
			"http.server.request.duration": 5000,
		},
	}

	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	// Send 100 unique values to the overridden metric — none should be dropped
	// because 100 < 5000
	for i := range 100 {
		dropped := p.shouldDrop("http.server.request.duration", "route", pcommon.NewValueStr(fmt.Sprintf("/api/v1/resource/%d", i)))
		require.False(t, dropped, "should not drop within override limit")
	}
}

// TestMetricOverrides_FallbackToGlobal verifies that metrics not listed in
// the override map use the global MaxCardinalityDeltaPerEpoch.
func TestMetricOverrides_FallbackToGlobal(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 5,
		EpochDurationSeconds:        300,
		MetricOverrides: map[string]int{
			"http.server.request.duration": 5000,
		},
	}

	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)

	// Send 20 unique values to a metric NOT in overrides — should drop after 5
	var dropCount int
	for i := range 20 {
		if p.shouldDrop("db.query.duration", "table_name", pcommon.NewValueStr(fmt.Sprintf("table_%d", i))) {
			dropCount++
		}
	}
	require.Positive(t, dropCount, "unspecified metric should enforce global limit")
}

// TestMetricOverrides_Validation verifies that invalid override values
// are caught during config validation.
func TestMetricOverrides_Validation(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 500,
		EpochDurationSeconds:        300,
		MetricOverrides: map[string]int{
			"bad_metric": -1,
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "metric_overrides")
}

// TestDropLogSampling_LimitEnforced verifies that only DropLogMaxPerEpoch
// enforcement warnings are emitted, and the rest are suppressed.
func TestDropLogSampling_LimitEnforced(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 5,
		EpochDurationSeconds:        300,
		DropLogMaxPerEpoch:          3,
		EnforcementMode:             EnforcementStripAndReaggregate,
	}

	core, logs := observer.New(zap.WarnLevel)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)
	p.logger = zap.New(core)

	// Send 50 unique attribute values through handleAttributes
	// After 5 unique values, shouldDrop returns true and drops start logging
	for i := range 50 {
		attrs := pmetric.NewMetrics().ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty().Attributes()
		attrs.PutStr("label_key", fmt.Sprintf("value_%d", i))
		p.handleAttributes("test.metric", attrs)
	}

	warnCount := logs.FilterMessage("Dropping high-cardinality attribute").Len()
	require.Equal(t, 3, warnCount, "should only log DropLogMaxPerEpoch warnings")
	require.Greater(t, p.dropsThisEpoch.Load(), int64(3), "total drops should exceed the log cap")
}

// TestDropLogSampling_Disabled verifies that setting DropLogMaxPerEpoch=0
// disables the cap, logging every enforcement event.
func TestDropLogSampling_Disabled(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 5,
		EpochDurationSeconds:        300,
		DropLogMaxPerEpoch:          0, // disabled — log everything
		EnforcementMode:             EnforcementStripAndReaggregate,
	}

	core, logs := observer.New(zap.WarnLevel)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)
	p.logger = zap.New(core)

	for i := range 30 {
		attrs := pmetric.NewMetrics().ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty().Attributes()
		attrs.PutStr("label_key", fmt.Sprintf("value_%d", i))
		p.handleAttributes("test.metric", attrs)
	}

	warnCount := logs.FilterMessage("Dropping high-cardinality attribute").Len()
	require.Greater(t, warnCount, 3, "with cap disabled, should log more than 3 warnings")
}

// TestDropLogSampling_ResetOnRotate verifies that the drop log counter
// resets after epoch rotation, allowing fresh logs in the new epoch.
func TestDropLogSampling_ResetOnRotate(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 5,
		EpochDurationSeconds:        300,
		DropLogMaxPerEpoch:          2,
		EnforcementMode:             EnforcementStripAndReaggregate,
	}

	core, logs := observer.New(zap.WarnLevel)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	next := new(consumertest.MetricsSink)
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	p := proc.(*cardinalityProcessor)
	p.logger = zap.New(core)

	// Fill epoch 1 — only 2 logs emitted
	for i := range 20 {
		attrs := pmetric.NewMetrics().ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty().Attributes()
		attrs.PutStr("label_key", fmt.Sprintf("value_%d", i))
		p.handleAttributes("test.metric", attrs)
	}
	require.Equal(t, 2, logs.FilterMessage("Dropping high-cardinality attribute").Len())

	// Rotate epoch — counters should reset
	p.rotate()

	// Fill epoch 2 — must re-send baseline (0-19) plus new values (20-39)
	// so the HLL accurately calculates the delta growth over the baseline.
	for i := range 40 {
		attrs := pmetric.NewMetrics().ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty().Attributes()
		attrs.PutStr("label_key", fmt.Sprintf("value_%d", i))
		p.handleAttributes("test.metric", attrs)
	}
	require.Equal(t, 4, logs.FilterMessage("Dropping high-cardinality attribute").Len(), "should have 2 logs from each epoch")
}

// TestOverflowAttributeMode verifies that enforcement_mode: overflow_attribute
// replaces the high-cardinality attribute value with the sentinel string
// "otel.cardinality_overflow" instead of removing the attribute or adding a tag.
// This avoids the Single-Writer violation because all overflow data points for
// a given (metric, attribute_key) collapse into a single overflow identity.
func TestOverflowAttributeMode(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1,
		EpochDurationSeconds:        300,
		EnforcementMode:             EnforcementOverflowAttribute,
	}

	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("api.request")
	m.SetEmptyGauge()

	for i := 1; i <= 3; i++ {
		dp := m.Gauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i * 10))
		dp.SetTimestamp(pcommon.Timestamp(i * 1000)) // Set explicit timestamps to test latest selection
		dp.Attributes().PutStr("region", "us-east")
		dp.Attributes().PutStr("user_id", fmt.Sprintf("user_%d", i))
	}

	err = proc.ConsumeMetrics(t.Context(), md)
	require.NoError(t, err)

	outMetrics := next.AllMetrics()
	require.Len(t, outMetrics, 1)

	outDpList := outMetrics[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()

	// 2 data points should remain: the 1st valid point, and the 2nd/3rd overflow points
	// which are successfully reaggregated into a single combined point.
	require.Equal(t, 2, outDpList.Len())

	overflowCount := 0
	for i := 0; i < outDpList.Len(); i++ {
		dp := outDpList.At(i)
		userVal, hasUser := dp.Attributes().Get("user_id")
		require.True(t, hasUser, "user_id attribute must still exist in overflow mode")

		if userVal.Str() == overflowSentinel {
			overflowCount++
			// For a Gauge, the reaggregated value must come from the data point
			// with the highest timestamp (point 3 -> value 30).
			assert.Equal(t, int64(30), dp.IntValue(), "merged overflow Gauge must take latest value (30)")
			assert.Equal(t, pcommon.Timestamp(3000), dp.Timestamp(), "merged overflow Gauge must have latest timestamp (3000)")
		} else {
			assert.Equal(t, "user_1", userVal.Str(), "first data point must keep its original user_id")
			assert.Equal(t, int64(10), dp.IntValue())
		}
	}

	assert.Equal(t, 1, overflowCount, "exactly one merged data point should hold the overflow sentinel")

	t.Logf("SUCCESS: Overflow points successfully reaggregated into single Sentinel point!")
}

// TestCumulativeSumFallsBackToTagOnly verifies that when enforcement_mode is
// strip_and_reaggregate, Cumulative Sum metrics fall back to tag_only behavior
// because reaggregation for cumulative temporality is not yet supported.
func TestCumulativeSumFallsBackToTagOnly(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1,
		EpochDurationSeconds:        300,
		EnforcementMode:             EnforcementStripAndReaggregate,
	}

	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("api.request.cumulative")
	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative) // Cumulative!

	for i := 1; i <= 3; i++ {
		dp := sum.DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i * 100))
		dp.Attributes().PutStr("region", "us-east")
		dp.Attributes().PutStr("user_id", fmt.Sprintf("user_%d", i))
	}

	err = proc.ConsumeMetrics(t.Context(), md)
	require.NoError(t, err)

	outMetrics := next.AllMetrics()
	require.Len(t, outMetrics, 1)

	outDpList := outMetrics[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()

	// All 3 data points should remain (tag_only fallback doesn't remove attributes).
	require.Equal(t, 3, outDpList.Len(),
		"cumulative sums should fall back to tag_only — no attributes removed")

	// At least one should have the overflow tag.
	taggedCount := 0
	for i := 0; i < outDpList.Len(); i++ {
		if _, hasTag := outDpList.At(i).Attributes().Get("otel.metric.overflow"); hasTag {
			taggedCount++
		}
	}

	assert.Positive(t, taggedCount,
		"cumulative sums should get otel.metric.overflow tag via fallback")

	t.Logf("SUCCESS: Cumulative Sum fell back to tag_only. %d/%d tagged.", taggedCount, outDpList.Len())
}

// TestUnsupportedMetricTypesFallBackToTagOnly verifies that Histogram,
// ExponentialHistogram, and Summary metrics fall back to tag_only behavior.
func TestUnsupportedMetricTypesFallBackToTagOnly(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1,
		EpochDurationSeconds:        300,
		EnforcementMode:             EnforcementStripAndReaggregate,
	}

	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// 1. Histogram
	mHist := sm.Metrics().AppendEmpty()
	mHist.SetName("api.request.histogram")
	hist := mHist.SetEmptyHistogram()
	for i := 1; i <= 3; i++ {
		dp := hist.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("user_id", fmt.Sprintf("hist_user_%d", i))
	}

	// 2. Exponential Histogram
	mExp := sm.Metrics().AppendEmpty()
	mExp.SetName("api.request.exphist")
	exp := mExp.SetEmptyExponentialHistogram()
	for i := 1; i <= 3; i++ {
		dp := exp.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("user_id", fmt.Sprintf("exp_user_%d", i))
	}

	// 3. Summary
	mSumm := sm.Metrics().AppendEmpty()
	mSumm.SetName("api.request.summary")
	summ := mSumm.SetEmptySummary()
	for i := 1; i <= 3; i++ {
		dp := summ.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("user_id", fmt.Sprintf("summ_user_%d", i))
	}

	err = proc.ConsumeMetrics(t.Context(), md)
	require.NoError(t, err)

	outMetrics := next.AllMetrics()
	require.Len(t, outMetrics, 1)

	metrics := outMetrics[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	require.Equal(t, 3, metrics.Len())

	// Helper to check tags manually since we can't easily use interfaces for concrete types.

	histDps := metrics.At(0).Histogram().DataPoints()
	histTagged := 0
	for i := 0; i < histDps.Len(); i++ {
		if _, hasTag := histDps.At(i).Attributes().Get("otel.metric.overflow"); hasTag {
			histTagged++
		}
	}
	assert.Positive(t, histTagged)

	expDps := metrics.At(1).ExponentialHistogram().DataPoints()
	expTagged := 0
	for i := 0; i < expDps.Len(); i++ {
		if _, hasTag := expDps.At(i).Attributes().Get("otel.metric.overflow"); hasTag {
			expTagged++
		}
	}
	assert.Positive(t, expTagged)

	summDps := metrics.At(2).Summary().DataPoints()
	summTagged := 0
	for i := 0; i < summDps.Len(); i++ {
		if _, hasTag := summDps.At(i).Attributes().Get("otel.metric.overflow"); hasTag {
			summTagged++
		}
	}
	assert.Positive(t, summTagged)
}

// TestProcessHistogramTypes verifies that Histogram, ExponentialHistogram,
// and Summary metrics are properly processed when not using strip_and_reaggregate.
func TestProcessHistogramTypes(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1,
		EpochDurationSeconds:        300,
		EnforcementMode:             EnforcementOverflowAttribute,
	}

	next := new(consumertest.MetricsSink)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, next)
	require.NoError(t, err)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	mHist := sm.Metrics().AppendEmpty()
	mHist.SetName("api.request.histogram")
	hist := mHist.SetEmptyHistogram()
	for i := 1; i <= 3; i++ {
		dp := hist.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("user_id", fmt.Sprintf("hist_user_%d", i))
	}

	mExp := sm.Metrics().AppendEmpty()
	mExp.SetName("api.request.exphist")
	exp := mExp.SetEmptyExponentialHistogram()
	for i := 1; i <= 3; i++ {
		dp := exp.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("user_id", fmt.Sprintf("exp_user_%d", i))
	}

	mSumm := sm.Metrics().AppendEmpty()
	mSumm.SetName("api.request.summary")
	summ := mSumm.SetEmptySummary()
	for i := 1; i <= 3; i++ {
		dp := summ.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("user_id", fmt.Sprintf("summ_user_%d", i))
	}

	err = proc.ConsumeMetrics(t.Context(), md)
	require.NoError(t, err)

	outMetrics := next.AllMetrics()
	require.Len(t, outMetrics, 1)

	metrics := outMetrics[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	require.Equal(t, 3, metrics.Len())

	// Verify that because spatial reaggregation is not supported for these types,
	// EnforcementOverflowAttribute falls back to EnforcementTagOnly behavior
	// to prevent duplicate identity collisions.
	histDps := metrics.At(0).Histogram().DataPoints()
	v, _ := histDps.At(1).Attributes().Get("user_id")
	assert.Equal(t, "hist_user_2", v.Str(), "Histogram must keep original value in fallback")
	marker, hasMarker := histDps.At(1).Attributes().Get("otel.metric.overflow")
	assert.True(t, hasMarker)
	assert.True(t, marker.Bool())

	expDps := metrics.At(1).ExponentialHistogram().DataPoints()
	v, _ = expDps.At(1).Attributes().Get("user_id")
	assert.Equal(t, "exp_user_2", v.Str(), "ExpHistogram must keep original value in fallback")
	marker, hasMarker = expDps.At(1).Attributes().Get("otel.metric.overflow")
	assert.True(t, hasMarker)
	assert.True(t, marker.Bool())

	summDps := metrics.At(2).Summary().DataPoints()
	v, _ = summDps.At(1).Attributes().Get("user_id")
	assert.Equal(t, "summ_user_2", v.Str(), "Summary must keep original value in fallback")
	marker, hasMarker = summDps.At(1).Attributes().Get("otel.metric.overflow")
	assert.True(t, hasMarker)
	assert.True(t, marker.Bool())
}

// TestEnforcementModeValidation verifies that invalid enforcement_mode values
// are rejected during config validation.
func TestEnforcementModeValidation(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100,
		EpochDurationSeconds:        30,
		EnforcementMode:             "invalid_mode",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enforcement_mode must be one of")

	// Valid modes should pass.
	for _, mode := range []EnforcementMode{EnforcementTagOnly, EnforcementOverflowAttribute, EnforcementStripAndReaggregate} {
		cfg.EnforcementMode = mode
		assert.NoError(t, cfg.Validate(), "mode %q should be valid", mode)
	}
}

// TestResolvedEnforcementMode verifies that resolvedEnforcementMode returns
// the configured mode or defaults to tag_only.
func TestResolvedEnforcementMode(t *testing.T) {
	// Default (empty) → tag_only
	cfg := &Config{}
	assert.Equal(t, EnforcementTagOnly, cfg.resolvedEnforcementMode())

	// Explicit EnforcementMode is returned as-is (lowercased)
	cfg = &Config{EnforcementMode: EnforcementTagOnly}
	assert.Equal(t, EnforcementTagOnly, cfg.resolvedEnforcementMode())

	cfg = &Config{EnforcementMode: EnforcementOverflowAttribute}
	assert.Equal(t, EnforcementOverflowAttribute, cfg.resolvedEnforcementMode())
}

// TestShouldDrop_NonStringAttributeTypes verifies that hashAttrValue
// distinguishes attribute values across every pcommon.ValueType, including
// Map and Slice values whose contents differ.
func TestShouldDrop_NonStringAttributeTypes(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 100,
		EpochDurationSeconds:        300,
	}
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, new(consumertest.MetricsSink))
	require.NoError(t, err)
	p := proc.(*cardinalityProcessor)

	// Int values that differ must hash differently. shouldDrop never returns
	// true while under the limit, so we instead inspect the tracker's HLL
	// cardinality estimate after the calls.
	intA := pcommon.NewValueInt(42)
	intB := pcommon.NewValueInt(43)
	p.shouldDrop("m", "k", intA)
	p.shouldDrop("m", "k", intB)

	// Map values with different contents must hash differently.
	mapA := pcommon.NewValueMap()
	mapA.Map().PutStr("inner", "alpha")
	mapB := pcommon.NewValueMap()
	mapB.Map().PutStr("inner", "beta")
	p.shouldDrop("m2", "k", mapA)
	p.shouldDrop("m2", "k", mapB)

	// Slice values with different contents must hash differently.
	sliceA := pcommon.NewValueSlice()
	sliceA.Slice().AppendEmpty().SetStr("a")
	sliceB := pcommon.NewValueSlice()
	sliceB.Slice().AppendEmpty().SetStr("b")
	p.shouldDrop("m3", "k", sliceA)
	p.shouldDrop("m3", "k", sliceB)

	for _, metricName := range []string{"m", "m2", "m3"} {
		shard := p.getShard(metricName)
		shard.mu.RLock()
		tr, ok := shard.trackers[trackerKey{metricName: metricName, attrKey: "k"}]
		shard.mu.RUnlock()
		require.True(t, ok, "tracker for %s should exist", metricName)
		// Two distinct values → HLL estimate must be ≥ 2.
		tr.mu.Lock()
		est := tr.current.Estimate()
		tr.mu.Unlock()
		assert.GreaterOrEqual(t, est, uint64(2),
			"tracker for %s should record 2 unique values, got %d", metricName, est)
	}
}

// TestHandleAttributes_DropLogMaxZero verifies that when DropLogMaxPerEpoch is
// 0 (unlimited), dropLogCount still increments on every drop so the per-epoch
// suppression summary reflects the true count.
func TestHandleAttributes_DropLogMaxZero(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 1,
		EpochDurationSeconds:        300,
		EnforcementMode:             EnforcementStripAndReaggregate,
		DropLogMaxPerEpoch:          0, // unlimited
	}
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, new(consumertest.MetricsSink))
	require.NoError(t, err)
	p := proc.(*cardinalityProcessor)

	// Push enough unique values to trigger the drop path repeatedly.
	attrs := pcommon.NewMap()
	for i := range 50 {
		attrs.Clear()
		attrs.PutStr("k", fmt.Sprintf("v%d", i))
		p.handleAttributes("metric", attrs)
	}

	// dropLogCount must reflect every overflow event, not be skipped by
	// the short-circuit. (We compare ≥ a healthy floor rather than exact
	// because the first ~MaxCardinalityDeltaPerEpoch inserts don't trip drops.)
	require.Positive(t, p.dropLogCount.Load(),
		"dropLogCount must increment even when DropLogMaxPerEpoch is unlimited")
	require.Equal(t, p.dropLogCount.Load(), p.dropsThisEpoch.Load(),
		"dropLogCount and dropsThisEpoch should match when nothing is suppressed")
}
