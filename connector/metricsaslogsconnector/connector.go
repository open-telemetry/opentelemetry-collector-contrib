// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaslogsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/metricsaslogsconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	attrMetricName                   = "metric.name"
	attrMetricType                   = "metric.type"
	attrMetricDescription            = "metric.description"
	attrMetricUnit                   = "metric.unit"
	attrMetricIsMonotonic            = "metric.is_monotonic"
	attrMetricAggregationTemporality = "metric.aggregation_temporality"

	attrGaugeValue = "gauge.value"
	attrSumValue   = "sum.value"

	attrHistogramCount          = "histogram.count"
	attrHistogramSum            = "histogram.sum"
	attrHistogramMin            = "histogram.min"
	attrHistogramMax            = "histogram.max"
	attrHistogramBucketCounts   = "histogram.bucket_counts"
	attrHistogramExplicitBounds = "histogram.explicit_bounds"

	attrExponentialHistogramCount     = "exponential_histogram.count"
	attrExponentialHistogramSum       = "exponential_histogram.sum"
	attrExponentialHistogramScale     = "exponential_histogram.scale"
	attrExponentialHistogramZeroCount = "exponential_histogram.zero_count"
	attrExponentialHistogramMin       = "exponential_histogram.min"
	attrExponentialHistogramMax       = "exponential_histogram.max"

	attrSummaryCount          = "summary.count"
	attrSummarySum            = "summary.sum"
	attrSummaryQuantileValues = "summary.quantile_values"
	attrQuantile              = "quantile"
	attrValue                 = "value"
)

type metricsAsLogs struct {
	logsConsumer consumer.Logs
	config       *Config
	logger       *zap.Logger
	component.StartFunc
	component.ShutdownFunc
}

func (*metricsAsLogs) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *metricsAsLogs) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	logs := plog.NewLogs()

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		m.processResourceMetrics(resourceMetric, logs)
	}

	if logs.ResourceLogs().Len() > 0 {
		return m.logsConsumer.ConsumeLogs(ctx, logs)
	}

	return nil
}

func (m *metricsAsLogs) processResourceMetrics(resourceMetric pmetric.ResourceMetrics, logs plog.Logs) {
	resourceLogs := logs.ResourceLogs().AppendEmpty()

	if m.config.IncludeResourceAttributes {
		resourceMetric.Resource().Attributes().CopyTo(resourceLogs.Resource().Attributes())
	}

	resourceLogs.SetSchemaUrl(resourceMetric.SchemaUrl())

	for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
		scopeMetric := resourceMetric.ScopeMetrics().At(j)
		m.processScopeMetrics(scopeMetric, resourceLogs)
	}
}

func (m *metricsAsLogs) processScopeMetrics(scopeMetric pmetric.ScopeMetrics, resourceLogs plog.ResourceLogs) {
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	if m.config.IncludeScopeInfo {
		scopeMetric.Scope().CopyTo(scopeLogs.Scope())
	}

	scopeLogs.SetSchemaUrl(scopeMetric.SchemaUrl())

	for k := 0; k < scopeMetric.Metrics().Len(); k++ {
		metric := scopeMetric.Metrics().At(k)
		m.processMetric(metric, &scopeLogs)
	}
}

func (m *metricsAsLogs) processMetric(metric pmetric.Metric, scopeLogs *plog.ScopeLogs) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		m.processGaugeDataPointsWithMetric(metric, scopeLogs)
	case pmetric.MetricTypeSum:
		m.processSumDataPointsWithMetric(metric, scopeLogs)
	case pmetric.MetricTypeHistogram:
		m.processHistogramDataPointsWithMetric(metric, scopeLogs)
	case pmetric.MetricTypeExponentialHistogram:
		m.processExponentialHistogramDataPointsWithMetric(metric, scopeLogs)
	case pmetric.MetricTypeSummary:
		m.processSummaryDataPointsWithMetric(metric, scopeLogs)
	default:
		m.logger.Warn("Unknown metric type", zap.String("type", metric.Type().String()))
	}
}

func (m *metricsAsLogs) processGaugeDataPointsWithMetric(metric pmetric.Metric, scopeLogs *plog.ScopeLogs) {
	gauge := metric.Gauge()
	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dataPoint := gauge.DataPoints().At(i)
		m.convertGaugeDataPointToLogRecordWithMetric(metric, dataPoint, scopeLogs)
	}
}

func (m *metricsAsLogs) processSumDataPointsWithMetric(metric pmetric.Metric, scopeLogs *plog.ScopeLogs) {
	sum := metric.Sum()
	for i := 0; i < sum.DataPoints().Len(); i++ {
		dataPoint := sum.DataPoints().At(i)
		m.convertSumDataPointToLogRecordWithMetric(metric, sum, dataPoint, scopeLogs)
	}
}

