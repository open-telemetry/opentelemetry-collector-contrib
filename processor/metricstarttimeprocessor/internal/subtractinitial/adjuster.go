// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subtractinitial // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/subtractinitial"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/datapointstorage"
)

// Type is the value users can use to configure the subtract initial point adjuster.
// The subtract initial point adjuster sets the start time of all points in a series by:
//   - Dropping the initial point, and recording its value and timestamp.
//   - Subtracting the initial point from all subsequent points, and using the timestamp of the initial point as the start timestamp.
const Type = "subtract_initial_point"

type Adjuster struct {
	referenceCache     *datapointstorage.DataPointCache
	previousValueCache *datapointstorage.DataPointCache
	set                component.TelemetrySettings
}

// NewAdjuster returns a new Adjuster which adjust metrics' start times based on the initial received points.
func NewAdjuster(set component.TelemetrySettings, gcInterval time.Duration) *Adjuster {
	return &Adjuster{
		referenceCache:     datapointstorage.NewDataPointCache(gcInterval),
		previousValueCache: datapointstorage.NewDataPointCache(gcInterval),
		set:                set,
	}
}

// AdjustMetrics takes a sequence of metrics and adjust their start times based on the initial and
// previous points in the timeseriesMap.
// For each metric:
// - Check if it exists in the map already.
// - If it doesn't, save its value in the reference cache and move on
// - If it does, find its reference cache value
// - Add new entry to the result with the value - reference value
// - When a reset is discovered, update the reference as well.
func (a *Adjuster) AdjustMetrics(_ context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	// Create a copy of metrics to store the results
	resultMetrics := pmetric.NewMetrics()
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)

		// Copy over resource info to the result.
		resResource := resultMetrics.ResourceMetrics().AppendEmpty()
		resResource.SetSchemaUrl(rm.SchemaUrl())
		rm.Resource().CopyTo(resResource.Resource())

		attrHash := pdatautil.MapHash(rm.Resource().Attributes())
		referenceTsm := a.referenceCache.Get(attrHash)
		previousValueTsm := a.previousValueCache.Get(attrHash)

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

			for k := 0; k < ilm.Metrics().Len(); k++ {
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
					a.adjustMetricHistogram(referenceTsm, previousValueTsm, metric, resMetric.SetEmptyHistogram())

				case pmetric.MetricTypeSummary:
					a.adjustMetricSummary(referenceTsm, previousValueTsm, metric, resMetric.SetEmptySummary())

				case pmetric.MetricTypeSum:
					a.adjustMetricSum(referenceTsm, previousValueTsm, metric, resMetric.SetEmptySum())

				case pmetric.MetricTypeExponentialHistogram:
					a.adjustMetricExponentialHistogram(referenceTsm, previousValueTsm, metric, resMetric.SetEmptyExponentialHistogram())

				case pmetric.MetricTypeEmpty:
					fallthrough

				default:
					// this shouldn't happen
					a.set.Logger.Info("Adjust - skipping unexpected point", zap.String("type", dataType.String()))
				}
			}
		}
		referenceTsm.Unlock()
		previousValueTsm.Unlock()
	}

	return resultMetrics, nil
}

func (a *Adjuster) adjustMetricHistogram(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, current pmetric.Metric, resHistogram pmetric.Histogram) {
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

		referenceTsi, found := referenceTsm.Get(current, currentDist.Attributes())
		if !found {
			// initialize everything. Don't add the datapoint to the result.
			referenceTsi.Histogram.StartTime = currentDist.StartTimestamp()
			referenceTsi.Histogram.Count = currentDist.Count()
			referenceTsi.Histogram.Sum = currentDist.Sum()
			continue
		}

		// Adjust the datapoint based on the reference value.
		adjustedPoint := pmetric.NewHistogramDataPoint()
		currentDist.CopyTo(adjustedPoint)
		adjustedPoint.SetStartTimestamp(referenceTsi.Histogram.StartTime)
		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			tmp := resHistogram.DataPoints().AppendEmpty()
			adjustedPoint.CopyTo(tmp)
			continue
		}
		adjustedPoint.SetCount(adjustedPoint.Count() - referenceTsi.Histogram.Count)
		adjustedPoint.SetSum(adjustedPoint.Sum() - referenceTsi.Histogram.Sum)

		previousTsi, found := previousValueTsm.Get(current, currentDist.Attributes())
		if !found {
			// First point after the reference. Not a reset.
		} else if adjustedPoint.Count() < previousTsi.Histogram.Count || adjustedPoint.Sum() < previousTsi.Histogram.Sum {
			// reset re-initialize everything using the non adjusted points start time.
			referenceTsi.Histogram.StartTime = currentDist.StartTimestamp()
			referenceTsi.Histogram.Count = 0
			referenceTsi.Histogram.Sum = 0

			adjustedPoint.SetCount(currentDist.Count())
			adjustedPoint.SetSum(currentDist.Sum())
			adjustedPoint.SetStartTimestamp(currentDist.StartTimestamp())

			tmp := resHistogram.DataPoints().AppendEmpty()
			adjustedPoint.CopyTo(tmp)
			continue
		}

		// Update previous values with the current point.
		previousTsi.Histogram.Count = adjustedPoint.Count()
		previousTsi.Histogram.Sum = adjustedPoint.Sum()
		previousTsi.Histogram.StartTime = adjustedPoint.StartTimestamp()

		tmp := resHistogram.DataPoints().AppendEmpty()
		adjustedPoint.CopyTo(tmp)
	}
}

