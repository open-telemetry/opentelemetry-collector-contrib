// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadatatest"
)

func TestNewTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)
}

func TestLeafSpanPruning_BasicAggregation(t *testing.T) {
	// Test: 3 identical leaf spans should be aggregated into 1 summary span
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithLeafSpans(t, 3, "SELECT", map[string]string{"db.operation": "select"})
	originalSpanCount := countSpans(td)
	assert.Equal(t, 4, originalSpanCount) // 1 parent + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After processing: should have 1 parent + 1 summary span
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)

	// Verify summary span exists with aggregation attributes
	summarySpan, found := findSummarySpan(td)
	require.True(t, found, "summary span should exist")

	// Check aggregation attributes
	attrs := summarySpan.Attributes()
	spanCount, exists := attrs.Get("aggregation.span_count")
	assert.True(t, exists, "aggregation.span_count should exist")
	assert.Equal(t, int64(3), spanCount.Int())
}

func TestLeafSpanPruning_BelowThreshold(t *testing.T) {
	// Test: 1 leaf span with min_spans_to_aggregate=2 should not be aggregated
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithLeafSpans(t, 1, "SELECT", map[string]string{"db.operation": "select"})
	originalSpanCount := countSpans(td)
	assert.Equal(t, 2, originalSpanCount) // 1 parent + 1 leaf span

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Should remain unchanged
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)
}

func TestLeafSpanPruning_MixedLeafAndNonLeaf(t *testing.T) {
	// Test: only aggregate leaf spans, not spans with children
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace: root -> intermediate -> 3 leaf spans
	td := createTestTraceWithIntermediateSpan(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 5, originalSpanCount) // 1 root + 1 intermediate + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 root + 1 intermediate + 1 summary
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)
}

func TestLeafSpanPruning_DifferentGroups(t *testing.T) {
	// Test: spans with different attributes should stay in separate groups
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.operation"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with mixed operations: 3 SELECT + 2 INSERT
	td := createTestTraceWithMixedOperations(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 6, originalSpanCount) // 1 parent + 3 SELECT + 2 INSERT

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 parent + 1 SELECT summary + 1 INSERT summary
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)
}

func TestLeafSpanPruning_EmptyTrace(t *testing.T) {
	// Test: empty trace should be handled gracefully
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := ptrace.NewTraces()

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)
	assert.Equal(t, 0, countSpans(td))
}

func TestLeafSpanPruning_SingleSpanTrace(t *testing.T) {
	// Test: single span trace (root only) should not be modified
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createSingleSpanTrace(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 1, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Should remain unchanged
	finalSpanCount := countSpans(td)
	assert.Equal(t, 1, finalSpanCount)
}

func TestLeafSpanPruning_StatusAggregation(t *testing.T) {
	// Test: spans with different status codes should be in separate groups
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with 4 OK spans and 2 Error spans (same name)
	td := createTestTraceWithMixedStatusSpans(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 7, originalSpanCount) // 1 parent + 4 OK + 2 Error

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 parent + 1 OK summary (4 spans) + 1 Error summary (2 spans)
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)

	// Verify we have both an OK summary and an Error summary
	okSummary, found := findSpanByNameAndStatus(td, "SELECT", ptrace.StatusCodeOk)
	require.True(t, found)
	okCount, _ := okSummary.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(4), okCount.Int())

	errorSummary, found := findSpanByNameAndStatus(td, "SELECT", ptrace.StatusCodeError)
	require.True(t, found)
	errorCount, _ := errorSummary.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(2), errorCount.Int())
}

func TestLeafSpanPruning_StatusBelowThreshold(t *testing.T) {
	// Test: 1 OK span + 1 Error span should not aggregate (each group below threshold)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithErrorSpan(t) // Creates 1 OK + 1 Error span
	originalSpanCount := countSpans(td)
	assert.Equal(t, 3, originalSpanCount) // 1 parent + 1 OK + 1 Error

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Should remain unchanged - neither group meets threshold
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)
}

func TestLeafSpanPruning_DurationStats(t *testing.T) {
	// Test: verify duration statistics are calculated correctly
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create spans with known durations: 100ns, 200ns, 300ns
	td := createTestTraceWithKnownDurations(t, []int64{100, 200, 300})

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	summarySpan, found := findSummarySpan(td)
	require.True(t, found)

	attrs := summarySpan.Attributes()

	minDuration, _ := attrs.Get("aggregation.duration_min_ns")
	assert.Equal(t, int64(100), minDuration.Int())

	maxDuration, _ := attrs.Get("aggregation.duration_max_ns")
	assert.Equal(t, int64(300), maxDuration.Int())

	avgDuration, _ := attrs.Get("aggregation.duration_avg_ns")
	assert.Equal(t, int64(200), avgDuration.Int()) // (100+200+300)/3 = 200

	totalDuration, _ := attrs.Get("aggregation.duration_total_ns")
	assert.Equal(t, int64(600), totalDuration.Int())
}

func TestLeafSpanPruningProcessorWithHistogram(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.AggregationHistogramBuckets = []time.Duration{
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
	}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithKnownDurations(t, []int64{
		int64(5 * time.Millisecond),
		int64(15 * time.Millisecond),
		int64(25 * time.Millisecond),
		int64(75 * time.Millisecond),
		int64(150 * time.Millisecond),
	})

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	assert.Equal(t, 2, countSpans(td))

	summarySpan, found := findSummarySpan(td)
	require.True(t, found)

	attrs := summarySpan.Attributes()

	bounds, exists := attrs.Get("aggregation.histogram_bucket_bounds_s")
	require.True(t, exists)
	require.Equal(t, 3, bounds.Slice().Len())
	expectedBounds := []float64{0.01, 0.05, 0.1}
	for i, expected := range expectedBounds {
		assert.InDelta(t, expected, bounds.Slice().At(i).Double(), 1e-9)
	}

	counts, exists := attrs.Get("aggregation.histogram_bucket_counts")
	require.True(t, exists)
	expectedCounts := []int64{1, 3, 4, 5}
	require.Equal(t, len(expectedCounts), counts.Slice().Len())
	for i, expected := range expectedCounts {
		assert.Equal(t, expected, counts.Slice().At(i).Int())
	}
}

func TestLeafSpanPruningProcessorWithHistogramDisabled(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.AggregationHistogramBuckets = []time.Duration{}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithKnownDurations(t, []int64{
		int64(5 * time.Millisecond),
		int64(15 * time.Millisecond),
		int64(25 * time.Millisecond),
		int64(75 * time.Millisecond),
		int64(150 * time.Millisecond),
	})

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	assert.Equal(t, 2, countSpans(td))

	summarySpan, found := findSummarySpan(td)
	require.True(t, found)

	attrs := summarySpan.Attributes()

	_, exists := attrs.Get("aggregation.histogram_bucket_bounds_s")
	assert.False(t, exists)

	_, exists = attrs.Get("aggregation.histogram_bucket_counts")
	assert.False(t, exists)
}

func TestLeafSpanPruning_GroupByNonStringAttributes(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.retries", "db.cached"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithNonStringAttributes(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)

	summaries := findAllSummarySpans(td)
	assert.Len(t, summaries, 2)

	var foundRetriesOne bool
	var foundRetriesTwo bool
	for _, summary := range summaries {
		retries, retriesExists := summary.Attributes().Get("db.retries")
		require.True(t, retriesExists)
		cached, cachedExists := summary.Attributes().Get("db.cached")
		require.True(t, cachedExists)
		count, _ := summary.Attributes().Get("aggregation.span_count")
		if retries.Int() == 1 && cached.Bool() {
			foundRetriesOne = true
			assert.Equal(t, int64(2), count.Int())
		}
		if retries.Int() == 2 && cached.Bool() {
			foundRetriesTwo = true
			assert.Equal(t, int64(2), count.Int())
		}
	}

	assert.True(t, foundRetriesOne)
	assert.True(t, foundRetriesTwo)
}

