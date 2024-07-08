// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"errors"
	"fmt"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"regexp"
	"time"
)

var (
	metricNamePattern = regexp.MustCompile(`(\w+)_(\w+)_(\w+)\z`)
	nameLabel         = "__name__"
)

func FromTimeSeries(timeSeries []prompb.TimeSeries, settings PRWToMetricSettings) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()

	for _, ts := range timeSeries {
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
		metric := scopeMetrics.Metrics().AppendEmpty()

		metricName, err := getMetricName(ts.Labels)
		if err != nil {
			return metrics, err
		}
		metric.SetName(metricName)
		settings.Logger.Debug("Metric name", zap.String("metric_name", metric.Name()))

		metricType, unit := getMetricTypeAndUnit(metricName)
		if unit != "" {
			metric.SetUnit(unit)
		}
		settings.Logger.Debug("Metric unit", zap.String("metric_name", metric.Name()), zap.String("metric_unit", metric.Unit()))

		for _, sample := range ts.Samples {
			dataPoint := pmetric.NewNumberDataPoint()
			dataPoint.SetDoubleValue(sample.Value)
			dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, sample.Timestamp*int64(time.Millisecond))))

			if isOlderThanThreshold(dataPoint.Timestamp().AsTime(), settings.TimeThreshold) {
				settings.Logger.Debug("Metric older than the threshold", zap.String("metric_name", metric.Name()), zap.Time("metric_timestamp", dataPoint.Timestamp().AsTime()))
				continue
			}

			addLabelsToDataPoint(dataPoint, ts.Labels)
			addDataPointToMetric(metric, metricType, dataPoint)

			settings.Logger.Debug("Metric sample",
				zap.String("metric_name", metric.Name()),
				zap.String("metric_unit", metric.Unit()),
				zap.Float64("metric_value", dataPoint.DoubleValue()),
				zap.Time("metric_timestamp", dataPoint.Timestamp().AsTime()),
				zap.String("metric_labels", fmt.Sprintf("%#v", dataPoint.Attributes())),
			)
		}
	}
	return metrics, nil
}

func getMetricName(labels []prompb.Label) (string, error) {
	for _, label := range labels {
		if label.Name == nameLabel {
			return label.Value, nil
		}
	}
	return "", errors.New("label name not found")
}

func getMetricTypeAndUnit(metricName string) (string, string) {
	match := metricNamePattern.FindStringSubmatch(metricName)
	if len(match) > 1 {
		lastSuffix := match[len(match)-1]
		if isValidSuffix(lastSuffix) {
			if len(match) > 2 {
				secondSuffix := match[len(match)-2]
				if isValidUnit(secondSuffix) {
					return lastSuffix, secondSuffix
				}
			}
			return lastSuffix, ""
		} else if isValidUnit(lastSuffix) {
			return "", lastSuffix
		}
	}
	return "", ""
}

func isOlderThanThreshold(timestamp time.Time, threshold int64) bool {
	return timestamp.Before(time.Now().Add(-time.Duration(threshold) * time.Hour))
}

func addLabelsToDataPoint(dataPoint pmetric.NumberDataPoint, labels []prompb.Label) {
	for _, label := range labels {
		labelName := label.Name
		if labelName == nameLabel {
			labelName = "key_name"
		}
		dataPoint.Attributes().PutStr(labelName, label.Value)
	}
}

func addDataPointToMetric(metric pmetric.Metric, metricType string, dataPoint pmetric.NumberDataPoint) {
	if isValidCumulativeSuffix(metricType) {
		metric.SetEmptySum()
		metric.Sum().SetIsMonotonic(true)
		metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dataPoint.CopyTo(metric.Sum().DataPoints().AppendEmpty())
	} else {
		metric.SetEmptyGauge()
		dataPoint.CopyTo(metric.Gauge().DataPoints().AppendEmpty())
	}
}

func isValidSuffix(suffix string) bool {
	switch suffix {
	case "max", "sum", "count", "total":
		return true
	}
	return false
}

func isValidCumulativeSuffix(suffix string) bool {
	switch suffix {
	case "sum", "count", "total":
		return true
	}
	return false
}

func isValidUnit(unit string) bool {
	switch unit {
	case "seconds", "bytes":
		return true
	}
	return false
}