func (a *Adjuster) adjustMetricExponentialHistogram(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, current pmetric.Metric, resExpHistogram pmetric.ExponentialHistogram) {
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

		referenceTsi, found := referenceTsm.Get(current, currentDist.Attributes())
		if !found {
			// initialize everything. Don't add the datapoint to the result.
			referenceTsi.Histogram.StartTime = currentDist.StartTimestamp()
			referenceTsi.Histogram.Count = currentDist.Count()
			referenceTsi.Histogram.Sum = currentDist.Sum()
			continue
		}

		// Adjust the datapoint based on the reference value.
		adjustedPoint := pmetric.NewExponentialHistogramDataPoint()
		currentDist.CopyTo(adjustedPoint)
		adjustedPoint.SetStartTimestamp(referenceTsi.Histogram.StartTime)
		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			tmp := resExpHistogram.DataPoints().AppendEmpty()
			adjustedPoint.CopyTo(tmp)
			continue
		}
		adjustedPoint.SetCount(adjustedPoint.Count() - referenceTsi.Histogram.Count)
		adjustedPoint.SetSum(adjustedPoint.Sum() - referenceTsi.Histogram.Sum)

		previousTsi, found := previousValueTsm.Get(current, currentDist.Attributes())
		if !found {
			// First point after the reference. Not a reset.
		} else if adjustedPoint.Count() < previousTsi.Histogram.Count || adjustedPoint.Sum() < previousTsi.Histogram.Sum {
			// reset re-initialize everything using the non adjusted points start time.
			referenceTsi.Histogram.StartTime = currentDist.StartTimestamp()
			referenceTsi.Histogram.Count = 0
			referenceTsi.Histogram.Sum = 0

			adjustedPoint.SetCount(currentDist.Count())
			adjustedPoint.SetSum(currentDist.Sum())
			adjustedPoint.SetStartTimestamp(currentDist.StartTimestamp())

			tmp := resExpHistogram.DataPoints().AppendEmpty()
			adjustedPoint.CopyTo(tmp)
			continue
		}

		// Update previous values with the current point.
		previousTsi.Histogram.Count = adjustedPoint.Count()
		previousTsi.Histogram.Sum = adjustedPoint.Sum()
		previousTsi.Histogram.StartTime = adjustedPoint.StartTimestamp()

		tmp := resExpHistogram.DataPoints().AppendEmpty()
		adjustedPoint.CopyTo(tmp)
	}
}

func (a *Adjuster) adjustMetricSum(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, current pmetric.Metric, resSum pmetric.Sum) {
	resSum.SetAggregationTemporality(current.Sum().AggregationTemporality())
	resSum.SetIsMonotonic(current.Sum().IsMonotonic())

	currentPoints := current.Sum().DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)

		referenceTsi, found := referenceTsm.Get(current, currentDist.Attributes())
		if !found {
			// initialize everything. Don't add the datapoint to the result.
			referenceTsi.Number.StartTime = currentDist.StartTimestamp()
			referenceTsi.Number.Value = currentDist.DoubleValue()
			continue
		}

		// Adjust the datapoint based on the reference value.
		adjustedPoint := pmetric.NewNumberDataPoint()
		currentDist.CopyTo(adjustedPoint)
		adjustedPoint.SetStartTimestamp(referenceTsi.Number.StartTime)
		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			tmp := resSum.DataPoints().AppendEmpty()
			adjustedPoint.CopyTo(tmp)
			continue
		}
		adjustedPoint.SetDoubleValue(adjustedPoint.DoubleValue() - referenceTsi.Number.Value)

		previousTsi, found := previousValueTsm.Get(current, currentDist.Attributes())
		if !found {
			// First point after the reference. Not a reset.
		} else if adjustedPoint.DoubleValue() < previousTsi.Number.Value {
			// reset re-initialize everything using the non adjusted points start time.
			referenceTsi.Number.StartTime = currentDist.StartTimestamp()
			referenceTsi.Number.Value = 0

			adjustedPoint.SetDoubleValue(currentDist.DoubleValue())
			adjustedPoint.SetStartTimestamp(currentDist.StartTimestamp())

			tmp := resSum.DataPoints().AppendEmpty()
			adjustedPoint.CopyTo(tmp)
			continue
		}

		// Update previous values with the current point.
		previousTsi.Number.Value = adjustedPoint.DoubleValue()
		previousTsi.Number.StartTime = adjustedPoint.StartTimestamp()

		tmp := resSum.DataPoints().AppendEmpty()
		adjustedPoint.CopyTo(tmp)
	}
}