func TestLeafSpanPruning_TemplateEventsAndLinksPreserved(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithTemplateEventsAndLinks(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	summarySpan, found := findSummarySpan(td)
	require.True(t, found)

	events := summarySpan.Events()
	require.Equal(t, 1, events.Len())
	assert.Equal(t, "template_event", events.At(0).Name())
	eventAttr, eventAttrExists := events.At(0).Attributes().Get("event.attr")
	require.True(t, eventAttrExists)
	assert.Equal(t, "value", eventAttr.Str())

	links := summarySpan.Links()
	require.Equal(t, 1, links.Len())
	linkAttr, linkAttrExists := links.At(0).Attributes().Get("link.kind")
	require.True(t, linkAttrExists)
	assert.Equal(t, "template", linkAttr.Str())
}

func TestAttributeLoss_RecordsMetricsAndSummaryAttributesWhenEnabled(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.EnableAttributeLossAnalysis = true
	cfg.AttributeLossExemplarSampleRate = 0

	tp, err := factory.CreateTraces(t.Context(), metadatatest.NewSettings(tel), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td, _ := createTestTraceWithAttributeLoss(t)
	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	leafSummary, found := findSummarySpanByName(td, "SELECT")
	require.True(t, found, "leaf summary span should exist")
	leafDiverse, exists := leafSummary.Attributes().Get("aggregation.diverse_attributes")
	require.True(t, exists)
	assert.Contains(t, leafDiverse.Str(), "db.system(1)")
	leafMissing, exists := leafSummary.Attributes().Get("aggregation.missing_attributes")
	require.True(t, exists)
	assert.Contains(t, leafMissing.Str(), "db.instance(2)")

	parentSummary, found := findSummarySpanByName(td, "handler")
	require.True(t, found, "parent summary span should exist")
	parentDiverse, exists := parentSummary.Attributes().Get("aggregation.diverse_attributes")
	require.True(t, exists)
	assert.Contains(t, parentDiverse.Str(), "service.instance.id(1)")
	parentMissing, exists := parentSummary.Attributes().Get("aggregation.missing_attributes")
	require.True(t, exists)
	assert.Contains(t, parentMissing.Str(), "handler.version(1)")

	assertHistogramRecordedOnceWithSum(t, tel, "otelcol_processor_spanpruning_leaf_attribute_diversity_loss")
	assertHistogramRecordedOnceWithSum(t, tel, "otelcol_processor_spanpruning_leaf_attribute_loss")
	assertHistogramRecordedOnceWithSum(t, tel, "otelcol_processor_spanpruning_parent_attribute_diversity_loss")
	assertHistogramRecordedOnceWithSum(t, tel, "otelcol_processor_spanpruning_parent_attribute_loss")
}

func TestAttributeLoss_ExemplarSampling(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.EnableAttributeLossAnalysis = true
	cfg.AttributeLossExemplarSampleRate = 1

	tp, err := factory.CreateTraces(t.Context(), metadatatest.NewSettings(tel), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td, traceID := createTestTraceWithAttributeLoss(t)
	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	assertHistogramHasTraceExemplar(t, tel, "otelcol_processor_spanpruning_leaf_attribute_diversity_loss", traceID)
	assertHistogramHasTraceExemplar(t, tel, "otelcol_processor_spanpruning_leaf_attribute_loss", traceID)
	assertHistogramHasTraceExemplar(t, tel, "otelcol_processor_spanpruning_parent_attribute_diversity_loss", traceID)
	assertHistogramHasTraceExemplar(t, tel, "otelcol_processor_spanpruning_parent_attribute_loss", traceID)
}

func TestAttributeLoss_DisabledByDefault(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	require.False(t, cfg.EnableAttributeLossAnalysis, "attribute loss analysis should be disabled by default")
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), metadatatest.NewSettings(tel), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td, _ := createTestTraceWithAttributeLoss(t)
	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	leafSummary, found := findSummarySpanByName(td, "SELECT")
	require.True(t, found, "leaf summary span should exist")
	_, exists := leafSummary.Attributes().Get("aggregation.diverse_attributes")
	assert.False(t, exists)
	_, exists = leafSummary.Attributes().Get("aggregation.missing_attributes")
	assert.False(t, exists)

	parentSummary, found := findSummarySpanByName(td, "handler")
	require.True(t, found, "parent summary span should exist")
	_, exists = parentSummary.Attributes().Get("aggregation.diverse_attributes")
	assert.False(t, exists)
	_, exists = parentSummary.Attributes().Get("aggregation.missing_attributes")
	assert.False(t, exists)

	_, err = tel.GetMetric("otelcol_processor_spanpruning_leaf_attribute_diversity_loss")
	assert.Error(t, err)
	_, err = tel.GetMetric("otelcol_processor_spanpruning_leaf_attribute_loss")
	assert.Error(t, err)
	_, err = tel.GetMetric("otelcol_processor_spanpruning_parent_attribute_diversity_loss")
	assert.Error(t, err)
	_, err = tel.GetMetric("otelcol_processor_spanpruning_parent_attribute_loss")
	assert.Error(t, err)
}

func TestOutlierMetrics_IQR(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 5
	cfg.GroupByAttributes = []string{"db.operation"}
	cfg.EnableOutlierAnalysis = true
	cfg.OutlierAnalysis.Method = OutlierMethodIQR
	cfg.OutlierAnalysis.PreserveOutliers = true
	cfg.OutlierAnalysis.MaxPreservedOutliers = 2
	cfg.OutlierAnalysis.IQRMultiplier = 1.5
	cfg.OutlierAnalysis.MinGroupSize = 7
	cfg.OutlierAnalysis.CorrelationMinOccurrence = 0.75
	cfg.OutlierAnalysis.CorrelationMaxNormalOccurrence = 0.25
	cfg.OutlierAnalysis.MaxCorrelatedAttributes = 5

	tp, err := factory.CreateTraces(t.Context(), metadatatest.NewSettings(tel), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithOutliers(t)
	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	metadatatest.AssertEqualProcessorSpanpruningOutliersDetected(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 2}},
		metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualProcessorSpanpruningOutliersPreserved(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 2}},
		metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualProcessorSpanpruningOutliersCorrelationsDetected(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp())
}

func TestOutlierMetrics_MAD(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 5
	cfg.GroupByAttributes = []string{"db.operation"}
	cfg.EnableOutlierAnalysis = true
	cfg.OutlierAnalysis.Method = OutlierMethodMAD
	cfg.OutlierAnalysis.PreserveOutliers = true
	cfg.OutlierAnalysis.MaxPreservedOutliers = 2
	cfg.OutlierAnalysis.MADMultiplier = 3.0
	cfg.OutlierAnalysis.MinGroupSize = 7
	cfg.OutlierAnalysis.CorrelationMinOccurrence = 0.75
	cfg.OutlierAnalysis.CorrelationMaxNormalOccurrence = 0.25
	cfg.OutlierAnalysis.MaxCorrelatedAttributes = 5

	tp, err := factory.CreateTraces(t.Context(), metadatatest.NewSettings(tel), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithOutliers(t)
	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	metadatatest.AssertEqualProcessorSpanpruningOutliersDetected(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 2}},
		metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualProcessorSpanpruningOutliersPreserved(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 2}},
		metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualProcessorSpanpruningOutliersCorrelationsDetected(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp())
}

func TestOutlierMetrics_NoPreservation(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 5
	cfg.GroupByAttributes = []string{"db.operation"}
	cfg.EnableOutlierAnalysis = true
	cfg.OutlierAnalysis.Method = OutlierMethodIQR
	cfg.OutlierAnalysis.PreserveOutliers = false
	cfg.OutlierAnalysis.IQRMultiplier = 1.5
	cfg.OutlierAnalysis.MinGroupSize = 7
	cfg.OutlierAnalysis.CorrelationMinOccurrence = 0.75
	cfg.OutlierAnalysis.CorrelationMaxNormalOccurrence = 0.25
	cfg.OutlierAnalysis.MaxCorrelatedAttributes = 5

	tp, err := factory.CreateTraces(t.Context(), metadatatest.NewSettings(tel), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithOutliers(t)
	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	metadatatest.AssertEqualProcessorSpanpruningOutliersDetected(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 2}},
		metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualProcessorSpanpruningOutliersCorrelationsDetected(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp())

	_, err = tel.GetMetric("otelcol_processor_spanpruning_outliers_preserved")
	assert.Error(t, err, "outliers_preserved should not be recorded when preserve_outliers is false")
}

