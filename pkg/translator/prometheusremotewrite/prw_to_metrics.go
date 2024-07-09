// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"errors"
	"fmt"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"regexp"
)

var (
	metricNamePattern = regexp.MustCompile(`(\w+)_(\w+)_(\w+)\z`)
	nameLabel         = "__name__"
)

// region proto v1

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
			if SetDataPointTimestamp(dataPoint, sample.Timestamp, settings) {
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

// endregion

func FromTimeSeriesV2(req *writev2.Request, settings PRWToMetricSettings) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	symbol := req.Symbols
	for _, ts := range req.Timeseries {
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
		metric := scopeMetrics.Metrics().AppendEmpty()

		metricName, err := getMetricNameBySymbol(ts, req.GetSymbols())
		if err != nil {
			settings.Logger.Warn("Metric name not found", zap.Error(err))
			continue
		}
		metric.SetName(metricName)
		metric.SetUnit(req.Symbols[ts.Metadata.UnitRef])
		metric.SetDescription(req.Symbols[ts.Metadata.HelpRef])
		metric.Metadata().PutStr("prometheus_type", ts.Metadata.Type.String())

		switch ts.Metadata.Type {
		case writev2.Metadata_METRIC_TYPE_HISTOGRAM:
			err := prwHistogramConvert(ts, &metric, symbol, settings)
			if err != nil {
				settings.Logger.Debug("Error converting histogram", zap.Error(err))
				continue
			}

		case writev2.Metadata_METRIC_TYPE_SUMMARY, writev2.Metadata_METRIC_TYPE_INFO, writev2.Metadata_METRIC_TYPE_STATESET:
			// todo summary is monotonic, info, stateset not monotonic
			// need more information, summary maybe count, sum or quantile values, check sum suffix
		default:
			settings.Logger.Debug("Unsupported metric type", zap.String("metric_name", metricName), zap.String("type", ts.Metadata.Type.String()))
			err := prwGaugeConvert(ts, &metric, symbol, settings)
			if err != nil {
				settings.Logger.Debug("Error converting gauge", zap.Error(err))
				continue
			}
		}
	}
	return metrics, nil
}

func prwHistogramConvert(ts writev2.TimeSeries, metric *pmetric.Metric, symbols []string, settings PRWToMetricSettings) error {
	histogramMetric := metric.SetEmptyHistogram()
	//todo use: metric.SetEmptyExponentialHistogram()
	histogramMetric.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	for _, histogram := range ts.Histograms {
		dataPoint := histogramMetric.DataPoints().AppendEmpty()

		if SetDataPointTimestamp(dataPoint, histogram.Timestamp, settings) {
			settings.Logger.Debug("Metric older than the threshold", zap.String("metric_name", metric.Name()), zap.Time("metric_timestamp", dataPoint.Timestamp().AsTime()))
			continue
		}

		dataPoint.SetCount(histogramCount(histogram))
		dataPoint.SetSum(histogram.Sum)

		// zero count
		zeroCount := histogramZeroCount(histogram)
		if zeroCount > 0 {
			dataPoint.BucketCounts().Append(uint64(zeroCount))
			dataPoint.ExplicitBounds().Append(histogram.ZeroThreshold)
		}

		switch histogram.Schema {
		case -53: //magic code:
			// Custom buckets
			for i, upperBound := range histogram.CustomValues {
				bucketCount := histogram.PositiveCounts[i]
				dataPoint.BucketCounts().Append(uint64(bucketCount))
				dataPoint.ExplicitBounds().Append(upperBound)
			}
		default:
			// Exponential buckets
			convertExponentialBuckets(histogram, dataPoint)
		}
		// histogram.ResetHint
		addLabelsToDataPointV2(dataPoint, ts.LabelsRefs, symbols)
	}
	return nil
}

func convertExponentialBuckets(histogram writev2.Histogram, dataPoint pmetric.HistogramDataPoint) {
	if len(histogram.NegativeCounts) > 0 {
		// negative float histogram
		for _, span := range histogram.NegativeSpans {
			for i := 0; i < int(span.Length); i++ {
				bucketIndex := int(span.Offset) + i
				bucketCount := histogram.NegativeCounts[bucketIndex]
				dataPoint.BucketCounts().Append(uint64(bucketCount))
				dataPoint.ExplicitBounds().Append(-float64(bucketIndex))
			}
		}
	} else {
		// negative integer histogram
		count := uint64(0)
		for _, span := range histogram.NegativeSpans {
			for i := 0; i < int(span.Length); i++ {
				bucketIndex := int(span.Offset) + i
				count += uint64(histogram.NegativeDeltas[bucketIndex])
				dataPoint.BucketCounts().Append(count)
				dataPoint.ExplicitBounds().Append(-float64(bucketIndex))
			}
		}
	}

	if len(histogram.PositiveCounts) > 0 {
		for _, span := range histogram.PositiveSpans {
			for i := 0; i < int(span.Length); i++ {
				bucketIndex := int(span.Offset) + i
				bucketCount := histogram.PositiveCounts[bucketIndex]
				dataPoint.BucketCounts().Append(uint64(bucketCount))
				dataPoint.ExplicitBounds().Append(float64(bucketIndex))
			}
		}
	} else {
		count := uint64(0)
		for _, span := range histogram.PositiveSpans {
			for i := 0; i < int(span.Length); i++ {
				bucketIndex := int(span.Offset) + i
				count += uint64(histogram.PositiveDeltas[bucketIndex])
				dataPoint.BucketCounts().Append(count)
				dataPoint.ExplicitBounds().Append(float64(bucketIndex))
			}
		}
	}
}

func histogramZeroCount(histogram writev2.Histogram) uint64 {
	switch zc := histogram.ZeroCount.(type) {
	case *writev2.Histogram_ZeroCountInt:
		return uint64(zc.ZeroCountInt)
	case *writev2.Histogram_ZeroCountFloat:
		return uint64(zc.ZeroCountFloat)
	default:
		return 0
	}
}

func histogramCount(histogram writev2.Histogram) uint64 {
	var bucketCount uint64
	switch count := histogram.Count.(type) {
	case *writev2.Histogram_CountFloat:
		bucketCount = uint64(count.CountFloat)
	case *writev2.Histogram_CountInt:
		bucketCount = count.CountInt
	}
	return bucketCount
}

func prwGaugeConvert(ts writev2.TimeSeries, metric *pmetric.Metric, symbol []string, settings PRWToMetricSettings) error {
	gauge := metric.SetEmptyGauge()
	for _, sample := range ts.Samples {
		dataPoint := pmetric.NewNumberDataPoint()
		if SetDataPointTimestamp(dataPoint, sample.Timestamp, settings) {
			settings.Logger.Debug("Metric older than the threshold", zap.String("metric_name", metric.Name()), zap.Time("metric_timestamp", dataPoint.Timestamp().AsTime()))
			continue
		}

		dataPoint.SetDoubleValue(sample.Value)
		addLabelsToDataPointV2(dataPoint, ts.LabelsRefs, symbol)
		dataPoint.CopyTo(gauge.DataPoints().AppendEmpty())
	}
	return nil
}

func getMetricNameBySymbol(ts writev2.TimeSeries, symbols []string) (string, error) {
	for i := 0; i < len(ts.LabelsRefs); i += 2 {
		if symbols[ts.LabelsRefs[i]] == nameLabel {
			return symbols[ts.LabelsRefs[i+1]], nil
		}
	}
	return "", errors.New("label name not found")
}
