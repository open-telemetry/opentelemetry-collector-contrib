// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func newTestMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	rm.Resource().Attributes().PutStr("host.name", "test-host")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")
	sm.Scope().SetVersion("v1.0.0")
	sm.Scope().Attributes().PutStr("scope.key", "scope-value")

	timestamp := pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))
	startTimestamp := pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))

	// Gauge metric.
	gauge := sm.Metrics().AppendEmpty()
	gauge.SetName("cpu.utilization")
	gdp := gauge.SetEmptyGauge().DataPoints().AppendEmpty()
	gdp.SetTimestamp(timestamp)
	gdp.SetDoubleValue(72.5)
	gdp.Attributes().PutStr("cpu", "cpu0")

	// Sum metric.
	sum := sm.Metrics().AppendEmpty()
	sum.SetName("http.request.count")
	sumMetric := sum.SetEmptySum()
	sumMetric.SetIsMonotonic(true)
	sumMetric.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sdp := sumMetric.DataPoints().AppendEmpty()
	sdp.SetTimestamp(timestamp)
	sdp.SetStartTimestamp(startTimestamp)
	sdp.SetIntValue(1024)
	sdp.Attributes().PutStr("method", "GET")

	// Histogram metric.
	hist := sm.Metrics().AppendEmpty()
	hist.SetName("http.request.duration")
	histMetric := hist.SetEmptyHistogram()
	histMetric.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	hdp := histMetric.DataPoints().AppendEmpty()
	hdp.SetTimestamp(timestamp)
	hdp.SetStartTimestamp(startTimestamp)
	hdp.SetCount(100)
	hdp.SetSum(5000.0)
	hdp.SetMin(1.0)
	hdp.SetMax(500.0)
	hdp.BucketCounts().FromRaw([]uint64{10, 20, 30, 25, 15})
	hdp.ExplicitBounds().FromRaw([]float64{10, 50, 100, 250})
	hdp.Attributes().PutStr("path", "/api/v1")

	// Summary metric.
	summary := sm.Metrics().AppendEmpty()
	summary.SetName("rpc.duration")
	sdps := summary.SetEmptySummary().DataPoints().AppendEmpty()
	sdps.SetTimestamp(timestamp)
	sdps.SetStartTimestamp(startTimestamp)
	sdps.SetCount(200)
	sdps.SetSum(10000.0)
	qv1 := sdps.QuantileValues().AppendEmpty()
	qv1.SetQuantile(0.5)
	qv1.SetValue(50.0)
	qv2 := sdps.QuantileValues().AppendEmpty()
	qv2.SetQuantile(0.99)
	qv2.SetValue(200.0)
	sdps.Attributes().PutStr("service", "backend")

	// ExponentialHistogram metric.
	expHist := sm.Metrics().AppendEmpty()
	expHist.SetName("http.response.size")
	expHistMetric := expHist.SetEmptyExponentialHistogram()
	expHistMetric.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	edp := expHistMetric.DataPoints().AppendEmpty()
	edp.SetTimestamp(timestamp)
	edp.SetStartTimestamp(startTimestamp)
	edp.SetCount(50)
	edp.SetSum(25000.0)
	edp.SetMin(10.0)
	edp.SetMax(2000.0)
	edp.SetScale(3)
	edp.SetZeroCount(2)
	edp.Positive().SetOffset(1)
	edp.Positive().BucketCounts().FromRaw([]uint64{5, 10, 15})
	edp.Negative().SetOffset(-1)
	edp.Negative().BucketCounts().FromRaw([]uint64{3, 7})
	edp.Attributes().PutStr("endpoint", "/download")

	return md
}

func TestPushMetricData_EmptyMetrics(t *testing.T) {
	cfg := &Config{
		MetricsTableName: "otel_metrics",
	}
	exp := newMetricsExporter(nil, cfg)

	md := pmetric.NewMetrics()
	err := exp.pushMetricData(t.Context(), md)
	assert.NoError(t, err)
}