func TestProcessorPreservesOutlierSpans(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 5
	cfg.GroupByAttributes = []string{"db.operation"}
	cfg.EnableOutlierAnalysis = true
	cfg.OutlierAnalysis.PreserveOutliers = true
	cfg.OutlierAnalysis.MaxPreservedOutliers = 2
	cfg.OutlierAnalysis.IQRMultiplier = 1.5
	cfg.OutlierAnalysis.MinGroupSize = 7
	cfg.OutlierAnalysis.CorrelationMinOccurrence = 0.75
	cfg.OutlierAnalysis.CorrelationMaxNormalOccurrence = 0.25
	cfg.OutlierAnalysis.MaxCorrelatedAttributes = 5

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithOutliers(t)
	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	var spans []ptrace.Span
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				spans = append(spans, ss.Spans().At(k))
			}
		}
	}

	var summarySpan ptrace.Span
	var outlierSpans []ptrace.Span
	for _, span := range spans {
		if isSummary, exists := span.Attributes().Get("aggregation.is_summary"); exists && isSummary.Bool() {
			summarySpan = span
		}
		if isOutlier, exists := span.Attributes().Get("aggregation.is_preserved_outlier"); exists && isOutlier.Bool() {
			outlierSpans = append(outlierSpans, span)
		}
	}

	require.False(t, summarySpan.SpanID().IsEmpty(), "summary span should exist")
	assert.Len(t, outlierSpans, 2, "should have 2 preserved outlier spans")

	spanCount, exists := summarySpan.Attributes().Get("aggregation.span_count")
	require.True(t, exists)
	assert.Equal(t, int64(8), spanCount.Int(), "summary should aggregate only normal spans")

	outlierCount, exists := summarySpan.Attributes().Get("aggregation.preserved_outlier_count")
	require.True(t, exists)
	assert.Equal(t, int64(2), outlierCount.Int())

	summarySpanIDStr := summarySpan.SpanID().String()
	for _, outlier := range outlierSpans {
		summaryRef, hasSummaryRef := outlier.Attributes().Get("aggregation.summary_span_id")
		require.True(t, hasSummaryRef)
		assert.Equal(t, summarySpanIDStr, summaryRef.Str())
	}
}

// TestProcessorSkipsAggregationWhenTooFewNormalSpans tests that aggregation is skipped
// when preserving outliers would leave too few normal spans to aggregate.
func TestProcessorSkipsAggregationWhenTooFewNormalSpans(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 11
	cfg.GroupByAttributes = []string{"db.operation"}
	cfg.EnableOutlierAnalysis = true
	cfg.OutlierAnalysis.Method = OutlierMethodIQR
	cfg.OutlierAnalysis.PreserveOutliers = true
	cfg.OutlierAnalysis.MaxPreservedOutliers = 0 // preserve all outliers
	cfg.OutlierAnalysis.IQRMultiplier = 1.5
	cfg.OutlierAnalysis.MinGroupSize = 7
	cfg.OutlierAnalysis.CorrelationMinOccurrence = 0.5
	cfg.OutlierAnalysis.CorrelationMaxNormalOccurrence = 0.5
	cfg.OutlierAnalysis.MaxCorrelatedAttributes = 5

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// 13 leaf spans: 10 normal + 3 outliers. After preserving outliers,
	// only 10 normal spans remain (< MinSpansToAggregate=11), so aggregation is skipped.
	td := createTestTraceWithManyOutliers(t)
	initialSpanCount := countSpans(td)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	assert.Equal(t, initialSpanCount, countSpans(td), "no spans should be aggregated")
	_, found := findSummarySpanByName(td, "SELECT")
	assert.False(t, found, "summary span should not be created when normal spans drop below threshold")
}

// Helper functions

func assertHistogramRecordedOnceWithSum(t *testing.T, tel *componenttest.Telemetry, metricName string) {
	t.Helper()
	hist := requireInt64HistogramMetric(t, tel, metricName)
	require.Len(t, hist.DataPoints, 1, "expected a single data point for %s", metricName)
	assert.Equal(t, uint64(1), hist.DataPoints[0].Count)
	assert.Equal(t, int64(1), hist.DataPoints[0].Sum)
}

func assertHistogramHasTraceExemplar(t *testing.T, tel *componenttest.Telemetry, metricName string, expectedTraceID pcommon.TraceID) {
	t.Helper()
	hist := requireInt64HistogramMetric(t, tel, metricName)
	require.NotEmpty(t, hist.DataPoints)
	require.NotEmpty(t, hist.DataPoints[0].Exemplars, "expected exemplar on %s", metricName)
	exemplar := hist.DataPoints[0].Exemplars[0]
	assert.Equal(t, expectedTraceID[:], exemplar.TraceID)
	assert.Len(t, exemplar.SpanID, 8)
}

func requireInt64HistogramMetric(t *testing.T, tel *componenttest.Telemetry, metricName string) metricdata.Histogram[int64] {
	t.Helper()
	metric, err := tel.GetMetric(metricName)
	require.NoError(t, err)

	hist, ok := metric.Data.(metricdata.Histogram[int64])
	require.True(t, ok, "metric %s should be an int64 histogram", metricName)
	return hist
}

func createTestTraceWithLeafSpans(t *testing.T, numLeafSpans int, spanName string, attrs map[string]string) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Create parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// Create leaf spans
	for i := range numLeafSpans {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName(spanName)
		span.SetStartTimestamp(pcommon.Timestamp(1000000000 + int64(i)*100))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100 + int64(i)*100))
		for k, v := range attrs {
			span.Attributes().PutStr(k, v)
		}
	}

	return td
}

func createTestTraceWithOutliers(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("handler")
	parentSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	parentSpan.SetEndTimestamp(pcommon.Timestamp(1001000000))

	baseTime := int64(1000000000)
	ms := int64(1000000)

	normalDurations := []int64{5, 6, 7, 8, 9, 10, 11, 12}
	outlierDurations := []int64{500, 600}

	for i, dur := range normalDurations {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(baseTime))
		span.SetEndTimestamp(pcommon.Timestamp(baseTime + dur*ms))
		span.Attributes().PutStr("db.operation", "SELECT")
		span.Attributes().PutStr("cache_hit", "true")
	}

	for i, dur := range outlierDurations {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(baseTime))
		span.SetEndTimestamp(pcommon.Timestamp(baseTime + dur*ms))
		span.Attributes().PutStr("db.operation", "SELECT")
		span.Attributes().PutStr("cache_hit", "false")
	}

	return td
}

func createTestTraceWithManyOutliers(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("handler")
	parentSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	parentSpan.SetEndTimestamp(pcommon.Timestamp(1001000000))

	baseTime := int64(1000000000)
	ms := int64(1000000)

	normalDurations := []int64{5, 6, 6, 7, 7, 8, 8, 9, 9, 10}
	for i, dur := range normalDurations {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(baseTime))
		span.SetEndTimestamp(pcommon.Timestamp(baseTime + dur*ms))
		span.Attributes().PutStr("db.operation", "SELECT")
	}

	outlierDurations := []int64{500, 600, 700}
	for i, dur := range outlierDurations {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(baseTime))
		span.SetEndTimestamp(pcommon.Timestamp(baseTime + dur*ms))
		span.Attributes().PutStr("db.operation", "SELECT")
	}

	return td
}

func createTestTraceWithAttributeLoss(t *testing.T) (ptrace.Traces, pcommon.TraceID) {
	t.Helper()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{5, 4, 3, 2, 1, 9, 8, 7, 6, 0, 1, 2, 3, 4, 5, 6})
	rootID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	handler1ID := pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0})
	handler2ID := pcommon.SpanID([8]byte{3, 0, 0, 0, 0, 0, 0, 0})

	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootID)
	root.SetName("root")
	root.SetStartTimestamp(pcommon.Timestamp(1000))
	root.SetEndTimestamp(pcommon.Timestamp(3000))

	handler1 := ss.Spans().AppendEmpty()
	handler1.SetTraceID(traceID)
	handler1.SetSpanID(handler1ID)
	handler1.SetParentSpanID(rootID)
	handler1.SetName("handler")
	handler1.Attributes().PutStr("service.instance.id", "svc-a")
	handler1.Attributes().PutStr("handler.version", "v1")
	handler1.SetStartTimestamp(pcommon.Timestamp(1100))
	handler1.SetEndTimestamp(pcommon.Timestamp(1600))

	handler2 := ss.Spans().AppendEmpty()
	handler2.SetTraceID(traceID)
	handler2.SetSpanID(handler2ID)
	handler2.SetParentSpanID(rootID)
	handler2.SetName("handler")
	handler2.Attributes().PutStr("service.instance.id", "svc-b")
	handler2.SetStartTimestamp(pcommon.Timestamp(1200))
	handler2.SetEndTimestamp(pcommon.Timestamp(2100))

	leafConfigs := []struct {
		spanID      pcommon.SpanID
		parentID    pcommon.SpanID
		dbSystem    string
		dbInstance  string
		start, stop pcommon.Timestamp
	}{
		{
			spanID:     pcommon.SpanID([8]byte{4, 0, 0, 0, 0, 0, 0, 0}),
			parentID:   handler1ID,
			dbSystem:   "postgres",
			dbInstance: "orders-db",
			start:      pcommon.Timestamp(1200),
			stop:       pcommon.Timestamp(1350),
		},
		{
			spanID:   pcommon.SpanID([8]byte{4, 1, 0, 0, 0, 0, 0, 0}),
			parentID: handler1ID,
			dbSystem: "mysql",
			start:    pcommon.Timestamp(1250),
			stop:     pcommon.Timestamp(1400),
		},
		{
			spanID:   pcommon.SpanID([8]byte{5, 0, 0, 0, 0, 0, 0, 0}),
			parentID: handler2ID,
			dbSystem: "postgres",
			start:    pcommon.Timestamp(1300),
			stop:     pcommon.Timestamp(1900),
		},
		{
			spanID:     pcommon.SpanID([8]byte{5, 1, 0, 0, 0, 0, 0, 0}),
			parentID:   handler2ID,
			dbSystem:   "mysql",
			dbInstance: "users-db",
			start:      pcommon.Timestamp(1350),
			stop:       pcommon.Timestamp(1500),
		},
	}

	for _, cfg := range leafConfigs {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(cfg.spanID)
		span.SetParentSpanID(cfg.parentID)
		span.SetName("SELECT")
		span.Attributes().PutStr("db.system", cfg.dbSystem)
		if cfg.dbInstance != "" {
			span.Attributes().PutStr("db.instance", cfg.dbInstance)
		}
		span.SetStartTimestamp(cfg.start)
		span.SetEndTimestamp(cfg.stop)
	}

	return td, traceID
}

