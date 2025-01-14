// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"errors"
	"os"
	"regexp"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var (
	errNoStartTimeMetrics             = errors.New("start_time metric is missing")
	errNoDataPointsStartTimeMetric    = errors.New("start time metric with no data points")
	errUnsupportedTypeStartTimeMetric = errors.New("unsupported data type for start time metric")

	// collectorStartTime is the approximate start time of the collector. Used
	// as a fallback start time for metrics that don't have a start time set.
	// Set when the component is initialized.
	collectorStartTime *time.Time
)

func init() {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		panic("Unable to find current process" + err.Error())
	}

	ct, err := p.CreateTime()
	if err != nil {
		panic("Unable to find process start time" + err.Error())
	}

	startTime := time.Unix(ct/1000, 0) // Convert milliseconds to seconds
	collectorStartTime = &startTime
}

type startTimeMetricAdjuster struct {
	startTimeMetricRegex *regexp.Regexp
	fallbackStartTime    *time.Time
	logger               *zap.Logger
}

// NewStartTimeMetricAdjuster returns a new MetricsAdjuster that adjust metrics' start times based on a start time metric.
func NewStartTimeMetricAdjuster(logger *zap.Logger, startTimeMetricRegex *regexp.Regexp, useCollectorStartTimeFallback bool) MetricsAdjuster {
	var fallbackStartTime *time.Time
	if useCollectorStartTimeFallback {
		fallbackStartTime = collectorStartTime
	}
	return &startTimeMetricAdjuster{
		startTimeMetricRegex: startTimeMetricRegex,
		fallbackStartTime:    fallbackStartTime,
		logger:               logger,
	}
}

func (stma *startTimeMetricAdjuster) AdjustMetrics(metrics pmetric.Metrics) error {
	startTime, err := stma.getStartTime(metrics)
	if err != nil {
		if stma.fallbackStartTime == nil {
			return err
		}
		stma.logger.Info("Couldn't get start time for metrics. Using fallback start time.", zap.Error(err))
		startTime = float64(stma.fallbackStartTime.Unix())
	}

	startTimeTs := timestampFromFloat64(startTime)
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
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
						dp.SetStartTimestamp(startTimeTs)
					}

				case pmetric.MetricTypeSummary:
					dataPoints := metric.Summary().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						dp.SetStartTimestamp(startTimeTs)
					}

				case pmetric.MetricTypeHistogram:
					dataPoints := metric.Histogram().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						dp.SetStartTimestamp(startTimeTs)
					}

				case pmetric.MetricTypeExponentialHistogram:
					dataPoints := metric.ExponentialHistogram().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						dp.SetStartTimestamp(startTimeTs)
					}

				case pmetric.MetricTypeEmpty:
					fallthrough

				default:
					stma.logger.Warn("Unknown metric type", zap.String("type", metric.Type().String()))
				}
			}
		}
	}

	return nil
}

func (stma *startTimeMetricAdjuster) getStartTime(metrics pmetric.Metrics) (float64, error) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)
				if stma.matchStartTimeMetric(metric.Name()) {
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

					case pmetric.MetricTypeEmpty, pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary:
						fallthrough
					default:
						return 0, errUnsupportedTypeStartTimeMetric
					}
				}
			}
		}
	}
	return 0.0, errNoStartTimeMetrics
}

func (stma *startTimeMetricAdjuster) matchStartTimeMetric(metricName string) bool {
	if stma.startTimeMetricRegex != nil {
		return stma.startTimeMetricRegex.MatchString(metricName)
	}

	return metricName == startTimeMetricName
}
