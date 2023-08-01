// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

const (
	testService   = "testService"
	testHost      = "testHost"
	testOperation = "testOperation"
)

var (
	TestLogTime          = time.Now()
	TestLogTimeUnixMilli = TestLogTime.UnixMilli()
	TestLogTimestamp     = pcommon.NewTimestampFromTime(TestLogTime)
)

// Logs

func fillLogOne(log plog.LogRecord) {
	log.SetTimestamp(TestLogTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetSeverityNumber(plog.SeverityNumberInfo)
	log.SetSeverityText("Info")
	log.SetSpanID([8]byte{0x01, 0x02, 0x04, 0x08})
	log.SetTraceID([16]byte{0x08, 0x04, 0x02, 0x01})

	attrs := log.Attributes()
	attrs.PutStr("app", "server")
	attrs.PutDouble("instance_num", 1)

	// nested body map
	attMap := log.Body().SetEmptyMap()
	attMap.PutDouble("23", 45)
	attMap.PutStr("foo", "bar")
	attMap.PutStr("message", "hello there")
	attNestedMap := attMap.PutEmptyMap("nested")
	attNestedMap.PutStr("string", "v1")
	attNestedMap.PutDouble("number", 499)
}

func fillLogTwo(log plog.LogRecord) {
	log.SetTimestamp(TestLogTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetSeverityNumber(plog.SeverityNumberInfo)
	log.SetSeverityText("Info")

	attrs := log.Attributes()
	attrs.PutStr("customer", "acme")
	attrs.PutDouble("number", 64)
	attrs.PutBool("bool", true)
	attrs.PutStr("env", "dev")
	log.Body().SetStr("something happened")
}
func fillLogNoTimestamp(log plog.LogRecord) {
	log.SetDroppedAttributesCount(1)
	log.SetSeverityNumber(plog.SeverityNumberInfo)
	log.SetSeverityText("Info")

	attrs := log.Attributes()
	attrs.PutStr("customer", "acme")
	attrs.PutDouble("number", 64)
	attrs.PutBool("bool", true)
	attrs.PutStr("env", "dev")
	log.Body().SetStr("something happened")
}

func generateLogsOneEmptyTimestamp() plog.Logs {
	ld := testdata.GenerateLogsOneEmptyLogRecord()
	logs := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	fillLogOne(logs.At(0))
	fillLogNoTimestamp(logs.AppendEmpty())
	return ld
}

func testLogsExporter(ld plog.Logs, t *testing.T, cfg *Config) error {
	var err error
	params := exportertest.NewNopCreateSettings()
	exporter, err := createLogsExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}
	err = exporter.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		return err
	}
	require.NoError(t, err)
	ctx := context.Background()
	err = exporter.ConsumeLogs(ctx, ld)
	if err != nil {
		return err
	}
	require.NoError(t, err)
	err = exporter.Shutdown(ctx)
	require.NoError(t, err)
	return nil
}

// Traces
func newTestTracesWithAttributes() ptrace.Traces {
	td := ptrace.NewTraces()
	for i := 0; i < 10; i++ {
		s := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		s.SetName(fmt.Sprintf("%s-%d", testOperation, i))
		s.SetTraceID(pcommon.TraceID([16]byte{byte(i), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}))
		s.SetSpanID(pcommon.SpanID([8]byte{byte(i), 0, 0, 0, 0, 0, 0, 2}))
		for j := 0; j < 5; j++ {
			s.Attributes().PutStr(fmt.Sprintf("k%d", j), fmt.Sprintf("v%d", j))
		}
		s.SetKind(ptrace.SpanKindServer)
	}
	return td
}

func newTestTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	s := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	s.SetName(testOperation)
	s.SetTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	s.SetSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 2})
	s.SetKind(ptrace.SpanKindServer)
	return td
}

func testTracesExporter(td ptrace.Traces, t *testing.T, cfg *Config) error {
	params := exportertest.NewNopCreateSettings()
	exporter, err := createTracesExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}
	err = exporter.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		return err
	}
	require.NoError(t, err)
	ctx := context.Background()
	err = exporter.ConsumeTraces(ctx, td)
	if err != nil {
		return err
	}
	require.NoError(t, err)
	err = exporter.Shutdown(ctx)
	require.NoError(t, err)
	return nil
}

