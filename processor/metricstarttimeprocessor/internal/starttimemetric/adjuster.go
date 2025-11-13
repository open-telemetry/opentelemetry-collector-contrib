// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starttimemetric // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/starttimemetric"

import (
	"context"
	"errors"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/datapointstorage"
)

const (
	// Type is the value users can use to configure the start time metric adjuster.
	Type                = "start_time_metric"
	startTimeMetricName = "process_start_time_seconds"
)

var (
	errNoStartTimeMetrics             = errors.New("start_time metric is missing")
	errNoDataPointsStartTimeMetric    = errors.New("start time metric with no data points")
	errUnsupportedTypeStartTimeMetric = errors.New("unsupported data type for start time metric")
	// approximateCollectorStartTime is the approximate start time of the
	// collector. Used as a fallback start time for metrics when the start time
	// metric is not found. Set when the component is initialized.
	approximateCollectorStartTime time.Time
)

func init() {
	approximateCollectorStartTime = time.Now()
}

type Adjuster struct {
	referenceValueCache  *datapointstorage.Cache
	startTimeMetricRegex *regexp.Regexp
	set                  component.TelemetrySettings
}

// NewAdjuster returns a new Adjuster which adjust metrics' start times based on the initial received points.
func NewAdjuster(set component.TelemetrySettings, startTimeMetricRegex *regexp.Regexp, gcInterval time.Duration) *Adjuster {
	return &Adjuster{
		referenceValueCache:  datapointstorage.NewCache(gcInterval),
		set:                  set,
		startTimeMetricRegex: startTimeMetricRegex,
	}
}

