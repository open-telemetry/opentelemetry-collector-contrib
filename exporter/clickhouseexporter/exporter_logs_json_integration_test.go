// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zaptest"
)

func testLogsJSONExporter(t *testing.T) {
	overrideJSONStringSetting := func(config *Config) {
		config.ConnectionParams["output_format_native_write_json_as_string"] = "1"
	}
	overrideLogsTableName := func(config *Config) {
		config.LogsTableName = "otel_logs_json"
	}
	exporter := newTestLogsJSONExporter(t, integrationTestEndpoint, overrideJSONStringSetting, overrideLogsTableName)
	verifyExportLogsJSON(t, exporter)
}

func newTestLogsJSONExporter(t *testing.T, dsn string, fns ...func(*Config)) *logsJSONExporter {
	exporter := newLogsJSONExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))

	require.NoError(t, exporter.start(context.Background(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.Background()) })
	return exporter
}

func verifyExportLogsJSON(t *testing.T, exporter *logsJSONExporter) {
	mustPushLogsJSONData(t, exporter, simpleLogs(100))

	type log struct {
		Timestamp          time.Time `ch:"Timestamp"`
		TraceID            string    `ch:"TraceId"`
		SpanID             string    `ch:"SpanId"`
		TraceFlags         uint8     `ch:"TraceFlags"`
		SeverityText       string    `ch:"SeverityText"`
		SeverityNumber     uint8     `ch:"SeverityNumber"`
		ServiceName        string    `ch:"ServiceName"`
		Body               string    `ch:"Body"`
		ResourceSchemaURL  string    `ch:"ResourceSchemaUrl"`
		ResourceAttributes string    `ch:"ResourceAttributes"`
		ScopeSchemaURL     string    `ch:"ScopeSchemaUrl"`
		ScopeName          string    `ch:"ScopeName"`
		ScopeVersion       string    `ch:"ScopeVersion"`
		ScopeAttributes    string    `ch:"ScopeAttributes"`
		LogAttributes      string    `ch:"LogAttributes"`
	}

	expectedLog := log{
		Timestamp:          telemetryTimestamp,
		TraceID:            "01020300000000000000000000000000",
		SpanID:             "0102030000000000",
		SeverityText:       "error",
		SeverityNumber:     18,
		ServiceName:        "test-service",
		Body:               "error message",
		ResourceSchemaURL:  "https://opentelemetry.io/schemas/1.4.0",
		ResourceAttributes: `{"service":{"name":"test-service"}}`,
		ScopeName:          "io.opentelemetry.contrib.clickhouse",
		ScopeVersion:       "1.0.0",
		ScopeSchemaURL:     "https://opentelemetry.io/schemas/1.7.0",
		ScopeAttributes:    `{"lib":"clickhouse"}`,
		LogAttributes:      `{"service":{"namespace":"default"}}`,
	}

	row := exporter.db.QueryRow(context.Background(), "SELECT * FROM otel_int_test.otel_logs_json")
	require.NoError(t, row.Err())

	var actualLog log
	err := row.ScanStruct(&actualLog)
	require.NoError(t, err)

	require.Equal(t, expectedLog, actualLog)
}

func mustPushLogsJSONData(t *testing.T, exporter *logsJSONExporter, ld plog.Logs) {
	err := exporter.pushLogsData(context.Background(), ld)
	require.NoError(t, err)
}
