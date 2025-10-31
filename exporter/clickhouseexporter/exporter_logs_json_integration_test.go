// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func testLogsJSONExporter(t *testing.T, endpoint string, mapBody bool) {
	overrideJSONStringSetting := func(config *Config) {
		config.ConnectionParams["output_format_native_write_json_as_string"] = "1"
	}
	overrideLogsTableName := func(config *Config) {
		config.LogsTableName = "otel_logs_json"
	}
	exporter := newTestLogsJSONExporter(t, endpoint, false, overrideJSONStringSetting, overrideLogsTableName)
	verifyExportLogsJSON(t, exporter, mapBody, false)
}

func testLogsJSONExporterSchemaFeatures(t *testing.T, endpoint string, mapBody bool) {
	overrideJSONStringSetting := func(config *Config) {
		config.ConnectionParams["output_format_native_write_json_as_string"] = "1"
	}
	overrideLogsTableName := func(config *Config) {
		config.LogsTableName = "otel_logs_json_schema_features"
	}
	exporter := newTestLogsJSONExporter(t, endpoint, true, overrideJSONStringSetting, overrideLogsTableName)
	verifyExportLogsJSON(t, exporter, mapBody, true)
}

func newTestLogsJSONExporter(t *testing.T, dsn string, testSchemaFeatures bool, fns ...func(*Config)) *logsJSONExporter {
	exporter := newLogsJSONExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))

	require.NoError(t, exporter.start(t.Context(), nil))

	// Tests the schema feature flags by disabling newer columns. The insert logic should adapt.
	if testSchemaFeatures {
		exporter.schemaFeatures.AttributeKeys = false
		exporter.schemaFeatures.EventName = false
		exporter.renderInsertLogsJSONSQL()
	}

	t.Cleanup(func() { _ = exporter.shutdown(t.Context()) })
	return exporter
}

func verifyExportLogsJSON(t *testing.T, exporter *logsJSONExporter, mapBody, testSchemaFeatures bool) {
	tableName := fmt.Sprintf("%q.%q", exporter.cfg.database(), exporter.cfg.LogsTableName)

	// Clear table for when mapBody test runs under same table name
	err := exporter.db.Exec(t.Context(), fmt.Sprintf("TRUNCATE %s", tableName))
	require.NoError(t, err)

	pushConcurrentlyNoError(t, func() error {
		return exporter.pushLogsData(t.Context(), simpleLogs(5000, mapBody))
	})

	type log struct {
		Timestamp              time.Time `ch:"Timestamp"`
		TraceID                string    `ch:"TraceId"`
		SpanID                 string    `ch:"SpanId"`
		TraceFlags             uint8     `ch:"TraceFlags"`
		SeverityText           string    `ch:"SeverityText"`
		SeverityNumber         uint8     `ch:"SeverityNumber"`
		ServiceName            string    `ch:"ServiceName"`
		Body                   string    `ch:"Body"`
		ResourceSchemaURL      string    `ch:"ResourceSchemaUrl"`
		ResourceAttributes     string    `ch:"ResourceAttributes"`
		ScopeSchemaURL         string    `ch:"ScopeSchemaUrl"`
		ScopeName              string    `ch:"ScopeName"`
		ScopeVersion           string    `ch:"ScopeVersion"`
		ScopeAttributes        string    `ch:"ScopeAttributes"`
		LogAttributes          string    `ch:"LogAttributes"`
		ResourceAttributesKeys []string  `ch:"ResourceAttributesKeys"`
		ScopeAttributesKeys    []string  `ch:"ScopeAttributesKeys"`
		LogAttributesKeys      []string  `ch:"LogAttributesKeys"`
		EventName              string    `ch:"EventName"`
	}

	expectedLog := log{
		Timestamp:              telemetryTimestamp,
		TraceID:                "01020300000000000000000000000000",
		SpanID:                 "0102030000000000",
		SeverityText:           "error",
		SeverityNumber:         18,
		ServiceName:            "test-service",
		Body:                   "error message",
		ResourceSchemaURL:      "https://opentelemetry.io/schemas/1.4.0",
		ResourceAttributes:     `{"service":{"name":"test-service"}}`,
		ScopeName:              "io.opentelemetry.contrib.clickhouse",
		ScopeVersion:           "1.0.0",
		ScopeSchemaURL:         "https://opentelemetry.io/schemas/1.7.0",
		ScopeAttributes:        `{"lib":"clickhouse"}`,
		LogAttributes:          `{"service":{"namespace":"default"}}`,
		ResourceAttributesKeys: []string{"service.name"},
		ScopeAttributesKeys:    []string{"lib"},
		LogAttributesKeys:      []string{"service.namespace"},
		EventName:              "event",
	}

	if mapBody {
		expectedLog.Body = `{"error":"message"}`
	}

	if testSchemaFeatures {
		expectedLog.ResourceAttributesKeys = []string{}
		expectedLog.ScopeAttributesKeys = []string{}
		expectedLog.LogAttributesKeys = []string{}
		expectedLog.EventName = ""
	}

	row := exporter.db.QueryRow(t.Context(), fmt.Sprintf("SELECT * FROM %s", tableName))
	require.NoError(t, row.Err())

	var actualLog log
	err = row.ScanStruct(&actualLog)
	require.NoError(t, err)

	require.Equal(t, expectedLog, actualLog)
}
