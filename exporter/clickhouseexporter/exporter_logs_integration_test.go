// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zaptest"
)

func testLogsExporter(t *testing.T, endpoint string, mapBody bool) {
	exporter := newTestLogsExporter(t, endpoint, false)
	verifyExportLogs(t, exporter, mapBody, false)
}

func testLogsExporterSchemaFeatures(t *testing.T, endpoint string, mapBody bool) {
	exporter := newTestLogsExporter(t, endpoint, true)
	verifyExportLogs(t, exporter, mapBody, true)
}

func newTestLogsExporter(t *testing.T, dsn string, testSchemaFeatures bool, fns ...func(*Config)) *logsExporter {
	exporter := newLogsExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))

	require.NoError(t, exporter.start(t.Context(), nil))

	// Tests the schema feature flags by disabling newer columns. The insert logic should adapt.
	if testSchemaFeatures {
		exporter.schemaFeatures.EventName = false
		exporter.renderInsertLogsSQL()
	}

	t.Cleanup(func() { _ = exporter.shutdown(t.Context()) })
	return exporter
}

func verifyExportLogs(t *testing.T, exporter *logsExporter, mapBody, testSchemaFeatures bool) {
	tableName := fmt.Sprintf("%q.%q", exporter.cfg.database(), exporter.cfg.LogsTableName)

	// Clear table for when mapBody test runs under same table name
	err := exporter.db.Exec(t.Context(), fmt.Sprintf("TRUNCATE %s", tableName))
	require.NoError(t, err)

	pushConcurrentlyNoError(t, func() error {
		return exporter.pushLogsData(t.Context(), simpleLogs(5000, mapBody))
	})

	type log struct {
		Timestamp          time.Time         `ch:"Timestamp"`
		TimestampTime      time.Time         `ch:"TimestampTime"`
		TraceID            string            `ch:"TraceId"`
		SpanID             string            `ch:"SpanId"`
		TraceFlags         uint8             `ch:"TraceFlags"`
		SeverityText       string            `ch:"SeverityText"`
		SeverityNumber     uint8             `ch:"SeverityNumber"`
		ServiceName        string            `ch:"ServiceName"`
		Body               string            `ch:"Body"`
		ResourceSchemaURL  string            `ch:"ResourceSchemaUrl"`
		ResourceAttributes map[string]string `ch:"ResourceAttributes"`
		ScopeSchemaURL     string            `ch:"ScopeSchemaUrl"`
		ScopeName          string            `ch:"ScopeName"`
		ScopeVersion       string            `ch:"ScopeVersion"`
		ScopeAttributes    map[string]string `ch:"ScopeAttributes"`
		LogAttributes      map[string]string `ch:"LogAttributes"`
		EventName          string            `ch:"EventName"`
	}

	expectedLog := log{
		Timestamp:         telemetryTimestamp,
		TimestampTime:     telemetryTimestamp,
		TraceID:           "01020300000000000000000000000000",
		SpanID:            "0102030000000000",
		SeverityText:      "error",
		SeverityNumber:    18,
		ServiceName:       "test-service",
		Body:              "error message",
		ResourceSchemaURL: "https://opentelemetry.io/schemas/1.4.0",
		ResourceAttributes: map[string]string{
			"service.name": "test-service",
		},
		ScopeSchemaURL: "https://opentelemetry.io/schemas/1.7.0",
		ScopeName:      "io.opentelemetry.contrib.clickhouse",
		ScopeVersion:   "1.0.0",
		ScopeAttributes: map[string]string{
			"lib": "clickhouse",
		},
		LogAttributes: map[string]string{
			"service.namespace": "default",
		},
		EventName: "event",
	}
	if mapBody {
		expectedLog.Body = `{"error":"message"}`
	}

	if testSchemaFeatures {
		expectedLog.EventName = ""
	}

	row := exporter.db.QueryRow(t.Context(), fmt.Sprintf("SELECT * FROM %s", tableName))
	require.NoError(t, row.Err())

	var actualLog log
	err = row.ScanStruct(&actualLog)
	require.NoError(t, err)

	require.Equal(t, expectedLog, actualLog)
}

func simpleLogs(count int, mapBody bool) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.4.0")
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.7.0")
	sl.Scope().SetName("io.opentelemetry.contrib.clickhouse")
	sl.Scope().SetVersion("1.0.0")
	sl.Scope().Attributes().PutStr("lib", "clickhouse")
	timestamp := telemetryTimestamp
	for i := range count {
		r := sl.LogRecords().AppendEmpty()
		r.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetObservedTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetSeverityNumber(plog.SeverityNumberError2)
		r.SetSeverityText("error")
		r.SetEventName("event")

		if mapBody {
			r.Body().SetEmptyMap()
			r.Body().Map().PutStr("error", "message")
		} else {
			r.Body().SetStr("error message")
		}

		r.Attributes().PutStr("service.namespace", "default")
		r.SetFlags(plog.DefaultLogRecordFlags)
		r.SetTraceID([16]byte{1, 2, 3, byte(i)})
		r.SetSpanID([8]byte{1, 2, 3, byte(i)})
	}
	return logs
}
