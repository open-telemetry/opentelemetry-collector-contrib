// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package truereset // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/truereset"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
)

const Type = "true_reset"

// InitialPointAdjuster takes a map from a metric instance to the initial point in the metrics instance
// and provides AdjustMetricSlice, which takes a sequence of metrics and adjust their start times based on
// the initial points.
type InitialPointAdjuster struct {
	jobsMap *JobsMap
	set     component.TelemetrySettings
}

// NewInitialPointAdjuster returns a new MetricsAdjuster that adjust metrics' start times based on the initial received points.
func NewInitialPointAdjuster(set component.TelemetrySettings, gcInterval time.Duration) *InitialPointAdjuster {
	return &InitialPointAdjuster{
		jobsMap: NewJobsMap(gcInterval),
		set:     set,
	}
}

// AdjustMetrics takes a sequence of metrics and adjust their start times based on the initial and
// previous points in the timeseriesMap.
func (a *InitialPointAdjuster) AdjustMetrics(_ context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		_, found := rm.Resource().Attributes().Get(semconv.AttributeServiceName)
		if !found {
			return metrics, errors.New("adjusting metrics without service.name")
		}

		_, found = rm.Resource().Attributes().Get(semconv.AttributeServiceInstanceID)
		if !found {
			return metrics, errors.New("adjusting metrics without service.instance.id")
		}
	}

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		job, _ := rm.Resource().Attributes().Get(semconv.AttributeServiceName)
		instance, _ := rm.Resource().Attributes().Get(semconv.AttributeServiceInstanceID)
		tsm := a.jobsMap.get(job.Str(), instance.Str())

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

func (a *InitialPointAdjuster) adjustMetricHistogram(tsm *timeseriesMap, current pmetric.Metric) {
	histogram := current.Histogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)

		tsi, found := tsm.get(current, currentDist.Attributes())
		if !found {
			// initialize everything.
			tsi.histogram.startTime = currentDist.StartTimestamp()
			tsi.histogram.previousCount = currentDist.Count()
			tsi.histogram.previousSum = currentDist.Sum()
			continue
		}

		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentDist.SetStartTimestamp(tsi.histogram.startTime)
			continue
		}

		if currentDist.Count() < tsi.histogram.previousCount || currentDist.Sum() < tsi.histogram.previousSum {
			// reset re-initialize everything.
			tsi.histogram.startTime = currentDist.StartTimestamp()
			tsi.histogram.previousCount = currentDist.Count()
			tsi.histogram.previousSum = currentDist.Sum()
			continue
		}

		// Update only previous values.
		tsi.histogram.previousCount = currentDist.Count()
		tsi.histogram.previousSum = currentDist.Sum()
		currentDist.SetStartTimestamp(tsi.histogram.startTime)
	}
}

func (a *InitialPointAdjuster) adjustMetricExponentialHistogram(tsm *timeseriesMap, current pmetric.Metric) {
	histogram := current.ExponentialHistogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)

		tsi, found := tsm.get(current, currentDist.Attributes())
		if !found {
			// initialize everything.
			tsi.histogram.startTime = currentDist.StartTimestamp()
			tsi.histogram.previousCount = currentDist.Count()
			tsi.histogram.previousSum = currentDist.Sum()
			continue
		}

		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentDist.SetStartTimestamp(tsi.histogram.startTime)
			continue
		}

		if currentDist.Count() < tsi.histogram.previousCount || currentDist.Sum() < tsi.histogram.previousSum {
			// reset re-initialize everything.
			tsi.histogram.startTime = currentDist.StartTimestamp()
			tsi.histogram.previousCount = currentDist.Count()
			tsi.histogram.previousSum = currentDist.Sum()
			continue
		}

		// Update only previous values.
		tsi.histogram.previousCount = currentDist.Count()
		tsi.histogram.previousSum = currentDist.Sum()
		currentDist.SetStartTimestamp(tsi.histogram.startTime)
	}
}

func (a *InitialPointAdjuster) adjustMetricSum(tsm *timeseriesMap, current pmetric.Metric) {
	currentPoints := current.Sum().DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentSum := currentPoints.At(i)

		tsi, found := tsm.get(current, currentSum.Attributes())
		if !found {
			// initialize everything.
			tsi.number.startTime = currentSum.StartTimestamp()
			tsi.number.previousValue = currentSum.DoubleValue()
			continue
		}

		if currentSum.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentSum.SetStartTimestamp(tsi.number.startTime)
			continue
		}

		if currentSum.DoubleValue() < tsi.number.previousValue {
			// reset re-initialize everything.
			tsi.number.startTime = currentSum.StartTimestamp()
			tsi.number.previousValue = currentSum.DoubleValue()
			continue
		}

		// Update only previous values.
		tsi.number.previousValue = currentSum.DoubleValue()
		currentSum.SetStartTimestamp(tsi.number.startTime)
	}
}

func (a *InitialPointAdjuster) adjustMetricSummary(tsm *timeseriesMap, current pmetric.Metric) {
	currentPoints := current.Summary().DataPoints()

	for i := 0; i < currentPoints.Len(); i++ {
		currentSummary := currentPoints.At(i)

		tsi, found := tsm.get(current, currentSummary.Attributes())
		if !found {
			// initialize everything.
			tsi.summary.startTime = currentSummary.StartTimestamp()
			tsi.summary.previousCount = currentSummary.Count()
			tsi.summary.previousSum = currentSummary.Sum()
			continue
		}

		if currentSummary.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentSummary.SetStartTimestamp(tsi.summary.startTime)
			continue
		}

		if (currentSummary.Count() != 0 &&
			tsi.summary.previousCount != 0 &&
			currentSummary.Count() < tsi.summary.previousCount) ||
			(currentSummary.Sum() != 0 &&
				tsi.summary.previousSum != 0 &&
				currentSummary.Sum() < tsi.summary.previousSum) {
			// reset re-initialize everything.
			tsi.summary.startTime = currentSummary.StartTimestamp()
			tsi.summary.previousCount = currentSummary.Count()
			tsi.summary.previousSum = currentSummary.Sum()
			continue
		}

		// Update only previous values.
		tsi.summary.previousCount = currentSummary.Count()
		tsi.summary.previousSum = currentSummary.Sum()
		currentSummary.SetStartTimestamp(tsi.summary.startTime)
	}
}
