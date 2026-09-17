// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

const (
	msNs      = int64(time.Millisecond)
	testStart = int64(1_000_000_000)
)

// summarySpec describes a summary span shaped like one an earlier run of this
// processor emitted, so tests can feed a prior run's output back in.
type summarySpec struct {
	spanID       pcommon.SpanID
	parentSpanID pcommon.SpanID
	name         string
	startNs      int64
	endNs        int64
	count        int64
	minNs        int64
	maxNs        int64
	totalNs      int64
	level        int64
	omitLevel    bool
	boundsS      []float64
	counts       []int64
	attrs        map[string]string
	// staleAttrs are extra `<prefix>*` attributes a previous run wrote that the
	// current run does not recompute.
	staleAttrs map[string]int64
}

// appendSummarySpan writes spec into ss as a summary span carrying the standard
// aggregation attributes.
func appendSummarySpan(ss ptrace.ScopeSpans, spec summarySpec) ptrace.Span {
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(testTraceID)
	span.SetSpanID(spec.spanID)
	span.SetParentSpanID(spec.parentSpanID)
	span.SetName(spec.name)
	span.SetStartTimestamp(pcommon.Timestamp(spec.startNs))
	span.SetEndTimestamp(pcommon.Timestamp(spec.endNs))

	for k, v := range spec.attrs {
		span.Attributes().PutStr(k, v)
	}

	attrs := span.Attributes()
	attrs.PutBool("aggregation.is_summary", true)
	attrs.PutInt("aggregation.span_count", spec.count)
	attrs.PutInt("aggregation.duration_min_ns", spec.minNs)
	attrs.PutInt("aggregation.duration_max_ns", spec.maxNs)
	attrs.PutInt("aggregation.duration_total_ns", spec.totalNs)
	if !spec.omitLevel {
		attrs.PutInt("aggregation.aggregation_level", spec.level)
	}
	if len(spec.boundsS) > 0 {
		bounds := attrs.PutEmptySlice("aggregation.histogram_bucket_bounds_s")
		for _, b := range spec.boundsS {
			bounds.AppendEmpty().SetDouble(b)
		}
		counts := attrs.PutEmptySlice("aggregation.histogram_bucket_counts")
		for _, c := range spec.counts {
			counts.AppendEmpty().SetInt(c)
		}
	}
	for k, v := range spec.staleAttrs {
		attrs.PutInt(k, v)
	}

	return span
}

// appendLeafSpan writes an ordinary leaf span of the given duration.
func appendLeafSpan(ss ptrace.ScopeSpans, spanID, parentSpanID pcommon.SpanID, name string, startNs, durationNs int64, attrs map[string]string) ptrace.Span {
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(testTraceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName(name)
	span.SetStartTimestamp(pcommon.Timestamp(startNs))
	span.SetEndTimestamp(pcommon.Timestamp(startNs + durationNs))
	for k, v := range attrs {
		span.Attributes().PutStr(k, v)
	}
	return span
}

func newSpanID(b byte) pcommon.SpanID {
	return pcommon.SpanID([8]byte{b, 0, 0, 0, 0, 0, 0, 0})
}

// newMergeTestTraces builds "parent" with one leaf-level summary (span_count 10,
// 1ms/5ms/30ms) plus numRaw new 2ms leaf spans that belong in the same group.
func newMergeTestTraces(numRaw int, summaryMutate func(*summarySpec)) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	parent := ss.Spans().AppendEmpty()
	parent.SetTraceID(testTraceID)
	parent.SetSpanID(newSpanID(1))
	parent.SetName("parent")

	spec := summarySpec{
		spanID:       newSpanID(9),
		parentSpanID: newSpanID(1),
		name:         "SELECT",
		startNs:      testStart,
		endNs:        testStart + 20*msNs,
		count:        10,
		minNs:        1 * msNs,
		maxNs:        5 * msNs,
		totalNs:      30 * msNs,
		attrs:        map[string]string{"db.operation": "select"},
		boundsS:      []float64{0.01, 0.05},
		counts:       []int64{4, 8, 10},
	}
	if summaryMutate != nil {
		summaryMutate(&spec)
	}
	appendSummarySpan(ss, spec)

	for i := range numRaw {
		appendLeafSpan(ss, newSpanID(byte(20+i)), newSpanID(1), "SELECT",
			testStart+int64(i)*msNs, 2*msNs, map[string]string{"db.operation": "select"})
	}

	return td
}

// mergeTestConfig returns a config with a small, easy-to-assert histogram.
func mergeTestConfig(t *testing.T, merge bool) *Config {
	t.Helper()
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.GroupByAttributes = []string{"db.operation"}
	cfg.MinSpansToAggregate = 5
	cfg.AggregationHistogramBuckets = []time.Duration{10 * time.Millisecond, 50 * time.Millisecond}
	cfg.MergeExistingSummaries = merge
	require.NoError(t, cfg.Validate())
	return cfg
}

func runProcessor(t *testing.T, cfg *Config, td ptrace.Traces) ptrace.Traces {
	t.Helper()
	tp, err := NewFactory().CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, tp.ConsumeTraces(t.Context(), td))
	return td
}

// intAttr returns an int attribute, failing the test when it is absent.
func intAttr(t *testing.T, span ptrace.Span, key string) int64 {
	t.Helper()
	v, ok := span.Attributes().Get(key)
	require.True(t, ok, "attribute %q should exist", key)
	return v.Int()
}

// int64Slice reads a slice attribute of ints.
func int64Slice(t *testing.T, span ptrace.Span, key string) []int64 {
	t.Helper()
	v, ok := span.Attributes().Get(key)
	require.True(t, ok, "attribute %q should exist", key)
	out := make([]int64, 0, v.Slice().Len())
	for i := 0; i < v.Slice().Len(); i++ {
		out = append(out, v.Slice().At(i).Int())
	}
	return out
}

