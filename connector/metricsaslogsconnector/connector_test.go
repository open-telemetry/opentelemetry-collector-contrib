// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaslogsconnector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestCapabilities(t *testing.T) {
	connector := &metricsAsLogs{}
	capabilities := connector.Capabilities()
	assert.False(t, capabilities.MutatesData)
}

func TestConsumeMetrics_EmptyMetrics(t *testing.T) {
	sink := &consumertest.LogsSink{}
	connector := &metricsAsLogs{
		logsConsumer: sink,
		config:       &Config{},
		logger:       zap.NewNop(),
	}

	metrics := pmetric.NewMetrics()
	err := connector.ConsumeMetrics(t.Context(), metrics)
	require.NoError(t, err)
	assert.Empty(t, sink.AllLogs())
}

func TestConsumeMetrics_GaugeMetric(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		setupMetric   func(pmetric.Metric)
		expectedAttrs map[string]any
		checkResource bool
		checkScope    bool
	}{
		{
			name: "int_gauge_with_resource_attributes",
			config: &Config{
				IncludeResourceAttributes: true,
				IncludeScopeInfo:          false,
			},
			setupMetric: func(metric pmetric.Metric) {
				metric.SetName("test_gauge")
				metric.SetDescription("Test gauge metric")
				metric.SetUnit("bytes")
				gauge := metric.SetEmptyGauge()
				dp := gauge.DataPoints().AppendEmpty()
				dp.SetIntValue(42)
				dp.Attributes().PutStr("label1", "value1")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567890, 0)))
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567800, 0)))
			},
			expectedAttrs: map[string]any{
				attrMetricName:        "test_gauge",
				attrMetricType:        "Gauge",
				attrMetricDescription: "Test gauge metric",
				attrMetricUnit:        "bytes",
				attrGaugeValue:        int64(42),
				"label1":              "value1",
			},
			checkResource: true,
		},
		{
			name: "double_gauge_with_scope_info",
			config: &Config{
				IncludeResourceAttributes: false,
				IncludeScopeInfo:          true,
			},
			setupMetric: func(metric pmetric.Metric) {
				metric.SetName("test_gauge_double")
				gauge := metric.SetEmptyGauge()
				dp := gauge.DataPoints().AppendEmpty()
				dp.SetDoubleValue(3.14)
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567890, 0)))
			},
			expectedAttrs: map[string]any{
				attrMetricName: "test_gauge_double",
				attrMetricType: "Gauge",
				attrGaugeValue: 3.14,
			},
			checkScope: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &consumertest.LogsSink{}
			connector := &metricsAsLogs{
				logsConsumer: sink,
				config:       tt.config,
				logger:       zap.NewNop(),
			}

			metrics := createTestMetrics(tt.setupMetric, tt.checkResource, tt.checkScope)
			err := connector.ConsumeMetrics(t.Context(), metrics)
			require.NoError(t, err)

			require.Len(t, sink.AllLogs(), 1)
			logs := sink.AllLogs()[0]
			validateLogRecord(t, logs, tt.expectedAttrs, tt.checkResource, tt.checkScope)
		})
	}
}