func createTestTraceWithNonStringAttributes(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	spanConfigs := []struct {
		spanID  pcommon.SpanID
		retries int64
		cached  bool
	}{
		{pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0}), 1, true},
		{pcommon.SpanID([8]byte{2, 1, 0, 0, 0, 0, 0, 0}), 1, true},
		{pcommon.SpanID([8]byte{3, 0, 0, 0, 0, 0, 0, 0}), 2, true},
		{pcommon.SpanID([8]byte{3, 1, 0, 0, 0, 0, 0, 0}), 2, true},
	}

	for i, cfg := range spanConfigs {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(cfg.spanID)
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutInt("db.retries", cfg.retries)
		span.Attributes().PutBool("db.cached", cfg.cached)
		span.SetStartTimestamp(pcommon.Timestamp(1000000000 + int64(i)*100))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100 + int64(i)*100))
	}

	return td
}

func createTestTraceWithTemplateEventsAndLinks(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// Template span with longest duration
	templateSpan := ss.Spans().AppendEmpty()
	templateSpan.SetTraceID(traceID)
	templateSpan.SetSpanID(pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0}))
	templateSpan.SetParentSpanID(parentSpanID)
	templateSpan.SetName("SELECT")
	templateSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	templateSpan.SetEndTimestamp(pcommon.Timestamp(1000000500))
	templateSpan.Attributes().PutStr("db.operation", "select")

	templateEvent := templateSpan.Events().AppendEmpty()
	templateEvent.SetName("template_event")
	templateEvent.Attributes().PutStr("event.attr", "value")

	templateLink := templateSpan.Links().AppendEmpty()
	templateLink.SetTraceID(pcommon.TraceID([16]byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9}))
	templateLink.SetSpanID(pcommon.SpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9}))
	templateLink.Attributes().PutStr("link.kind", "template")

	// Shorter span without events or links
	otherSpan := ss.Spans().AppendEmpty()
	otherSpan.SetTraceID(traceID)
	otherSpan.SetSpanID(pcommon.SpanID([8]byte{2, 1, 0, 0, 0, 0, 0, 0}))
	otherSpan.SetParentSpanID(parentSpanID)
	otherSpan.SetName("SELECT")
	otherSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	otherSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
	otherSpan.Attributes().PutStr("db.operation", "select")

	return td
}

func createTestTraceWithIntermediateSpan(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	intermediateSpanID := pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0})

	// Root span
	rootSpan := ss.Spans().AppendEmpty()
	rootSpan.SetTraceID(traceID)
	rootSpan.SetSpanID(rootSpanID)
	rootSpan.SetName("root")

	// Intermediate span (child of root, parent of leaves)
	intermediateSpan := ss.Spans().AppendEmpty()
	intermediateSpan.SetTraceID(traceID)
	intermediateSpan.SetSpanID(intermediateSpanID)
	intermediateSpan.SetParentSpanID(rootSpanID)
	intermediateSpan.SetName("intermediate")

	// 3 leaf spans (children of intermediate)
	for i := range 3 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(intermediateSpanID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createTestTraceWithMixedOperations(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 3 SELECT spans
	for i := range 3 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "select")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2 INSERT spans
	for i := range 2 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "insert")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createSingleSpanTrace(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0}))
	span.SetName("root")

	return td
}

func createTestTraceWithErrorSpan(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// Leaf span with OK status
	span1 := ss.Spans().AppendEmpty()
	span1.SetTraceID(traceID)
	span1.SetSpanID(pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0}))
	span1.SetParentSpanID(parentSpanID)
	span1.SetName("SELECT")
	span1.Status().SetCode(ptrace.StatusCodeOk)
	span1.SetStartTimestamp(pcommon.Timestamp(1000000000))
	span1.SetEndTimestamp(pcommon.Timestamp(1000000100))

	// Leaf span with Error status
	span2 := ss.Spans().AppendEmpty()
	span2.SetTraceID(traceID)
	span2.SetSpanID(pcommon.SpanID([8]byte{2, 1, 0, 0, 0, 0, 0, 0}))
	span2.SetParentSpanID(parentSpanID)
	span2.SetName("SELECT")
	span2.Status().SetCode(ptrace.StatusCodeError)
	span2.Status().SetMessage("query failed")
	span2.SetStartTimestamp(pcommon.Timestamp(1000000000))
	span2.SetEndTimestamp(pcommon.Timestamp(1000000100))

	return td
}

func createTestTraceWithKnownDurations(t *testing.T, durationsNs []int64) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// Leaf spans with specific durations
	baseTime := int64(1000000000)
	for i, duration := range durationsNs {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(baseTime))
		span.SetEndTimestamp(pcommon.Timestamp(baseTime + duration))
	}

	return td
}

func countSpans(td ptrace.Traces) int {
	count := 0
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			count += ilss.At(j).Spans().Len()
		}
	}
	return count
}

// findSummarySpan finds the first summary span (with is_summary attribute set to true)
func findSummarySpan(td ptrace.Traces) (ptrace.Span, bool) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				isSummary, exists := span.Attributes().Get("aggregation.is_summary")
				if exists && isSummary.Bool() {
					return span, true
				}
			}
		}
	}
	return ptrace.Span{}, false
}

func findSpanByNameAndStatus(td ptrace.Traces, spanName string, statusCode ptrace.StatusCode) (ptrace.Span, bool) {
	// findSpanByNameAndStatus finds a summary span by exact name and status code
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				isSummary, exists := span.Attributes().Get("aggregation.is_summary")
				if exists && isSummary.Bool() && span.Name() == spanName && span.Status().Code() == statusCode {
					return span, true
				}
			}
		}
	}
	return ptrace.Span{}, false
}

func createTestTraceWithMixedStatusSpans(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 4 leaf spans with OK status
	for i := range 4 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2 leaf spans with Error status
	for i := range 2 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.Status().SetCode(ptrace.StatusCodeError)
		span.Status().SetMessage("query failed")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

// Glob pattern matching tests

func TestLeafSpanPruning_GlobPatternWildcard(t *testing.T) {
	// Test: "db.*" pattern matches db.operation, db.name, db.statement
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.*"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with spans having multiple db.* attributes
	td := createTestTraceWithMultipleDbAttrs(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 4, originalSpanCount) // 1 parent + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// All 3 leaf spans have same db.* values, should aggregate to 1
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount) // 1 parent + 1 summary

	summarySpan, found := findSummarySpan(td)
	require.True(t, found)

	attrs := summarySpan.Attributes()
	spanCount, _ := attrs.Get("aggregation.span_count")
	assert.Equal(t, int64(3), spanCount.Int())
}

func TestLeafSpanPruning_GlobPatternSeparatesGroups(t *testing.T) {
	// Test: spans with different db.* values should be in separate groups
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.*"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with spans having different db.operation values
	td := createTestTraceWithDifferentDbOperations(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 5, originalSpanCount) // 1 parent + 2 select + 2 insert

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// 2 select spans -> 1 summary, 2 insert spans -> 1 summary
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount) // 1 parent + 2 summaries
}

func TestLeafSpanPruning_GlobPatternMultiplePatterns(t *testing.T) {
	// Test: multiple glob patterns ["db.*", "http.*"]
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.*", "http.*"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithDbAndHTTPAttrs(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 4, originalSpanCount) // 1 parent + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// All spans have same db.* and http.* values, should aggregate
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)
}

