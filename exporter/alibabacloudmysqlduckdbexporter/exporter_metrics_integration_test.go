// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package alibabacloudmysqlduckdbexporter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

func TestMetricsExporterIntegration(t *testing.T) {
	dsn := getMySQLDSN(t)
	cfg := newTestConfig(dsn)

	exporter := newMetricsExporter(zaptest.NewLogger(t), cfg)
	require.NoError(t, exporter.start(t.Context(), nil))
	t.Cleanup(func() { _ = exporter.shutdown(t.Context()) })

	db := queryDB(t, dsn)

	t.Run("Gauge", func(t *testing.T) {
		truncateTable(t, db, cfg.Database, cfg.gaugeTableName())

		metrics := simpleGaugeMetrics(5)
		require.NoError(t, exporter.pushMetricsData(t.Context(), metrics))

		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM `otel_test`.`otel_metrics_gauge`").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 5, count)

		// Verify first row
		var (
			serviceName string
			metricName  string
			metricUnit  string
			value       float64
			timeUnix    time.Time
			attrRaw     []byte
		)
		err = db.QueryRow(`SELECT service_name, metric_name, metric_unit, value, time_unix, attributes
			FROM `+"`otel_test`.`otel_metrics_gauge`"+` ORDER BY id LIMIT 1`).Scan(
			&serviceName, &metricName, &metricUnit, &value, &timeUnix, &attrRaw,
		)
		require.NoError(t, err)

		assert.Equal(t, "test-service", serviceName)
		assert.Equal(t, "cpu.utilization", metricName)
		assert.Equal(t, "%", metricUnit)
		assert.Equal(t, 0.5, value)

		var attrs map[string]string
		require.NoError(t, json.Unmarshal(attrRaw, &attrs))
		assert.Equal(t, "cpu0", attrs["cpu"])
	})

	t.Run("Sum", func(t *testing.T) {
		truncateTable(t, db, cfg.Database, cfg.sumTableName())

		metrics := simpleSumMetrics(5)
		require.NoError(t, exporter.pushMetricsData(t.Context(), metrics))

		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM `otel_test`.`otel_metrics_sum`").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 5, count)

		var (
			metricName    string
			value         float64
			aggTemp       int
			isMonotonic   bool
		)
		err = db.QueryRow(`SELECT metric_name, value, aggregation_temporality, is_monotonic
			FROM `+"`otel_test`.`otel_metrics_sum`"+` ORDER BY id LIMIT 1`).Scan(
			&metricName, &value, &aggTemp, &isMonotonic,
		)
		require.NoError(t, err)

		assert.Equal(t, "request.count", metricName)
		assert.Equal(t, 100.0, value)
		assert.Equal(t, int(pmetric.AggregationTemporalityCumulative), aggTemp)
		assert.True(t, isMonotonic)
	})

	t.Run("Histogram", func(t *testing.T) {
		truncateTable(t, db, cfg.Database, cfg.histogramTableName())

		metrics := simpleHistogramMetrics(5)
		require.NoError(t, exporter.pushMetricsData(t.Context(), metrics))

		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM `otel_test`.`otel_metrics_histogram`").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 5, count)

		var (
			metricName     string
			histCount      uint64
			histSum        *float64
			bucketCountRaw []byte
			boundsRaw      []byte
		)
		err = db.QueryRow(`SELECT metric_name, count, sum, bucket_counts, explicit_bounds
			FROM `+"`otel_test`.`otel_metrics_histogram`"+` ORDER BY id LIMIT 1`).Scan(
			&metricName, &histCount, &histSum, &bucketCountRaw, &boundsRaw,
		)
		require.NoError(t, err)

		assert.Equal(t, "request.duration", metricName)
		assert.Equal(t, uint64(10), histCount)
		require.NotNil(t, histSum)
		assert.Equal(t, 500.0, *histSum)

		var bucketCounts []uint64
		require.NoError(t, json.Unmarshal(bucketCountRaw, &bucketCounts))
		assert.Equal(t, []uint64{1, 2, 3, 4}, bucketCounts)

		var bounds []float64
		require.NoError(t, json.Unmarshal(boundsRaw, &bounds))
		assert.Equal(t, []float64{10, 50, 100}, bounds)
	})

	t.Run("Summary", func(t *testing.T) {
		truncateTable(t, db, cfg.Database, cfg.summaryTableName())

		metrics := simpleSummaryMetrics(5)
		require.NoError(t, exporter.pushMetricsData(t.Context(), metrics))

		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM `otel_test`.`otel_metrics_summary`").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 5, count)

		var (
			metricName   string
			summaryCount uint64
			summarySum   float64
			quantileRaw  []byte
		)
		err = db.QueryRow(`SELECT metric_name, count, sum, quantile_values
			FROM `+"`otel_test`.`otel_metrics_summary`"+` ORDER BY id LIMIT 1`).Scan(
			&metricName, &summaryCount, &summarySum, &quantileRaw,
		)
		require.NoError(t, err)

		assert.Equal(t, "request.latency", metricName)
		assert.Equal(t, uint64(100), summaryCount)
		assert.Equal(t, 5000.0, summarySum)

		var quantiles []quantileValueJSON
		require.NoError(t, json.Unmarshal(quantileRaw, &quantiles))
		assert.Len(t, quantiles, 3)
		assert.Equal(t, 0.5, quantiles[0].Quantile)
		assert.Equal(t, 50.0, quantiles[0].Value)
	})
}

func simpleGaugeMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("io.opentelemetry.contrib.mysql")

	m := sm.Metrics().AppendEmpty()
	m.SetName("cpu.utilization")
	m.SetDescription("CPU utilization")
	m.SetUnit("%")

	gauge := m.SetEmptyGauge()
	for i := range count {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp.Add(time.Duration(i) * time.Second)))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp))
		dp.SetDoubleValue(0.5)
		dp.Attributes().PutStr("cpu", "cpu0")
	}
	return metrics
}

func simpleSumMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("io.opentelemetry.contrib.mysql")

	m := sm.Metrics().AppendEmpty()
	m.SetName("request.count")
	m.SetDescription("Request count")
	m.SetUnit("1")

	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sum.SetIsMonotonic(true)

	for i := range count {
		dp := sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp.Add(time.Duration(i) * time.Second)))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp))
		dp.SetDoubleValue(100.0)
		dp.Attributes().PutStr("method", "GET")
	}
	return metrics
}

func simpleHistogramMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("io.opentelemetry.contrib.mysql")

	m := sm.Metrics().AppendEmpty()
	m.SetName("request.duration")
	m.SetDescription("Request duration histogram")
	m.SetUnit("ms")

	hist := m.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	for i := range count {
		dp := hist.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp.Add(time.Duration(i) * time.Second)))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp))
		dp.SetCount(10)
		dp.SetSum(500.0)
		dp.SetMin(1.0)
		dp.SetMax(200.0)
		dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
		dp.ExplicitBounds().FromRaw([]float64{10, 50, 100})
		dp.Attributes().PutStr("path", "/api")
	}
	return metrics
}

func simpleSummaryMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("io.opentelemetry.contrib.mysql")

	m := sm.Metrics().AppendEmpty()
	m.SetName("request.latency")
	m.SetDescription("Request latency summary")
	m.SetUnit("ms")

	summary := m.SetEmptySummary()
	for i := range count {
		dp := summary.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp.Add(time.Duration(i) * time.Second)))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp))
		dp.SetCount(100)
		dp.SetSum(5000.0)

		q1 := dp.QuantileValues().AppendEmpty()
		q1.SetQuantile(0.5)
		q1.SetValue(50.0)
		q2 := dp.QuantileValues().AppendEmpty()
		q2.SetQuantile(0.9)
		q2.SetValue(90.0)
		q3 := dp.QuantileValues().AppendEmpty()
		q3.SetQuantile(0.99)
		q3.SetValue(99.0)
	}
	return metrics
}