func TestGetValue(t *testing.T) {
	t.Run("int value", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetIntValue(42)
		assert.InDelta(t, 42.0, getValue(dp), 0.0001)
	})

	t.Run("double value", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetDoubleValue(3.14)
		assert.InDelta(t, 3.14, getValue(dp), 0.0001)
	})

	t.Run("empty value", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		assert.InDelta(t, 0.0, getValue(dp), 0.0001)
	})
}

func TestAggregationTemporalityToString(t *testing.T) {
	tests := []struct {
		name     string
		at       pmetric.AggregationTemporality
		expected string
	}{
		{"cumulative", pmetric.AggregationTemporalityCumulative, "Cumulative"},
		{"delta", pmetric.AggregationTemporalityDelta, "Delta"},
		{"unspecified", pmetric.AggregationTemporalityUnspecified, "Unspecified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, aggregationTemporalityToString(tt.at))
		})
	}
}

func TestUint64SliceToString(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, "", uint64SliceToString(nil))
		assert.Equal(t, "", uint64SliceToString([]uint64{}))
	})

	t.Run("single", func(t *testing.T) {
		assert.Equal(t, "42", uint64SliceToString([]uint64{42}))
	})

	t.Run("multiple", func(t *testing.T) {
		assert.Equal(t, "1,2,3,4,5", uint64SliceToString([]uint64{1, 2, 3, 4, 5}))
	})
}

func TestFloat64SliceToString(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, "", float64SliceToString(nil))
		assert.Equal(t, "", float64SliceToString([]float64{}))
	})

	t.Run("single", func(t *testing.T) {
		assert.Equal(t, "3.14", float64SliceToString([]float64{3.14}))
	})

	t.Run("multiple", func(t *testing.T) {
		assert.Equal(t, "10,50,100,250", float64SliceToString([]float64{10, 50, 100, 250}))
	})

	t.Run("integer values", func(t *testing.T) {
		// float64 integer values should not have decimal point
		assert.Equal(t, "1,2,3", float64SliceToString([]float64{1.0, 2.0, 3.0}))
	})
}

func TestExtractQuantiles(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		qvs := pmetric.NewSummaryDataPointValueAtQuantileSlice()
		q, v := extractQuantiles(qvs)
		assert.Equal(t, "", q)
		assert.Equal(t, "", v)
	})

	t.Run("single quantile", func(t *testing.T) {
		qvs := pmetric.NewSummaryDataPointValueAtQuantileSlice()
		qv := qvs.AppendEmpty()
		qv.SetQuantile(0.5)
		qv.SetValue(100.0)
		q, v := extractQuantiles(qvs)
		assert.Equal(t, "0.5", q)
		assert.Equal(t, "100", v)
	})

	t.Run("multiple quantiles", func(t *testing.T) {
		qvs := pmetric.NewSummaryDataPointValueAtQuantileSlice()
		qv1 := qvs.AppendEmpty()
		qv1.SetQuantile(0.5)
		qv1.SetValue(50.0)
		qv2 := qvs.AppendEmpty()
		qv2.SetQuantile(0.9)
		qv2.SetValue(90.0)
		qv3 := qvs.AppendEmpty()
		qv3.SetQuantile(0.99)
		qv3.SetValue(99.0)
		q, v := extractQuantiles(qvs)
		assert.Equal(t, "0.5,0.9,0.99", q)
		assert.Equal(t, "50,90,99", v)
	})
}

