// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package truereset // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/truereset"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
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

				case pmetric.MetricTypeEmpty:
					fallthrough

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

func (a *Adjuster) adjustMetricHistogram(tsm *datapointstorage.TimeseriesMap, current pmetric.Metric) {
	histogram := current.Histogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)

		tsi, found := tsm.Get(current, currentDist.Attributes())
		if !found {
			// initialize everything.
			tsi.Histogram = pmetric.NewHistogramDataPoint()
			currentDist.CopyTo(tsi.Histogram)
			continue
		}

		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentDist.SetStartTimestamp(tsi.Histogram.StartTimestamp())
			continue
		}

		if tsi.IsResetHistogram(currentDist) {
			// reset re-initialize everything.
			currentDist.CopyTo(tsi.Histogram)
			continue
		}

		// Update only previous values.
		currentDist.SetStartTimestamp(tsi.Histogram.StartTimestamp())
		currentDist.CopyTo(tsi.Histogram)
	}
}

func (a *Adjuster) adjustMetricExponentialHistogram(tsm *datapointstorage.TimeseriesMap, current pmetric.Metric) {
	histogram := current.ExponentialHistogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)

		tsi, found := tsm.Get(current, currentDist.Attributes())
		if !found {
			// initialize everything.
			tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
			currentDist.CopyTo(tsi.ExponentialHistogram)
			continue
		}

		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentDist.SetStartTimestamp(tsi.ExponentialHistogram.StartTimestamp())
			continue
		}

		if tsi.IsResetExponentialHistogram(currentDist) {
			// reset re-initialize everything.
			currentDist.CopyTo(tsi.ExponentialHistogram)
			continue
		}

		// Update only previous values.
		currentDist.SetStartTimestamp(tsi.ExponentialHistogram.StartTimestamp())
		currentDist.CopyTo(tsi.ExponentialHistogram)
	}
}

func (a *Adjuster) adjustMetricSum(tsm *datapointstorage.TimeseriesMap, current pmetric.Metric) {
	currentPoints := current.Sum().DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentSum := currentPoints.At(i)

		tsi, found := tsm.Get(current, currentSum.Attributes())
		if !found {
			// initialize everything.
			tsi.Number = pmetric.NewNumberDataPoint()
			currentSum.CopyTo(tsi.Number)
			continue
		}

		if currentSum.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentSum.SetStartTimestamp(tsi.Number.StartTimestamp())
			continue
		}

		if tsi.IsResetSum(currentSum) {
			// reset re-initialize everything.
			currentSum.CopyTo(tsi.Number)
			continue
		}

		// Update only previous values.
		currentSum.SetStartTimestamp(tsi.Number.StartTimestamp())
		currentSum.CopyTo(tsi.Number)
	}
}

func (a *Adjuster) adjustMetricSummary(tsm *datapointstorage.TimeseriesMap, current pmetric.Metric) {
	currentPoints := current.Summary().DataPoints()

	for i := 0; i < currentPoints.Len(); i++ {
		currentSummary := currentPoints.At(i)

		tsi, found := tsm.Get(current, currentSummary.Attributes())
		if !found {
			// initialize everything.
			tsi.Summary = pmetric.NewSummaryDataPoint()
			currentSummary.CopyTo(tsi.Summary)
			continue
		}

		if currentSummary.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentSummary.SetStartTimestamp(tsi.Summary.StartTimestamp())
			continue
		}

		if tsi.IsResetSummary(currentSummary) {
			// reset re-initialize everything.
			currentSummary.CopyTo(tsi.Summary)
			continue
		}

		// Update only previous values.
		currentSummary.SetStartTimestamp(tsi.Summary.StartTimestamp())
		currentSummary.CopyTo(tsi.Summary)
	}
}