func TestConsumeMetrics_SumMetric(t *testing.T) {
	tests := []struct {
		name          string
		isMonotonic   bool
		temporality   pmetric.AggregationTemporality
		expectedAttrs map[string]any
	}{
		{
			name:        "monotonic_cumulative_sum",
			isMonotonic: true,
			temporality: pmetric.AggregationTemporalityCumulative,
			expectedAttrs: map[string]any{
				attrMetricName:                   "test_sum",
				attrMetricType:                   "Sum",
				attrSumValue:                     int64(100),
				attrMetricIsMonotonic:            true,
				attrMetricAggregationTemporality: "Cumulative",
			},
		},
		{
			name:        "non_monotonic_delta_sum",
			isMonotonic: false,
			temporality: pmetric.AggregationTemporalityDelta,
			expectedAttrs: map[string]any{
				attrMetricName:                   "test_sum",
				attrMetricType:                   "Sum",
				attrSumValue:                     int64(100),
				attrMetricIsMonotonic:            false,
				attrMetricAggregationTemporality: "Delta",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &consumertest.LogsSink{}
			connector := &metricsAsLogs{
				logsConsumer: sink,
				config:       &Config{},
				logger:       zap.NewNop(),
			}

			setupMetric := func(metric pmetric.Metric) {
				metric.SetName("test_sum")
				sum := metric.SetEmptySum()
				sum.SetIsMonotonic(tt.isMonotonic)
				sum.SetAggregationTemporality(tt.temporality)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetIntValue(100)
				dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567890, 0)))
			}

			metrics := createTestMetrics(setupMetric, false, false)
			err := connector.ConsumeMetrics(t.Context(), metrics)
			require.NoError(t, err)

			require.Len(t, sink.AllLogs(), 1)
			logs := sink.AllLogs()[0]
			validateLogRecord(t, logs, tt.expectedAttrs, false, false)
		})
	}
}

func TestConsumeMetrics_HistogramMetric(t *testing.T) {
	sink := &consumertest.LogsSink{}
	connector := &metricsAsLogs{
		logsConsumer: sink,
		config:       &Config{},
		logger:       zap.NewNop(),
	}

	setupMetric := func(metric pmetric.Metric) {
		metric.SetName("test_histogram")
		histogram := metric.SetEmptyHistogram()
		histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp := histogram.DataPoints().AppendEmpty()
		dp.SetCount(10)
		dp.SetSum(55.5)
		dp.SetMin(1.0)
		dp.SetMax(10.0)
		dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
		dp.ExplicitBounds().FromRaw([]float64{1.0, 5.0, 10.0})
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567890, 0)))
	}

	metrics := createTestMetrics(setupMetric, false, false)
	err := connector.ConsumeMetrics(t.Context(), metrics)
	require.NoError(t, err)

	require.Len(t, sink.AllLogs(), 1)
	logs := sink.AllLogs()[0]
	lr := getLogRecord(t, logs)

	assert.Equal(t, "test_histogram", lr.Attributes().AsRaw()[attrMetricName])
	assert.Equal(t, "Histogram", lr.Attributes().AsRaw()[attrMetricType])
	assert.Equal(t, int64(10), lr.Attributes().AsRaw()[attrHistogramCount])
	assert.Equal(t, 55.5, lr.Attributes().AsRaw()[attrHistogramSum])
	assert.Equal(t, 1.0, lr.Attributes().AsRaw()[attrHistogramMin])
	assert.Equal(t, 10.0, lr.Attributes().AsRaw()[attrHistogramMax])
	assert.Equal(t, "Cumulative", lr.Attributes().AsRaw()[attrMetricAggregationTemporality])

	// Check bucket counts slice
	bucketCountsAttr, exists := lr.Attributes().Get(attrHistogramBucketCounts)
	require.True(t, exists)
	bucketCountsSlice := bucketCountsAttr.Slice()
	require.Equal(t, 4, bucketCountsSlice.Len())
	assert.Equal(t, int64(1), bucketCountsSlice.At(0).Int())
	assert.Equal(t, int64(2), bucketCountsSlice.At(1).Int())
	assert.Equal(t, int64(3), bucketCountsSlice.At(2).Int())
	assert.Equal(t, int64(4), bucketCountsSlice.At(3).Int())

	// Check explicit bounds slice
	boundsAttr, exists := lr.Attributes().Get(attrHistogramExplicitBounds)
	require.True(t, exists)
	boundsSlice := boundsAttr.Slice()
	require.Equal(t, 3, boundsSlice.Len())
	assert.Equal(t, 1.0, boundsSlice.At(0).Double())
	assert.Equal(t, 5.0, boundsSlice.At(1).Double())
	assert.Equal(t, 10.0, boundsSlice.At(2).Double())
}

