// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"math"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Attribute suffixes (appended to the configured AggregationAttributePrefix)
// that identify a summary span and carry the rollups a later run needs in order
// to merge into it.
const (
	attrIsSummary       = "is_summary"
	attrSpanCount       = "span_count"
	attrDurationMin     = "duration_min_ns"
	attrDurationMax     = "duration_max_ns"
	attrDurationTotal   = "duration_total_ns"
	attrAggregationLvl  = "aggregation_level"
	attrHistogramBounds = "histogram_bucket_bounds_s"
	attrHistogramCounts = "histogram_bucket_counts"
)

// aggregationAttrSuffixes lists every attribute suffix this processor writes,
// including those only written onto preserved outlier and exemplar spans, since
// a span an earlier run kept can become a later run's template. The prefix is
// configurable, so matching on it alone would also claim a user's own
// attributes in that namespace.
var aggregationAttrSuffixes = map[string]struct{}{
	attrIsSummary:                   {},
	attrSpanCount:                   {},
	attrDurationMin:                 {},
	attrDurationMax:                 {},
	attrDurationTotal:               {},
	attrAggregationLvl:              {},
	attrHistogramBounds:             {},
	attrHistogramCounts:             {},
	"duration_avg_ns":               {},
	"duration_median_ns":            {},
	"outlier_correlated_attributes": {},
	"preserved_outlier_count":       {},
	"preserved_outlier_span_ids":    {},
	"is_preserved_outlier":          {},
	"summary_span_id":               {},
	"exemplar_count":                {},
	"exemplar_span_ids":             {},
	"is_exemplar":                   {},
	"diverse_attributes":            {},
	"missing_attributes":            {},
}

// isAggregationAttr reports whether key is one of this processor's own
// aggregation attributes written under prefix.
func isAggregationAttr(key, prefix string) bool {
	suffix, found := strings.CutPrefix(key, prefix)
	if !found {
		return false
	}
	_, ours := aggregationAttrSuffixes[suffix]
	return ours
}

// existingSummary holds the rollup statistics read back off a summary span
// produced by an earlier run of this processor. A later run folds these into
// its own statistics instead of treating the summary as a single raw span.
//
// level is the aggregation level the summary was created at (0 = leaf-level,
// >=1 = parent-level). It is deliberately named "level" rather than "depth":
// spanNode.depth() means tree-depth-from-root, which is a different concept.
type existingSummary struct {
	spanCount   int64
	minDuration time.Duration
	maxDuration time.Duration
	sumDuration time.Duration
	level       int

	// Historical histogram, when the run that created the summary had
	// histograms enabled. bucketBoundsS holds the upper bounds in seconds
	// (excluding +Inf) and bucketCounts holds the cumulative counts, so
	// len(bucketCounts) == len(bucketBoundsS)+1.
	hasHistogram  bool
	bucketBoundsS []float64
	bucketCounts  []int64
}

// readExistingSummary reports whether span is a summary span emitted by a prior
// run, returning its rollup statistics. A span only qualifies when it carries
// `<prefix>is_summary=true` plus every scalar rollup needed to fold it into a
// merged summary; a partially annotated span is treated as an ordinary span so
// a missing rollup can never corrupt min/max/avg.
func readExistingSummary(span ptrace.Span, prefix string) (*existingSummary, bool) {
	attrs := span.Attributes()

	v, ok := attrs.Get(prefix + attrIsSummary)
	if !ok || v.Type() != pcommon.ValueTypeBool || !v.Bool() {
		return nil, false
	}

	count, ok := getInt(attrs, prefix+attrSpanCount)
	if !ok || count < 1 {
		return nil, false
	}
	minNs, ok := getInt(attrs, prefix+attrDurationMin)
	if !ok {
		return nil, false
	}
	maxNs, ok := getInt(attrs, prefix+attrDurationMax)
	if !ok {
		return nil, false
	}
	totalNs, ok := getInt(attrs, prefix+attrDurationTotal)
	if !ok {
		return nil, false
	}

	// Reject incoherent rollups rather than folding them into min/max/avg. total
	// cannot be below max, which is one of the durations summed into it.
	if minNs < 0 || maxNs < minNs || totalNs < maxNs {
		return nil, false
	}

	summary := &existingSummary{
		spanCount:   count,
		minDuration: time.Duration(minNs),
		maxDuration: time.Duration(maxNs),
		sumDuration: time.Duration(totalNs),
	}

	// Summaries written before aggregation_level existed simply report level 0.
	// A leaf-level summary is indistinguishable from that, and both are handled
	// by the leaf path, so the fallback is safe.
	if level, ok := getInt(attrs, prefix+attrAggregationLvl); ok && level > 0 {
		summary.level = int(level)
	}

	summary.bucketBoundsS, summary.bucketCounts, summary.hasHistogram = readHistogram(attrs, prefix)

	return summary, true
}

