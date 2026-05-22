// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package hologresexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter/internal/metadata"
)

// --- Metrics: Sum ---

func TestIntegration_MetricsSumInsertAndQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:              testDSN,
		MetricsTableName: prefix + "_metrics",
		CreateSchema:     true,
		TTL:              24 * time.Hour,
	}

	exp := newMetricsExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool,
			cfg.MetricsTableName+"_gauge",
			cfg.MetricsTableName+"_sum",
			cfg.MetricsTableName+"_histogram",
			cfg.MetricsTableName+"_summary",
			cfg.MetricsTableName+"_exp_histogram",
		)
		_ = exp.shutdown(ctx)
	}()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "sum-test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("http.request.count")
	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Minute)))
	dp.SetDoubleValue(42.5)
	dp.Attributes().PutStr("http.method", "GET")

	require.NoError(t, exp.pushMetricData(ctx, md))

	sumTable := cfg.MetricsTableName + "_sum"
	var count int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", sumTable)).Scan(&count))
	assert.Equal(t, 1, count)

	var metricName string
	var isMonotonic bool
	var aggTemp string
	require.NoError(t, pool.QueryRow(ctx,
		fmt.Sprintf("SELECT metric_name, is_monotonic, aggregation_temporality FROM %s LIMIT 1", sumTable)).
		Scan(&metricName, &isMonotonic, &aggTemp))
	assert.Equal(t, "http.request.count", metricName)
	assert.True(t, isMonotonic)
	assert.Equal(t, "Cumulative", aggTemp)
}

// --- Metrics: Histogram ---

func TestIntegration_MetricsHistogramInsertAndQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:              testDSN,
		MetricsTableName: prefix + "_metrics",
		CreateSchema:     true,
		TTL:              24 * time.Hour,
	}

	exp := newMetricsExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool,
			cfg.MetricsTableName+"_gauge",
			cfg.MetricsTableName+"_sum",
			cfg.MetricsTableName+"_histogram",
			cfg.MetricsTableName+"_summary",
			cfg.MetricsTableName+"_exp_histogram",
		)
		_ = exp.shutdown(ctx)
	}()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "hist-test-service")
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("http.request.duration")
	hist := metric.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := hist.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Minute)))
	dp.SetCount(100)
	dp.SetSum(5000.0)
	dp.SetMin(10.0)
	dp.SetMax(200.0)
	dp.BucketCounts().Append(10, 30, 40, 15, 5)
	dp.ExplicitBounds().Append(25.0, 50.0, 100.0, 150.0)
	dp.Attributes().PutStr("endpoint", "/api/users")

	require.NoError(t, exp.pushMetricData(ctx, md))

	histTable := cfg.MetricsTableName + "_histogram"
	var cnt int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", histTable)).Scan(&cnt))
	assert.Equal(t, 1, cnt)

	var metricName, bucketCounts, explicitBounds, aggTemp string
	var histCount int64
	var histSum float64
	require.NoError(t, pool.QueryRow(ctx,
		fmt.Sprintf("SELECT metric_name, count, sum, bucket_counts, explicit_bounds, aggregation_temporality FROM %s LIMIT 1", histTable)).
		Scan(&metricName, &histCount, &histSum, &bucketCounts, &explicitBounds, &aggTemp))
	assert.Equal(t, "http.request.duration", metricName)
	assert.Equal(t, int64(100), histCount)
	assert.InDelta(t, 5000.0, histSum, 0.01)
	assert.Contains(t, bucketCounts, "10")
	assert.Contains(t, explicitBounds, "25")
	assert.Equal(t, "Delta", aggTemp)
}

// --- Metrics: Summary ---

func TestIntegration_MetricsSummaryInsertAndQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:              testDSN,
		MetricsTableName: prefix + "_metrics",
		CreateSchema:     true,
		TTL:              24 * time.Hour,
	}

	exp := newMetricsExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool,
			cfg.MetricsTableName+"_gauge",
			cfg.MetricsTableName+"_sum",
			cfg.MetricsTableName+"_histogram",
			cfg.MetricsTableName+"_summary",
			cfg.MetricsTableName+"_exp_histogram",
		)
		_ = exp.shutdown(ctx)
	}()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "summary-test-service")
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("rpc.duration")
	summary := metric.SetEmptySummary()
	dp := summary.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Minute)))
	dp.SetCount(200)
	dp.SetSum(15000.0)
	qv1 := dp.QuantileValues().AppendEmpty()
	qv1.SetQuantile(0.5)
	qv1.SetValue(50.0)
	qv2 := dp.QuantileValues().AppendEmpty()
	qv2.SetQuantile(0.99)
	qv2.SetValue(180.0)
	dp.Attributes().PutStr("rpc.method", "GetUser")

	require.NoError(t, exp.pushMetricData(ctx, md))

	summaryTable := cfg.MetricsTableName + "_summary"
	var cnt int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", summaryTable)).Scan(&cnt))
	assert.Equal(t, 1, cnt)

	var metricName, quantileValues, quantileCounts string
	require.NoError(t, pool.QueryRow(ctx,
		fmt.Sprintf("SELECT metric_name, quantile_values, quantile_counts FROM %s LIMIT 1", summaryTable)).
		Scan(&metricName, &quantileValues, &quantileCounts))
	assert.Equal(t, "rpc.duration", metricName)
	assert.Contains(t, quantileValues, "0.5")
	assert.Contains(t, quantileCounts, "50")
}

// --- Metrics: Exponential Histogram ---

func TestIntegration_MetricsExpHistogramInsertAndQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:              testDSN,
		MetricsTableName: prefix + "_metrics",
		CreateSchema:     true,
		TTL:              24 * time.Hour,
	}

	exp := newMetricsExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool,
			cfg.MetricsTableName+"_gauge",
			cfg.MetricsTableName+"_sum",
			cfg.MetricsTableName+"_histogram",
			cfg.MetricsTableName+"_summary",
			cfg.MetricsTableName+"_exp_histogram",
		)
		_ = exp.shutdown(ctx)
	}()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "exphist-test-service")
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("http.latency.exp")
	expHist := metric.SetEmptyExponentialHistogram()
	expHist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := expHist.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Minute)))
	dp.SetCount(50)
	dp.SetSum(2500.0)
	dp.SetMin(5.0)
	dp.SetMax(150.0)
	dp.SetScale(3)
	dp.SetZeroCount(2)
	dp.Positive().SetOffset(1)
	dp.Positive().BucketCounts().Append(5, 10, 15, 12, 8)
	dp.Negative().SetOffset(0)
	dp.Negative().BucketCounts().Append(1, 2, 3)
	dp.Attributes().PutStr("endpoint", "/api/data")

	require.NoError(t, exp.pushMetricData(ctx, md))

	expHistTable := cfg.MetricsTableName + "_exp_histogram"
	var cnt int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", expHistTable)).Scan(&cnt))
	assert.Equal(t, 1, cnt)

	var metricName string
	var scale, positiveOffset, negativeOffset int32
	var zeroCount int64
	require.NoError(t, pool.QueryRow(ctx,
		fmt.Sprintf("SELECT metric_name, scale, zero_count, positive_offset, negative_offset FROM %s LIMIT 1", expHistTable)).
		Scan(&metricName, &scale, &zeroCount, &positiveOffset, &negativeOffset))
	assert.Equal(t, "http.latency.exp", metricName)
	assert.Equal(t, int32(3), scale)
	assert.Equal(t, int64(2), zeroCount)
	assert.Equal(t, int32(1), positiveOffset)
	assert.Equal(t, int32(0), negativeOffset)
}

// --- Large batch write ---