func TestLeafSpanPruning_GlobPatternExactMatch(t *testing.T) {
	// Test: pattern without wildcard "db.operation" matches exactly
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.operation"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithMultipleDbAttrs(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Should still group by db.operation exactly
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)
}

func TestLeafSpanPruning_InvalidGlobPattern(t *testing.T) {
	// Test: invalid glob pattern should return error during creation
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GroupByAttributes = []string{"[invalid"}

	_, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid glob pattern")
}

// Helper functions for glob pattern tests

func createTestTraceWithMultipleDbAttrs(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 3 leaf spans with identical db.* attributes
	for i := range 3 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "select")
		span.Attributes().PutStr("db.name", "users")
		span.Attributes().PutStr("db.system", "postgresql")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createTestTraceWithDifferentDbOperations(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 2 SELECT spans
	for i := range 2 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "select")
		span.Attributes().PutStr("db.name", "users")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2 INSERT spans
	for i := range 2 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "insert")
		span.Attributes().PutStr("db.name", "users")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createTestTraceWithDbAndHTTPAttrs(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 3 leaf spans with both db.* and http.* attributes
	for i := range 3 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("api_call")
		span.Attributes().PutStr("db.operation", "select")
		span.Attributes().PutStr("db.name", "users")
		span.Attributes().PutStr("http.method", "GET")
		span.Attributes().PutStr("http.route", "/api/users")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

// TestLeafSpanPruning_RecursiveParentAggregation tests that parent spans are aggregated
// when all their children are aggregated
func TestLeafSpanPruning_RecursiveParentAggregation(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.op"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create the complex trace from the plan example
	td := createTestTraceWithRecursiveAggregation(t)

	// Before: 1 root + 3 OK handlers + 3 OK SELECTs + 2 Error handlers + 2 Error SELECTs + 1 OK handler + 1 INSERT + 1 worker + 1 SELECT = 15 spans
	originalSpanCount := countSpans(td)
	assert.Equal(t, 15, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 root + 1 OK handler_aggregated + 1 OK SELECT_aggregated + 1 Error handler_aggregated + 1 Error SELECT_aggregated + 1 OK handler + 1 INSERT + 1 worker + 1 SELECT = 9 spans
	finalSpanCount := countSpans(td)
	assert.Equal(t, 9, finalSpanCount)

	// Verify aggregated spans exist
	handlerOKAgg, found := findSpanByName(td, "handler", "Ok")
	require.True(t, found, "OK handler summary should exist")

	handlerErrorAgg, found := findSpanByName(td, "handler", "Error")
	require.True(t, found, "Error handler summary should exist")

	selectOKAgg, found := findSpanByName(td, "SELECT", "Ok")
	require.True(t, found, "OK SELECT summary should exist")

	selectErrorAgg, found := findSpanByName(td, "SELECT", "Error")
	require.True(t, found, "Error SELECT summary should exist")

	// Verify span counts
	handlerOKCount, _ := handlerOKAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(3), handlerOKCount.Int())

	handlerErrorCount, _ := handlerErrorAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(2), handlerErrorCount.Int())

	selectOKCount, _ := selectOKAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(3), selectOKCount.Int())

	selectErrorCount, _ := selectErrorAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(2), selectErrorCount.Int())

	// Verify parent-child relationships
	// SELECT_aggregated (OK) should be child of handler_aggregated (OK)
	assert.Equal(t, handlerOKAgg.SpanID(), selectOKAgg.ParentSpanID())

	// SELECT_aggregated (Error) should be child of handler_aggregated (Error)
	assert.Equal(t, handlerErrorAgg.SpanID(), selectErrorAgg.ParentSpanID())

	// Verify non-aggregated spans still exist
	foundInsert := findSpanByExactName(td, "INSERT")
	require.True(t, foundInsert, "INSERT span should still exist")

	foundWorker := findSpanByExactName(td, "worker")
	require.True(t, foundWorker, "worker span should still exist")
}

// TestLeafSpanPruning_ParentNotAggregatedIfChildrenMixed tests that parents are not
// aggregated if some children are aggregated but others are not
func TestLeafSpanPruning_ParentNotAggregatedIfChildrenMixed(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithMixedChildren(t)

	// Before: 1 root + 2 handlers + 3 SELECTs + 1 INSERT = 7 spans
	originalSpanCount := countSpans(td)
	assert.Equal(t, 7, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 root + 2 handlers + 1 SELECT_aggregated + 1 INSERT = 5 spans
	// Handlers should NOT be aggregated because one has a non-aggregated child (INSERT)
	finalSpanCount := countSpans(td)
	assert.Equal(t, 5, finalSpanCount)

	// Verify handler_aggregated does NOT exist
	_, found := findSummarySpanByName(td, "handler")
	assert.False(t, found, "handler summary should NOT exist")

	// Verify SELECT_aggregated exists
	selectAgg, found := findSummarySpanByName(td, "SELECT")
	require.True(t, found, "SELECT summary should exist")

	selectCount, _ := selectAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(3), selectCount.Int())

	// Verify original handler spans still exist
	handlers := findAllSpansByExactName(td, "handler")
	assert.Len(t, handlers, 2, "both handler spans should still exist")
}

// TestLeafSpanPruning_RootSpansNotAggregated tests that root spans (with no parent)
// are never aggregated
func TestLeafSpanPruning_RootSpansNotAggregated(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithMultipleRoots(t)

	// Before: 3 root spans + 6 leaf spans (2 per root) = 9 spans
	originalSpanCount := countSpans(td)
	assert.Equal(t, 9, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 3 root spans + 1 SELECT_aggregated = 4 spans
	// Root spans should NOT be aggregated even though all children are aggregated
	finalSpanCount := countSpans(td)
	assert.Equal(t, 4, finalSpanCount)

	// Verify all root spans still exist
	roots := findAllSpansByExactName(td, "root")
	assert.Len(t, roots, 3, "all root spans should still exist")

	// Verify SELECT_aggregated exists
	selectAgg, found := findSummarySpanByName(td, "SELECT")
	require.True(t, found, "SELECT summary should exist")

	selectCount, _ := selectAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(6), selectCount.Int())
}

// TestLeafSpanPruning_ThreeLevelAggregation tests aggregation across three levels
func TestLeafSpanPruning_ThreeLevelAggregation(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.MaxParentDepth = -1 // unlimited for this test

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithThreeLevels(t)

	// Before: 1 root + 2 middleware + 4 handlers (2 per middleware) + 8 SELECTs (2 per handler) = 15 spans
	originalSpanCount := countSpans(td)
	assert.Equal(t, 15, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 root + 1 middleware_aggregated + 1 handler_aggregated + 1 SELECT_aggregated = 4 spans
	finalSpanCount := countSpans(td)
	assert.Equal(t, 4, finalSpanCount)

	// Verify all aggregated spans exist
	middlewareAgg, found := findSummarySpanByName(td, "middleware")
	require.True(t, found, "middleware summary should exist")

	handlerAgg, found := findSummarySpanByName(td, "handler")
	require.True(t, found, "handler summary should exist")

	selectAgg, found := findSummarySpanByName(td, "SELECT")
	require.True(t, found, "SELECT summary should exist")

	// Verify parent-child relationships
	// handler summary should be child of middleware summary
	assert.Equal(t, middlewareAgg.SpanID(), handlerAgg.ParentSpanID())

	// SELECT summary should be child of handler summary
	assert.Equal(t, handlerAgg.SpanID(), selectAgg.ParentSpanID())

	// Verify span counts
	middlewareCount, _ := middlewareAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(2), middlewareCount.Int())

	handlerCount, _ := handlerAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(4), handlerCount.Int())

	selectCount, _ := selectAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(8), selectCount.Int())
}

// Helper functions for new tests

