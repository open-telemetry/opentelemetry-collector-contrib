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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zaptest"
)

func TestLogsExporterIntegration(t *testing.T) {
	dsn := getMySQLDSN(t)
	cfg := newTestConfig(dsn)

	exporter := newLogsExporter(zaptest.NewLogger(t), cfg)
	require.NoError(t, exporter.start(t.Context(), nil))
	t.Cleanup(func() { _ = exporter.shutdown(t.Context()) })

	// Truncate before test
	db := queryDB(t, dsn)
	truncateTable(t, db, cfg.Database, cfg.LogsTableName)

	// Push test data
	logs := simpleLogs(10)
	require.NoError(t, exporter.pushLogsData(t.Context(), logs))

	// Verify
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM `otel_test`.`otel_logs`").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 10, count)

	// Verify first row content
	var (
		ts           time.Time
		traceID      string
		spanID       string
		traceFlags   uint8
		severityText string
		severityNum  uint8
		serviceName  string
		body         string
		resURL       string
		resAttrRaw   []byte
		scopeURL     string
		scopeName    string
		scopeVersion string
		scopeAttrRaw []byte
		logAttrRaw   []byte
	)
	err = db.QueryRow(`SELECT timestamp, trace_id, span_id, trace_flags, severity_text,
		severity_number, service_name, body, resource_schema_url, resource_attributes,
		scope_schema_url, scope_name, scope_version, scope_attributes, log_attributes
		FROM `+"`otel_test`.`otel_logs`"+` ORDER BY id LIMIT 1`).Scan(
		&ts, &traceID, &spanID, &traceFlags, &severityText,
		&severityNum, &serviceName, &body, &resURL, &resAttrRaw,
		&scopeURL, &scopeName, &scopeVersion, &scopeAttrRaw, &logAttrRaw,
	)
	require.NoError(t, err)

	assert.Equal(t, "01020300000000000000000000000000", traceID)
	assert.Equal(t, "0102030000000000", spanID)
	assert.Equal(t, "error", severityText)
	assert.Equal(t, uint8(18), severityNum)
	assert.Equal(t, "test-service", serviceName)
	assert.Equal(t, "error message", body)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.4.0", resURL)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.7.0", scopeURL)
	assert.Equal(t, "io.opentelemetry.contrib.mysql", scopeName)
	assert.Equal(t, "1.0.0", scopeVersion)

	// Verify JSON attributes
	var resAttrs map[string]string
	require.NoError(t, json.Unmarshal(resAttrRaw, &resAttrs))
	assert.Equal(t, "test-service", resAttrs["service.name"])

	var scopeAttrs map[string]string
	require.NoError(t, json.Unmarshal(scopeAttrRaw, &scopeAttrs))
	assert.Equal(t, "mysql", scopeAttrs["lib"])

	var logAttrs map[string]string
	require.NoError(t, json.Unmarshal(logAttrRaw, &logAttrs))
	assert.Equal(t, "default", logAttrs["service.namespace"])

	// Verify TTL event was created
	var eventCount int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA = ? AND EVENT_NAME = ?",
		cfg.Database, "ttl_otel_logs",
	).Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount, "TTL event ttl_otel_logs should exist")
}

func simpleLogs(count int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.4.0")
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.7.0")
	sl.Scope().SetName("io.opentelemetry.contrib.mysql")
	sl.Scope().SetVersion("1.0.0")
	sl.Scope().Attributes().PutStr("lib", "mysql")
	for i := range count {
		r := sl.LogRecords().AppendEmpty()
		r.SetTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp))
		r.SetObservedTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp))
		r.SetSeverityNumber(plog.SeverityNumberError2)
		r.SetSeverityText("error")
		r.Body().SetStr("error message")
		r.Attributes().PutStr("service.namespace", "default")
		r.SetFlags(plog.DefaultLogRecordFlags)
		r.SetTraceID([16]byte{1, 2, 3, byte(i)})
		r.SetSpanID([8]byte{1, 2, 3, byte(i)})
	}
	return logs
}