func TestIntegration_LargeBatchTraces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:             testDSN,
		TracesTableName: prefix + "_traces",
		CreateSchema:    true,
		TTL:             24 * time.Hour,
	}

	exp := newTracesExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool, cfg.TracesTableName)
		_ = exp.shutdown(ctx)
	}()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "batch-trace-service")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("batch-scope")

	const numSpans = 150
	for i := range numSpans {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(pcommon.TraceID([16]byte{byte(i), 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
		span.SetSpanID(pcommon.SpanID([8]byte{byte(i), 2, 3, 4, 5, 6, 7, 8}))
		span.SetName(fmt.Sprintf("span-%d", i))
		span.SetKind(ptrace.SpanKindClient)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Second)))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.Attributes().PutInt("index", int64(i))
	}

	require.NoError(t, exp.pushTraceData(ctx, td))

	var count int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", cfg.TracesTableName)).Scan(&count))
	assert.Equal(t, numSpans, count)
}

func TestIntegration_LargeBatchLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:           testDSN,
		LogsTableName: prefix + "_logs",
		CreateSchema:  true,
		TTL:           24 * time.Hour,
	}

	exp := newLogsExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool, cfg.LogsTableName)
		_ = exp.shutdown(ctx)
	}()

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "batch-log-service")
	sl := rl.ScopeLogs().AppendEmpty()

	const numLogs = 200
	for i := range numLogs {
		log := sl.LogRecords().AppendEmpty()
		log.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		log.SetSeverityText("INFO")
		log.SetSeverityNumber(plog.SeverityNumberInfo)
		log.Body().SetStr(fmt.Sprintf("log message %d", i))
		log.Attributes().PutInt("index", int64(i))
	}

	require.NoError(t, exp.pushLogData(ctx, ld))

	var count int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", cfg.LogsTableName)).Scan(&count))
	assert.Equal(t, numLogs, count)
}

func TestIntegration_LargeBatchMetrics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:              testDSN,
		MetricsTableName: prefix + "_metrics",
		CreateSchema:     true,
		TTL:              24 * time.Hour,
	}

	exp := newMetricsExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool,
			cfg.MetricsTableName+"_gauge",
			cfg.MetricsTableName+"_sum",
			cfg.MetricsTableName+"_histogram",
			cfg.MetricsTableName+"_summary",
			cfg.MetricsTableName+"_exp_histogram",
		)
		_ = exp.shutdown(ctx)
	}()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "batch-metric-service")
	sm := rm.ScopeMetrics().AppendEmpty()

	const numMetrics = 100
	for i := range numMetrics {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(fmt.Sprintf("metric.gauge.%d", i))
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(float64(i) * 1.5)
		dp.Attributes().PutInt("index", int64(i))
	}

	require.NoError(t, exp.pushMetricData(ctx, md))

	gaugeTable := cfg.MetricsTableName + "_gauge"
	var count int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", gaugeTable)).Scan(&count))
	assert.Equal(t, numMetrics, count)
}

// --- DDL: TTL, Idempotency ---

func TestIntegration_DDL_WithTTL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, testDSN)
	require.NoError(t, err)
	defer db.Close()

	prefix := uniqPrefix(t)
	tracesTable := prefix + "_traces_ttl"
	logsTable := prefix + "_logs_ttl"
	metricsTable := prefix + "_metrics_ttl"

	defer dropTablesQuiet(ctx, db,
		tracesTable,
		logsTable,
		metricsTable+"_gauge",
		metricsTable+"_sum",
		metricsTable+"_histogram",
		metricsTable+"_summary",
		metricsTable+"_exp_histogram",
	)

	// Use a 7-day TTL
	ttl := 7 * 24 * time.Hour
	require.NoError(t, createTracesTable(ctx, db, tracesTable, ttl))
	require.NoError(t, createLogsTable(ctx, db, logsTable, ttl))
	require.NoError(t, createMetricsTables(ctx, db, metricsTable, ttl))

	// Verify all tables exist
	for _, tbl := range []string{
		tracesTable, logsTable,
		metricsTable + "_gauge", metricsTable + "_sum",
		metricsTable + "_histogram", metricsTable + "_summary",
		metricsTable + "_exp_histogram",
	} {
		var exists bool
		err := db.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", tbl).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "table %s should exist", tbl)
	}
}

