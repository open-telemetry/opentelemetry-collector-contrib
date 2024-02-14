// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mezmoexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var buildInfo = component.BuildInfo{
	Version: "1.0",
}

func createSimpleLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	sl := rl.ScopeLogs().AppendEmpty()

	for i := 0; i < numberOfLogs; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStr("10byteslog")
		logRecord.Attributes().PutStr(conventions.AttributeServiceName, "myapp")
		logRecord.Attributes().PutStr("my-label", "myapp-type")
		logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
		logRecord.Attributes().PutStr("custom", "custom")
		logRecord.SetTimestamp(ts)
	}

	return logs
}

func createMinimalAttributesLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()

	for i := 0; i < numberOfLogs; i++ {
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStr("minimal attribute log")
	}

	return logs
}

// Creates a logs set that exceeds the maximum message side we can send in one HTTP POST
func createMaxLogData() plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	sl := rl.ScopeLogs().AppendEmpty()

	var lineLen = maxMessageSize
	var lineCnt = (maxBodySize / lineLen) * 2

	for i := 0; i < lineCnt; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStr(randString(maxMessageSize))
		logRecord.SetTimestamp(ts)
	}

	return logs
}

func createSizedPayloadLogData(payloadSize int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	sl := rl.ScopeLogs().AppendEmpty()

	maxMsg := randString(payloadSize)

	ts := pcommon.Timestamp(0)
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr(maxMsg)
	logRecord.SetTimestamp(ts)

	return logs
}

type testServer struct {
	instance *httptest.Server
	url      string
}

type httpAssertionCallback func(req *http.Request, body mezmoLogBody) (int, string)
type testServerParams struct {
	t                  *testing.T
	assertionsCallback httpAssertionCallback
}

// Creates an HTTP server to test log delivery payloads by applying a set of
// assertions through the assertCB function.
func createHTTPServer(params *testServerParams) testServer {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			params.t.Fatal(err)
		}

		var logBody mezmoLogBody
		if err = json.Unmarshal(body, &logBody); err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
		}

		statusCode, responseBody := params.assertionsCallback(r, logBody)

		w.WriteHeader(statusCode)
		if len(responseBody) > 0 {
			_, err = w.Write([]byte(responseBody))
			assert.NoError(params.t, err)
		}
	}))

	serverURL, err := url.Parse(httpServer.URL)
	assert.NoError(params.t, err)

	server := testServer{
		instance: httpServer,
		url:      serverURL.String(),
	}

	return server
}

func createExporter(t *testing.T, config *Config, logger *zap.Logger) *mezmoExporter {
	exporter := newLogsExporter(config, componenttest.NewNopTelemetrySettings(), buildInfo, logger)
	require.NotNil(t, exporter)

	err := exporter.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	return exporter
}

func createLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	return logger, logObserver
}

func TestLogsExporter(t *testing.T) {
	httpServerParams := testServerParams{
		t: t,
		assertionsCallback: func(req *http.Request, body mezmoLogBody) (int, string) {
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			assert.Equal(t, "mezmo-otel-exporter/"+buildInfo.Version, req.Header.Get("User-Agent"))
			return http.StatusOK, ""
		},
	}
	server := createHTTPServer(&httpServerParams)
	defer server.instance.Close()

	log, _ := createLogger()
	config := &Config{
		IngestURL: server.url,
	}
	exporter := createExporter(t, config, log)

	t.Run("Test simple log data", func(t *testing.T) {
		var logs = createSimpleLogData(3)
		err := exporter.pushLogData(context.Background(), logs)
		require.NoError(t, err)
	})

	t.Run("Test max message size", func(t *testing.T) {
		var logs = createSizedPayloadLogData(maxMessageSize)
		err := exporter.pushLogData(context.Background(), logs)
		require.NoError(t, err)
	})

	t.Run("Test max body size", func(t *testing.T) {
		var logs = createMaxLogData()
		err := exporter.pushLogData(context.Background(), logs)
		require.NoError(t, err)
	})
}

func TestAddsRequiredAttributes(t *testing.T) {
	httpServerParams := testServerParams{
		t: t,
		assertionsCallback: func(req *http.Request, body mezmoLogBody) (int, string) {
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			assert.Equal(t, "mezmo-otel-exporter/"+buildInfo.Version, req.Header.Get("User-Agent"))

			lines := body.Lines
			for _, line := range lines {
				assert.True(t, line.Timestamp > 0)
				assert.Equal(t, line.Level, "info")
				assert.Equal(t, line.App, "")
				assert.Equal(t, line.Line, "minimal attribute log")
			}

			return http.StatusOK, ""
		},
	}
	server := createHTTPServer(&httpServerParams)
	defer server.instance.Close()

	log, _ := createLogger()
	config := &Config{
		IngestURL: server.url,
	}
	exporter := createExporter(t, config, log)

	logs := createMinimalAttributesLogData(4)
	err := exporter.pushLogData(context.Background(), logs)
	require.NoError(t, err)
}

func Test404IngestError(t *testing.T) {
	log, logObserver := createLogger()

	httpServerParams := testServerParams{
		t: t,
		assertionsCallback: func(req *http.Request, body mezmoLogBody) (int, string) {
			return http.StatusNotFound, `{"foo":"bar"}`
		},
	}
	server := createHTTPServer(&httpServerParams)
	defer server.instance.Close()

	config := &Config{
		IngestURL: fmt.Sprintf("%s/foobar", server.url),
	}
	exporter := createExporter(t, config, log)

	logs := createSizedPayloadLogData(1)
	err := exporter.pushLogData(context.Background(), logs)
	require.NoError(t, err)

	assert.Equal(t, logObserver.Len(), 2)

	logLine := logObserver.All()[0]
	assert.Equal(t, logLine.Message, "got http status (/foobar): 404 Not Found")
	assert.Equal(t, logLine.Level, zapcore.ErrorLevel)

	logLine = logObserver.All()[1]
	assert.Equal(t, logLine.Message, "http response")
	assert.Equal(t, logLine.Level, zapcore.DebugLevel)

	responseField := logLine.Context[0]
	assert.Equal(t, responseField.Key, "response")
	assert.Equal(t, responseField.String, `{"foo":"bar"}`)
}
