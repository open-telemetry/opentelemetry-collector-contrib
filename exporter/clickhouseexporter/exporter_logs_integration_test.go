// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap/zaptest"
)

func testLogsExporter(t *testing.T, endpoint string) {
	exporter := newTestLogsExporter(t, endpoint)
	verifyExportLogs(t, exporter)
}

func newTestLogsExporter(t *testing.T, dsn string, fns ...func(*Config)) *logsExporter {
	exporter := newLogsExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))

	require.NoError(t, exporter.start(context.Background(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.Background()) })
	return exporter
}

func verifyExportLogs(t *testing.T, exporter *logsExporter) {
	// 3 pushes
	mustPushLogsData(t, exporter, simpleLogs(5000))
	mustPushLogsData(t, exporter, simpleLogs(5000))
	mustPushLogsData(t, exporter, simpleLogs(5000))

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
	}

	row := exporter.db.QueryRow(context.Background(), "SELECT * FROM otel_int_test.otel_logs")
	require.NoError(t, row.Err())

	var actualLog log
	err := row.ScanStruct(&actualLog)
	require.NoError(t, err)

	require.Equal(t, expectedLog, actualLog)
}

func mustPushLogsData(t *testing.T, exporter *logsExporter, ld plog.Logs) {
	err := exporter.pushLogsData(context.Background(), ld)
	require.NoError(t, err)
}

func simpleLogs(count int) plog.Logs {
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
	for i := 0; i < count; i++ {
		r := sl.LogRecords().AppendEmpty()
		r.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetObservedTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetSeverityNumber(plog.SeverityNumberError2)
		r.SetSeverityText("error")
		r.Body().SetStr("error message")
		r.Attributes().PutStr(string(conventions.ServiceNamespaceKey), "default")
		r.SetFlags(plog.DefaultLogRecordFlags)
		r.SetTraceID([16]byte{1, 2, 3, byte(i)})
		r.SetSpanID([8]byte{1, 2, 3, byte(i)})
	}
	return logs
}