func createTestTraceWithRecursiveAggregation(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootSpanID)
	root.SetName("root")
	root.Status().SetCode(ptrace.StatusCodeOk)

	// 3x handler (OK) -> SELECT (OK, db.op=select)
	for i := range 3 {
		handlerID := pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0})
		handler := ss.Spans().AppendEmpty()
		handler.SetTraceID(traceID)
		handler.SetSpanID(handlerID)
		handler.SetParentSpanID(rootSpanID)
		handler.SetName("handler")
		handler.Status().SetCode(ptrace.StatusCodeOk)

		selectSpan := ss.Spans().AppendEmpty()
		selectSpan.SetTraceID(traceID)
		selectSpan.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		selectSpan.SetParentSpanID(handlerID)
		selectSpan.SetName("SELECT")
		selectSpan.Attributes().PutStr("db.op", "select")
		selectSpan.Status().SetCode(ptrace.StatusCodeOk)
		selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
		selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2x handler (Error) -> SELECT (Error, db.op=select)
	for i := range 2 {
		handlerID := pcommon.SpanID([8]byte{4, byte(i), 0, 0, 0, 0, 0, 0})
		handler := ss.Spans().AppendEmpty()
		handler.SetTraceID(traceID)
		handler.SetSpanID(handlerID)
		handler.SetParentSpanID(rootSpanID)
		handler.SetName("handler")
		handler.Status().SetCode(ptrace.StatusCodeError)

		selectSpan := ss.Spans().AppendEmpty()
		selectSpan.SetTraceID(traceID)
		selectSpan.SetSpanID(pcommon.SpanID([8]byte{5, byte(i), 0, 0, 0, 0, 0, 0}))
		selectSpan.SetParentSpanID(handlerID)
		selectSpan.SetName("SELECT")
		selectSpan.Attributes().PutStr("db.op", "select")
		selectSpan.Status().SetCode(ptrace.StatusCodeError)
		selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
		selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 1x handler (OK) -> INSERT (OK, db.op=insert) - below threshold
	handlerID := pcommon.SpanID([8]byte{6, 0, 0, 0, 0, 0, 0, 0})
	handler := ss.Spans().AppendEmpty()
	handler.SetTraceID(traceID)
	handler.SetSpanID(handlerID)
	handler.SetParentSpanID(rootSpanID)
	handler.SetName("handler")
	handler.Status().SetCode(ptrace.StatusCodeOk)

	insertSpan := ss.Spans().AppendEmpty()
	insertSpan.SetTraceID(traceID)
	insertSpan.SetSpanID(pcommon.SpanID([8]byte{7, 0, 0, 0, 0, 0, 0, 0}))
	insertSpan.SetParentSpanID(handlerID)
	insertSpan.SetName("INSERT")
	insertSpan.Attributes().PutStr("db.op", "insert")
	insertSpan.Status().SetCode(ptrace.StatusCodeOk)
	insertSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	insertSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))

	// 1x worker (OK) -> SELECT (OK, db.op=select) - different parent name
	workerID := pcommon.SpanID([8]byte{8, 0, 0, 0, 0, 0, 0, 0})
	worker := ss.Spans().AppendEmpty()
	worker.SetTraceID(traceID)
	worker.SetSpanID(workerID)
	worker.SetParentSpanID(rootSpanID)
	worker.SetName("worker")
	worker.Status().SetCode(ptrace.StatusCodeOk)

	selectSpan := ss.Spans().AppendEmpty()
	selectSpan.SetTraceID(traceID)
	selectSpan.SetSpanID(pcommon.SpanID([8]byte{9, 0, 0, 0, 0, 0, 0, 0}))
	selectSpan.SetParentSpanID(workerID)
	selectSpan.SetName("SELECT")
	selectSpan.Attributes().PutStr("db.op", "select")
	selectSpan.Status().SetCode(ptrace.StatusCodeOk)
	selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))

	return td
}

func createTestTraceWithMixedChildren(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootSpanID)
	root.SetName("root")

	// Handler 1 with 3 SELECTs (will be aggregated)
	handler1ID := pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0})
	handler1 := ss.Spans().AppendEmpty()
	handler1.SetTraceID(traceID)
	handler1.SetSpanID(handler1ID)
	handler1.SetParentSpanID(rootSpanID)
	handler1.SetName("handler")
	handler1.Status().SetCode(ptrace.StatusCodeOk)

	for i := range 3 {
		selectSpan := ss.Spans().AppendEmpty()
		selectSpan.SetTraceID(traceID)
		selectSpan.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		selectSpan.SetParentSpanID(handler1ID)
		selectSpan.SetName("SELECT")
		selectSpan.Status().SetCode(ptrace.StatusCodeOk)
		selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
		selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// Handler 2 with 1 INSERT (not aggregated - mixed children)
	handler2ID := pcommon.SpanID([8]byte{4, 0, 0, 0, 0, 0, 0, 0})
	handler2 := ss.Spans().AppendEmpty()
	handler2.SetTraceID(traceID)
	handler2.SetSpanID(handler2ID)
	handler2.SetParentSpanID(rootSpanID)
	handler2.SetName("handler")
	handler2.Status().SetCode(ptrace.StatusCodeOk)

	insertSpan := ss.Spans().AppendEmpty()
	insertSpan.SetTraceID(traceID)
	insertSpan.SetSpanID(pcommon.SpanID([8]byte{5, 0, 0, 0, 0, 0, 0, 0}))
	insertSpan.SetParentSpanID(handler2ID)
	insertSpan.SetName("INSERT")
	insertSpan.Status().SetCode(ptrace.StatusCodeOk)
	insertSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	insertSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))

	return td
}

func createTestTraceWithMultipleRoots(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	// 3 root spans, each with 2 SELECT children
	for i := range 3 {
		rootID := pcommon.SpanID([8]byte{byte(i + 1), 0, 0, 0, 0, 0, 0, 0})
		root := ss.Spans().AppendEmpty()
		root.SetTraceID(traceID)
		root.SetSpanID(rootID)
		root.SetName("root")
		root.Status().SetCode(ptrace.StatusCodeOk)

		for j := range 2 {
			selectSpan := ss.Spans().AppendEmpty()
			selectSpan.SetTraceID(traceID)
			selectSpan.SetSpanID(pcommon.SpanID([8]byte{byte(i + 4), byte(j), 0, 0, 0, 0, 0, 0}))
			selectSpan.SetParentSpanID(rootID)
			selectSpan.SetName("SELECT")
			selectSpan.Status().SetCode(ptrace.StatusCodeOk)
			selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
			selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
		}
	}

	return td
}

func createTestTraceWithThreeLevels(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootSpanID)
	root.SetName("root")

	spanIDCounter := byte(2)

	// 2 middleware spans
	for range 2 {
		middlewareID := pcommon.SpanID([8]byte{spanIDCounter, 0, 0, 0, 0, 0, 0, 0})
		spanIDCounter++

		middleware := ss.Spans().AppendEmpty()
		middleware.SetTraceID(traceID)
		middleware.SetSpanID(middlewareID)
		middleware.SetParentSpanID(rootSpanID)
		middleware.SetName("middleware")
		middleware.Status().SetCode(ptrace.StatusCodeOk)

		// Each middleware has 2 handler spans
		for range 2 {
			handlerID := pcommon.SpanID([8]byte{spanIDCounter, 0, 0, 0, 0, 0, 0, 0})
			spanIDCounter++

			handler := ss.Spans().AppendEmpty()
			handler.SetTraceID(traceID)
			handler.SetSpanID(handlerID)
			handler.SetParentSpanID(middlewareID)
			handler.SetName("handler")
			handler.Status().SetCode(ptrace.StatusCodeOk)

			// Each handler has 2 SELECT spans
			for range 2 {
				selectSpan := ss.Spans().AppendEmpty()
				selectSpan.SetTraceID(traceID)
				selectSpan.SetSpanID(pcommon.SpanID([8]byte{spanIDCounter, 0, 0, 0, 0, 0, 0, 0}))
				spanIDCounter++
				selectSpan.SetParentSpanID(handlerID)
				selectSpan.SetName("SELECT")
				selectSpan.Status().SetCode(ptrace.StatusCodeOk)
				selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
				selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
			}
		}
	}

	return td
}

func findSpanByName(td ptrace.Traces, nameSubstring, statusCode string) (ptrace.Span, bool) {
	// findSpanByName finds a summary span by name substring and status code string
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				isSummary, exists := span.Attributes().Get("aggregation.is_summary")
				if strings.Contains(span.Name(), nameSubstring) && span.Status().Code().String() == statusCode && exists && isSummary.Bool() {
					return span, true
				}
			}
		}
	}
	return ptrace.Span{}, false
}

func findSpanByExactName(td ptrace.Traces, name string) bool {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.Name() == name {
					return true
				}
			}
		}
	}
	return false
}

// findSummarySpanByName finds a summary span (with is_summary attribute) by exact name
func findSummarySpanByName(td ptrace.Traces, name string) (ptrace.Span, bool) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				isSummary, exists := span.Attributes().Get("aggregation.is_summary")
				if span.Name() == name && exists && isSummary.Bool() {
					return span, true
				}
			}
		}
	}
	return ptrace.Span{}, false
}

func findAllSpansByExactName(td ptrace.Traces, name string) []ptrace.Span {
	var result []ptrace.Span
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.Name() == name {
					result = append(result, span)
				}
			}
		}
	}
	return result
}

