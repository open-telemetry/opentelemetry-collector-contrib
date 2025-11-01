// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package truereset // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/truereset"

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

// Type is the value users can use to configure the true reset point adjuster.
// The true reset point adjuster sets the start time of all points in a series following:
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#cumulative-streams-inserting-true-reset-points.
// This involves setting the start time using the following strategy:
//   - The initial point in a series has its start time set to that point's end time.
//   - All subsequent points in the series have their start time set to the initial point's end time.
//
// Note that when a reset is detected (eg: value of a counter is decreasing) - the strategy will set the
// start time of the reset point as point timestamp - 1ms.
const Type = "true_reset_point"

// Adjuster takes a map from a metric instance to the initial point in the metrics instance
// and provides AdjustMetric, which takes a sequence of metrics and adjust their start times based on
// the initial points.
type Adjuster struct {
	startTimeCache *datapointstorage.Cache
	set            component.TelemetrySettings
}

// NewAdjuster returns a new Adjuster which adjust metrics' start times based on the initial received points.
func NewAdjuster(set component.TelemetrySettings, gcInterval time.Duration) *Adjuster {
	return &Adjuster{
		startTimeCache: datapointstorage.NewCache(gcInterval),
		set:            set,
	}
}

// AdjustMetrics takes a sequence of metrics and adjust their start times based on the initial and
// previous points in the timeseriesMap.
func (a *Adjuster) AdjustMetrics(_ context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		attrHash := pdatautil.MapHash(rm.Resource().Attributes())
		tsm, _ := a.startTimeCache.Get(attrHash)

		// The lock on the relevant timeseriesMap is held throughout the adjustment process to ensure that
		// nothing else can modify the data used for adjustment.
		tsm.Lock()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)
				switch dataType := metric.Type(); dataType {
				case pmetric.MetricTypeGauge:
					// gauges don't need to be adjusted so no additional processing is necessary

				case pmetric.MetricTypeHistogram:
					a.adjustMetricHistogram(tsm, metric)

				case pmetric.MetricTypeSummary:
					a.adjustMetricSummary(tsm, metric)

				case pmetric.MetricTypeSum:
					a.adjustMetricSum(tsm, metric)

				case pmetric.MetricTypeExponentialHistogram:
					a.adjustMetricExponentialHistogram(tsm, metric)

				default:
					// this shouldn't happen
					a.set.Logger.Info("Adjust - skipping unexpected point", zap.String("type", dataType.String()))
				}
			}
		}
		tsm.Unlock()
	}
	return metrics, nil
}

func (*Adjuster) adjustMetricHistogram(tsm *datapointstorage.TimeseriesMap, current pmetric.Metric) {
	histogram := current.Histogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)
		if st := currentDist.StartTimestamp(); st != 0 && st != currentDist.Timestamp() {
			// Report point as is if the start timestamp is already set.
			continue
		}

		refTsi, found := tsm.Get(current, currentDist.Attributes())
		if !found {
			// initialize everything.
			refTsi.Histogram = datapointstorage.HistogramInfo{
				PreviousCount: currentDist.Count(), PreviousSum: currentDist.Sum(),
				StartTime:            currentDist.Timestamp(),
				PreviousBucketCounts: currentDist.BucketCounts().AsRaw(),
				ExplicitBounds:       currentDist.ExplicitBounds().AsRaw(),
			}

			// For the first point, set the start time as the point timestamp.
			currentDist.SetStartTimestamp(currentDist.Timestamp())
			continue
		}

		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentDist.SetStartTimestamp(refTsi.Histogram.StartTime)
			continue
		}

		if refTsi.IsResetHistogram(currentDist) {
			// reset re-initialize everything.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentDist.Timestamp().AsTime().Add(-1 * time.Millisecond))
			refTsi.Histogram.StartTime = resetStartTimeStamp
		}

		// Update only previous values.
		refTsi.Histogram.PreviousCount, refTsi.Histogram.PreviousSum = currentDist.Count(), currentDist.Sum()
		refTsi.Histogram.PreviousBucketCounts = currentDist.BucketCounts().AsRaw()
		currentDist.SetStartTimestamp(refTsi.Histogram.StartTime)
	}
}