func TestIntegration_DDL_Idempotency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, testDSN)
	require.NoError(t, err)
	defer db.Close()

	prefix := uniqPrefix(t)
	tracesTable := prefix + "_traces_idem"

	defer dropTablesQuiet(ctx, db, tracesTable)

	// Create table twice - should not error (CREATE TABLE IF NOT EXISTS)
	require.NoError(t, createTracesTable(ctx, db, tracesTable, 0))
	require.NoError(t, createTracesTable(ctx, db, tracesTable, 0))
}

// --- Start/Shutdown lifecycle ---

func TestIntegration_LifecycleStartPushShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:             testDSN,
		TracesTableName: prefix + "_traces",
		CreateSchema:    true,
		TTL:             0,
	}

	exp := newTracesExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer dropTablesQuiet(ctx, pool, cfg.TracesTableName)

	// Push data
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "lifecycle-test")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("lifecycle-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Second)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	require.NoError(t, exp.pushTraceData(ctx, td))

	// Shutdown
	require.NoError(t, exp.shutdown(ctx))
}

func TestIntegration_CreateSchemaFalse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Pre-create the table
	db, err := pgxpool.New(ctx, testDSN)
	require.NoError(t, err)
	defer db.Close()

	prefix := uniqPrefix(t)
	tableName := prefix + "_traces_noschema"
	defer dropTablesQuiet(ctx, db, tableName)

	require.NoError(t, createTracesTable(ctx, db, tableName, 0))

	// Start with CreateSchema=false
	cfg := &Config{
		DSN:             testDSN,
		TracesTableName: tableName,
		CreateSchema:    false,
	}
	exp := newTracesExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	defer func() { _ = exp.shutdown(ctx) }()

	// Push data should still work
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "noschema-test")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("noschema-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Second)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	require.NoError(t, exp.pushTraceData(ctx, td))
}

// --- Factory creation ---

func TestIntegration_FactoryCreateTracesExporter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.DSN = testDSN
	cfg.TracesTableName = prefix + "_traces_factory"
	cfg.CreateSchema = true
	cfg.TTL = 24 * time.Hour

	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateTraces(ctx, set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	require.NoError(t, exp.Start(ctx, nil))
	defer func() {
		_ = exp.Shutdown(ctx)
		db, _ := pgxpool.New(ctx, testDSN)
		if db != nil {
			dropTablesQuiet(ctx, db, cfg.TracesTableName)
			db.Close()
		}
	}()

	// Push through the factory-created exporter
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "factory-test")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("factory-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Second)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	require.NoError(t, exp.ConsumeTraces(ctx, td))
}

func TestIntegration_FactoryCreateLogsExporter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.DSN = testDSN
	cfg.LogsTableName = prefix + "_logs_factory"
	cfg.CreateSchema = true

	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateLogs(ctx, set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	require.NoError(t, exp.Start(ctx, nil))
	defer func() {
		_ = exp.Shutdown(ctx)
		db, _ := pgxpool.New(ctx, testDSN)
		if db != nil {
			dropTablesQuiet(ctx, db, cfg.LogsTableName)
			db.Close()
		}
	}()

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "factory-log-test")
	sl := rl.ScopeLogs().AppendEmpty()
	log := sl.LogRecords().AppendEmpty()
	log.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	log.SetSeverityText("WARN")
	log.Body().SetStr("factory log test message")
	require.NoError(t, exp.ConsumeLogs(ctx, ld))
}

func TestIntegration_FactoryCreateMetricsExporter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.DSN = testDSN
	cfg.MetricsTableName = prefix + "_metrics_factory"
	cfg.CreateSchema = true

	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateMetrics(ctx, set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	require.NoError(t, exp.Start(ctx, nil))
	defer func() {
		_ = exp.Shutdown(ctx)
		db, _ := pgxpool.New(ctx, testDSN)
		if db != nil {
			dropTablesQuiet(ctx, db,
				cfg.MetricsTableName+"_gauge",
				cfg.MetricsTableName+"_sum",
				cfg.MetricsTableName+"_histogram",
				cfg.MetricsTableName+"_summary",
				cfg.MetricsTableName+"_exp_histogram",
			)
			db.Close()
		}
	}()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "factory-metric-test")
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("factory.gauge")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(99.9)
	require.NoError(t, exp.ConsumeMetrics(ctx, md))
}