func TestConsumeMetrics_ExponentialHistogramMetric(t *testing.T) {
	sink := &consumertest.LogsSink{}
	connector := &metricsAsLogs{
		logsConsumer: sink,
		config:       &Config{},
		logger:       zap.NewNop(),
	}

	setupMetric := func(metric pmetric.Metric) {
		metric.SetName("test_exp_histogram")
		expHistogram := metric.SetEmptyExponentialHistogram()
		expHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := expHistogram.DataPoints().AppendEmpty()
		dp.SetCount(20)
		dp.SetSum(100.0)
		dp.SetScale(2)
		dp.SetZeroCount(1)
		dp.SetMin(0.5)
		dp.SetMax(50.0)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567890, 0)))
	}

	metrics := createTestMetrics(setupMetric, false, false)
	err := connector.ConsumeMetrics(t.Context(), metrics)
	require.NoError(t, err)

	require.Len(t, sink.AllLogs(), 1)
	logs := sink.AllLogs()[0]
	lr := getLogRecord(t, logs)

	expectedAttrs := map[string]any{
		attrMetricName:                    "test_exp_histogram",
		attrMetricType:                    "ExponentialHistogram",
		attrExponentialHistogramCount:     int64(20),
		attrExponentialHistogramSum:       100.0,
		attrExponentialHistogramScale:     int64(2),
		attrExponentialHistogramZeroCount: int64(1),
		attrExponentialHistogramMin:       0.5,
		attrExponentialHistogramMax:       50.0,
		attrMetricAggregationTemporality:  "Delta",
	}

	for key, expected := range expectedAttrs {
		actual := lr.Attributes().AsRaw()[key]
		assert.Equal(t, expected, actual, "attribute %s mismatch", key)
	}
}

func TestConsumeMetrics_SummaryMetric(t *testing.T) {
	sink := &consumertest.LogsSink{}
	connector := &metricsAsLogs{
		logsConsumer: sink,
		config:       &Config{},
		logger:       zap.NewNop(),
	}

	setupMetric := func(metric pmetric.Metric) {
		metric.SetName("test_summary")
		summary := metric.SetEmptySummary()
		dp := summary.DataPoints().AppendEmpty()
		dp.SetCount(100)
		dp.SetSum(500.0)

		// Add quantile values
		qv1 := dp.QuantileValues().AppendEmpty()
		qv1.SetQuantile(0.5)
		qv1.SetValue(10.0)

		qv2 := dp.QuantileValues().AppendEmpty()
		qv2.SetQuantile(0.95)
		qv2.SetValue(45.0)

		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567890, 0)))
	}

	metrics := createTestMetrics(setupMetric, false, false)
	err := connector.ConsumeMetrics(t.Context(), metrics)
	require.NoError(t, err)

	require.Len(t, sink.AllLogs(), 1)
	logs := sink.AllLogs()[0]
	lr := getLogRecord(t, logs)

	assert.Equal(t, "test_summary", lr.Attributes().AsRaw()[attrMetricName])
	assert.Equal(t, "Summary", lr.Attributes().AsRaw()[attrMetricType])
	assert.Equal(t, int64(100), lr.Attributes().AsRaw()[attrSummaryCount])
	assert.Equal(t, 500.0, lr.Attributes().AsRaw()[attrSummarySum])

	// Check quantile values slice
	quantilesAttr, exists := lr.Attributes().Get(attrSummaryQuantileValues)
	require.True(t, exists)
	quantilesSlice := quantilesAttr.Slice()
	require.Equal(t, 2, quantilesSlice.Len())

	// Check first quantile
	q1 := quantilesSlice.At(0).Map()
	assert.Equal(t, 0.5, q1.AsRaw()[attrQuantile])
	assert.Equal(t, 10.0, q1.AsRaw()[attrValue])

	// Check second quantile
	q2 := quantilesSlice.At(1).Map()
	assert.Equal(t, 0.95, q2.AsRaw()[attrQuantile])
	assert.Equal(t, 45.0, q2.AsRaw()[attrValue])
}

