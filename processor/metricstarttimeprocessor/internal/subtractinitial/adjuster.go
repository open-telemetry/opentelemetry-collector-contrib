// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subtractinitial // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/subtractinitial"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

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
	// timeseries for reset detection.
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
	// Create a copy of metrics to store the results.
	resultMetrics := pmetric.NewMetrics()
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)

		// Copy over resource info to the result.
		resResource := resultMetrics.ResourceMetrics().AppendEmpty()
		resResource.SetSchemaUrl(rm.SchemaUrl())
		rm.Resource().CopyTo(resResource.Resource())

		attrHash := pdatautil.MapHash(rm.Resource().Attributes())
		referenceTsm, _ := a.referenceCache.Get(attrHash)
		previousValueTsm, _ := a.previousValueCache.Get(attrHash)

		// The lock on the relevant timeseriesMap is held throughout the adjustment process to ensure that
		// nothing else can modify the data used for adjustment.
		referenceTsm.Lock()
		previousValueTsm.Lock()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)

			// Copy over scope info to the result.
			resScope := resResource.ScopeMetrics().AppendEmpty()
			resScope.SetSchemaUrl(ilm.SchemaUrl())
			ilm.Scope().CopyTo(resScope.Scope())

			for k := range ilm.Metrics().Len() {
				metric := ilm.Metrics().At(k)

				// Copy over metric info to the result.
				resMetric := resScope.Metrics().AppendEmpty()
				resMetric.SetName(metric.Name())
				resMetric.SetDescription(metric.Description())
				resMetric.SetUnit(metric.Unit())
				metric.Metadata().CopyTo(resMetric.Metadata())

				switch dataType := metric.Type(); dataType {
				case pmetric.MetricTypeGauge:
					// gauges don't need to be adjusted so no additional processing is necessary
					metric.CopyTo(resMetric)

				case pmetric.MetricTypeHistogram:
					adjustMetricHistogram(referenceTsm, previousValueTsm, metric, resMetric.SetEmptyHistogram())

				case pmetric.MetricTypeSummary:
					adjustMetricSummary(referenceTsm, previousValueTsm, metric, resMetric.SetEmptySummary())

				case pmetric.MetricTypeSum:
					adjustMetricSum(referenceTsm, previousValueTsm, metric, resMetric.SetEmptySum())

				case pmetric.MetricTypeExponentialHistogram:
					adjustMetricExponentialHistogram(referenceTsm, previousValueTsm, metric, resMetric.SetEmptyExponentialHistogram())

				case pmetric.MetricTypeEmpty:
					fallthrough

				default:
					a.set.Logger.Error("Adjust - skipping unexpected point", zap.String("type", dataType.String()))
				}
			}
		}
		referenceTsm.Unlock()
		previousValueTsm.Unlock()
	}

	return resultMetrics, nil
}

func adjustMetricHistogram(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, current pmetric.Metric, resHistogram pmetric.Histogram) {
	resHistogram.SetAggregationTemporality(current.Histogram().AggregationTemporality())

	histogram := current.Histogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		histogram.CopyTo(resHistogram)
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)
		pointStartTime := currentDist.StartTimestamp()
		if pointStartTime != 0 && pointStartTime != currentDist.Timestamp() {
			// Report point as is.
			currentDist.CopyTo(resHistogram.DataPoints().AppendEmpty())
			continue
		}

		referenceTsi, found := referenceTsm.Get(current, currentDist.Attributes())
		if !found {
			// First time we see this point. Skip it and use as a reference point for the next points.
			referenceTsi.Histogram = pmetric.NewHistogramDataPoint()
			currentDist.CopyTo(referenceTsi.Histogram)
			continue
		}

		// Adjust the datapoint based on the reference value.
		adjustedPoint := pmetric.NewHistogramDataPoint()
		currentDist.CopyTo(adjustedPoint)
		adjustedPoint.SetStartTimestamp(referenceTsi.Histogram.StartTimestamp())
		if adjustedPoint.Flags().NoRecordedValue() {
			adjustedPoint.CopyTo(resHistogram.DataPoints().AppendEmpty())
			continue
		}
		isReset := datapointstorage.IsResetHistogram(adjustedPoint, referenceTsi.Histogram)
		subtractHistogramDataPoint(adjustedPoint, referenceTsi.Histogram)

		previousTsi, found := previousValueTsm.Get(current, currentDist.Attributes())
		if isReset || (found && datapointstorage.IsResetHistogram(adjustedPoint, previousTsi.Histogram)) {
			// reset re-initialize everything and use the non adjusted points start time.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentDist.StartTimestamp().AsTime().Add(-1 * time.Millisecond))
			currentDist.SetStartTimestamp(resetStartTimeStamp)

			// Update the reference value with the current point.
			referenceTsi.Histogram = pmetric.NewHistogramDataPoint()
			previousTsi.Histogram = pmetric.NewHistogramDataPoint()
			referenceTsi.Histogram.SetStartTimestamp(currentDist.StartTimestamp())
			currentDist.ExplicitBounds().CopyTo(referenceTsi.Histogram.ExplicitBounds())
			referenceTsi.Histogram.BucketCounts().FromRaw(make([]uint64, currentDist.BucketCounts().Len()))

			currentDist.CopyTo(resHistogram.DataPoints().AppendEmpty())
			currentDist.CopyTo(previousTsi.Histogram)
			continue
		} else if !found {
			// First point after the reference. Not a reset.
			previousTsi.Histogram = pmetric.NewHistogramDataPoint()
		}

		// Update previous values with the current point.
		adjustedPoint.CopyTo(previousTsi.Histogram)
		adjustedPoint.CopyTo(resHistogram.DataPoints().AppendEmpty())
	}
}