// --- Complex attributes ---

func TestIntegration_ComplexAttributes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:             testDSN,
		TracesTableName: prefix + "_traces",
		CreateSchema:    true,
		TTL:             24 * time.Hour,
	}

	exp := newTracesExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool, cfg.TracesTableName)
		_ = exp.shutdown(ctx)
	}()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	res := rs.Resource().Attributes()
	res.PutStr("service.name", "complex-attrs-test")

	// Nested map attribute
	nestedMap := res.PutEmptyMap("config")
	nestedMap.PutStr("region", "us-east-1")
	nestedMap.PutInt("retry_count", 3)
	innerMap := nestedMap.PutEmptyMap("inner")
	innerMap.PutBool("enabled", true)

	// Slice attribute
	sliceAttr := res.PutEmptySlice("tags")
	sliceAttr.AppendEmpty().SetStr("production")
	sliceAttr.AppendEmpty().SetStr("critical")
	sliceAttr.AppendEmpty().SetInt(42)

	// Bytes attribute
	res.PutEmptyBytes("binary_data").Append(0xDE, 0xAD, 0xBE, 0xEF)

	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("complex-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Second)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Complex span attributes
	span.Attributes().PutStr("simple", "value")
	spanMap := span.Attributes().PutEmptyMap("nested")
	spanMap.PutDouble("score", 98.5)

	require.NoError(t, exp.pushTraceData(ctx, td))

	var count int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", cfg.TracesTableName)).Scan(&count))
	assert.Equal(t, 1, count)
}

// --- Empty data push ---

func TestIntegration_EmptyDataPush(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)

	t.Run("empty_traces", func(t *testing.T) {
		cfg := &Config{
			DSN:             testDSN,
			TracesTableName: prefix + "_traces_empty",
			CreateSchema:    true,
		}
		exp := newTracesExporter(zap.NewNop(), cfg)
		require.NoError(t, exp.start(ctx, nil))
		defer func() {
			pool := exp.db.(*hologresPool).pool
			dropTablesQuiet(ctx, pool, cfg.TracesTableName)
			_ = exp.shutdown(ctx)
		}()

		td := ptrace.NewTraces() // empty, no spans
		require.NoError(t, exp.pushTraceData(ctx, td))
	})

	t.Run("empty_logs", func(t *testing.T) {
		cfg := &Config{
			DSN:           testDSN,
			LogsTableName: prefix + "_logs_empty",
			CreateSchema:  true,
		}
		exp := newLogsExporter(zap.NewNop(), cfg)
		require.NoError(t, exp.start(ctx, nil))
		defer func() {
			pool := exp.db.(*hologresPool).pool
			dropTablesQuiet(ctx, pool, cfg.LogsTableName)
			_ = exp.shutdown(ctx)
		}()

		ld := plog.NewLogs() // empty, no log records
		require.NoError(t, exp.pushLogData(ctx, ld))
	})

	t.Run("empty_metrics", func(t *testing.T) {
		cfg := &Config{
			DSN:              testDSN,
			MetricsTableName: prefix + "_metrics_empty",
			CreateSchema:     true,
		}
		exp := newMetricsExporter(zap.NewNop(), cfg)
		require.NoError(t, exp.start(ctx, nil))
		defer func() {
			pool := exp.db.(*hologresPool).pool
			dropTablesQuiet(ctx, pool,
				cfg.MetricsTableName+"_gauge",
				cfg.MetricsTableName+"_sum",
				cfg.MetricsTableName+"_histogram",
				cfg.MetricsTableName+"_summary",
				cfg.MetricsTableName+"_exp_histogram",
			)
			_ = exp.shutdown(ctx)
		}()

		md := pmetric.NewMetrics() // empty, no metrics
		require.NoError(t, exp.pushMetricData(ctx, md))
	})
}

// --- Span events and links ---