// float64Slice reads a slice attribute of doubles.
func float64Slice(t *testing.T, span ptrace.Span, key string) []float64 {
	t.Helper()
	v, ok := span.Attributes().Get(key)
	require.True(t, ok, "attribute %q should exist", key)
	out := make([]float64, 0, v.Slice().Len())
	for i := 0; i < v.Slice().Len(); i++ {
		out = append(out, v.Slice().At(i).Double())
	}
	return out
}

// TestMergeDisabled_SummaryIsAnOrdinarySpan pins the single-pass behavior a
// second pruning pass must not disturb: with merge_existing_summaries off, a
// span carrying summary attributes gets no special treatment anywhere.
func TestMergeDisabled_SummaryIsAnOrdinarySpan(t *testing.T) {
	t.Run("no threshold bypass", func(t *testing.T) {
		// Summary + 2 raw spans is 3 members, below min_spans_to_aggregate=5.
		// Without merging there is no bypass, so nothing is aggregated.
		cfg := mergeTestConfig(t, false)
		td := runProcessor(t, cfg, newMergeTestTraces(2, nil))

		assert.Equal(t, 4, countSpans(td), "trace should be untouched")
		summary, found := findSummarySpan(td)
		require.True(t, found)
		assert.Equal(t, newSpanID(9), summary.SpanID(), "the original summary span should survive as-is")
		assert.Equal(t, int64(10), intAttr(t, summary, "aggregation.span_count"))
	})

	t.Run("counted as one unweighted sample", func(t *testing.T) {
		cfg := mergeTestConfig(t, false)
		cfg.MinSpansToAggregate = 2
		cfg.EnableAttributeLossAnalysis = true
		// Shrink the summary's envelope to 1ms so a raw span becomes the
		// template and the summary's own attributes are visibly lost.
		td := runProcessor(t, cfg, newMergeTestTraces(2, func(s *summarySpec) {
			s.endNs = s.startNs + msNs
		}))

		require.Equal(t, 2, countSpans(td))
		summary, found := findSummarySpan(td)
		require.True(t, found)

		// Three spans, each contributing one duration sample: the summary's own
		// 1ms envelope plus two 2ms raw spans. The stored rollups are ignored.
		assert.Equal(t, int64(3), intAttr(t, summary, "aggregation.span_count"))
		assert.Equal(t, 1*msNs, intAttr(t, summary, "aggregation.duration_min_ns"))
		assert.Equal(t, 2*msNs, intAttr(t, summary, "aggregation.duration_max_ns"))
		assert.Equal(t, 5*msNs, intAttr(t, summary, "aggregation.duration_total_ns"))

		// Histogram bounds come from the configuration, not the input summary,
		// and the input summary's counts are not folded in.
		assert.Equal(t, []float64{0.01, 0.05}, float64Slice(t, summary, "aggregation.histogram_bucket_bounds_s"))
		assert.Equal(t, []int64{3, 3, 3}, int64Slice(t, summary, "aggregation.histogram_bucket_counts"))

		// Attribute-loss analysis still sees the aggregation attributes, since
		// prefix filtering is part of the merge feature.
		missing, ok := summary.Attributes().Get("aggregation.missing_attributes")
		require.True(t, ok)
		assert.Contains(t, missing.Str(), "aggregation.")
	})
}

// TestAggregationLevelIsAlwaysWritten covers the half of the change that is not
// gated: a later run cannot recover how much subtree a childless summary stands
// for unless every run records it.
func TestAggregationLevelIsAlwaysWritten(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.MaxParentDepth = 1
	require.False(t, cfg.MergeExistingSummaries)

	// root -> 2 handlers -> 3 leaves each. The leaves aggregate, which makes
	// both handlers eligible for parent aggregation, so the output has one
	// summary at each level.
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(testTraceID)
	root.SetSpanID(newSpanID(1))
	root.SetName("root")
	for h := range 2 {
		handlerID := newSpanID(byte(2 + h))
		appendLeafSpan(ss, handlerID, newSpanID(1), "handler", testStart, 30*msNs, nil)
		for i := range 3 {
			appendLeafSpan(ss, newSpanID(byte(20+h*10+i)), handlerID, "SELECT",
				testStart+int64(i)*msNs, 2*msNs, nil)
		}
	}

	td = runProcessor(t, cfg, td)

	levels := map[int64]int{}
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				if v, ok := spans.At(k).Attributes().Get("aggregation.aggregation_level"); ok {
					levels[v.Int()]++
				}
			}
		}
	}
	assert.Equal(t, map[int64]int{0: 1, 1: 1}, levels)
}

// TestSummaryDropsStaleAggregationAttributes covers the unconditional attribute
// hygiene fix: a template span's aggregation attributes are cleared before the
// fresh set is written, so a value this run did not recompute cannot survive.
func TestSummaryDropsStaleAggregationAttributes(t *testing.T) {
	for _, merge := range []bool{false, true} {
		name := "merge_disabled"
		if merge {
			name = "merge_enabled"
		}
		t.Run(name, func(t *testing.T) {
			cfg := mergeTestConfig(t, merge)
			cfg.MinSpansToAggregate = 2
			// The summary's 20ms envelope makes it the longest span in the
			// group, so it is picked as the template.
			td := runProcessor(t, cfg, newMergeTestTraces(2, func(s *summarySpec) {
				s.staleAttrs = map[string]int64{
					"aggregation.duration_median_ns": 4 * msNs,
					"aggregation.exemplar_count":     3,
				}
			}))

			summary, found := findSummarySpan(td)
			require.True(t, found)
			_, hasMedian := summary.Attributes().Get("aggregation.duration_median_ns")
			assert.False(t, hasMedian, "stale median from the template must not leak through")
			_, hasExemplars := summary.Attributes().Get("aggregation.exemplar_count")
			assert.False(t, hasExemplars, "stale exemplar count from the template must not leak through")
			// Business attributes from the template are still preserved.
			op, ok := summary.Attributes().Get("db.operation")
			require.True(t, ok)
			assert.Equal(t, "select", op.Str())
		})
	}
}