// Tests
func TestExportErrors(tester *testing.T) {
	type ExportErrorsTest struct {
		status int
	}
	var ExportErrorsTests = []ExportErrorsTest{
		{http.StatusUnauthorized},
		{http.StatusBadGateway},
		{http.StatusInternalServerError},
		{http.StatusForbidden},
		{http.StatusMethodNotAllowed},
		{http.StatusNotFound},
		{http.StatusBadRequest},
	}
	for _, test := range ExportErrorsTests {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(test.status)
		}))
		cfg := &Config{
			Region: "",
			Token:  "token",
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: server.URL,
			},
		}
		td := newTestTracesWithAttributes()
		ld := testdata.GenerateLogsManyLogRecordsSameResource(10)
		err := testTracesExporter(td, tester, cfg)
		fmt.Println(err.Error())
		require.Error(tester, err)
		err = testLogsExporter(ld, tester, cfg)
		fmt.Println(err.Error())
		server.Close()
		require.Error(tester, err)
	}

}

func TestNullTracesExporterConfig(tester *testing.T) {
	params := exportertest.NewNopCreateSettings()
	_, err := newLogzioTracesExporter(nil, params)
	assert.Error(tester, err, "Null exporter config should produce error")
}

func TestNullExporterConfig(tester *testing.T) {
	params := exportertest.NewNopCreateSettings()
	_, err := newLogzioExporter(nil, params)
	assert.Error(tester, err, "Null exporter config should produce error")
}

func gUnzipData(data []byte) (resData []byte, err error) {
	b := bytes.NewBuffer(data)

	var r io.Reader
	r, err = gzip.NewReader(b)
	if err != nil {
		return
	}

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return
	}
	resData = resB.Bytes()
	return
}

func TestPushTraceData(tester *testing.T) {
	var recordedRequests []byte
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		recordedRequests, _ = io.ReadAll(req.Body)
		rw.WriteHeader(http.StatusOK)
	}))
	cfg := Config{
		Token:  "token",
		Region: "",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint:    server.URL,
			Compression: configcompression.Gzip,
		},
	}
	defer server.Close()
	td := newTestTraces()
	res := td.ResourceSpans().At(0).Resource()
	res.Attributes().PutStr(conventions.AttributeServiceName, testService)
	res.Attributes().PutStr(conventions.AttributeHostName, testHost)
	err := testTracesExporter(td, tester, &cfg)
	require.NoError(tester, err)
	var newSpan logzioSpan
	decoded, _ := gUnzipData(recordedRequests)
	requests := strings.Split(string(decoded), "\n")
	assert.NoError(tester, json.Unmarshal([]byte(requests[0]), &newSpan))
	assert.Equal(tester, testOperation, newSpan.OperationName)
	assert.Equal(tester, testService, newSpan.Process.ServiceName)
	var newService logzioService
	assert.NoError(tester, json.Unmarshal([]byte(requests[1]), &newService))
	assert.Equal(tester, testOperation, newService.OperationName)
	assert.Equal(tester, testService, newService.ServiceName)
}

func TestPushLogsData(tester *testing.T) {
	var recordedRequests []byte
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		recordedRequests, _ = io.ReadAll(req.Body)
		rw.WriteHeader(http.StatusOK)
	}))
	cfg := Config{
		Token:  "token",
		Region: "",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint:    server.URL,
			Compression: configcompression.Gzip,
		},
	}
	defer server.Close()
	ld := testdata.GenerateLogsManyLogRecordsSameResource(2)
	res := ld.ResourceLogs().At(0).Resource()
	res.Attributes().PutStr(conventions.AttributeServiceName, testService)
	res.Attributes().PutStr(conventions.AttributeHostName, testHost)
	err := testLogsExporter(ld, tester, &cfg)
	require.NoError(tester, err)
	var jsonLog map[string]interface{}
	decoded, _ := gUnzipData(recordedRequests)
	requests := strings.Split(string(decoded), "\n")
	assert.NoError(tester, json.Unmarshal([]byte(requests[0]), &jsonLog))
	assert.Equal(tester, testHost, jsonLog["host.name"])
	assert.Equal(tester, testService, jsonLog["service.name"])

}
