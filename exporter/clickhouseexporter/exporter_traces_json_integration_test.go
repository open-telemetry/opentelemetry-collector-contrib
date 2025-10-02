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

func testTracesJSONExporter(t *testing.T, endpoint string) {
	overrideJSONStringSetting := func(config *Config) {
		config.ConnectionParams["output_format_native_write_json_as_string"] = "1"
	}
	overrideTracesTableName := func(config *Config) {
		config.TracesTableName = "otel_traces_json"
	}
	exporter := newTestTracesJSONExporter(t, endpoint, false, overrideJSONStringSetting, overrideTracesTableName)
	verifyExportTracesJSON(t, exporter, false)
}

func testTracesJSONExporterSchemaFeatures(t *testing.T, endpoint string) {
	overrideJSONStringSetting := func(config *Config) {
		config.ConnectionParams["output_format_native_write_json_as_string"] = "1"
	}
	overrideTracesTableName := func(config *Config) {
		config.TracesTableName = "otel_traces_json_schema_features"
	}
	exporter := newTestTracesJSONExporter(t, endpoint, true, overrideJSONStringSetting, overrideTracesTableName)
	verifyExportTracesJSON(t, exporter, true)
}

func newTestTracesJSONExporter(t *testing.T, dsn string, testSchemaFeatures bool, fns ...func(*Config)) *tracesJSONExporter {
	exporter := newTracesJSONExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))

	require.NoError(t, exporter.start(t.Context(), nil))

	// Tests the schema feature flags by disabling newer columns. The insert logic should adapt.
	if testSchemaFeatures {
		exporter.schemaFeatures.AttributeKeys = false
		exporter.renderInsertTracesJSONSQL()
	}

	t.Cleanup(func() { _ = exporter.shutdown(t.Context()) })
	return exporter
}

func verifyExportTracesJSON(t *testing.T, exporter *tracesJSONExporter, testSchemaFeatures bool) {
	pushConcurrentlyNoError(t, func() error {
		return exporter.pushTraceData(t.Context(), simpleTraces(5000))
	})

	type trace struct {
		Timestamp              time.Time   `ch:"Timestamp"`
		TraceID                string      `ch:"TraceId"`
		SpanID                 string      `ch:"SpanId"`
		ParentSpanID           string      `ch:"ParentSpanId"`
		TraceState             string      `ch:"TraceState"`
		SpanName               string      `ch:"SpanName"`
		SpanKind               string      `ch:"SpanKind"`
		ServiceName            string      `ch:"ServiceName"`
		ResourceAttributes     string      `ch:"ResourceAttributes"`
		ScopeName              string      `ch:"ScopeName"`
		ScopeVersion           string      `ch:"ScopeVersion"`
		SpanAttributes         string      `ch:"SpanAttributes"`
		Duration               uint64      `ch:"Duration"`
		StatusCode             string      `ch:"StatusCode"`
		StatusMessage          string      `ch:"StatusMessage"`
		EventsTimestamp        []time.Time `ch:"Events.Timestamp"`
		EventsName             []string    `ch:"Events.Name"`
		EventsAttributes       []string    `ch:"Events.Attributes"`
		LinksTraceID           []string    `ch:"Links.TraceId"`
		LinksSpanID            []string    `ch:"Links.SpanId"`
		LinksTraceState        []string    `ch:"Links.TraceState"`
		LinksAttributes        []string    `ch:"Links.Attributes"`
		ResourceAttributesKeys []string    `ch:"ResourceAttributesKeys"`
		SpanAttributesKeys     []string    `ch:"SpanAttributesKeys"`
	}

	expectedTrace := trace{
		Timestamp:          telemetryTimestamp,
		TraceID:            "01020300000000000000000000000000",
		SpanID:             "0102030000000000",
		ParentSpanID:       "0102040000000000",
		TraceState:         "trace state",
		SpanName:           "call db",
		SpanKind:           "Internal",
		ServiceName:        "test-service",
		ResourceAttributes: `{"service":{"name":"test-service"}}`,
		ScopeName:          "io.opentelemetry.contrib.clickhouse",
		ScopeVersion:       "1.0.0",
		SpanAttributes:     `{"service":{"name":"v"}}`,
		Duration:           60000000000,
		StatusCode:         "Error",
		StatusMessage:      "error",
		EventsTimestamp: []time.Time{
			telemetryTimestamp,
		},
		EventsName: []string{"event1"},
		EventsAttributes: []string{
			`{"level":"info"}`,
		},
		LinksTraceID: []string{
			"01020500000000000000000000000000",
		},
		LinksSpanID: []string{
			"0102050000000000",
		},
		LinksTraceState: []string{
			"error",
		},
		LinksAttributes: []string{
			`{"k":"v"}`,
		},
		ResourceAttributesKeys: []string{"service.name"},
		SpanAttributesKeys:     []string{"service.name"},
	}

	if testSchemaFeatures {
		expectedTrace.ResourceAttributesKeys = []string{}
		expectedTrace.SpanAttributesKeys = []string{}
	}

	row := exporter.db.QueryRow(t.Context(), fmt.Sprintf("SELECT * FROM %q.%q", exporter.cfg.database(), exporter.cfg.TracesTableName))
	require.NoError(t, row.Err())

	var actualTrace trace
	err := row.ScanStruct(&actualTrace)
	require.NoError(t, err)

	require.Equal(t, expectedTrace, actualTrace)
}
