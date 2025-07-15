// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subtractinitial // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/subtractinitial"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/datapointstorage"
)

// Type is the value users can use to configure the subtract initial point adjuster.
// The subtract initial point adjuster sets the start time of all points in a series by:
//   - Dropping the initial point, and recording its value and timestamp.
//   - Subtracting the initial point from all subsequent points, and using the timestamp of the initial point as the start timestamp.
//
// Note that when a reset is detected (eg: value of a counter is decreasing) - the strategy will set the
// start time of the reset point as point timestamp - 1ms.
const Type = "subtract_initial_point"

type Adjuster struct {
	// referenceCache stores the initial point of each
	// timeseries. Subsequent points are normalized against this point.
	referenceCache *datapointstorage.Cache
	// previousValueCache to store the previous value of each
	// timeseries provided to the adjuster for reset detection.
	previousValueCache *datapointstorage.Cache
	set                component.TelemetrySettings
}

// NewAdjuster returns a new Adjuster which adjust metrics' start times based on the initial received points.
func NewAdjuster(set component.TelemetrySettings, gcInterval time.Duration) *Adjuster {
	return &Adjuster{
		referenceCache:     datapointstorage.NewCache(gcInterval),
		previousValueCache: datapointstorage.NewCache(gcInterval),
		set:                set,
	}
}

// AdjustMetrics adjusts the start time of metrics based on the initial received
// points.
//
// It uses two caches: referenceCache to store the initial point of each
// timeseries, and previousValueCache to store the previous value of each
// timeseries for reset detection.
//
// If a point has not been seen before, it will be dropped and cached as the
// reference point. For each subsequent point, it will normalize against the
// reference cached point reporting the delta. If a reset is detected, the
// current point will be reported as is, and the reference point will be
// updated. The function returns a new pmetric.Metrics containing the adjusted
// metrics.
func (a *Adjuster) AdjustMetrics(_ context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		attrHash := pdatautil.MapHash(rm.Resource().Attributes())
		referenceTsm, _ := a.referenceCache.Get(attrHash)
		previousValueTsm, _ := a.previousValueCache.Get(attrHash)

		// The lock on the relevant timeseriesMap is held throughout the adjustment process to ensure that
		// nothing else can modify the data used for adjustment.
		referenceTsm.Lock()
		previousValueTsm.Lock()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := range ilm.Metrics().Len() {
				metric := ilm.Metrics().At(k)
				switch dataType := metric.Type(); dataType {
				case pmetric.MetricTypeHistogram:
					adjustMetricHistogram(referenceTsm, previousValueTsm, metric)

				case pmetric.MetricTypeSummary:
					adjustMetricSummary(referenceTsm, previousValueTsm, metric)

				case pmetric.MetricTypeSum:
					adjustMetricSum(referenceTsm, previousValueTsm, metric)

				case pmetric.MetricTypeExponentialHistogram:
					adjustMetricExponentialHistogram(referenceTsm, previousValueTsm, metric)
				}
			}
		}
		referenceTsm.Unlock()
		previousValueTsm.Unlock()
	}

	return metrics, nil
}

func adjustMetricHistogram(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, metric pmetric.Metric) {
	histogram := metric.Histogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	histogram.DataPoints().RemoveIf(func(currentDist pmetric.HistogramDataPoint) bool {
		pointStartTime := currentDist.StartTimestamp()
		if pointStartTime != 0 && pointStartTime != currentDist.Timestamp() {
			// Report point as is.
			return false
		}

		referenceTsi, found := referenceTsm.Get(metric, currentDist.Attributes())
		// previousTsi always exists when reference Tsi is found
		previousTsi, _ := previousValueTsm.Get(metric, currentDist.Attributes())
		if !found {
			// First time we see this point. Skip it and use as a reference point for the next points.
			referenceTsi.Histogram = pmetric.NewHistogramDataPoint()
			minimalHistogramCopyTo(currentDist, referenceTsi.Histogram)
			// Use the timestamp of the dropped point as the start timestamp for future points.
			referenceTsi.Histogram.SetStartTimestamp(referenceTsi.Histogram.Timestamp())
			previousTsi.Histogram = pmetric.NewHistogramDataPoint()
			minimalHistogramCopyTo(currentDist, previousTsi.Histogram)
			return true
		}

		// Adjust the datapoint based on the reference value.
		currentDist.SetStartTimestamp(referenceTsi.Histogram.StartTimestamp())
		if currentDist.Flags().NoRecordedValue() {
			return false
		}

		if datapointstorage.IsResetHistogram(currentDist, previousTsi.Histogram) {
			// reset re-initialize everything and use the non adjusted points start time.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentDist.Timestamp().AsTime().Add(-1 * time.Millisecond))
			currentDist.SetStartTimestamp(resetStartTimeStamp)

			// Update the reference value with the metric point.
			referenceTsi.Histogram = pmetric.NewHistogramDataPoint()
			previousTsi.Histogram = pmetric.NewHistogramDataPoint()
			referenceTsi.Histogram.SetStartTimestamp(resetStartTimeStamp)
			currentDist.ExplicitBounds().CopyTo(referenceTsi.Histogram.ExplicitBounds())
			referenceTsi.Histogram.BucketCounts().FromRaw(make([]uint64, currentDist.BucketCounts().Len()))
			minimalHistogramCopyTo(currentDist, previousTsi.Histogram)
		} else {
			minimalHistogramCopyTo(currentDist, previousTsi.Histogram)
			subtractHistogramDataPoint(currentDist, referenceTsi.Histogram)
		}
		return false
	})
}