func TestLeafMerge_EndToEnd(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	td := runProcessor(t, cfg, newMergeTestTraces(2, nil))

	// The group holds 3 members, below min_spans_to_aggregate=5, but an
	// existing summary bypasses the floor.
	require.Equal(t, 2, countSpans(td), "parent plus one merged summary")
	summary, found := findSummarySpan(td)
	require.True(t, found)
	assert.NotEqual(t, newSpanID(9), summary.SpanID(), "merged summary gets a fresh span ID")

	// 10 spans behind the old summary plus 2 new raw spans.
	assert.Equal(t, int64(12), intAttr(t, summary, "aggregation.span_count"))
	// min stays the old summary's stored 1ms; max stays its stored 5ms; the
	// summary's own 20ms envelope is never treated as a duration sample.
	assert.Equal(t, 1*msNs, intAttr(t, summary, "aggregation.duration_min_ns"))
	assert.Equal(t, 5*msNs, intAttr(t, summary, "aggregation.duration_max_ns"))
	assert.Equal(t, 34*msNs, intAttr(t, summary, "aggregation.duration_total_ns"))
	assert.Equal(t, 34*msNs/12, intAttr(t, summary, "aggregation.duration_avg_ns"))
	assert.Equal(t, int64(0), intAttr(t, summary, "aggregation.aggregation_level"))

	// The envelope still spans everything the group covers.
	assert.Equal(t, pcommon.Timestamp(testStart), summary.StartTimestamp())
	assert.Equal(t, pcommon.Timestamp(testStart+20*msNs), summary.EndTimestamp())
}

func TestLeafMerge_LoneSummaryIsNotReEmitted(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	td := runProcessor(t, cfg, newMergeTestTraces(0, nil))

	require.Equal(t, 2, countSpans(td))
	summary, found := findSummarySpan(td)
	require.True(t, found)
	assert.Equal(t, newSpanID(9), summary.SpanID(), "a summary with nothing to merge keeps its span ID")
	assert.Equal(t, int64(10), intAttr(t, summary, "aggregation.span_count"))
}

func TestLeafMerge_HistogramAnchoredToStoredBounds(t *testing.T) {
	t.Run("bounds preserved and counts added", func(t *testing.T) {
		cfg := mergeTestConfig(t, true)
		// A different configured bucket set must not disturb the historical
		// counts: the merged histogram keeps the bounds they were built with.
		cfg.AggregationHistogramBuckets = []time.Duration{time.Second}
		td := runProcessor(t, cfg, newMergeTestTraces(2, nil))

		summary, found := findSummarySpan(td)
		require.True(t, found)
		assert.Equal(t, []float64{0.01, 0.05}, float64Slice(t, summary, "aggregation.histogram_bucket_bounds_s"))
		// Two new 2ms spans land in the first (<=10ms) bucket and, being
		// cumulative, in every bucket above it.
		assert.Equal(t, []int64{6, 10, 12}, int64Slice(t, summary, "aggregation.histogram_bucket_counts"))
	})

	t.Run("skipped when the existing summary has none", func(t *testing.T) {
		cfg := mergeTestConfig(t, true)
		td := runProcessor(t, cfg, newMergeTestTraces(2, func(s *summarySpec) {
			s.boundsS = nil
			s.counts = nil
		}))

		summary, found := findSummarySpan(td)
		require.True(t, found)
		_, hasBounds := summary.Attributes().Get("aggregation.histogram_bucket_bounds_s")
		assert.False(t, hasBounds, "no historical distribution to merge, so none is fabricated")
		_, hasCounts := summary.Attributes().Get("aggregation.histogram_bucket_counts")
		assert.False(t, hasCounts)
		// The scalar rollups still merge normally.
		assert.Equal(t, int64(12), intAttr(t, summary, "aggregation.span_count"))
	})

	t.Run("skipped when two summaries disagree on bounds", func(t *testing.T) {
		cfg := mergeTestConfig(t, true)
		td := newMergeTestTraces(1, nil)
		ss := td.ResourceSpans().At(0).ScopeSpans().At(0)
		appendSummarySpan(ss, summarySpec{
			spanID:       newSpanID(10),
			parentSpanID: newSpanID(1),
			name:         "SELECT",
			startNs:      testStart,
			endNs:        testStart + 8*msNs,
			count:        4,
			minNs:        2 * msNs,
			maxNs:        6 * msNs,
			totalNs:      16 * msNs,
			attrs:        map[string]string{"db.operation": "select"},
			boundsS:      []float64{0.02, 0.2},
			counts:       []int64{1, 3, 4},
		})

		td = runProcessor(t, cfg, td)
		summary, found := findSummarySpan(td)
		require.True(t, found)
		_, hasBounds := summary.Attributes().Get("aggregation.histogram_bucket_bounds_s")
		assert.False(t, hasBounds, "mismatched bounds cannot be combined, so the histogram is dropped")
		// 10 + 4 prior spans plus the one new raw span.
		assert.Equal(t, int64(15), intAttr(t, summary, "aggregation.span_count"))
		assert.Equal(t, 1*msNs, intAttr(t, summary, "aggregation.duration_min_ns"))
		assert.Equal(t, 6*msNs, intAttr(t, summary, "aggregation.duration_max_ns"))
		assert.Equal(t, 48*msNs, intAttr(t, summary, "aggregation.duration_total_ns"))
	})
}