func (m *metricsAsLogs) processHistogramDataPointsWithMetric(metric pmetric.Metric, scopeLogs *plog.ScopeLogs) {
	histogram := metric.Histogram()
	for i := 0; i < histogram.DataPoints().Len(); i++ {
		dataPoint := histogram.DataPoints().At(i)
		m.convertHistogramDataPointToLogRecordWithMetric(metric, histogram, dataPoint, scopeLogs)
	}
}

func (m *metricsAsLogs) processExponentialHistogramDataPointsWithMetric(metric pmetric.Metric, scopeLogs *plog.ScopeLogs) {
	expHistogram := metric.ExponentialHistogram()
	for i := 0; i < expHistogram.DataPoints().Len(); i++ {
		dataPoint := expHistogram.DataPoints().At(i)
		m.convertExponentialHistogramDataPointToLogRecordWithMetric(metric, expHistogram, dataPoint, scopeLogs)
	}
}

func (m *metricsAsLogs) processSummaryDataPointsWithMetric(metric pmetric.Metric, scopeLogs *plog.ScopeLogs) {
	summary := metric.Summary()
	for i := 0; i < summary.DataPoints().Len(); i++ {
		dataPoint := summary.DataPoints().At(i)
		m.convertSummaryDataPointToLogRecordWithMetric(metric, dataPoint, scopeLogs)
	}
}

func (m *metricsAsLogs) convertGaugeDataPointToLogRecordWithMetric(metric pmetric.Metric, dataPoint pmetric.NumberDataPoint, scopeLogs *plog.ScopeLogs) {
	logRecord := scopeLogs.LogRecords().AppendEmpty()

	m.setLogRecordFromDataPoint(logRecord, metric.Name(), metric.Type().String(), dataPoint.Attributes(), dataPoint.Timestamp(), dataPoint.StartTimestamp())
	m.addCommonMetricAttributes(logRecord, metric)
	m.addNumberDataPointAttributes(logRecord, dataPoint, attrGaugeValue)
}

func (m *metricsAsLogs) convertSumDataPointToLogRecordWithMetric(metric pmetric.Metric, sum pmetric.Sum, dataPoint pmetric.NumberDataPoint, scopeLogs *plog.ScopeLogs) {
	logRecord := scopeLogs.LogRecords().AppendEmpty()

	m.setLogRecordFromDataPoint(logRecord, metric.Name(), metric.Type().String(), dataPoint.Attributes(), dataPoint.Timestamp(), dataPoint.StartTimestamp())
	m.addCommonMetricAttributes(logRecord, metric)
	logRecord.Attributes().PutBool(attrMetricIsMonotonic, sum.IsMonotonic())
	m.addAggregationTemporalityAttribute(logRecord, sum.AggregationTemporality())
	m.addNumberDataPointAttributes(logRecord, dataPoint, attrSumValue)
}

func (m *metricsAsLogs) convertHistogramDataPointToLogRecordWithMetric(metric pmetric.Metric, histogram pmetric.Histogram, dataPoint pmetric.HistogramDataPoint, scopeLogs *plog.ScopeLogs) {
	logRecord := scopeLogs.LogRecords().AppendEmpty()

	m.setLogRecordFromDataPoint(logRecord, metric.Name(), metric.Type().String(), dataPoint.Attributes(), dataPoint.Timestamp(), dataPoint.StartTimestamp())
	m.addCommonMetricAttributes(logRecord, metric)
	m.addAggregationTemporalityAttribute(logRecord, histogram.AggregationTemporality())
	m.addHistogramDataPointAttributes(logRecord, dataPoint)
}

func (m *metricsAsLogs) convertExponentialHistogramDataPointToLogRecordWithMetric(metric pmetric.Metric, expHistogram pmetric.ExponentialHistogram, dataPoint pmetric.ExponentialHistogramDataPoint, scopeLogs *plog.ScopeLogs) {
	logRecord := scopeLogs.LogRecords().AppendEmpty()

	m.setLogRecordFromDataPoint(logRecord, metric.Name(), metric.Type().String(), dataPoint.Attributes(), dataPoint.Timestamp(), dataPoint.StartTimestamp())
	m.addCommonMetricAttributes(logRecord, metric)
	m.addAggregationTemporalityAttribute(logRecord, expHistogram.AggregationTemporality())
	m.addExponentialHistogramDataPointAttributes(logRecord, dataPoint)
}

func (m *metricsAsLogs) convertSummaryDataPointToLogRecordWithMetric(metric pmetric.Metric, dataPoint pmetric.SummaryDataPoint, scopeLogs *plog.ScopeLogs) {
	logRecord := scopeLogs.LogRecords().AppendEmpty()

	m.setLogRecordFromDataPoint(logRecord, metric.Name(), metric.Type().String(), dataPoint.Attributes(), dataPoint.Timestamp(), dataPoint.StartTimestamp())
	m.addCommonMetricAttributes(logRecord, metric)
	m.addSummaryDataPointAttributes(logRecord, dataPoint)
}