// AdjustMetrics adjusts the start time of metrics based on a different metric in the batch.
func (a *Adjuster) AdjustMetrics(_ context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	startTime, err := a.getStartTime(metrics)
	if err != nil {
		a.set.Logger.Debug("Couldn't get start time for metrics. Using fallback start time.", zap.Error(err), zap.Time("fallback_start_time", approximateCollectorStartTime))
		startTime = float64(approximateCollectorStartTime.Unix())
	}

	startTimeTs := timestampFromFloat64(startTime)
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		attrHash := pdatautil.MapHash(rm.Resource().Attributes())
		tsm, _ := a.referenceValueCache.Get(attrHash)

		// The lock on the relevant timeseriesMap is held throughout the adjustment process to ensure that
		// nothing else can modify the data used for adjustment.
		tsm.Lock()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					continue

				case pmetric.MetricTypeSum:
					dataPoints := metric.Sum().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						if st := dp.StartTimestamp(); st != 0 && st != dp.Timestamp() {
							// Report point as is if the start timestamp is already set.
							continue
						}
						refTsi, found := tsm.Get(metric, dp.Attributes())
						if !found {
							refTsi.Number = datapointstorage.NumberInfo{StartTime: startTimeTs}
						} else if refTsi.IsResetSum(dp) {
							refTsi.Number.StartTime = pcommon.NewTimestampFromTime(dp.Timestamp().AsTime().Add(-1 * time.Millisecond))
						}
						refTsi.Number.PreviousDoubleValue = dp.DoubleValue()
						refTsi.Number.PreviousIntValue = dp.IntValue()
						dp.SetStartTimestamp(refTsi.Number.StartTime)
					}

				case pmetric.MetricTypeSummary:
					dataPoints := metric.Summary().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						if st := dp.StartTimestamp(); st != 0 && st != dp.Timestamp() {
							// Report point as is if the start timestamp is already set.
							continue
						}
						refTsi, found := tsm.Get(metric, dp.Attributes())
						if !found {
							refTsi.Summary = datapointstorage.SummaryInfo{StartTime: startTimeTs}
						} else if refTsi.IsResetSummary(dp) {
							refTsi.Summary.StartTime = pcommon.NewTimestampFromTime(dp.Timestamp().AsTime().Add(-1 * time.Millisecond))
						}
						refTsi.Summary.PreviousCount, refTsi.Summary.PreviousSum = dp.Count(), dp.Sum()
						dp.SetStartTimestamp(refTsi.Summary.StartTime)
					}

				case pmetric.MetricTypeHistogram:
					dataPoints := metric.Histogram().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						if st := dp.StartTimestamp(); st != 0 && st != dp.Timestamp() {
							// Report point as is if the start timestamp is already set.
							continue
						}
						refTsi, found := tsm.Get(metric, dp.Attributes())
						if !found {
							refTsi.Histogram = datapointstorage.HistogramInfo{StartTime: startTimeTs, ExplicitBounds: dp.ExplicitBounds().AsRaw()}
						} else if refTsi.IsResetHistogram(dp) {
							refTsi.Histogram.StartTime = pcommon.NewTimestampFromTime(dp.Timestamp().AsTime().Add(-1 * time.Millisecond))
						}
						refTsi.Histogram.PreviousCount, refTsi.Histogram.PreviousSum = dp.Count(), dp.Sum()
						refTsi.Histogram.PreviousBucketCounts = dp.BucketCounts().AsRaw()
						dp.SetStartTimestamp(refTsi.Histogram.StartTime)
					}

				case pmetric.MetricTypeExponentialHistogram:
					dataPoints := metric.ExponentialHistogram().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						if st := dp.StartTimestamp(); st != 0 && st != dp.Timestamp() {
							// Report point as is if the start timestamp is already set.
							continue
						}
						refTsi, found := tsm.Get(metric, dp.Attributes())
						if !found {
							refTsi.ExponentialHistogram = datapointstorage.ExponentialHistogramInfo{StartTime: startTimeTs, Scale: dp.Scale()}
						} else if refTsi.IsResetExponentialHistogram(dp) {
							refTsi.ExponentialHistogram.StartTime = pcommon.NewTimestampFromTime(dp.Timestamp().AsTime().Add(-1 * time.Millisecond))
						}
						refTsi.ExponentialHistogram.PreviousPositive = datapointstorage.NewExponentialHistogramBucketInfo(dp.Positive())
						refTsi.ExponentialHistogram.PreviousNegative = datapointstorage.NewExponentialHistogramBucketInfo(dp.Negative())
						refTsi.ExponentialHistogram.PreviousCount, refTsi.ExponentialHistogram.PreviousSum, refTsi.ExponentialHistogram.PreviousZeroCount = dp.Count(), dp.Sum(), dp.ZeroCount()
						dp.SetStartTimestamp(refTsi.ExponentialHistogram.StartTime)
					}

				default:
					a.set.Logger.Warn("Unknown metric type", zap.String("type", metric.Type().String()))
				}
			}
		}
		tsm.Unlock()
	}
	return metrics, nil
}

func timestampFromFloat64(ts float64) pcommon.Timestamp {
	secs := int64(ts)
	nanos := int64((ts - float64(secs)) * 1e9)
	return pcommon.Timestamp(secs*1e9 + nanos)
}

func (a *Adjuster) getStartTime(metrics pmetric.Metrics) (float64, error) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)
				if a.matchStartTimeMetric(metric.Name()) {
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						if metric.Gauge().DataPoints().Len() == 0 {
							return 0.0, errNoDataPointsStartTimeMetric
						}
						return metric.Gauge().DataPoints().At(0).DoubleValue(), nil

					case pmetric.MetricTypeSum:
						if metric.Sum().DataPoints().Len() == 0 {
							return 0.0, errNoDataPointsStartTimeMetric
						}
						return metric.Sum().DataPoints().At(0).DoubleValue(), nil

					default:
						return 0, errUnsupportedTypeStartTimeMetric
					}
				}
			}
		}
	}
	return 0.0, errNoStartTimeMetrics
}

func (a *Adjuster) matchStartTimeMetric(metricName string) bool {
	if a.startTimeMetricRegex != nil {
		return a.startTimeMetricRegex.MatchString(metricName)
	}

	return metricName == startTimeMetricName
}