func TestLeafMerge_AttributeLossIgnoresAggregationAttributes(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	cfg.EnableAttributeLossAnalysis = true

	t.Run("bookkeeping is never reported as lost", func(t *testing.T) {
		td := runProcessor(t, cfg, newMergeTestTraces(2, nil))

		summary, found := findSummarySpan(td)
		require.True(t, found)
		// Every span in the group shares db.operation and only the prior summary
		// carries aggregation bookkeeping, so with the filter working there is no
		// loss at all left to report.
		_, hasDiverse := summary.Attributes().Get("aggregation.diverse_attributes")
		assert.False(t, hasDiverse, "no business attribute differs across the group")
		_, hasMissing := summary.Attributes().Get("aggregation.missing_attributes")
		assert.False(t, hasMissing, "the summary's own bookkeeping is not lost business data")
	})

	t.Run("a real business attribute is still reported", func(t *testing.T) {
		td := ptrace.NewTraces()
		ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		parent := ss.Spans().AppendEmpty()
		parent.SetTraceID(testTraceID)
		parent.SetSpanID(newSpanID(1))
		parent.SetName("parent")

		// The summary is the template, so it has to be the raw spans that carry
		// db.shard: an attribute the template holds survives and is not lost.
		appendSummarySpan(ss, summarySpec{
			spanID: newSpanID(9), parentSpanID: newSpanID(1), name: "SELECT",
			startNs: testStart, endNs: testStart + 20*msNs,
			count: 10, minNs: 1 * msNs, maxNs: 5 * msNs, totalNs: 30 * msNs,
			attrs: map[string]string{"db.operation": "select"},
		})
		for i := range 2 {
			appendLeafSpan(ss, newSpanID(byte(20+i)), newSpanID(1), "SELECT",
				testStart+int64(i)*msNs, 2*msNs,
				map[string]string{"db.operation": "select", "db.shard": "7"})
		}

		td = runProcessor(t, cfg, td)

		summary, found := findSummarySpan(td)
		require.True(t, found)
		missing, ok := summary.Attributes().Get("aggregation.missing_attributes")
		require.True(t, ok, "db.shard is absent from the template, so it is genuinely lost")
		assert.Contains(t, missing.Str(), "db.shard")
		assert.NotContains(t, missing.Str(), "aggregation.")
	})
}

// newMergeTestTracesWithDurations is newMergeTestTraces with explicit raw span
// durations, so a test can put a genuine duration outlier among them.
func newMergeTestTracesWithDurations(durationsNs []int64) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	parent := ss.Spans().AppendEmpty()
	parent.SetTraceID(testTraceID)
	parent.SetSpanID(newSpanID(1))
	parent.SetName("parent")

	appendSummarySpan(ss, summarySpec{
		spanID:       newSpanID(9),
		parentSpanID: newSpanID(1),
		name:         "SELECT",
		startNs:      testStart,
		endNs:        testStart + 20*msNs,
		count:        10,
		minNs:        1 * msNs,
		maxNs:        5 * msNs,
		totalNs:      30 * msNs,
		attrs:        map[string]string{"db.operation": "select"},
	})

	for i, d := range durationsNs {
		appendLeafSpan(ss, newSpanID(byte(20+i)), newSpanID(1), "SELECT",
			testStart+int64(i)*msNs, d, map[string]string{"db.operation": "select"})
	}

	return td
}

func TestLeafMerge_OutlierPreservationSkipsExistingSummary(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	cfg.EnableOutlierAnalysis = true
	cfg.OutlierAnalysis.PreserveOutliers = true
	cfg.OutlierAnalysis.MinGroupSize = 4
	require.NoError(t, cfg.Validate())

	// The 50ms span is the only outlier. The summary's 20ms envelope is wider
	// than anything actually observed, so it would be preserved ahead of it if
	// treated as a sample.
	td := runProcessor(t, cfg, newMergeTestTracesWithDurations(
		[]int64{2 * msNs, 2 * msNs, 2 * msNs, 2 * msNs, 2 * msNs, 2 * msNs, 2 * msNs, 50 * msNs},
	))

	assert.False(t, spanPresent(td, newSpanID(9)),
		"the prior summary merged away instead of being preserved as an outlier")

	outlier, found := findSpan(td, newSpanID(27))
	require.True(t, found, "the 50ms span is preserved whole")
	_, isOutlier := outlier.Attributes().Get("aggregation.is_preserved_outlier")
	assert.True(t, isOutlier)

	summary, found := findSummarySpan(td)
	require.True(t, found)
	// The 10 spans behind the prior summary plus the seven normal ones; the
	// preserved outlier is kept individually rather than folded in.
	assert.Equal(t, int64(17), intAttr(t, summary, "aggregation.span_count"))
}

func TestLeafMerge_ExemplarSamplingSkipsExistingSummary(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	cfg.EnableExemplarSampling = true
	cfg.ExemplarSampling.PrecisionMultiplier = 1.0
	require.NoError(t, cfg.Validate())

	// ceil(sqrt(8)) = 3 exemplars are drawn from the eight raw spans. The draw
	// runs over the raw members only, so the prior summary cannot be sampled.
	td := runProcessor(t, cfg, newMergeTestTracesWithDurations(
		[]int64{2 * msNs, 2 * msNs, 2 * msNs, 2 * msNs, 2 * msNs, 2 * msNs, 2 * msNs, 2 * msNs},
	))

	assert.False(t, spanPresent(td, newSpanID(9)),
		"the prior summary merged away instead of being drawn as an exemplar")

	summary, found := findSummarySpan(td)
	require.True(t, found)
	assert.Equal(t, int64(3), intAttr(t, summary, "aggregation.exemplar_count"))
	// The 10 spans behind the prior summary plus the five not drawn.
	assert.Equal(t, int64(15), intAttr(t, summary, "aggregation.span_count"))
}