func (*metricsAsLogs) addAggregationTemporalityAttribute(logRecord plog.LogRecord, temporality pmetric.AggregationTemporality) {
	logRecord.Attributes().PutStr(attrMetricAggregationTemporality, temporality.String())
}

func (*metricsAsLogs) addCommonMetricAttributes(logRecord plog.LogRecord, metric pmetric.Metric) {
	logRecord.Attributes().PutStr(attrMetricDescription, metric.Description())
	logRecord.Attributes().PutStr(attrMetricUnit, metric.Unit())
}

func (*metricsAsLogs) addNumberDataPointAttributes(logRecord plog.LogRecord, dataPoint pmetric.NumberDataPoint, valueAttr string) {
	switch dataPoint.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		logRecord.Attributes().PutInt(valueAttr, dataPoint.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		logRecord.Attributes().PutDouble(valueAttr, dataPoint.DoubleValue())
	}
}

func (*metricsAsLogs) addHistogramDataPointAttributes(logRecord plog.LogRecord, dataPoint pmetric.HistogramDataPoint) {
	logRecord.Attributes().PutInt(attrHistogramCount, int64(dataPoint.Count()))
	logRecord.Attributes().PutDouble(attrHistogramSum, dataPoint.Sum())

	if dataPoint.HasMin() {
		logRecord.Attributes().PutDouble(attrHistogramMin, dataPoint.Min())
	}
	if dataPoint.HasMax() {
		logRecord.Attributes().PutDouble(attrHistogramMax, dataPoint.Max())
	}

	bucketCountsSlice := logRecord.Attributes().PutEmptySlice(attrHistogramBucketCounts)
	for i := 0; i < dataPoint.BucketCounts().Len(); i++ {
		bucketCountsSlice.AppendEmpty().SetInt(int64(dataPoint.BucketCounts().At(i)))
	}

	explicitBoundsSlice := logRecord.Attributes().PutEmptySlice(attrHistogramExplicitBounds)
	for i := 0; i < dataPoint.ExplicitBounds().Len(); i++ {
		explicitBoundsSlice.AppendEmpty().SetDouble(dataPoint.ExplicitBounds().At(i))
	}
}

func (*metricsAsLogs) addExponentialHistogramDataPointAttributes(logRecord plog.LogRecord, dataPoint pmetric.ExponentialHistogramDataPoint) {
	logRecord.Attributes().PutInt(attrExponentialHistogramCount, int64(dataPoint.Count()))
	logRecord.Attributes().PutDouble(attrExponentialHistogramSum, dataPoint.Sum())
	logRecord.Attributes().PutInt(attrExponentialHistogramScale, int64(dataPoint.Scale()))
	logRecord.Attributes().PutInt(attrExponentialHistogramZeroCount, int64(dataPoint.ZeroCount()))

	if dataPoint.HasMin() {
		logRecord.Attributes().PutDouble(attrExponentialHistogramMin, dataPoint.Min())
	}
	if dataPoint.HasMax() {
		logRecord.Attributes().PutDouble(attrExponentialHistogramMax, dataPoint.Max())
	}
}

func (*metricsAsLogs) addSummaryDataPointAttributes(logRecord plog.LogRecord, dataPoint pmetric.SummaryDataPoint) {
	logRecord.Attributes().PutInt(attrSummaryCount, int64(dataPoint.Count()))
	logRecord.Attributes().PutDouble(attrSummarySum, dataPoint.Sum())

	if dataPoint.QuantileValues().Len() > 0 {
		quantilesSlice := logRecord.Attributes().PutEmptySlice(attrSummaryQuantileValues)
		for i := 0; i < dataPoint.QuantileValues().Len(); i++ {
			qv := dataPoint.QuantileValues().At(i)
			quantileMap := quantilesSlice.AppendEmpty().SetEmptyMap()
			quantileMap.PutDouble(attrQuantile, qv.Quantile())
			quantileMap.PutDouble(attrValue, qv.Value())
		}
	}
}

func (*metricsAsLogs) setLogRecordFromDataPoint(logRecord plog.LogRecord, metricName, metricType string, attributes pcommon.Map, timestamp, startTimestamp pcommon.Timestamp) {
	logRecord.SetTimestamp(timestamp)
	if startTimestamp != 0 {
		logRecord.SetObservedTimestamp(startTimestamp)
	}

	logRecord.Body().SetStr("metric converted to log")

	// Copy datapoint attributes first, before adding metric-specific attributes
	attributes.CopyTo(logRecord.Attributes())

	logRecord.Attributes().PutStr(attrMetricName, metricName)
	logRecord.Attributes().PutStr(attrMetricType, metricType)
}