func (a *Adjuster) adjustMetricSummary(referenceTsm, previousValueTsm *datapointstorage.TimeseriesMap, current pmetric.Metric, resSummary pmetric.Summary) {
	currentPoints := current.Summary().DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)

		referenceTsi, found := referenceTsm.Get(current, currentDist.Attributes())
		if !found {
			// initialize everything. Don't add the datapoint to the result.
			referenceTsi.Summary.StartTime = currentDist.StartTimestamp()
			referenceTsi.Summary.Count = currentDist.Count()
			referenceTsi.Summary.Sum = currentDist.Sum()
			continue
		}

		// Adjust the datapoint based on the reference value.
		adjustedPoint := pmetric.NewSummaryDataPoint()
		currentDist.CopyTo(adjustedPoint)
		adjustedPoint.SetStartTimestamp(referenceTsi.Summary.StartTime)
		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			tmp := resSummary.DataPoints().AppendEmpty()
			adjustedPoint.CopyTo(tmp)
			continue
		}
		adjustedPoint.SetCount(adjustedPoint.Count() - referenceTsi.Summary.Count)
		adjustedPoint.SetSum(adjustedPoint.Sum() - referenceTsi.Summary.Sum)

		previousTsi, found := previousValueTsm.Get(current, currentDist.Attributes())
		if !found {
			// First point after the reference. Not a reset.
		} else if adjustedPoint.Count() < previousTsi.Summary.Count || adjustedPoint.Sum() < previousTsi.Summary.Sum {
			// reset re-initialize everything using the non adjusted points start time.
			referenceTsi.Summary.StartTime = currentDist.StartTimestamp()
			referenceTsi.Summary.Count = 0
			referenceTsi.Summary.Sum = 0

			adjustedPoint.SetCount(currentDist.Count())
			adjustedPoint.SetSum(currentDist.Sum())
			adjustedPoint.SetStartTimestamp(currentDist.StartTimestamp())

			tmp := resSummary.DataPoints().AppendEmpty()
			adjustedPoint.CopyTo(tmp)
			continue
		}

		// Update previous values with the current point.
		previousTsi.Summary.Count = adjustedPoint.Count()
		previousTsi.Summary.Sum = adjustedPoint.Sum()
		previousTsi.Summary.StartTime = adjustedPoint.StartTimestamp()

		tmp := resSummary.DataPoints().AppendEmpty()
		adjustedPoint.CopyTo(tmp)
	}
}

// subtractHistogramDataPoint subtracts b from a.
func subtractHistogramDataPoint(a, b pmetric.HistogramDataPoint) {
	// Use the timestamp from the normalization point
	a.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	a.SetCount(a.Count() - b.Count())
	// We drop points without a sum, so no need to check here.
	a.SetSum(a.Sum() - b.Sum())
	aBuckets := a.BucketCounts()
	bBuckets := b.BucketCounts()
	newBuckets := make([]uint64, aBuckets.Len())
	for i := 0; i < aBuckets.Len(); i++ {
		newBuckets[i] = aBuckets.At(i) - bBuckets.At(i)
	}
	a.BucketCounts().FromRaw(newBuckets)
}

// subtractExponentialHistogramDataPoint subtracts b from a.
func subtractExponentialHistogramDataPoint(a, b pmetric.ExponentialHistogramDataPoint) {
	// Use the timestamp from the normalization point
	a.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	a.SetCount(a.Count() - b.Count())
	// We drop points without a sum, so no need to check here.
	a.SetSum(a.Sum() - b.Sum())
	a.SetZeroCount(a.ZeroCount() - b.ZeroCount())
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