// TestParentMerge_EndToEnd covers a late-arriving, structurally complete subtree
// reattaching beside an existing parent-level summary at the same level.
func TestParentMerge_EndToEnd(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	cfg.MinSpansToAggregate = 5
	cfg.MaxParentDepth = 1

	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	root := ss.Spans().AppendEmpty()
	root.SetTraceID(testTraceID)
	root.SetSpanID(newSpanID(1))
	root.SetName("root")

	// A parent-level summary from an earlier run: childless, but tagged with
	// the level it was created at.
	appendSummarySpan(ss, summarySpec{
		spanID:       newSpanID(9),
		parentSpanID: newSpanID(1),
		name:         "handler",
		startNs:      testStart,
		endNs:        testStart + 50*msNs,
		count:        20,
		minNs:        10 * msNs,
		maxNs:        40 * msNs,
		totalNs:      400 * msNs,
		level:        1,
	})

	// A late-arriving subtree of the same shape: one handler with 5 leaves.
	appendLeafSpan(ss, newSpanID(3), newSpanID(1), "handler", testStart, 30*msNs, nil)
	for i := range 5 {
		appendLeafSpan(ss, newSpanID(byte(20+i)), newSpanID(3), "SELECT",
			testStart+int64(i)*msNs, 2*msNs, map[string]string{"db.operation": "select"})
	}

	td = runProcessor(t, cfg, td)

	// root, the merged parent-level summary, and the leaf summary beneath it.
	require.Equal(t, 3, countSpans(td))

	parentSummary, found := findSummarySpanByLevel(td, 1)
	require.True(t, found, "a level-1 summary should exist")
	assert.Equal(t, "handler", parentSummary.Name())
	assert.Equal(t, newSpanID(1), parentSummary.ParentSpanID())
	// 20 spans behind the old summary plus the one newly arrived handler.
	assert.Equal(t, int64(21), parentSummary.Attributes().AsRaw()["aggregation.span_count"])
	assert.Equal(t, 10*msNs, intAttr(t, parentSummary, "aggregation.duration_min_ns"))
	assert.Equal(t, 40*msNs, intAttr(t, parentSummary, "aggregation.duration_max_ns"))
	assert.Equal(t, 430*msNs, intAttr(t, parentSummary, "aggregation.duration_total_ns"))

	leafSummary, found := findSummarySpanByLevel(td, 0)
	require.True(t, found, "the late leaves should still aggregate")
	assert.Equal(t, int64(5), intAttr(t, leafSummary, "aggregation.span_count"))
	assert.Equal(t, parentSummary.SpanID(), leafSummary.ParentSpanID(),
		"the leaf summary reattaches under the merged parent summary")
}

// TestParentMerge_RequiresAggregationLevel documents the accepted no-op for
// summaries written before aggregation_level existed: without it a childless
// parent-level summary reads as a leaf and simply does not merge.
func TestParentMerge_RequiresAggregationLevel(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	cfg.MinSpansToAggregate = 5
	cfg.MaxParentDepth = 1

	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	root := ss.Spans().AppendEmpty()
	root.SetTraceID(testTraceID)
	root.SetSpanID(newSpanID(1))
	root.SetName("root")

	appendSummarySpan(ss, summarySpec{
		spanID:       newSpanID(9),
		parentSpanID: newSpanID(1),
		name:         "handler",
		startNs:      testStart,
		endNs:        testStart + 50*msNs,
		count:        20,
		minNs:        10 * msNs,
		maxNs:        40 * msNs,
		totalNs:      400 * msNs,
		omitLevel:    true,
	})

	appendLeafSpan(ss, newSpanID(3), newSpanID(1), "handler", testStart, 30*msNs, nil)
	for i := range 5 {
		appendLeafSpan(ss, newSpanID(byte(20+i)), newSpanID(3), "SELECT",
			testStart+int64(i)*msNs, 2*msNs, map[string]string{"db.operation": "select"})
	}

	td = runProcessor(t, cfg, td)

	// The old summary is left alone; only the late subtree aggregates.
	survived := false
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				if spans.At(k).SpanID() == newSpanID(9) {
					survived = true
				}
			}
		}
	}
	assert.True(t, survived, "a summary with no aggregation_level is a safe no-op, not an error")
}

// findSummarySpanByLevel returns the first summary span created at the given
// aggregation level.
func findSummarySpanByLevel(td ptrace.Traces, level int64) (ptrace.Span, bool) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				v, ok := span.Attributes().Get("aggregation.aggregation_level")
				if !ok || v.Int() != level {
					continue
				}
				if isSummary, ok := span.Attributes().Get("aggregation.is_summary"); ok && isSummary.Bool() {
					return span, true
				}
			}
		}
	}
	return ptrace.Span{}, false
}

func TestReadExistingSummary(t *testing.T) {
	base := func() ptrace.Span {
		ss := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		return appendSummarySpan(ss, summarySpec{
			spanID:  newSpanID(9),
			name:    "SELECT",
			startNs: testStart,
			endNs:   testStart + msNs,
			count:   10,
			minNs:   1 * msNs,
			maxNs:   5 * msNs,
			totalNs: 30 * msNs,
			level:   2,
			boundsS: []float64{0.01, 0.05},
			counts:  []int64{4, 8, 10},
		})
	}

	t.Run("complete summary", func(t *testing.T) {
		got, ok := readExistingSummary(base(), "aggregation.")
		require.True(t, ok)
		assert.Equal(t, int64(10), got.spanCount)
		assert.Equal(t, time.Millisecond, got.minDuration)
		assert.Equal(t, 5*time.Millisecond, got.maxDuration)
		assert.Equal(t, 30*time.Millisecond, got.sumDuration)
		assert.Equal(t, 2, got.level)
		assert.True(t, got.hasHistogram)
		assert.Equal(t, []float64{0.01, 0.05}, got.bucketBoundsS)
		assert.Equal(t, []int64{4, 8, 10}, got.bucketCounts)
	})

	t.Run("different prefix does not match", func(t *testing.T) {
		_, ok := readExistingSummary(base(), "batch.")
		assert.False(t, ok)
	})

	tests := []struct {
		name   string
		mutate func(ptrace.Span)
	}{
		{"not a summary", func(s ptrace.Span) { s.Attributes().PutBool("aggregation.is_summary", false) }},
		{"is_summary wrong type", func(s ptrace.Span) { s.Attributes().PutStr("aggregation.is_summary", "true") }},
		{"missing span_count", func(s ptrace.Span) { s.Attributes().Remove("aggregation.span_count") }},
		{"zero span_count", func(s ptrace.Span) { s.Attributes().PutInt("aggregation.span_count", 0) }},
		{"missing min", func(s ptrace.Span) { s.Attributes().Remove("aggregation.duration_min_ns") }},
		{"missing max", func(s ptrace.Span) { s.Attributes().Remove("aggregation.duration_max_ns") }},
		{"missing total", func(s ptrace.Span) { s.Attributes().Remove("aggregation.duration_total_ns") }},
	}
	for _, tt := range tests {
		t.Run(tt.name+" is treated as an ordinary span", func(t *testing.T) {
			span := base()
			tt.mutate(span)
			_, ok := readExistingSummary(span, "aggregation.")
			assert.False(t, ok)
		})
	}

	histogramTests := []struct {
		name   string
		mutate func(ptrace.Span)
	}{
		{"missing counts", func(s ptrace.Span) { s.Attributes().Remove("aggregation.histogram_bucket_counts") }},
		{"missing bounds", func(s ptrace.Span) { s.Attributes().Remove("aggregation.histogram_bucket_bounds_s") }},
		{"length mismatch", func(s ptrace.Span) {
			counts, _ := s.Attributes().Get("aggregation.histogram_bucket_counts")
			counts.Slice().AppendEmpty().SetInt(11)
		}},
	}
	for _, tt := range histogramTests {
		t.Run("histogram "+tt.name+" still yields scalars", func(t *testing.T) {
			span := base()
			tt.mutate(span)
			got, ok := readExistingSummary(span, "aggregation.")
			require.True(t, ok)
			assert.False(t, got.hasHistogram)
			assert.Equal(t, int64(10), got.spanCount)
		})
	}
}