func adjustMetricExponentialHistogram(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, metric pmetric.Metric) {
	histogram := metric.ExponentialHistogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	histogram.DataPoints().RemoveIf(func(currentDist pmetric.ExponentialHistogramDataPoint) bool {
		pointStartTime := currentDist.StartTimestamp()
		if pointStartTime != 0 && pointStartTime != currentDist.Timestamp() {
			// Report point as is.
			return false
		}

		referenceTsi, found := referenceTsm.Get(metric, currentDist.Attributes())
		// previousTsi always exists when reference Tsi is found
		previousTsi, _ := previousValueTsm.Get(metric, currentDist.Attributes())
		if !found {
			// First time we see this point. Skip it and use as a reference point for the next points.
			referenceTsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
			minimalExponentialHistogramCopyTo(currentDist, referenceTsi.ExponentialHistogram)
			// Use the timestamp of the dropped point as the start timestamp for future points.
			referenceTsi.ExponentialHistogram.SetStartTimestamp(referenceTsi.ExponentialHistogram.Timestamp())
			previousTsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
			minimalExponentialHistogramCopyTo(currentDist, previousTsi.ExponentialHistogram)
			return true
		}

		// Adjust the datapoint based on the reference value.
		currentDist.SetStartTimestamp(referenceTsi.ExponentialHistogram.StartTimestamp())
		if currentDist.Flags().NoRecordedValue() {
			return false
		}

		if datapointstorage.IsResetExponentialHistogram(currentDist, previousTsi.ExponentialHistogram) {
			// reset re-initialize everything and use the non adjusted points start time.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentDist.Timestamp().AsTime().Add(-1 * time.Millisecond))
			currentDist.SetStartTimestamp(resetStartTimeStamp)

			referenceTsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
			previousTsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
			referenceTsi.ExponentialHistogram.SetStartTimestamp(resetStartTimeStamp)
			referenceTsi.ExponentialHistogram.SetScale(currentDist.Scale())
			referenceTsi.ExponentialHistogram.Positive().BucketCounts().FromRaw(make([]uint64, currentDist.Positive().BucketCounts().Len()))
			referenceTsi.ExponentialHistogram.Negative().BucketCounts().FromRaw(make([]uint64, currentDist.Negative().BucketCounts().Len()))
			minimalExponentialHistogramCopyTo(currentDist, previousTsi.ExponentialHistogram)
		} else {
			minimalExponentialHistogramCopyTo(currentDist, previousTsi.ExponentialHistogram)
			subtractExponentialHistogramDataPoint(currentDist, referenceTsi.ExponentialHistogram)
		}
		return false
	})
}

