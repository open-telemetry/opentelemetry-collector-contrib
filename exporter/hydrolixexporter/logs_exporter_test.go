// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestLogsExporter_ConvertLogs(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()

	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	logRecord.SetSeverityText("INFO")
	logRecord.Body().SetStr("Test log message")
	logRecord.Attributes().PutStr("key1", "value1")
	logRecord.Attributes().PutStr("http.response.status_code", "200")
	logRecord.Attributes().PutStr("http.route", "/api/test")
	logRecord.Attributes().PutStr("http.request.method", "GET")

	// Set trace context
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	logRecord.SetTraceID(traceID)
	logRecord.SetSpanID(spanID)

	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl.Scope().SetName("test-scope")
	sl.Scope().SetVersion("1.0.0")

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8080",
			Timeout:  30 * time.Second,
		},
		HDXTable:     "test_table",
		HDXTransform: "test_transform",
		HDXUsername:  "user",
		HDXPassword:  "pass",
	}

	exporter := newLogsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	result := exporter.convertToHydrolixLogs(logs)

	require.Len(t, result, 1)
	assert.Equal(t, "Test log message", result[0].Body)
	assert.Equal(t, "INFO", result[0].SeverityText)
	assert.Equal(t, int32(plog.SeverityNumberInfo), result[0].SeverityNumber)
	assert.Equal(t, "test-service", result[0].ServiceName)
	assert.Equal(t, "test-scope", result[0].ScopeName)
	assert.Equal(t, "1.0.0", result[0].ScopeVersion)
	assert.Equal(t, "200", result[0].HTTPStatusCode)
	assert.Equal(t, "/api/test", result[0].HTTPRoute)
	assert.Equal(t, "GET", result[0].HTTPMethod)
	assert.NotEmpty(t, result[0].TraceID)
	assert.NotEmpty(t, result[0].SpanID)
	assert.Greater(t, result[0].Timestamp, uint64(0))
	assert.Greater(t, result[0].ObservedTimestamp, uint64(0))
	assert.Greater(t, len(result[0].LogAttributes), 0)
}

func TestLogsExporter_ConvertMultipleLogs(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")

	sl := rl.ScopeLogs().AppendEmpty()

	// Add multiple log records
	for i := 0; i < 3; i++ {
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		logRecord.Body().SetStr("Test log message")
		logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	}

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8080",
		},
	}

	exporter := newLogsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	result := exporter.convertToHydrolixLogs(logs)

	require.Len(t, result, 3)
	for i := 0; i < 3; i++ {
		assert.Equal(t, "Test log message", result[i].Body)
		assert.Equal(t, "test-service", result[i].ServiceName)
	}
}

func TestLogsExporter_PushLogs(t *testing.T) {
	// Create a test server
	var receivedData []HydrolixLog
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		err = json.Unmarshal(body, &receivedData)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test logs
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()

	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	logRecord.SetSeverityText("INFO")
	logRecord.Body().SetStr("Test log message")

	// Create exporter
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		HDXTable:     "test_table",
		HDXTransform: "test_transform",
		HDXUsername:  "testuser",
		HDXPassword:  "testpass",
	}

	exporter := newLogsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))

	// Push logs
	err := exporter.pushLogs(context.Background(), logs)
	require.NoError(t, err)

	// Verify received data
	require.Len(t, receivedData, 1)
	assert.Equal(t, "Test log message", receivedData[0].Body)
	assert.Equal(t, "INFO", receivedData[0].SeverityText)

	// Verify headers
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.Equal(t, "test_table", receivedHeaders.Get("x-hdx-table"))
	assert.Equal(t, "test_transform", receivedHeaders.Get("x-hdx-transform"))
	assert.NotEmpty(t, receivedHeaders.Get("Authorization"))
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Basic")
}

func TestLogsExporter_PushLogsError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("Test log message")

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		HDXTable:     "test_table",
		HDXTransform: "test_transform",
		HDXUsername:  "user",
		HDXPassword:  "pass",
	}

	exporter := newLogsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	err := exporter.pushLogs(context.Background(), logs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code")
}

func TestLogsExporter_ConvertLogsWithDifferentSeverities(t *testing.T) {
	severities := []struct {
		number plog.SeverityNumber
		text   string
	}{
		{plog.SeverityNumberTrace, "TRACE"},
		{plog.SeverityNumberDebug, "DEBUG"},
		{plog.SeverityNumberInfo, "INFO"},
		{plog.SeverityNumberWarn, "WARN"},
		{plog.SeverityNumberError, "ERROR"},
		{plog.SeverityNumberFatal, "FATAL"},
	}

	for _, severity := range severities {
		t.Run(severity.text, func(t *testing.T) {
			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			sl := rl.ScopeLogs().AppendEmpty()
			logRecord := sl.LogRecords().AppendEmpty()

			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			logRecord.SetSeverityNumber(severity.number)
			logRecord.SetSeverityText(severity.text)
			logRecord.Body().SetStr("Test log message")

			cfg := &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "http://localhost:8080",
				},
			}

			exporter := newLogsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
			result := exporter.convertToHydrolixLogs(logs)

			require.Len(t, result, 1)
			assert.Equal(t, severity.text, result[0].SeverityText)
			assert.Equal(t, int32(severity.number), result[0].SeverityNumber)
		})
	}
}