func (*Adjuster) adjustMetricExponentialHistogram(tsm *datapointstorage.TimeseriesMap, current pmetric.Metric) {
	histogram := current.ExponentialHistogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)
		if st := currentDist.StartTimestamp(); st != 0 && st != currentDist.Timestamp() {
			// Report point as is if the start timestamp is already set.
			continue
		}

		refTsi, found := tsm.Get(current, currentDist.Attributes())
		if !found {
			// initialize everything.
			refTsi.ExponentialHistogram = datapointstorage.ExponentialHistogramInfo{
				PreviousCount: currentDist.Count(), PreviousSum: currentDist.Sum(), PreviousZeroCount: currentDist.ZeroCount(),
				Scale:            currentDist.Scale(),
				StartTime:        currentDist.Timestamp(),
				PreviousPositive: datapointstorage.NewExponentialHistogramBucketInfo(currentDist.Positive()),
				PreviousNegative: datapointstorage.NewExponentialHistogramBucketInfo(currentDist.Negative()),
			}

			// For the first point, set the start time as the point timestamp.
			currentDist.SetStartTimestamp(currentDist.Timestamp())
			continue
		}

		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentDist.SetStartTimestamp(refTsi.ExponentialHistogram.StartTime)
			continue
		}

		if refTsi.IsResetExponentialHistogram(currentDist) {
			// reset re-initialize everything.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentDist.Timestamp().AsTime().Add(-1 * time.Millisecond))
			refTsi.ExponentialHistogram.StartTime = resetStartTimeStamp
			refTsi.ExponentialHistogram.Scale = currentDist.Scale()
		}

		// Update only previous values.
		refTsi.ExponentialHistogram.PreviousPositive = datapointstorage.NewExponentialHistogramBucketInfo(currentDist.Positive())
		refTsi.ExponentialHistogram.PreviousNegative = datapointstorage.NewExponentialHistogramBucketInfo(currentDist.Negative())
		refTsi.ExponentialHistogram.PreviousCount, refTsi.ExponentialHistogram.PreviousSum, refTsi.ExponentialHistogram.PreviousZeroCount = currentDist.Count(), currentDist.Sum(), currentDist.ZeroCount()
		currentDist.SetStartTimestamp(refTsi.ExponentialHistogram.StartTime)
	}
}

func (*Adjuster) adjustMetricSum(tsm *datapointstorage.TimeseriesMap, current pmetric.Metric) {
	currentPoints := current.Sum().DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentSum := currentPoints.At(i)
		if st := currentSum.StartTimestamp(); st != 0 && st != currentSum.Timestamp() {
			// Report point as is if the start timestamp is already set.
			continue
		}

		refTsi, found := tsm.Get(current, currentSum.Attributes())
		if !found {
			// initialize everything.
			refTsi.Number = datapointstorage.NumberInfo{
				PreviousDoubleValue: currentSum.DoubleValue(),
				PreviousIntValue:    currentSum.IntValue(),
				StartTime:           currentSum.Timestamp(),
			}

			// For the first point, set the start time as the point timestamp.
			currentSum.SetStartTimestamp(currentSum.Timestamp())
			continue
		}

		if currentSum.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentSum.SetStartTimestamp(refTsi.Number.StartTime)
			continue
		}

		if refTsi.IsResetSum(currentSum) {
			// reset re-initialize everything.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentSum.Timestamp().AsTime().Add(-1 * time.Millisecond))
			refTsi.Number.StartTime = resetStartTimeStamp
		}

		// Update only previous values.
		refTsi.Number.PreviousDoubleValue = currentSum.DoubleValue()
		refTsi.Number.PreviousIntValue = currentSum.IntValue()
		currentSum.SetStartTimestamp(refTsi.Number.StartTime)
	}
}

func (*Adjuster) adjustMetricSummary(tsm *datapointstorage.TimeseriesMap, current pmetric.Metric) {
	currentPoints := current.Summary().DataPoints()

	for i := 0; i < currentPoints.Len(); i++ {
		currentSummary := currentPoints.At(i)
		if st := currentSummary.StartTimestamp(); st != 0 && st != currentSummary.Timestamp() {
			// Report point as is if the start timestamp is already set.
			continue
		}

		refTsi, found := tsm.Get(current, currentSummary.Attributes())
		if !found {
			// initialize everything.
			refTsi.Summary = datapointstorage.SummaryInfo{
				PreviousCount: currentSummary.Count(), PreviousSum: currentSummary.Sum(),
				StartTime: currentSummary.Timestamp(),
			}

			// For the first point, set the start time as the point timestamp.
			currentSummary.SetStartTimestamp(currentSummary.Timestamp())
			continue
		}

		if currentSummary.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentSummary.SetStartTimestamp(refTsi.Summary.StartTime)
			continue
		}

		if refTsi.IsResetSummary(currentSummary) {
			// reset re-initialize everything.
			resetStartTimeStamp := pcommon.NewTimestampFromTime(currentSummary.Timestamp().AsTime().Add(-1 * time.Millisecond))
			refTsi.Summary.StartTime = resetStartTimeStamp
		}

		// Update only previous values.
		refTsi.Summary.PreviousCount, refTsi.Summary.PreviousSum = currentSummary.Count(), currentSummary.Sum()
		currentSummary.SetStartTimestamp(refTsi.Summary.StartTime)
	}
}