func adjustMetricSum(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, metric pmetric.Metric) {
	sum := metric.Sum()
	if sum.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only handle cumulative temporality sums
		return
	}

	sum.DataPoints().RemoveIf(func(currentSum pmetric.NumberDataPoint) bool {
		pointStartTime := currentSum.StartTimestamp()
		if pointStartTime != 0 && pointStartTime != currentSum.Timestamp() {
			// Report point as is.
			return false
		}

		referenceTsi, found := referenceTsm.Get(metric, currentSum.Attributes())
		// previousTsi always exists when reference Tsi is found
		previousTsi, _ := previousValueTsm.Get(metric, currentSum.Attributes())
		if !found {
			// First time we see this point. Skip it and use as a reference point for the next points.
			referenceTsi.Number = pmetric.NewNumberDataPoint()
			minimalSumCopyTo(currentSum, referenceTsi.Number)
			// Use the timestamp of the dropped point as the start timestamp for future points.
			referenceTsi.Number.SetStartTimestamp(referenceTsi.Number.Timestamp())
			previousTsi.Number = pmetric.NewNumberDataPoint()
			minimalSumCopyTo(currentSum, previousTsi.Number)
			return true
		}

		// Adjust the datapoint based on the reference value.
		currentSum.SetStartTimestamp(referenceTsi.Number.StartTimestamp())
		if currentSum.Flags().NoRecordedValue() {
			return false
		}

		if datapointstorage.IsResetSum(currentSum, previousTsi.Number) {
			// reset re-initialize everything and use the non adjusted points start time.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentSum.Timestamp().AsTime().Add(-1 * time.Millisecond))
			currentSum.SetStartTimestamp(resetStartTimeStamp)

			referenceTsi.Number = pmetric.NewNumberDataPoint()
			previousTsi.Number = pmetric.NewNumberDataPoint()
			referenceTsi.Number.SetStartTimestamp(resetStartTimeStamp)
			minimalSumCopyTo(currentSum, previousTsi.Number)
		} else {
			minimalSumCopyTo(currentSum, previousTsi.Number)
			currentSum.SetDoubleValue(currentSum.DoubleValue() - referenceTsi.Number.DoubleValue())
		}
		return false
	})
}

func adjustMetricSummary(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, metric pmetric.Metric) {
	metric.Summary().DataPoints().RemoveIf(func(currentSummary pmetric.SummaryDataPoint) bool {
		pointStartTime := currentSummary.StartTimestamp()
		if pointStartTime != 0 && pointStartTime != currentSummary.Timestamp() {
			// Report point as is.
			return false
		}

		referenceTsi, found := referenceTsm.Get(metric, currentSummary.Attributes())
		// previousTsi always exists when reference Tsi is found
		previousTsi, _ := previousValueTsm.Get(metric, currentSummary.Attributes())
		if !found {
			// First time we see this point. Skip it and use as a reference point for the next points.
			referenceTsi.Summary = pmetric.NewSummaryDataPoint()
			minimalSummaryCopyTo(currentSummary, referenceTsi.Summary)
			// Use the timestamp of the dropped point as the start timestamp for future points.
			referenceTsi.Summary.SetStartTimestamp(referenceTsi.Summary.Timestamp())
			previousTsi.Summary = pmetric.NewSummaryDataPoint()
			minimalSummaryCopyTo(currentSummary, previousTsi.Summary)
			return true
		}

		// Adjust the datapoint based on the reference value.
		currentSummary.SetStartTimestamp(referenceTsi.Summary.StartTimestamp())
		if currentSummary.Flags().NoRecordedValue() {
			return false
		}

		if datapointstorage.IsResetSummary(currentSummary, previousTsi.Summary) {
			// reset re-initialize everything and use the non adjusted points start time.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentSummary.Timestamp().AsTime().Add(-1 * time.Millisecond))
			currentSummary.SetStartTimestamp(resetStartTimeStamp)

			referenceTsi.Summary = pmetric.NewSummaryDataPoint()
			previousTsi.Summary = pmetric.NewSummaryDataPoint()
			referenceTsi.Summary.SetStartTimestamp(resetStartTimeStamp)
			minimalSummaryCopyTo(currentSummary, previousTsi.Summary)
		} else {
			currentSummary.SetStartTimestamp(referenceTsi.Summary.StartTimestamp())
			minimalSummaryCopyTo(currentSummary, previousTsi.Summary)
			currentSummary.SetCount(currentSummary.Count() - referenceTsi.Summary.Count())
			currentSummary.SetSum(currentSummary.Sum() - referenceTsi.Summary.Sum())
		}
		return false
	})
}

// subtractHistogramDataPoint subtracts b from a.
func subtractHistogramDataPoint(a, b pmetric.HistogramDataPoint) {
	a.SetStartTimestamp(b.StartTimestamp())
	a.SetCount(a.Count() - b.Count())
	a.SetSum(a.Sum() - b.Sum())
	aBuckets := a.BucketCounts()
	bBuckets := b.BucketCounts()
	if bBuckets.Len() != aBuckets.Len() {
		// Post reset, the reference histogram will have no buckets.
		return
	}
	newBuckets := make([]uint64, aBuckets.Len())
	for i := 0; i < aBuckets.Len(); i++ {
		newBuckets[i] = aBuckets.At(i) - bBuckets.At(i)
	}
	a.BucketCounts().FromRaw(newBuckets)
}