func TestConsumeMetrics_UnknownMetricType(t *testing.T) {
	sink := &consumertest.LogsSink{}

	// Create a custom logger to capture log messages
	var logMessages []string
	logger := zap.NewExample().WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
		logMessages = append(logMessages, entry.Message)
		return nil
	}))

	connector := &metricsAsLogs{
		logsConsumer: sink,
		config:       &Config{},
		logger:       logger,
	}

	// Create metrics with an unknown type (we'll simulate this by not setting any type)
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("unknown_metric")
	// Note: Not setting any metric type will result in MetricTypeEmpty

	err := connector.ConsumeMetrics(t.Context(), metrics)
	require.NoError(t, err)

	// Should produce resource and scope logs but no log records since unknown metric type is skipped
	require.Len(t, sink.AllLogs(), 1)
	logs := sink.AllLogs()[0]

	require.Equal(t, 1, logs.ResourceLogs().Len())
	rl := logs.ResourceLogs().At(0)
	require.Equal(t, 1, rl.ScopeLogs().Len())
	sl := rl.ScopeLogs().At(0)

	// No log records should be created for unknown metric types
	assert.Equal(t, 0, sl.LogRecords().Len())

	// Should have logged a warning about unknown metric type
	assert.Contains(t, logMessages, "Unknown metric type")
}

// Helper functions

func createTestMetrics(setupMetric func(pmetric.Metric), withResource, withScope bool) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	if withResource {
		rm.Resource().Attributes().PutStr("service.name", "test-service")
		rm.Resource().Attributes().PutStr("service.version", "1.0.0")
		rm.SetSchemaUrl("https://opentelemetry.io/schemas/1.0.0")
	}

	sm := rm.ScopeMetrics().AppendEmpty()

	if withScope {
		sm.Scope().SetName("test-scope")
		sm.Scope().SetVersion("1.0.0")
		sm.SetSchemaUrl("https://opentelemetry.io/schemas/1.0.0")
	}

	metric := sm.Metrics().AppendEmpty()
	setupMetric(metric)

	return metrics
}

func validateLogRecord(t *testing.T, logs plog.Logs, expectedAttrs map[string]any, checkResource, checkScope bool) {
	require.Equal(t, 1, logs.ResourceLogs().Len())
	rl := logs.ResourceLogs().At(0)

	if checkResource {
		assert.Equal(t, "test-service", rl.Resource().Attributes().AsRaw()["service.name"])
		assert.Equal(t, "1.0.0", rl.Resource().Attributes().AsRaw()["service.version"])
		assert.Equal(t, "https://opentelemetry.io/schemas/1.0.0", rl.SchemaUrl())
	}

	require.Equal(t, 1, rl.ScopeLogs().Len())
	sl := rl.ScopeLogs().At(0)

	if checkScope {
		assert.Equal(t, "test-scope", sl.Scope().Name())
		assert.Equal(t, "1.0.0", sl.Scope().Version())
		assert.Equal(t, "https://opentelemetry.io/schemas/1.0.0", sl.SchemaUrl())
	}

	require.Equal(t, 1, sl.LogRecords().Len())
	lr := sl.LogRecords().At(0)

	// Check log body
	assert.Equal(t, "metric converted to log", lr.Body().AsString())

	// Check all expected attributes
	for key, expected := range expectedAttrs {
		actual := lr.Attributes().AsRaw()[key]
		assert.Equal(t, expected, actual, "attribute %s mismatch", key)
	}
}

func getLogRecord(t *testing.T, logs plog.Logs) plog.LogRecord {
	require.Equal(t, 1, logs.ResourceLogs().Len())
	rl := logs.ResourceLogs().At(0)
	require.Equal(t, 1, rl.ScopeLogs().Len())
	sl := rl.ScopeLogs().At(0)
	require.Equal(t, 1, sl.LogRecords().Len())
	return sl.LogRecords().At(0)
}
