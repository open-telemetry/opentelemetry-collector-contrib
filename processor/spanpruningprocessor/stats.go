// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// aggregationData tracks statistics and time ranges for a group of spans in
// a single pass, replacing separate calculations for efficiency.
//
// count is a weighted total: a raw span counts once, while a summary span from
// a prior run counts for the whole group it already stands for.
type aggregationData struct {
	count         int64
	minDuration   time.Duration
	maxDuration   time.Duration
	sumDuration   time.Duration
	bucketBoundsS []float64 // histogram upper bounds in seconds; empty when no histogram is emitted
	bucketCounts  []int64
	earliestStart pcommon.Timestamp
	latestEnd     pcommon.Timestamp
}

// calculateAggregationData derives span counts and duration stats for the
// provided nodes in one traversal.
//
// Nodes that are summary spans from a prior run contribute their stored
// rollups: their span_count as weight and their stored min/max/total folded
// into the running statistics. Their own EndTimestamp-StartTimestamp is never
// used as a duration sample — that is the envelope of a whole prior group, not
// an observation — but it still bounds the merged summary's time range, which
// is exactly what earliestStart/latestEnd want.
func (p *spanPruningProcessor) calculateAggregationData(nodes []*spanNode) aggregationData {
	var data aggregationData

	histogramBuckets := p.resolveHistogramBuckets(nodes, &data)

	first := true
	for _, node := range nodes {
		if summary := node.existingSummary; summary != nil {
			data.updateWithSummary(node.span, summary, first)
		} else {
			data.updateWithSpan(node.span, first, histogramBuckets)
		}
		first = false
	}

	return data
}

// resolveHistogramBuckets decides which bucket bounds the group's histogram
// uses and seeds the cumulative counts, returning the bounds new raw spans are
// bucketed against (nil when no histogram is emitted).
//
// With no prior summary in the group this is simply the configured bucket set.
// When merging, the histogram is anchored to the bounds the existing summary
// already stores, so the historical counts stay meaningful even if the current
// configuration uses a different bucket set. The histogram is dropped entirely
// — rather than fabricating the missing distribution — when the existing
// summary has no histogram, or when two existing summaries in one group
// disagree about their bounds.
func (p *spanPruningProcessor) resolveHistogramBuckets(nodes []*spanNode, data *aggregationData) []time.Duration {
	if len(p.config.AggregationHistogramBuckets) == 0 {
		return nil
	}

	var (
		boundsS []float64
		counts  []int64
		seen    bool
	)
	for _, node := range nodes {
		summary := node.existingSummary
		if summary == nil {
			continue
		}
		if !summary.hasHistogram {
			return nil
		}
		if !seen {
			boundsS = summary.bucketBoundsS
			counts = make([]int64, len(summary.bucketCounts))
			seen = true
		} else if !sameBounds(boundsS, summary.bucketBoundsS) {
			return nil
		}
		for i, c := range summary.bucketCounts {
			counts[i] += c
		}
	}

	if !seen {
		data.bucketBoundsS = durationsToBounds(p.config.AggregationHistogramBuckets)
		data.bucketCounts = make([]int64, len(p.config.AggregationHistogramBuckets)+1)
		return p.config.AggregationHistogramBuckets
	}

	data.bucketBoundsS = boundsS
	data.bucketCounts = counts
	return boundsToDurations(boundsS)
}

// updateWithSummary folds a prior run's summary span into the aggregation
// statistics using its stored rollups.
func (data *aggregationData) updateWithSummary(span ptrace.Span, summary *existingSummary, isFirst bool) {
	if isFirst {
		data.minDuration = summary.minDuration
		data.maxDuration = summary.maxDuration
		data.earliestStart = span.StartTimestamp()
		data.latestEnd = span.EndTimestamp()
	} else {
		if summary.minDuration < data.minDuration {
			data.minDuration = summary.minDuration
		}
		if summary.maxDuration > data.maxDuration {
			data.maxDuration = summary.maxDuration
		}
		if span.StartTimestamp() < data.earliestStart {
			data.earliestStart = span.StartTimestamp()
		}
		if span.EndTimestamp() > data.latestEnd {
			data.latestEnd = span.EndTimestamp()
		}
	}
	data.sumDuration += summary.sumDuration
	data.count += summary.spanCount
	// The summary's own histogram counts were already seeded by
	// resolveHistogramBuckets; adding them again here would double count.
}

// updateWithSpan incorporates a single span into the aggregation statistics,
// tracking min/max durations and time ranges.
func (data *aggregationData) updateWithSpan(span ptrace.Span, isFirst bool, histogramBuckets []time.Duration) {
	startTime := span.StartTimestamp().AsTime()
	endTime := span.EndTimestamp().AsTime()
	duration := endTime.Sub(startTime)

	// Calculate duration statistics
	if isFirst {
		data.minDuration = duration
		data.maxDuration = duration
		data.earliestStart = span.StartTimestamp()
		data.latestEnd = span.EndTimestamp()
	} else {
		if duration < data.minDuration {
			data.minDuration = duration
		}
		if duration > data.maxDuration {
			data.maxDuration = duration
		}
		if span.StartTimestamp() < data.earliestStart {
			data.earliestStart = span.StartTimestamp()
		}
		if span.EndTimestamp() > data.latestEnd {
			data.latestEnd = span.EndTimestamp()
		}
	}
	data.sumDuration += duration
	data.count++

	if len(histogramBuckets) > 0 {
		bucketIndex := len(histogramBuckets)
		for i, bucket := range histogramBuckets {
			if duration <= bucket {
				bucketIndex = i
				break
			}
		}

		for i := bucketIndex; i < len(data.bucketCounts); i++ {
			data.bucketCounts[i]++
		}
	}
}