// subtractExponentialHistogramDataPoint subtracts b from a.
func subtractExponentialHistogramDataPoint(a, b pmetric.ExponentialHistogramDataPoint) {
	a.SetStartTimestamp(b.StartTimestamp())
	a.SetCount(a.Count() - b.Count())
	a.SetSum(a.Sum() - b.Sum())
	a.SetZeroCount(a.ZeroCount() - b.ZeroCount())
	if a.Positive().BucketCounts().Len() != b.Positive().BucketCounts().Len() ||
		a.Negative().BucketCounts().Len() != b.Negative().BucketCounts().Len() {
		// Post reset, the reference histogram will have no buckets.
		return
	}
	a.Positive().BucketCounts().FromRaw(subtractExponentialBuckets(a.Positive(), b.Positive()))
	a.Negative().BucketCounts().FromRaw(subtractExponentialBuckets(a.Negative(), b.Negative()))
}

// subtractExponentialBuckets subtracts b from a.
func subtractExponentialBuckets(a, b pmetric.ExponentialHistogramDataPointBuckets) []uint64 {
	newBuckets := make([]uint64, a.BucketCounts().Len())
	offsetDiff := int(a.Offset() - b.Offset())
	for i := 0; i < a.BucketCounts().Len(); i++ {
		bOffset := i + offsetDiff
		// if there is no corresponding bucket for the starting BucketCounts, don't normalize
		if bOffset < 0 || bOffset >= b.BucketCounts().Len() {
			newBuckets[i] = a.BucketCounts().At(i)
		} else {
			newBuckets[i] = a.BucketCounts().At(i) - b.BucketCounts().At(bOffset)
		}
	}
	return newBuckets
}

// minimalHistogramCopyTo is equivalent to a.CopyTo(b) without copying attributes or exemplars
func minimalHistogramCopyTo(a, b pmetric.HistogramDataPoint) {
	// Copy attributes and exemplars to temporary structures so they are not copied to b.
	tmpAttrs := pcommon.NewMap()
	tmpExemplars := pmetric.NewExemplarSlice()
	a.Attributes().MoveTo(tmpAttrs)
	a.Exemplars().MoveAndAppendTo(tmpExemplars)
	a.CopyTo(b)
	// Restore attributes and exemplars.
	tmpAttrs.MoveTo(a.Attributes())
	tmpExemplars.MoveAndAppendTo(a.Exemplars())
}

// minimalExponentialHistogramCopyTo is equivalent to a.CopyTo(b) without copying attributes or exemplars
func minimalExponentialHistogramCopyTo(a, b pmetric.ExponentialHistogramDataPoint) {
	// Copy attributes and exemplars to temporary structures so they are not copied to b.
	tmpAttrs := pcommon.NewMap()
	tmpExemplars := pmetric.NewExemplarSlice()
	a.Attributes().MoveTo(tmpAttrs)
	a.Exemplars().MoveAndAppendTo(tmpExemplars)
	a.CopyTo(b)
	// Restore attributes and exemplars.
	tmpAttrs.MoveTo(a.Attributes())
	tmpExemplars.MoveAndAppendTo(a.Exemplars())
}

// minimalSumCopyTo is equivalent to a.CopyTo(b) without copying attributes or exemplars
func minimalSumCopyTo(a, b pmetric.NumberDataPoint) {
	// Copy attributes and exemplars to temporary structures so they are not copied to b.
	tmpAttrs := pcommon.NewMap()
	tmpExemplars := pmetric.NewExemplarSlice()
	a.Attributes().MoveTo(tmpAttrs)
	a.Exemplars().MoveAndAppendTo(tmpExemplars)
	a.CopyTo(b)
	// Restore attributes and exemplars.
	tmpAttrs.MoveTo(a.Attributes())
	tmpExemplars.MoveAndAppendTo(a.Exemplars())
}

// minimalSummaryCopyTo is equivalent to a.CopyTo(b) without copying attributes or exemplars
func minimalSummaryCopyTo(a, b pmetric.SummaryDataPoint) {
	// Copy attributes to a temporary map so they are not copied to b.
	tmpAttrs := pcommon.NewMap()
	a.Attributes().MoveTo(tmpAttrs)
	a.CopyTo(b)
	// Restore attributes.
	tmpAttrs.MoveTo(a.Attributes())
}
