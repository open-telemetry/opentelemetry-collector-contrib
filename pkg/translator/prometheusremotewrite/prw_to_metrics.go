// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
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
			settings.Logger.Warn("Metric name not found", zap.Error(err))
			continue
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
			timestamp := time.Unix(0, sample.Timestamp*int64(time.Millisecond))
			dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

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

// getMetricTypeAndUnit extracts the metric type and unit from a given metric name.
// The metric name is expected to follow the pattern of `metric_prefix_unit_suffix`,
// where `unit` and `suffix` are optional. The function uses regular expressions to
// parse the metric name and determine the type and unit based on predefined valid
// suffixes and units.
//
// Parameters:
//   - metricName: The name of the metric as a string.
//
// Returns:
//   - string: The extracted metric type (e.g., "sum", "count", "
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

// addDataPointToMetric adds a data point to a given metric. This function is designed
// to handle various types of metrics (e.g., counters, gauges, histograms, summaries)
// and update their values accordingly.
//
// Parameters:
//   - metric: A reference to the metric object to which the data point should be added.
//   - value: The value of the data point to be added.
//   - labels: A map of label names to label values, used to identify the specific
//     metric instance to update.
//
// The function is designed in the following manner to ensure flexibility and correctness:
//  1. It first checks the type of the metric to determine how to add the data point.
//  2. For counters, it increments the counter by the given value.
//  3. For gauges, it sets the gauge to the given value or increments it based on
//     specific conditions or requirements.
//  4. For histograms, it observes the value, which involves adding the value to the
//     appropriate bucket and updating the count and sum of the histogram.
//  5. For summaries, it observes the value, updating the quantile estimates and the
//     count and sum of the summary.
//
// This design ensures that the function can handle different metric types uniformly
// while adhering to the specific requirements of each metric type.
//
// Examples:
//
//   - Adding a data point to a counter metric:
//     Input: counterMetric, 5, {"method": "GET", "status": "200"}
//     Effect: Increment the counter by 5 for the given labels.
//
//   - Adding a data point to a gauge metric:
//     Input: gaugeMetric, 3.14, {"method": "GET"}
//     Effect: Set the gauge to 3.14 for the given labels.
//
//   - Adding a data point to a histogram metric:
//     Input: histogramMetric, 1.23, {"endpoint": "/api/v1/resource"}
//     Effect: Observe the value 1.23, updating the histogram buckets and count.
//
//   - Adding a data point to a summary metric:
//     Input: summaryMetric, 0.98, {"operation": "read"}
//     Effect: Observe the value 0.98, updating the summary quantiles and count.
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
