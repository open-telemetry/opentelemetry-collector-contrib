// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
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
		require.NoError(t, exporter.renderInsertLogsSQL())
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
		ObservedTimestamp  time.Time         `ch:"ObservedTimestamp"`
		Timestamp          time.Time         `ch:"Timestamp"`
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
		ObservedTimestamp: telemetryTimestamp.Add(123456789 * time.Nanosecond),
		Timestamp:         telemetryTimestamp,
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
		r.SetObservedTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(123456789 * time.Nanosecond)))
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

func testLogsObservedTimestamp(t *testing.T, endpoint string) {
	verify := func(t *testing.T, db driver.Conn, table string, push func(plog.Logs) error) {
		require.NoError(t, db.Exec(t.Context(), fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS ObservedTimestamp DateTime64(9)", table)))
		for _, fixture := range []struct {
			name      string
			timestamp int64
			observed  int64
		}{
			{name: "distinct_nanoseconds", timestamp: 1750000000123456789, observed: 1750000000987654321},
			{name: "missing_event_time", observed: 1750000000987654321},
			{name: "missing_observed_time", timestamp: 1750000000123456789},
		} {
			t.Run(fixture.name, func(t *testing.T) {
				require.NoError(t, db.Exec(t.Context(), "TRUNCATE TABLE "+table))
				wire := fmt.Appendf(nil, `{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"timeUnixNano":"%d","observedTimeUnixNano":"%d","body":{"stringValue":"synthetic-observed-timestamp"}}]}]}]}`, fixture.timestamp, fixture.observed)
				logs, err := (&plog.JSONUnmarshaler{}).UnmarshalLogs(wire)
				require.NoError(t, err)
				require.NoError(t, push(logs))
				var timestamp, observed int64
				require.NoError(t, db.QueryRow(t.Context(), "SELECT toUnixTimestamp64Nano(Timestamp), toUnixTimestamp64Nano(ObservedTimestamp) FROM "+table).Scan(&timestamp, &observed))
				expectedTimestamp := fixture.timestamp
				if expectedTimestamp == 0 {
					expectedTimestamp = fixture.observed
				}
				require.Equal(t, expectedTimestamp, timestamp)
				require.Equal(t, fixture.observed, observed)
			})
		}
	}
	t.Run("map", func(t *testing.T) {
		exporter := newTestLogsExporter(t, endpoint, false, func(cfg *Config) { cfg.LogsTableName = "otel_logs_observed" })
		verify(t, exporter.db, fmt.Sprintf("%q.%q", exporter.cfg.database(), exporter.cfg.LogsTableName), func(logs plog.Logs) error { return exporter.pushLogsData(t.Context(), logs) })
	})
	t.Run("json", func(t *testing.T) {
		exporter := newTestLogsJSONExporter(t, endpoint, false, func(cfg *Config) { cfg.LogsTableName = "otel_logs_json_observed" })
		verify(t, exporter.db, fmt.Sprintf("%q.%q", exporter.cfg.database(), exporter.cfg.LogsTableName), func(logs plog.Logs) error { return exporter.pushLogsData(t.Context(), logs) })
	})
}