func adjustMetricExponentialHistogram(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, current pmetric.Metric, resExpHistogram pmetric.ExponentialHistogram) {
	resExpHistogram.SetAggregationTemporality(current.ExponentialHistogram().AggregationTemporality())

	histogram := current.ExponentialHistogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		histogram.CopyTo(resExpHistogram)
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)
		pointStartTime := currentDist.StartTimestamp()
		if pointStartTime != 0 && pointStartTime != currentDist.Timestamp() {
			// Report point as is.
			currentDist.CopyTo(resExpHistogram.DataPoints().AppendEmpty())
			continue
		}

		referenceTsi, found := referenceTsm.Get(current, currentDist.Attributes())
		if !found {
			// First time we see this point. Skip it and use as a reference point for the next points.
			referenceTsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
			currentDist.CopyTo(referenceTsi.ExponentialHistogram)
			continue
		}

		// Adjust the datapoint based on the reference value.
		adjustedPoint := pmetric.NewExponentialHistogramDataPoint()
		currentDist.CopyTo(adjustedPoint)
		adjustedPoint.SetStartTimestamp(referenceTsi.ExponentialHistogram.StartTimestamp())
		if adjustedPoint.Flags().NoRecordedValue() {
			adjustedPoint.CopyTo(resExpHistogram.DataPoints().AppendEmpty())
			continue
		}

		isReset := datapointstorage.IsResetExponentialHistogram(adjustedPoint, referenceTsi.ExponentialHistogram)
		subtractExponentialHistogramDataPoint(adjustedPoint, referenceTsi.ExponentialHistogram)

		previousTsi, found := previousValueTsm.Get(current, currentDist.Attributes())
		if isReset || (found && datapointstorage.IsResetExponentialHistogram(adjustedPoint, previousTsi.ExponentialHistogram)) {
			// reset re-initialize everything and use the non adjusted points start time.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentDist.StartTimestamp().AsTime().Add(-1 * time.Millisecond))
			currentDist.SetStartTimestamp(resetStartTimeStamp)

			referenceTsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
			previousTsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
			referenceTsi.ExponentialHistogram.SetStartTimestamp(currentDist.StartTimestamp())
			referenceTsi.ExponentialHistogram.SetScale(currentDist.Scale())
			referenceTsi.ExponentialHistogram.Positive().BucketCounts().FromRaw(make([]uint64, currentDist.Positive().BucketCounts().Len()))
			referenceTsi.ExponentialHistogram.Negative().BucketCounts().FromRaw(make([]uint64, currentDist.Negative().BucketCounts().Len()))

			currentDist.CopyTo(resExpHistogram.DataPoints().AppendEmpty())
			currentDist.CopyTo(previousTsi.ExponentialHistogram)
			continue
		} else if !found {
			// First point after the reference. Not a reset.
			previousTsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
		}

		// Update previous values with the current point.
		adjustedPoint.CopyTo(previousTsi.ExponentialHistogram)
		adjustedPoint.CopyTo(resExpHistogram.DataPoints().AppendEmpty())
	}
}