func TestCalculateAggregationData_WeightedMerge(t *testing.T) {
	p := &spanPruningProcessor{config: &Config{
		AggregationHistogramBuckets: []time.Duration{10 * time.Millisecond, 50 * time.Millisecond},
	}}

	// Two raw spans of 2ms and 60ms plus a summary standing for 10 spans.
	nodes := createSpanNodesWithDurations(t, []int64{2 * msNs, 60 * msNs, 20 * msNs})
	nodes[2].existingSummary = &existingSummary{
		spanCount:     10,
		minDuration:   time.Millisecond,
		maxDuration:   5 * time.Millisecond,
		sumDuration:   30 * time.Millisecond,
		hasHistogram:  true,
		bucketBoundsS: []float64{0.01, 0.05},
		bucketCounts:  []int64{4, 8, 10},
	}

	data := p.calculateAggregationData(nodes)

	assert.Equal(t, int64(12), data.count)
	assert.Equal(t, time.Millisecond, data.minDuration, "the summary's stored min wins over the raw 2ms span")
	assert.Equal(t, 60*time.Millisecond, data.maxDuration, "the raw 60ms span wins over the summary's stored 5ms max")
	assert.Equal(t, 92*time.Millisecond, data.sumDuration)
	assert.Equal(t, []float64{0.01, 0.05}, data.bucketBoundsS)
	// 2ms lands in the first bucket, 60ms only in the +Inf bucket.
	assert.Equal(t, []int64{5, 9, 12}, data.bucketCounts)
}

func TestCalculateAggregationData_SummaryEnvelopeIsNotASample(t *testing.T) {
	p := &spanPruningProcessor{config: &Config{}}

	// A lone summary whose envelope (20ms) is far wider than any duration it
	// actually observed.
	nodes := createSpanNodesWithDurations(t, []int64{20 * msNs, 3 * msNs})
	nodes[0].existingSummary = &existingSummary{
		spanCount:   10,
		minDuration: time.Millisecond,
		maxDuration: 5 * time.Millisecond,
		sumDuration: 30 * time.Millisecond,
	}
	// Every span opens at testStart, which would make earliestStart right either
	// way. Open the envelope earlier so only it can produce the correct bound.
	nodes[0].span.SetStartTimestamp(pcommon.Timestamp(testStart - 5*msNs))

	data := p.calculateAggregationData(nodes)

	assert.Equal(t, int64(11), data.count)
	assert.Equal(t, time.Millisecond, data.minDuration)
	assert.Equal(t, 5*time.Millisecond, data.maxDuration, "20ms envelope must not become the max")
	assert.Equal(t, 33*time.Millisecond, data.sumDuration,
		"the widened envelope is still not a duration sample")
	// The envelope still bounds the merged summary's time range, at both ends.
	assert.Equal(t, pcommon.Timestamp(testStart-5*msNs), data.earliestStart)
	assert.Equal(t, pcommon.Timestamp(testStart+20*msNs), data.latestEnd)
}

func TestAnalyzeAttributeLoss_IgnoresPrefix(t *testing.T) {
	ss := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	summary := appendSummarySpan(ss, summarySpec{
		spanID: newSpanID(9), name: "SELECT", count: 10, minNs: 1, maxNs: 5, totalNs: 30,
		attrs: map[string]string{"db.operation": "select", "tier": "gold"},
	})
	raw := appendLeafSpan(ss, newSpanID(20), newSpanID(1), "SELECT", testStart, msNs,
		map[string]string{"db.operation": "select"})

	// The raw span is the template, so anything only the summary carries counts
	// as lost.
	nodes := []*spanNode{{span: summary, scopeSpans: ss}, {span: raw, scopeSpans: ss}}

	withPrefix := analyzeAttributeLoss(nodes, nodes[1], "aggregation.")
	for _, entry := range append(append([]attributeCardinality{}, withPrefix.diverse...), withPrefix.missing...) {
		assert.NotContains(t, entry.key, "aggregation.")
	}
	assert.Equal(t, []attributeCardinality{{key: "tier", uniqueValues: 1}}, withPrefix.missing)

	// Without the prefix the summary's own bookkeeping is misread as lost
	// business data, which is exactly what the filter exists to prevent.
	withoutPrefix := analyzeAttributeLoss(nodes, nodes[1], "")
	assert.Greater(t, len(withoutPrefix.missing), len(withPrefix.missing))
}

func TestRawNodes(t *testing.T) {
	nodes := createSpanNodesWithDurations(t, []int64{1, 2, 3})
	assert.Equal(t, nodes, rawNodes(nodes), "returns the input untouched when no summaries are present")

	nodes[1].existingSummary = &existingSummary{spanCount: 5}
	assert.Equal(t, []*spanNode{nodes[0], nodes[2]}, rawNodes(nodes))
}