func TestPushMetricData_GaugeConversion(t *testing.T) {
	md := newTestMetrics()

	rm := md.ResourceMetrics().At(0)
	serviceName := getServiceName(rm.Resource())
	assert.Equal(t, "test-service", serviceName)

	resourceAttrs, err := attributesToJSON(rm.Resource().Attributes())
	require.NoError(t, err)
	assert.Contains(t, string(resourceAttrs), `"service.name":"test-service"`)

	sm := rm.ScopeMetrics().At(0)
	assert.Equal(t, "test-scope", sm.Scope().Name())
	assert.Equal(t, "v1.0.0", sm.Scope().Version())

	// First metric is Gauge.
	gauge := sm.Metrics().At(0)
	assert.Equal(t, "cpu.utilization", gauge.Name())
	assert.Equal(t, pmetric.MetricTypeGauge, gauge.Type())

	dp := gauge.Gauge().DataPoints().At(0)
	assert.InDelta(t, 72.5, getValue(dp), 0.0001)

	attrs, err := attributesToJSON(dp.Attributes())
	require.NoError(t, err)
	assert.Contains(t, string(attrs), `"cpu":"cpu0"`)
}

func TestPushMetricData_SumConversion(t *testing.T) {
	md := newTestMetrics()

	sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0)

	// Second metric is Sum.
	sumMetric := sm.Metrics().At(1)
	assert.Equal(t, "http.request.count", sumMetric.Name())
	assert.Equal(t, pmetric.MetricTypeSum, sumMetric.Type())

	sum := sumMetric.Sum()
	assert.True(t, sum.IsMonotonic())
	assert.Equal(t, "Cumulative", aggregationTemporalityToString(sum.AggregationTemporality()))

	dp := sum.DataPoints().At(0)
	assert.InDelta(t, 1024.0, getValue(dp), 0.0001)
}

func TestPushMetricData_HistogramConversion(t *testing.T) {
	md := newTestMetrics()

	sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0)

	// Third metric is Histogram.
	histMetric := sm.Metrics().At(2)
	assert.Equal(t, "http.request.duration", histMetric.Name())
	assert.Equal(t, pmetric.MetricTypeHistogram, histMetric.Type())

	hist := histMetric.Histogram()
	assert.Equal(t, "Delta", aggregationTemporalityToString(hist.AggregationTemporality()))

	dp := hist.DataPoints().At(0)
	assert.Equal(t, uint64(100), dp.Count())
	assert.InDelta(t, 5000.0, dp.Sum(), 0.0001)
	assert.InDelta(t, 1.0, dp.Min(), 0.0001)
	assert.InDelta(t, 500.0, dp.Max(), 0.0001)
	assert.Equal(t, "10,20,30,25,15", uint64SliceToString(dp.BucketCounts().AsRaw()))
	assert.Equal(t, "10,50,100,250", float64SliceToString(dp.ExplicitBounds().AsRaw()))
}

func TestPushMetricData_SummaryConversion(t *testing.T) {
	md := newTestMetrics()

	sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0)

	// Fourth metric is Summary.
	summaryMetric := sm.Metrics().At(3)
	assert.Equal(t, "rpc.duration", summaryMetric.Name())
	assert.Equal(t, pmetric.MetricTypeSummary, summaryMetric.Type())

	dp := summaryMetric.Summary().DataPoints().At(0)
	assert.Equal(t, uint64(200), dp.Count())
	assert.InDelta(t, 10000.0, dp.Sum(), 0.0001)

	q, v := extractQuantiles(dp.QuantileValues())
	assert.Equal(t, "0.5,0.99", q)
	assert.Equal(t, "50,200", v)
}

func TestPushMetricData_ExpHistogramConversion(t *testing.T) {
	md := newTestMetrics()

	sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0)

	// Fifth metric is ExponentialHistogram.
	expHistMetric := sm.Metrics().At(4)
	assert.Equal(t, "http.response.size", expHistMetric.Name())
	assert.Equal(t, pmetric.MetricTypeExponentialHistogram, expHistMetric.Type())

	expHist := expHistMetric.ExponentialHistogram()
	assert.Equal(t, "Cumulative", aggregationTemporalityToString(expHist.AggregationTemporality()))

	dp := expHist.DataPoints().At(0)
	assert.Equal(t, uint64(50), dp.Count())
	assert.InDelta(t, 25000.0, dp.Sum(), 0.0001)
	assert.InDelta(t, 10.0, dp.Min(), 0.0001)
	assert.InDelta(t, 2000.0, dp.Max(), 0.0001)
	assert.Equal(t, int32(3), dp.Scale())
	assert.Equal(t, uint64(2), dp.ZeroCount())
	assert.Equal(t, int32(1), dp.Positive().Offset())
	assert.Equal(t, "5,10,15", uint64SliceToString(dp.Positive().BucketCounts().AsRaw()))
	assert.Equal(t, int32(-1), dp.Negative().Offset())
	assert.Equal(t, "3,7", uint64SliceToString(dp.Negative().BucketCounts().AsRaw()))
}