func adjustMetricSum(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, current pmetric.Metric, resSum pmetric.Sum) {
	sum := current.Sum()
	if sum.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		sum.CopyTo(resSum)
		return
	}
	resSum.SetAggregationTemporality(sum.AggregationTemporality())
	resSum.SetIsMonotonic(sum.IsMonotonic())

	currentPoints := sum.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentSum := currentPoints.At(i)
		pointStartTime := currentSum.StartTimestamp()
		if pointStartTime != 0 && pointStartTime != currentSum.Timestamp() {
			// Report point as is.
			currentSum.CopyTo(resSum.DataPoints().AppendEmpty())
			continue
		}

		referenceTsi, found := referenceTsm.Get(current, currentSum.Attributes())
		if !found {
			// First time we see this point. Skip it and use as a reference point for the next points.
			referenceTsi.Number = pmetric.NewNumberDataPoint()
			currentSum.CopyTo(referenceTsi.Number)
			continue
		}

		// Adjust the datapoint based on the reference value.
		adjustedPoint := pmetric.NewNumberDataPoint()
		currentSum.CopyTo(adjustedPoint)
		adjustedPoint.SetStartTimestamp(referenceTsi.Number.StartTimestamp())
		if adjustedPoint.Flags().NoRecordedValue() {
			adjustedPoint.CopyTo(resSum.DataPoints().AppendEmpty())
			continue
		}
		isReset := datapointstorage.IsResetSum(adjustedPoint, referenceTsi.Number)
		adjustedPoint.SetDoubleValue(adjustedPoint.DoubleValue() - referenceTsi.Number.DoubleValue())

		previousTsi, found := previousValueTsm.Get(current, currentSum.Attributes())
		if isReset || (found && datapointstorage.IsResetSum(adjustedPoint, previousTsi.Number)) {
			// reset re-initialize everything and use the non adjusted points start time.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentSum.StartTimestamp().AsTime().Add(-1 * time.Millisecond))
			currentSum.SetStartTimestamp(resetStartTimeStamp)

			referenceTsi.Number = pmetric.NewNumberDataPoint()
			previousTsi.Number = pmetric.NewNumberDataPoint()
			referenceTsi.Number.SetStartTimestamp(currentSum.StartTimestamp())

			currentSum.CopyTo(resSum.DataPoints().AppendEmpty())
			currentSum.CopyTo(previousTsi.Number)
			continue
		} else if !found {
			// First point after the reference. Not a reset.
			previousTsi.Number = pmetric.NewNumberDataPoint()
		}

		// Update previous values with the current point.
		adjustedPoint.CopyTo(previousTsi.Number)
		adjustedPoint.CopyTo(resSum.DataPoints().AppendEmpty())
	}
}

func adjustMetricSummary(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, current pmetric.Metric, resSummary pmetric.Summary) {
	currentPoints := current.Summary().DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentSummary := currentPoints.At(i)
		pointStartTime := currentSummary.StartTimestamp()
		if pointStartTime != 0 && pointStartTime != currentSummary.Timestamp() {
			// Report point as is.
			currentSummary.CopyTo(resSummary.DataPoints().AppendEmpty())
			continue
		}

		referenceTsi, found := referenceTsm.Get(current, currentSummary.Attributes())
		if !found {
			// First time we see this point. Skip it and use as a reference point for the next points.
			referenceTsi.Summary = pmetric.NewSummaryDataPoint()
			currentSummary.CopyTo(referenceTsi.Summary)
			continue
		}

		// Adjust the datapoint based on the reference value.
		adjustedPoint := pmetric.NewSummaryDataPoint()
		currentSummary.CopyTo(adjustedPoint)
		adjustedPoint.SetStartTimestamp(referenceTsi.Summary.StartTimestamp())
		if adjustedPoint.Flags().NoRecordedValue() {
			adjustedPoint.CopyTo(resSummary.DataPoints().AppendEmpty())
			continue
		}

		isReset := datapointstorage.IsResetSummary(adjustedPoint, referenceTsi.Summary)
		adjustedPoint.SetCount(adjustedPoint.Count() - referenceTsi.Summary.Count())
		adjustedPoint.SetSum(adjustedPoint.Sum() - referenceTsi.Summary.Sum())

		previousTsi, found := previousValueTsm.Get(current, currentSummary.Attributes())
		if isReset || (found && datapointstorage.IsResetSummary(adjustedPoint, previousTsi.Summary)) {
			// reset re-initialize everything and use the non adjusted points start time.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentSummary.StartTimestamp().AsTime().Add(-1 * time.Millisecond))
			currentSummary.SetStartTimestamp(resetStartTimeStamp)

			referenceTsi.Summary = pmetric.NewSummaryDataPoint()
			previousTsi.Summary = pmetric.NewSummaryDataPoint()
			referenceTsi.Summary.SetStartTimestamp(currentSummary.StartTimestamp())

			currentSummary.CopyTo(resSummary.DataPoints().AppendEmpty())
			currentSummary.CopyTo(previousTsi.Summary)
			continue
		} else if !found {
			// First point after the reference. Not a reset.
			previousTsi.Summary = pmetric.NewSummaryDataPoint()
		}

		// Update previous values with the current point.
		adjustedPoint.CopyTo(previousTsi.Summary)
		adjustedPoint.CopyTo(resSummary.DataPoints().AppendEmpty())
	}
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