func TestGroupMeetsMinimum(t *testing.T) {
	p := &spanPruningProcessor{config: &Config{MinSpansToAggregate: 5}}

	nodes := createSpanNodesWithDurations(t, []int64{1, 2, 3})
	assert.False(t, p.groupMeetsMinimum(nodes), "3 raw spans stay below the floor")

	nodes[0].existingSummary = &existingSummary{spanCount: 10}
	assert.True(t, p.groupMeetsMinimum(nodes), "an existing summary bypasses the floor")

	lone := createSpanNodesWithDurations(t, []int64{1})
	lone[0].existingSummary = &existingSummary{spanCount: 10}
	assert.False(t, p.groupMeetsMinimum(lone), "a lone summary has nothing to merge with")
}

// findSpan returns the span with the given ID.
func findSpan(td ptrace.Traces, id pcommon.SpanID) (ptrace.Span, bool) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				if spans.At(k).SpanID() == id {
					return spans.At(k), true
				}
			}
		}
	}
	return ptrace.Span{}, false
}

func spanPresent(td ptrace.Traces, id pcommon.SpanID) bool {
	_, ok := findSpan(td, id)
	return ok
}

// summaryCountsByName returns the span_count of every summary span with the
// given name.
func summaryCountsByName(td ptrace.Traces, name string) []int64 {
	var counts []int64
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.Name() != name {
					continue
				}
				isSummary, ok := span.Attributes().Get("aggregation.is_summary")
				if !ok || !isSummary.Bool() {
					continue
				}
				if v, ok := span.Attributes().Get("aggregation.span_count"); ok {
					counts = append(counts, v.Int())
				}
			}
		}
	}
	return counts
}

// TestMerge_AggregationLevelIsClamped pins the ceiling on a stored aggregation
// level, which bounds the parent-candidate loop. Unclamped, one attribute spins
// that loop for as many iterations as it claims.
func TestMerge_AggregationLevelIsClamped(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	cfg.MaxParentDepth = -1
	require.NoError(t, cfg.Validate())

	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(testTraceID)
	root.SetSpanID(newSpanID(1))
	root.SetName("root")

	appendSummarySpan(ss, summarySpec{
		spanID:       newSpanID(9),
		parentSpanID: newSpanID(1),
		name:         "handler",
		startNs:      testStart,
		endNs:        testStart + 50*msNs,
		count:        20,
		minNs:        10 * msNs,
		maxNs:        40 * msNs,
		totalNs:      400 * msNs,
		// Large enough that an unclamped loop cannot finish in any tolerable
		// budget; 1e9 completes in seconds and would let the bug through.
		level: 1 << 40,
	})

	ctx := t.Context()
	tp, err := NewFactory().CreateTraces(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// The clamped path returns immediately, so the budget only rules out the
	// unbounded walk.
	errCh := make(chan error, 1)
	go func() { errCh <- tp.ConsumeTraces(ctx, td) }()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("processing did not finish: the stored aggregation level bounded the loop unclamped")
	}

	assert.True(t, spanPresent(td, newSpanID(9)),
		"a level above the tree height has no partner to merge with, so the summary is left alone")
}

func TestReadExistingSummary_RejectsIncoherentRollups(t *testing.T) {
	base := func() ptrace.Span {
		ss := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		return appendSummarySpan(ss, summarySpec{
			spanID: newSpanID(9), name: "SELECT",
			count: 10, minNs: 1 * msNs, maxNs: 5 * msNs, totalNs: 30 * msNs,
		})
	}

	// The fixture is coherent, so each rejection below is caused by its mutation
	// rather than by the fixture itself.
	_, ok := readExistingSummary(base(), "aggregation.")
	require.True(t, ok)

	tests := []struct {
		name   string
		mutate func(ptrace.Span)
	}{
		{"negative min", func(s ptrace.Span) {
			s.Attributes().PutInt("aggregation.duration_min_ns", -1)
		}},
		{"max below min", func(s ptrace.Span) {
			s.Attributes().PutInt("aggregation.duration_max_ns", 0)
		}},
		{"total below max", func(s ptrace.Span) {
			s.Attributes().PutInt("aggregation.duration_total_ns", 4*msNs)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := base()
			tt.mutate(span)
			_, ok := readExistingSummary(span, "aggregation.")
			assert.False(t, ok,
				"an incoherent rollup would propagate into the merged min/max and skew the average")
		})
	}
}

func TestReadHistogram_RejectsMalformedBuckets(t *testing.T) {
	build := func(boundsS []float64, counts []int64) ptrace.Span {
		ss := ptrace.NewTraces().ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		return appendSummarySpan(ss, summarySpec{
			spanID: newSpanID(9), name: "SELECT",
			count: 10, minNs: 1 * msNs, maxNs: 5 * msNs, totalNs: 30 * msNs,
			boundsS: boundsS, counts: counts,
		})
	}

	got, ok := readExistingSummary(build([]float64{0.01, 0.05}, []int64{4, 8, 10}), "aggregation.")
	require.True(t, ok)
	require.True(t, got.hasHistogram, "the well-formed fixture is accepted")

	tests := []struct {
		name    string
		boundsS []float64
		counts  []int64
	}{
		{"descending bounds", []float64{0.05, 0.01}, []int64{4, 8, 10}},
		{"duplicate bounds", []float64{0.01, 0.01}, []int64{4, 8, 10}},
		{"non-positive bound", []float64{0, 0.05}, []int64{4, 8, 10}},
		{"negative count", []float64{0.01, 0.05}, []int64{-1, 8, 10}},
		{"decreasing counts", []float64{0.01, 0.05}, []int64{8, 4, 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := readExistingSummary(build(tt.boundsS, tt.counts), "aggregation.")
			require.True(t, ok, "the scalar rollups are unaffected")
			assert.False(t, got.hasHistogram,
				"new spans are bucketed against these bounds, so a malformed set is dropped")
		})
	}
}