func TestMetricsColumnCounts(t *testing.T) {
	// Verify constant calculations match DDL definitions.
	assert.Equal(t, 10, gaugeColumnsCount)
	assert.Equal(t, 13, sumColumnsCount)
	assert.Equal(t, 17, histogramColumnsCount)
	assert.Equal(t, 14, summaryColumnsCount)
	assert.Equal(t, 21, expHistogramColumnsCount)

	// Verify all column counts fit within PostgreSQL parameter limit.
	for _, count := range []int{gaugeColumnsCount, sumColumnsCount, histogramColumnsCount, summaryColumnsCount, expHistogramColumnsCount} {
		maxPerBatch := maxInsertParams / count
		assert.True(t, maxPerBatch*count <= maxInsertParams)
		assert.True(t, maxPerBatch > 0)
	}
}

func TestMetricsBatch_AddRow(t *testing.T) {
	batch := &metricsBatch{
		tableName:   "test_table",
		columns:     "a, b, c",
		columnCount: 3,
	}

	batch.addRow(1, "two", 3.0)
	batch.addRow(4, "five", 6.0)

	assert.Len(t, batch.rows, 2)
	assert.Equal(t, 1, batch.rows[0][0])
	assert.Equal(t, "five", batch.rows[1][1])
}

func TestMetricsBatch_InsertEmpty(t *testing.T) {
	batch := &metricsBatch{
		tableName:   "test_table",
		columns:     "a, b",
		columnCount: 2,
	}

	// Inserting with no rows should be a no-op.
	err := batch.insert(t.Context(), nil)
	assert.NoError(t, err)
}

func TestMetricsBatch_Insert_WithDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 2))

	batch := &metricsBatch{
		tableName:   "test_table",
		columns:     "a, b, c",
		columnCount: 3,
	}
	batch.addRow(1, "two", 3.0)
	batch.addRow(4, "five", 6.0)

	err = batch.insert(t.Context(), db)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMetricsBatch_Insert_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("insert failed"))

	batch := &metricsBatch{
		tableName:   "test_table",
		columns:     "a, b",
		columnCount: 2,
	}
	batch.addRow(1, "two")

	err = batch.insert(t.Context(), db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert failed")
}

func TestPushMetricData_WithDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// newTestMetrics() produces 5 metric types, each with data -> 5 INSERT calls.
	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 1))

	cfg := &Config{MetricsTableName: "otel_metrics"}
	exp := &metricsExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	md := newTestMetrics()
	err = exp.pushMetricData(t.Context(), md)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPushMetricData_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// First batch (gauge) insert fails.
	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("db error"))

	cfg := &Config{MetricsTableName: "otel_metrics"}
	exp := &metricsExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	md := newTestMetrics()
	err = exp.pushMetricData(t.Context(), md)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestMetricsExporter_Shutdown(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		exp := &metricsExporter{
			logger: zap.NewNop(),
			cfg:    &Config{},
		}
		err := exp.shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("with db", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)

		mock.ExpectClose()

		exp := &metricsExporter{
			logger: zap.NewNop(),
			cfg:    &Config{},
			db:     db,
		}
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	})
}

func TestMetricsExporter_Start_DBError(t *testing.T) {
	exp := &metricsExporter{
		logger: zap.NewNop(),
		cfg: &Config{
			DSN: "postgresql://user:pass@localhost:1/db",
		},
	}
	err := exp.start(context.Background(), nil)
	require.Error(t, err)
}
