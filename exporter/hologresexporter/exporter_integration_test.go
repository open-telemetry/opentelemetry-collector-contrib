// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package hologresexporter

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const testDSN = "postgresql://LTAI5tG7Q7uw5puUBpkMFXkt:wDVVrVjLH1L08RzNt77wbKGPc4E0Ur@hgpostcn-cn-ypo4sjx5v005-cn-hangzhou.hologres.aliyuncs.com:80/otel?sslmode=disable"

// uniqPrefix returns a unique table-name prefix using current Unix nanoseconds
// to avoid collisions across concurrent test runs.
func uniqPrefix(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("otel_it_%d", time.Now().UnixNano())
}

// dropTablesQuiet drops the given tables, ignoring errors. No transactions used.
func dropTablesQuiet(ctx context.Context, db *sql.DB, tables ...string) {
	for _, tbl := range tables {
		_, _ = db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", tbl))
	}
}

// TestIntegration_Connection verifies basic connectivity against the Hologres instance.
func TestIntegration_Connection(t *testing.T) {
	db, err := sql.Open("pgx", testDSN)
	require.NoError(t, err)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, db.PingContext(ctx))

	var result int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT 1").Scan(&result))
	assert.Equal(t, 1, result)
}

// TestIntegration_CreateTables exercises the DDL helpers against a real Hologres instance.
func TestIntegration_CreateTables(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", testDSN)
	require.NoError(t, err)
	defer db.Close()

	prefix := uniqPrefix(t)
	tracesTable := prefix + "_traces"
	logsTable := prefix + "_logs"
	metricsTable := prefix + "_metrics"

	defer dropTablesQuiet(ctx, db,
		tracesTable,
		logsTable,
		metricsTable+"_gauge",
		metricsTable+"_sum",
		metricsTable+"_histogram",
		metricsTable+"_summary",
		metricsTable+"_exp_histogram",
	)

	require.NoError(t, createTracesTable(ctx, db, tracesTable, 24*time.Hour), "failed to create traces table")
	require.NoError(t, createLogsTable(ctx, db, logsTable, 24*time.Hour), "failed to create logs table")
	require.NoError(t, createMetricsTables(ctx, db, metricsTable, 24*time.Hour), "failed to create metrics tables")

	for _, tbl := range []string{
		tracesTable,
		logsTable,
		metricsTable + "_gauge",
		metricsTable + "_sum",
		metricsTable + "_histogram",
		metricsTable + "_summary",
		metricsTable + "_exp_histogram",
	} {
		var exists bool
		err := db.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
			tbl).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "table %s should exist", tbl)
	}
}

// TestIntegration_TracesInsertAndQuery writes traces through the exporter and reads them back.
func TestIntegration_TracesInsertAndQuery(t *testing.T) {
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
	defer func() {
		dropTablesQuiet(ctx, exp.db, cfg.TracesTableName)
		_ = exp.shutdown(ctx)
	}()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test-scope")
	ss.Scope().SetVersion("1.0.0")

	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("test-span")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Second)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutInt("http.status_code", 200)

	require.NoError(t, exp.pushTraceData(ctx, td), "failed to insert traces")

	var count int
	require.NoError(t, exp.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s", cfg.TracesTableName)).Scan(&count))
	assert.Equal(t, 1, count, "should have 1 trace record")

	var serviceName, spanName, statusCode string
	require.NoError(t, exp.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT service_name, span_name, status_code FROM %s LIMIT 1", cfg.TracesTableName)).
		Scan(&serviceName, &spanName, &statusCode))
	assert.Equal(t, "test-service", serviceName)
	assert.Equal(t, "test-span", spanName)
	assert.Equal(t, "STATUS_CODE_OK", statusCode)
}

// TestIntegration_LogsInsertAndQuery writes logs through the exporter and reads them back.
func TestIntegration_LogsInsertAndQuery(t *testing.T) {
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
	defer func() {
		dropTablesQuiet(ctx, exp.db, cfg.LogsTableName)
		_ = exp.shutdown(ctx)
	}()

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-log-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("test-scope")

	log := sl.LogRecords().AppendEmpty()
	log.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	log.SetSeverityText("ERROR")
	log.SetSeverityNumber(plog.SeverityNumberError)
	log.Body().SetStr("test error message: something went wrong")
	log.Attributes().PutStr("error.type", "NullPointerException")

	require.NoError(t, exp.pushLogData(ctx, ld), "failed to insert logs")

	var count int
	require.NoError(t, exp.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s", cfg.LogsTableName)).Scan(&count))
	assert.Equal(t, 1, count)

	var serviceName, severityText, body string
	require.NoError(t, exp.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT service_name, severity_text, body FROM %s LIMIT 1", cfg.LogsTableName)).
		Scan(&serviceName, &severityText, &body))
	assert.Equal(t, "test-log-service", serviceName)
	assert.Equal(t, "ERROR", severityText)
	assert.Contains(t, body, "something went wrong")
}

// TestIntegration_MetricsGaugeInsertAndQuery writes a gauge metric and reads it back.
func TestIntegration_MetricsGaugeInsertAndQuery(t *testing.T) {
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
	defer func() {
		dropTablesQuiet(ctx, exp.db,
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
	rm.Resource().Attributes().PutStr("service.name", "test-metrics-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")

	metric := sm.Metrics().AppendEmpty()
	metric.SetName("cpu_usage")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(75.5)
	dp.Attributes().PutStr("host", "server-01")

	require.NoError(t, exp.pushMetricData(ctx, md), "failed to insert metrics")

	gaugeTable := cfg.MetricsTableName + "_gauge"
	var count int
	require.NoError(t, exp.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s", gaugeTable)).Scan(&count))
	assert.Equal(t, 1, count)

	var metricName, serviceName string
	var value float64
	require.NoError(t, exp.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT metric_name, service_name, value FROM %s LIMIT 1", gaugeTable)).
		Scan(&metricName, &serviceName, &value))
	assert.Equal(t, "cpu_usage", metricName)
	assert.Equal(t, "test-metrics-service", serviceName)
	assert.InDelta(t, 75.5, value, 0.0001)
}
