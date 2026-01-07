// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// aggregationData holds both statistics and time range for a group of spans
// Combines what was previously calculateStats and findTimeRange to save an iteration
type aggregationData struct {
	count         int64
	minDuration   time.Duration
	maxDuration   time.Duration
	sumDuration   time.Duration
	bucketCounts  []int64
	earliestStart pcommon.Timestamp
	latestEnd     pcommon.Timestamp
}

// calculateAggregationData computes statistics and time range in a single pass
// Takes []*spanNode instead of []spanInfo to avoid intermediate conversions
func (p *spanPruningProcessor) calculateAggregationData(nodes []*spanNode) aggregationData {
	data := aggregationData{
		count: int64(len(nodes)),
	}

	// Initialize histogram bucket counts
	if len(p.config.AggregationHistogramBuckets) > 0 {
		data.bucketCounts = make([]int64, len(p.config.AggregationHistogramBuckets)+1)
	}

	for i, node := range nodes {
		span := node.span
		data.updateWithSpan(span, i == 0, p.config.AggregationHistogramBuckets)
	}

	return data
}

// updateWithSpan updates aggregation data with a single span
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

	// Update histogram bucket counts (cumulative)
	if len(histogramBuckets) > 0 {
		// Find which bucket this duration belongs to
		bucketIndex := len(histogramBuckets) // default to +Inf bucket
		for j, bucket := range histogramBuckets {
			if duration <= bucket {
				bucketIndex = j
				break
			}
		}
		// Increment all buckets from bucketIndex to the end (cumulative histogram)
		for j := bucketIndex; j < len(data.bucketCounts); j++ {
			data.bucketCounts[j]++
		}
	}
}