// TestLeafSpanPruning_LongestDurationTemplate tests that the span with the longest
// duration is used as the template for the summary span
func TestLeafSpanPruning_LongestDurationTemplate(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.operation"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with spans of varying durations and unique identifying attributes
	// The span with the longest duration should become the template
	td := createTestTraceWithVaryingDurations(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Find the summary span
	summarySpan, found := findSummarySpan(td)
	require.True(t, found, "summary span should exist")

	// Verify the template attribute from the longest-duration span is present
	// The span with 500ns duration should be the template
	identifier, exists := summarySpan.Attributes().Get("span.identifier")
	require.True(t, exists, "span.identifier attribute should exist")
	assert.Equal(t, "longest", identifier.Str(), "summary should use attributes from longest-duration span")

	// Verify duration stats
	attrs := summarySpan.Attributes()
	minDuration, _ := attrs.Get("aggregation.duration_min_ns")
	assert.Equal(t, int64(100), minDuration.Int())

	maxDuration, _ := attrs.Get("aggregation.duration_max_ns")
	assert.Equal(t, int64(500), maxDuration.Int())
}

func createTestTraceWithVaryingDurations(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// Define spans with different durations and unique identifiers
	// Duration order: 100ns, 500ns, 200ns - the 500ns span should be template
	spanConfigs := []struct {
		duration   int64
		identifier string
	}{
		{100, "short"},
		{500, "longest"}, // This one should be the template
		{200, "medium"},
	}

	baseTime := int64(1000000000)
	for i, cfg := range spanConfigs {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.SetStartTimestamp(pcommon.Timestamp(baseTime))
		span.SetEndTimestamp(pcommon.Timestamp(baseTime + cfg.duration))
		span.Attributes().PutStr("db.operation", "select")          // Grouping key - same for all
		span.Attributes().PutStr("span.identifier", cfg.identifier) // Unique per span
	}

	return td
}

// TraceState grouping tests for Consistent Probability Sampling (CPS) compatibility

func TestLeafSpanPruning_TraceStateGrouping_SameTraceState(t *testing.T) {
	// Test: spans with identical TraceState should be aggregated together
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithSameTraceState(t, "ot=th:fd70a4;rv:12345")
	originalSpanCount := countSpans(td)
	assert.Equal(t, 4, originalSpanCount) // 1 parent + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// All 3 leaf spans have same TraceState, should aggregate to 1
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount) // 1 parent + 1 summary

	summarySpan, found := findSummarySpan(td)
	require.True(t, found)

	attrs := summarySpan.Attributes()
	spanCount, _ := attrs.Get("aggregation.span_count")
	assert.Equal(t, int64(3), spanCount.Int())

	// Verify TraceState is preserved in summary span
	assert.Equal(t, "ot=th:fd70a4;rv:12345", summarySpan.TraceState().AsRaw())
}

func TestLeafSpanPruning_TraceStateGrouping_DifferentThresholds(t *testing.T) {
	// Test: spans with different th (threshold) values should be in separate groups
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithDifferentTraceStates(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 6, originalSpanCount) // 1 parent + 3 spans (th:fd70a4) + 2 spans (th:fa00)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// 3 spans with th:fd70a4 -> 1 summary
	// 2 spans with th:fa00 -> 1 summary
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount) // 1 parent + 2 summaries

	// Verify we have two summary spans with different TraceState values
	summaries := findAllSummarySpans(td)
	assert.Len(t, summaries, 2, "should have 2 summary spans")

	// Collect TraceState values from summaries
	traceStates := make(map[string]int64)
	for _, summary := range summaries {
		ts := summary.TraceState().AsRaw()
		count, _ := summary.Attributes().Get("aggregation.span_count")
		traceStates[ts] = count.Int()
	}

	assert.Equal(t, int64(3), traceStates["ot=th:fd70a4;rv:12345"], "th:fd70a4 group should have 3 spans")
	assert.Equal(t, int64(2), traceStates["ot=th:fa00;rv:12345"], "th:fa00 group should have 2 spans")
}

func TestLeafSpanPruning_TraceStateGrouping_MixedWithEmpty(t *testing.T) {
	// Test: spans with TraceState and spans without should be in separate groups
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithMixedTraceState(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 6, originalSpanCount) // 1 parent + 3 spans (with TraceState) + 2 spans (empty)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// 3 spans with TraceState -> 1 summary
	// 2 spans with empty TraceState -> 1 summary
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount) // 1 parent + 2 summaries

	summaries := findAllSummarySpans(td)
	assert.Len(t, summaries, 2, "should have 2 summary spans")

	// Verify TraceState values are preserved correctly
	var withTS, withoutTS ptrace.Span
	for _, s := range summaries {
		if s.TraceState().AsRaw() == "" {
			withoutTS = s
		} else {
			withTS = s
		}
	}

	assert.Equal(t, "ot=th:fd70a4;rv:12345", withTS.TraceState().AsRaw())
	count1, _ := withTS.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(3), count1.Int())

	assert.Empty(t, withoutTS.TraceState().AsRaw())
	count2, _ := withoutTS.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(2), count2.Int())
}

func TestLeafSpanPruning_TraceStateGrouping_EmptyTraceState(t *testing.T) {
	// Test: spans with empty TraceState should be grouped together
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithSameTraceState(t, "") // Empty TraceState
	originalSpanCount := countSpans(td)
	assert.Equal(t, 4, originalSpanCount) // 1 parent + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// All 3 leaf spans have empty TraceState, should aggregate to 1
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)

	summarySpan, found := findSummarySpan(td)
	require.True(t, found)
	assert.Empty(t, summarySpan.TraceState().AsRaw())

	attrs := summarySpan.Attributes()
	spanCount, _ := attrs.Get("aggregation.span_count")
	assert.Equal(t, int64(3), spanCount.Int())
}

// TestParentKeyCollisionAcrossDepths verifies that when the same span name
// appears at multiple tree depths (causing buildParentGroupKey collisions),
// nodes from overwritten groups are not incorrectly removed. This is a
// regression test for a bug where the depth-2 parent group overwrites the
// depth-1 entry in aggregationGroups, leaving depth-1 nodes marked for
// removal but absent from the final plan.
//
// Trace structure (3 copies to meet MinSpansToAggregate=2 for parents):
//
//	root
//	├── svc [depth-2 parent candidate]
//	│   ├── svc [depth-1 parent candidate, SAME name as depth-2]
//	│   │   ├── SELECT (leaf)
//	│   │   └── SELECT (leaf)
//	│   └── svc
//	│       ├── SELECT (leaf)
//	│       └── SELECT (leaf)
//	├── svc
//	│   ├── svc
//	│   │   ├── SELECT (leaf)
//	│   │   └── SELECT (leaf)
//	│   └── svc
//	│       ├── SELECT (leaf)
//	│       └── SELECT (leaf)
//	└── svc
//	    ├── svc
//	    │   ├── SELECT (leaf)
//	    │   └── SELECT (leaf)
//	    └── svc
//	        ├── SELECT (leaf)
//	        └── SELECT (leaf)
//
// Expected result (22 spans → 4 spans):
//
//	root
//	├── summary(svc, aggregates 3 outer svc spans)
//	│   └── summary(svc, aggregates 6 inner svc spans)
//	│       └── summary(SELECT, aggregates 12 leaf spans)
func TestParentKeyCollisionAcrossDepths(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.MaxParentDepth = -1

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithParentKeyCollision(t)

	// Before: 1 root + 3 outer-svc + 6 inner-svc + 12 SELECT = 22 spans
	originalSpanCount := countSpans(td)
	assert.Equal(t, 22, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	finalSpanCount := countSpans(td)

	// Verify every non-summary span that remains is NOT an orphan: its parent
	// must either be another span in the output or it must be the root.
	remainingByID := make(map[pcommon.SpanID]ptrace.Span)
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				remainingByID[span.SpanID()] = span
			}
		}
	}
	for id, span := range remainingByID {
		parentID := span.ParentSpanID()
		if parentID.IsEmpty() {
			continue // root span
		}
		_, parentExists := remainingByID[parentID]
		assert.True(t, parentExists, "span %s (name=%s) has dangling parent %s — parent was removed but span was kept",
			id, span.Name(), parentID)
	}

	// With the bug, depth-1 "svc" nodes (6 spans) get incorrectly removed,
	// dropping the count too low. The expected result: root + 3 summaries.
	t.Logf("original=%d final=%d", originalSpanCount, finalSpanCount)
	assert.Equal(t, 4, finalSpanCount,
		"expected root + 3 summary spans (SELECT, inner svc, outer svc)")
}

