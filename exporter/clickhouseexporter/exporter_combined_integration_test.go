// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package clickhouseexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testLogsCombinedExporter verifies that Map-based and JSON-based logs exporters
// can operate side-by-side against the same ClickHouse instance.
func testLogsCombinedExporter(t *testing.T, endpoint string, mapBody bool) {
	overrideJSONStringSetting := func(config *Config) {
		config.ConnectionParams["output_format_native_write_json_as_string"] = "1"
	}

	// Create Map-based exporter writing to a dedicated table
	mapExporter := newTestLogsExporter(t, endpoint, false, func(config *Config) {
		config.LogsTableName = "otel_logs_combined_map"
	})

	// Create JSON-based exporter writing to a different table
	jsonExporter := newTestLogsJSONExporter(t, endpoint, false, overrideJSONStringSetting, func(config *Config) {
		config.LogsTableName = "otel_logs_combined_json"
	})

	logs := simpleLogs(100, mapBody)

	// Push the same data through both exporters
	require.NoError(t, mapExporter.pushLogsData(t.Context(), logs))
	require.NoError(t, jsonExporter.pushLogsData(t.Context(), logs))

	// Verify Map exporter wrote map[string]string attributes
	{
		type mapLog struct {
			Timestamp          time.Time         `ch:"Timestamp"`
			ServiceName        string            `ch:"ServiceName"`
			Body               string            `ch:"Body"`
			ResourceAttributes map[string]string `ch:"ResourceAttributes"`
			LogAttributes      map[string]string `ch:"LogAttributes"`
		}

		tableName := fmt.Sprintf("%q.%q", mapExporter.cfg.database(), mapExporter.cfg.LogsTableName)
		row := mapExporter.db.QueryRow(t.Context(), fmt.Sprintf("SELECT Timestamp, ServiceName, Body, ResourceAttributes, LogAttributes FROM %s LIMIT 1", tableName))
		require.NoError(t, row.Err())

		var actual mapLog
		require.NoError(t, row.ScanStruct(&actual))
		require.Equal(t, "test-service", actual.ServiceName)
		require.Equal(t, map[string]string{"service.name": "test-service"}, actual.ResourceAttributes)
		require.Equal(t, map[string]string{"service.namespace": "default"}, actual.LogAttributes)
	}

	// Verify JSON exporter wrote JSON string attributes
	{
		type jsonLog struct {
			Timestamp          time.Time `ch:"Timestamp"`
			ServiceName        string    `ch:"ServiceName"`
			Body               string    `ch:"Body"`
			ResourceAttributes string    `ch:"ResourceAttributes"`
			LogAttributes      string    `ch:"LogAttributes"`
		}

		tableName := fmt.Sprintf("%q.%q", jsonExporter.cfg.database(), jsonExporter.cfg.LogsTableName)
		row := jsonExporter.db.QueryRow(t.Context(), fmt.Sprintf("SELECT Timestamp, ServiceName, Body, ResourceAttributes, LogAttributes FROM %s LIMIT 1", tableName))
		require.NoError(t, row.Err())

		var actual jsonLog
		require.NoError(t, row.ScanStruct(&actual))
		require.Equal(t, "test-service", actual.ServiceName)
		require.Equal(t, `{"service":{"name":"test-service"}}`, actual.ResourceAttributes)
		require.Equal(t, `{"service":{"namespace":"default"}}`, actual.LogAttributes)
	}
}

// testTracesCombinedExporter verifies that Map-based and JSON-based traces exporters
// can operate side-by-side against the same ClickHouse instance.
func testTracesCombinedExporter(t *testing.T, endpoint string) {
	overrideJSONStringSetting := func(config *Config) {
		config.ConnectionParams["output_format_native_write_json_as_string"] = "1"
	}

	// Create Map-based exporter writing to a dedicated table
	mapExporter := newTestTracesExporter(t, endpoint, func(config *Config) {
		config.TracesTableName = "otel_traces_combined_map"
	})

	// Create JSON-based exporter writing to a different table
	jsonExporter := newTestTracesJSONExporter(t, endpoint, false, overrideJSONStringSetting, func(config *Config) {
		config.TracesTableName = "otel_traces_combined_json"
	})

	traces := simpleTraces(100)

	// Push the same data through both exporters
	require.NoError(t, mapExporter.pushTraceData(t.Context(), traces))
	require.NoError(t, jsonExporter.pushTraceData(t.Context(), traces))

	// Verify Map exporter wrote map[string]string attributes
	{
		type mapTrace struct {
			Timestamp          time.Time         `ch:"Timestamp"`
			ServiceName        string            `ch:"ServiceName"`
			SpanName           string            `ch:"SpanName"`
			ResourceAttributes map[string]string `ch:"ResourceAttributes"`
			SpanAttributes     map[string]string `ch:"SpanAttributes"`
		}

		row := mapExporter.db.QueryRow(t.Context(), fmt.Sprintf("SELECT Timestamp, ServiceName, SpanName, ResourceAttributes, SpanAttributes FROM %q.%q LIMIT 1",
			mapExporter.cfg.database(), mapExporter.cfg.TracesTableName))
		require.NoError(t, row.Err())

		var actual mapTrace
		require.NoError(t, row.ScanStruct(&actual))
		require.Equal(t, "test-service", actual.ServiceName)
		require.Equal(t, map[string]string{"service.name": "test-service"}, actual.ResourceAttributes)
		require.Equal(t, map[string]string{"service.name": "v"}, actual.SpanAttributes)
	}

	// Verify JSON exporter wrote JSON string attributes
	{
		type jsonTrace struct {
			Timestamp          time.Time `ch:"Timestamp"`
			ServiceName        string    `ch:"ServiceName"`
			SpanName           string    `ch:"SpanName"`
			ResourceAttributes string    `ch:"ResourceAttributes"`
			SpanAttributes     string    `ch:"SpanAttributes"`
		}

		row := jsonExporter.db.QueryRow(t.Context(), fmt.Sprintf("SELECT Timestamp, ServiceName, SpanName, ResourceAttributes, SpanAttributes FROM %q.%q LIMIT 1",
			jsonExporter.cfg.database(), jsonExporter.cfg.TracesTableName))
		require.NoError(t, row.Err())

		var actual jsonTrace
		require.NoError(t, row.ScanStruct(&actual))
		require.Equal(t, "test-service", actual.ServiceName)
		require.Equal(t, `{"service":{"name":"test-service"}}`, actual.ResourceAttributes)
		require.Equal(t, `{"service":{"name":"v"}}`, actual.SpanAttributes)
	}
}