// getInt reads an int attribute, reporting false when the key is absent or
// holds a different type.
func getInt(attrs pcommon.Map, key string) (int64, bool) {
	v, ok := attrs.Get(key)
	if !ok || v.Type() != pcommon.ValueTypeInt {
		return 0, false
	}
	return v.Int(), true
}

// readHistogram extracts a summary span's stored histogram. It reports false
// unless both slices are present, well typed, and consistently sized
// (len(counts) == len(bounds)+1).
func readHistogram(attrs pcommon.Map, prefix string) (boundsS []float64, counts []int64, ok bool) {
	boundsVal, exists := attrs.Get(prefix + attrHistogramBounds)
	if !exists || boundsVal.Type() != pcommon.ValueTypeSlice {
		return nil, nil, false
	}
	countsVal, exists := attrs.Get(prefix + attrHistogramCounts)
	if !exists || countsVal.Type() != pcommon.ValueTypeSlice {
		return nil, nil, false
	}

	boundsSlice := boundsVal.Slice()
	countsSlice := countsVal.Slice()
	if countsSlice.Len() != boundsSlice.Len()+1 {
		return nil, nil, false
	}

	// New spans are bucketed against these by a first-fit scan, so bounds must be
	// positive, finite and strictly ascending or durations land in wrong buckets.
	boundsS = make([]float64, 0, boundsSlice.Len())
	for i := 0; i < boundsSlice.Len(); i++ {
		b := boundsSlice.At(i)
		if b.Type() != pcommon.ValueTypeDouble {
			return nil, nil, false
		}
		bound := b.Double()
		if bound <= 0 || math.IsNaN(bound) || math.IsInf(bound, 0) {
			return nil, nil, false
		}
		if i > 0 && bound <= boundsS[i-1] {
			return nil, nil, false
		}
		boundsS = append(boundsS, bound)
	}
	// Counts are cumulative, so they must be non-negative and non-decreasing.
	counts = make([]int64, 0, countsSlice.Len())
	for i := 0; i < countsSlice.Len(); i++ {
		c := countsSlice.At(i)
		if c.Type() != pcommon.ValueTypeInt {
			return nil, nil, false
		}
		count := c.Int()
		if count < 0 || (i > 0 && count < counts[i-1]) {
			return nil, nil, false
		}
		counts = append(counts, count)
	}

	return boundsS, counts, true
}

// boundsToDurations converts histogram upper bounds expressed in seconds back
// to durations so new raw spans can be bucketed against the historical bounds.
func boundsToDurations(boundsS []float64) []time.Duration {
	out := make([]time.Duration, 0, len(boundsS))
	for _, b := range boundsS {
		out = append(out, time.Duration(math.Round(b*float64(time.Second))))
	}
	return out
}

// durationsToBounds converts configured histogram bucket bounds to the seconds
// representation stored on summary spans.
func durationsToBounds(buckets []time.Duration) []float64 {
	out := make([]float64, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, float64(b)/float64(time.Second))
	}
	return out
}

// sameBounds reports whether two sets of histogram bounds are identical.
func sameBounds(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// containsExistingSummary reports whether any node in the group is a summary
// span left over from a prior run.
func containsExistingSummary(nodes []*spanNode) bool {
	for _, n := range nodes {
		if n.existingSummary != nil {
			return true
		}
	}
	return false
}

// rawNodes returns the members of a group that are not existing summaries.
// Sample-based analyses (outlier detection, exemplar sampling) need individual
// duration observations, which a summary span does not have: its start/end
// timestamps describe the envelope of a whole prior group, not one operation.
// It returns the input slice unchanged when the group holds no summaries.
func rawNodes(nodes []*spanNode) []*spanNode {
	if !containsExistingSummary(nodes) {
		return nodes
	}
	out := make([]*spanNode, 0, len(nodes))
	for _, n := range nodes {
		if n.existingSummary == nil {
			out = append(out, n)
		}
	}
	return out
}

// groupMeetsMinimum reports whether a leaf group is large enough to aggregate.
// A group holding an existing summary bypasses MinSpansToAggregate — the
// summary already stands for a whole prior group — but still needs a second
// member, so an untouched summary with nothing to merge is never re-emitted
// (which would churn its SpanID for no benefit).
func (p *spanPruningProcessor) groupMeetsMinimum(nodes []*spanNode) bool {
	if len(nodes) >= p.config.MinSpansToAggregate {
		return true
	}
	return len(nodes) >= 2 && containsExistingSummary(nodes)
}