func createTestTraceWithParentKeyCollision(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootSpanID)
	root.SetName("root")

	spanIDCounter := byte(2)
	nextID := func() pcommon.SpanID {
		id := pcommon.SpanID([8]byte{spanIDCounter, 0, 0, 0, 0, 0, 0, 0})
		spanIDCounter++
		return id
	}

	// 3 outer "svc" spans (same name/kind/status → same parent group key)
	for range 3 {
		outerID := nextID()
		outer := ss.Spans().AppendEmpty()
		outer.SetTraceID(traceID)
		outer.SetSpanID(outerID)
		outer.SetParentSpanID(rootSpanID)
		outer.SetName("svc")
		outer.Status().SetCode(ptrace.StatusCodeOk)

		// Each outer has 2 inner "svc" spans (same name → key collision with outer)
		for range 2 {
			innerID := nextID()
			inner := ss.Spans().AppendEmpty()
			inner.SetTraceID(traceID)
			inner.SetSpanID(innerID)
			inner.SetParentSpanID(outerID)
			inner.SetName("svc")
			inner.Status().SetCode(ptrace.StatusCodeOk)

			// Each inner has 2 leaf SELECT spans
			for range 2 {
				leaf := ss.Spans().AppendEmpty()
				leaf.SetTraceID(traceID)
				leaf.SetSpanID(nextID())
				leaf.SetParentSpanID(innerID)
				leaf.SetName("SELECT")
				leaf.Status().SetCode(ptrace.StatusCodeOk)
				leaf.SetStartTimestamp(pcommon.Timestamp(1000000000))
				leaf.SetEndTimestamp(pcommon.Timestamp(1000000100))
			}
		}
	}

	return td
}

// Helper functions for TraceState tests

func createTestTraceWithSameTraceState(t *testing.T, traceState string) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 3 leaf spans with identical TraceState
	for i := range 3 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.TraceState().FromRaw(traceState)
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createTestTraceWithDifferentTraceStates(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 3 leaf spans with th:fd70a4 (1% sampling)
	for i := range 3 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.TraceState().FromRaw("ot=th:fd70a4;rv:12345")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2 leaf spans with th:fa00 (2% sampling)
	for i := range 2 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.TraceState().FromRaw("ot=th:fa00;rv:12345")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createTestTraceWithMixedTraceState(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 3 leaf spans with TraceState
	for i := range 3 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.TraceState().FromRaw("ot=th:fd70a4;rv:12345")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2 leaf spans WITHOUT TraceState (empty)
	for i := range 2 {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		// No TraceState set (empty)
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func findAllSummarySpans(td ptrace.Traces) []ptrace.Span {
	var result []ptrace.Span
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				isSummary, exists := span.Attributes().Get("aggregation.is_summary")
				if exists && isSummary.Bool() {
					result = append(result, span)
				}
			}
		}
	}
	return result
}

// TestOutlierReparentingDeterministicAcrossDepths is a regression test for
// same-named parents at different depths. Previously the leaf group key used
// only the parent's name, so SELECT children of a "handler" at depth 1 and a
// "handler" at depth 2 collapsed into one group; the summary (and any preserved
// outliers) then anchored to whichever parent map iteration visited first,
// producing a non-deterministic outlier depth. With depth in the leaf key the
// two depths form separate groups, so the deep-handler outliers stay at depth 3
// on every run and carry provenance recording their original position.
func TestOutlierReparentingDeterministicAcrossDepths(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 5
	cfg.MaxParentDepth = -1
	cfg.EnableOutlierAnalysis = true
	cfg.OutlierAnalysis = OutlierAnalysisConfig{
		Method:                         OutlierMethodIQR,
		IQRMultiplier:                  1.5,
		MinGroupSize:                   7,
		PreserveOutliers:               true,
		MaxPreservedOutliers:           0,
		CorrelationMinOccurrence:       0.5,
		CorrelationMaxNormalOccurrence: 0.5,
		MaxCorrelatedAttributes:        5,
		MinOutlierThresholdPercent:     0.1,
	}

	deepHandlerID := pcommon.SpanID([8]byte{4, 0, 0, 0, 0, 0, 0, 0})

	observedDepths := make(map[int]bool)
	const iterations = 50
	for range iterations {
		tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)

		td := createTraceWithSameNamedParentsAtDifferentDepths(t)
		require.NoError(t, tp.ConsumeTraces(t.Context(), td))

		// The two depths must form separate groups (not one merged summary),
		// so we get one summary per handler.
		require.Len(t, findAllSummarySpans(td), 2, "shallow and deep handlers must aggregate separately")

		outliers := findPreservedOutlierSpans(td)
		require.Len(t, outliers, 2, "both deep-handler outliers should be preserved")

		for _, o := range outliers {
			observedDepths[spanDepthInTrace(td, o)] = true
			// Outliers stay anchored under their real parent (the deep handler).
			assert.Equal(t, deepHandlerID, o.ParentSpanID())
		}
	}

	assert.Equal(t, map[int]bool{3: true}, observedDepths,
		"preserved-outlier depth must be stable at 3 across runs; got %v", observedDepths)
}

// findPreservedOutlierSpans returns all spans tagged as preserved outliers.
func findPreservedOutlierSpans(td ptrace.Traces) []ptrace.Span {
	var out []ptrace.Span
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		for j := 0; j < rss.At(i).ScopeSpans().Len(); j++ {
			spans := rss.At(i).ScopeSpans().At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				s := spans.At(k)
				if v, ok := s.Attributes().Get("aggregation.is_preserved_outlier"); ok && v.Bool() {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// spanDepthInTrace counts ancestor hops from span to the root within td.
func spanDepthInTrace(td ptrace.Traces, span ptrace.Span) int {
	byID := make(map[pcommon.SpanID]ptrace.Span)
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		for j := 0; j < rss.At(i).ScopeSpans().Len(); j++ {
			spans := rss.At(i).ScopeSpans().At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				s := spans.At(k)
				byID[s.SpanID()] = s
			}
		}
	}
	depth := 0
	for current := span; ; {
		parentID := current.ParentSpanID()
		if parentID.IsEmpty() {
			break
		}
		parent, ok := byID[parentID]
		if !ok {
			break
		}
		depth++
		current = parent
	}
	return depth
}

// createTraceWithSameNamedParentsAtDifferentDepths builds a trace with two
// "handler" spans — one at depth 1 (under root) and one at depth 2 (under
// middleware). The deep handler has 7 normal + 2 outlier SELECT children so its
// leaf group alone satisfies MinGroupSize for outlier detection.
func createTraceWithSameNamedParentsAtDifferentDepths(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	shallowHandlerID := pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0})
	middlewareID := pcommon.SpanID([8]byte{3, 0, 0, 0, 0, 0, 0, 0})
	deepHandlerID := pcommon.SpanID([8]byte{4, 0, 0, 0, 0, 0, 0, 0})

	baseNs := int64(1_000_000_000)
	ms := int64(1_000_000)

	nextID := func() pcommon.SpanID {
		var id [8]byte
		id[0] = byte(ss.Spans().Len() + 10)
		return pcommon.SpanID(id)
	}

	addSpan := func(id, parentID pcommon.SpanID, name string, durationMs int64, cacheHit string) {
		s := ss.Spans().AppendEmpty()
		s.SetTraceID(traceID)
		s.SetSpanID(id)
		s.SetParentSpanID(parentID)
		s.SetName(name)
		s.SetStartTimestamp(pcommon.Timestamp(baseNs))
		s.SetEndTimestamp(pcommon.Timestamp(baseNs + durationMs*ms))
		if cacheHit != "" {
			s.Attributes().PutStr("cache_hit", cacheHit)
		}
	}

	addSpan(rootID, pcommon.SpanID{}, "root", 700, "")
	addSpan(shallowHandlerID, rootID, "handler", 10, "")     // depth 1
	addSpan(middlewareID, rootID, "middleware", 650, "")     // depth 1
	addSpan(deepHandlerID, middlewareID, "handler", 600, "") // depth 2

	// Shallow handler: 5 normal SELECTs (depth 2) — aggregates, no outliers.
	for _, dur := range []int64{5, 6, 7, 8, 9} {
		addSpan(nextID(), shallowHandlerID, "SELECT", dur, "true")
	}

	// Deep handler: 7 normal + 2 outlier SELECTs (depth 3).
	for _, dur := range []int64{5, 6, 7, 8, 9, 10, 11} {
		addSpan(nextID(), deepHandlerID, "SELECT", dur, "true")
	}
	for _, dur := range []int64{500, 600} {
		addSpan(nextID(), deepHandlerID, "SELECT", dur, "false")
	}

	return td
}