func TestIntegration_SpanEventsAndLinks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:             testDSN,
		TracesTableName: prefix + "_traces",
		CreateSchema:    true,
		TTL:             24 * time.Hour,
	}

	exp := newTracesExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool, cfg.TracesTableName)
		_ = exp.shutdown(ctx)
	}()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "events-links-test")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("events-links-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Second)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Add events
	event1 := span.Events().AppendEmpty()
	event1.SetName("exception")
	event1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	event1.Attributes().PutStr("exception.type", "NullPointerException")
	event1.Attributes().PutStr("exception.message", "null reference")

	event2 := span.Events().AppendEmpty()
	event2.SetName("log")
	event2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	event2.Attributes().PutStr("message", "processing request")

	// Add links
	link1 := span.Links().AppendEmpty()
	link1.SetTraceID(pcommon.TraceID([16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}))
	link1.SetSpanID(pcommon.SpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1}))
	link1.Attributes().PutStr("link.type", "parent")

	link2 := span.Links().AppendEmpty()
	link2.SetTraceID(pcommon.TraceID([16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}))
	link2.SetSpanID(pcommon.SpanID([8]byte{3, 3, 3, 3, 3, 3, 3, 3}))
	link2.Attributes().PutStr("link.type", "follows_from")

	require.NoError(t, exp.pushTraceData(ctx, td))

	var count int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", cfg.TracesTableName)).Scan(&count))
	assert.Equal(t, 1, count)

	// Verify events and links are stored as non-empty JSON
	var events, links string
	require.NoError(t, pool.QueryRow(ctx,
		fmt.Sprintf("SELECT events::text, links::text FROM %s LIMIT 1", cfg.TracesTableName)).
		Scan(&events, &links))
	assert.Contains(t, events, "exception")
	assert.Contains(t, events, "NullPointerException")
	assert.Contains(t, links, "parent")
}

// --- Log timestamp fallback ---

func TestIntegration_LogTimestampFallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:           testDSN,
		LogsTableName: prefix + "_logs",
		CreateSchema:  true,
		TTL:           24 * time.Hour,
	}

	exp := newLogsExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool, cfg.LogsTableName)
		_ = exp.shutdown(ctx)
	}()

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "timestamp-fallback-test")
	sl := rl.ScopeLogs().AppendEmpty()

	log := sl.LogRecords().AppendEmpty()
	// Timestamp is 0 (not set), should use ObservedTimestamp
	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	log.SetSeverityText("DEBUG")
	log.Body().SetStr("fallback timestamp test")

	require.NoError(t, exp.pushLogData(ctx, ld))

	var count int
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", cfg.LogsTableName)).Scan(&count))
	assert.Equal(t, 1, count)
}

// --- Metrics with IntValue (covers getValue int branch) ---

func TestIntegration_MetricsGaugeIntValue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	prefix := uniqPrefix(t)
	cfg := &Config{
		DSN:              testDSN,
		MetricsTableName: prefix + "_metrics",
		CreateSchema:     true,
		TTL:              24 * time.Hour,
	}

	exp := newMetricsExporter(zap.NewNop(), cfg)
	require.NoError(t, exp.start(ctx, nil))
	pool := exp.db.(*hologresPool).pool
	defer func() {
		dropTablesQuiet(ctx, pool,
			cfg.MetricsTableName+"_gauge",
			cfg.MetricsTableName+"_sum",
			cfg.MetricsTableName+"_histogram",
			cfg.MetricsTableName+"_summary",
			cfg.MetricsTableName+"_exp_histogram",
		)
		_ = exp.shutdown(ctx)
	}()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "int-gauge-test")
	sm := rm.ScopeMetrics().AppendEmpty()

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("process.threads")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetIntValue(42) // IntValue instead of DoubleValue

	require.NoError(t, exp.pushMetricData(ctx, md))

	gaugeTable := cfg.MetricsTableName + "_gauge"
	var value float64
	require.NoError(t, pool.QueryRow(ctx,
		fmt.Sprintf("SELECT value FROM %s LIMIT 1", gaugeTable)).Scan(&value))
	assert.InDelta(t, 42.0, value, 0.0001)
}