// TestSummaryStrip_KeepsUserAttributesInPrefixNamespace pins which attributes
// the template copy drops: only this processor's own names, since the prefix is
// configurable and a user may keep their own attributes under it.
func TestSummaryStrip_KeepsUserAttributesInPrefixNamespace(t *testing.T) {
	for _, merge := range []bool{false, true} {
		name := "merge off"
		if merge {
			name = "merge on"
		}
		t.Run(name, func(t *testing.T) {
			cfg := mergeTestConfig(t, merge)
			cfg.MinSpansToAggregate = 2
			require.NoError(t, cfg.Validate())

			// The summary's envelope makes it the template either way, so both a
			// stale rollup and a user attribute reach the new summary's copy.
			td := runProcessor(t, cfg, newMergeTestTraces(3, func(spec *summarySpec) {
				spec.staleAttrs = map[string]int64{"aggregation.duration_median_ns": 999}
				spec.attrs = map[string]string{
					"db.operation":     "select",
					"aggregation.tier": "gold",
				}
			}))

			summary, found := findSummarySpan(td)
			require.True(t, found)

			tier, ok := summary.Attributes().Get("aggregation.tier")
			require.True(t, ok, "a user attribute merely sharing the prefix is not ours to drop")
			assert.Equal(t, "gold", tier.Str())

			_, stale := summary.Attributes().Get("aggregation.duration_median_ns")
			assert.False(t, stale,
				"outlier analysis is off, so this rollup is not recomputed and must not leak from the template")
		})
	}
}

// TestParentMerge_SameTreeDepthDifferentLevelsBothAggregate pins that a seeded
// summary and raw parents sharing a tree depth, but formed at different
// aggregation depths, stay in separate groups. Sharing a key drops one group
// after its nodes are marked for removal, deleting those spans uncovered.
func TestParentMerge_SameTreeDepthDifferentLevelsBothAggregate(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	cfg.MinSpansToAggregate = 5
	cfg.MaxParentDepth = -1
	require.NoError(t, cfg.Validate())

	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	root := ss.Spans().AppendEmpty()
	root.SetTraceID(testTraceID)
	root.SetSpanID(newSpanID(1))
	root.SetName("root")

	// Two raw "B" parents at tree depth 1, each with five aggregating leaves, so
	// they form a candidate group at aggregation depth 1.
	for _, pid := range []byte{3, 4} {
		appendLeafSpan(ss, newSpanID(pid), newSpanID(1), "B", testStart, 30*msNs, nil)
		for i := range 5 {
			appendLeafSpan(ss, newSpanID(pid*10+byte(i)), newSpanID(pid), "SELECT",
				testStart+int64(i)*msNs, 2*msNs, map[string]string{"db.operation": "select"})
		}
	}

	// Two childless "B" summaries, also at tree depth 1, stored at level 2, so
	// they are seeded into the loop at aggregation depth 2.
	for _, id := range []byte{9, 10} {
		appendSummarySpan(ss, summarySpec{
			spanID:       newSpanID(id),
			parentSpanID: newSpanID(1),
			name:         "B",
			startNs:      testStart,
			endNs:        testStart + 50*msNs,
			count:        20,
			minNs:        10 * msNs,
			maxNs:        40 * msNs,
			totalNs:      400 * msNs,
			level:        2,
		})
	}

	td = runProcessor(t, cfg, td)

	assert.ElementsMatch(t, []int64{2, 40}, summaryCountsByName(td, "B"),
		"both groups produce a summary: the two raw parents, and the two summaries standing for 20 spans each")

	leafSummary, found := findSummarySpanByLevel(td, 0)
	require.True(t, found)
	assert.True(t, spanPresent(td, leafSummary.ParentSpanID()),
		"the leaf summary must hang off a span that still exists")
}

// TestMerge_RoundTripsThisProcessorsOwnOutput feeds a real first-pass result
// back in with later spans appended. Other tests hand-write the summary, so
// this is the only one pairing what createSummarySpanWithParent writes with
// what readExistingSummary reads.
func TestMerge_RoundTripsThisProcessorsOwnOutput(t *testing.T) {
	cfg := mergeTestConfig(t, true)
	cfg.MinSpansToAggregate = 3
	require.NoError(t, cfg.Validate())

	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	parent := ss.Spans().AppendEmpty()
	parent.SetTraceID(testTraceID)
	parent.SetSpanID(newSpanID(1))
	parent.SetName("parent")
	for i := range 5 {
		appendLeafSpan(ss, newSpanID(byte(20+i)), newSpanID(1), "SELECT",
			testStart+int64(i)*msNs, 2*msNs, map[string]string{"db.operation": "select"})
	}

	// Pass 1: five 2ms leaves collapse into one summary.
	td = runProcessor(t, cfg, td)
	first, found := findSummarySpan(td)
	require.True(t, found)
	require.Equal(t, int64(5), intAttr(t, first, "aggregation.span_count"))

	// Three 4ms spans of the same shape arrive beside that summary.
	late := td.ResourceSpans().At(0).ScopeSpans().At(0)
	for i := range 3 {
		appendLeafSpan(late, newSpanID(byte(40+i)), newSpanID(1), "SELECT",
			testStart+int64(i)*msNs, 4*msNs, map[string]string{"db.operation": "select"})
	}

	// Pass 2: they fold into the summary rather than sitting beside it.
	td = runProcessor(t, cfg, td)
	merged, found := findSummarySpan(td)
	require.True(t, found)
	assert.Equal(t, int64(8), intAttr(t, merged, "aggregation.span_count"),
		"the five spans behind the first summary plus the three that arrived late")
	assert.Equal(t, 2*msNs, intAttr(t, merged, "aggregation.duration_min_ns"))
	assert.Equal(t, 4*msNs, intAttr(t, merged, "aggregation.duration_max_ns"))
	assert.Equal(t, (5*2+3*4)*msNs, intAttr(t, merged, "aggregation.duration_total_ns"))
	// The first pass stored the configured bounds, so the second pass reuses them
	// and adds the three late spans into the 10ms bucket.
	assert.Equal(t, []float64{0.01, 0.05}, float64Slice(t, merged, "aggregation.histogram_bucket_bounds_s"))
	assert.Equal(t, []int64{8, 8, 8}, int64Slice(t, merged, "aggregation.histogram_bucket_counts"))
}
