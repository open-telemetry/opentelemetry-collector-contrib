// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
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
	"go.opentelemetry.io/collector/pdata/testdata"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
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
	ld := testdata.GenerateLogs(1)
	logs := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	ld.ResourceLogs().At(0).ScopeLogs().At(0).Scope().SetName("logScopeName")
	fillLogOne(logs.At(0))
	fillLogNoTimestamp(logs.AppendEmpty())
	return ld
}

func testLogsExporter(ld plog.Logs, t *testing.T, cfg *Config) error {
	var err error
	params := exportertest.NewNopSettings()
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
	params := exportertest.NewNopSettings()
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
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(test.status)
		}))
		clientConfig := confighttp.NewDefaultClientConfig()
		clientConfig.Endpoint = server.URL

		cfg := &Config{
			Region:       "",
			Token:        "token",
			ClientConfig: clientConfig,
		}
		td := newTestTracesWithAttributes()
		ld := testdata.GenerateLogs(10)
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
	params := exportertest.NewNopSettings()
	_, err := newLogzioTracesExporter(nil, params)
	assert.Error(tester, err, "Null exporter config should produce error")
}

func TestNullExporterConfig(tester *testing.T) {
	params := exportertest.NewNopSettings()
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
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	clientConfig.Compression = configcompression.TypeGzip
	cfg := Config{
		Token:        "token",
		Region:       "",
		ClientConfig: clientConfig,
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
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg := Config{
		Token:        "token",
		Region:       "",
		ClientConfig: clientConfig,
	}
	defer server.Close()
	ld := generateLogsOneEmptyTimestamp()
	res := ld.ResourceLogs().At(0).Resource()
	res.Attributes().PutStr(conventions.AttributeServiceName, testService)
	res.Attributes().PutStr(conventions.AttributeHostName, testHost)
	err := testLogsExporter(ld, tester, &cfg)
	require.NoError(tester, err)
	requests := plogotlp.NewExportRequest()
	err = requests.UnmarshalProto(recordedRequests)
	require.NoError(tester, err)
	resultLogs := requests.Logs()
	assert.Equal(tester, ld.LogRecordCount(), resultLogs.LogRecordCount())
	assert.Equal(tester, ld.ResourceLogs().At(0).Resource().Attributes().AsRaw(), resultLogs.ResourceLogs().At(0).Resource().Attributes().AsRaw())
	assert.Equal(tester, ld.ResourceLogs(), resultLogs.ResourceLogs())
}

func TestTracesPartialSuccessHandler(t *testing.T) {
	exporter, err := newLogzioExporter(&Config{}, exportertest.NewNopSettings())
	require.NoError(t, err)

	exportResponse := ptraceotlp.NewExportResponse()
	partial := exportResponse.PartialSuccess()
	partial.SetErrorMessage("Partial success error")
	partial.SetRejectedSpans(5)

	protoBytes, err := exportResponse.MarshalProto()
	require.NoError(t, err)

	logger, _ := observer.New(zap.WarnLevel)
	exporter.logger = zap.New(logger)

	err = exporter.tracesPartialSuccessHandler(protoBytes, protobufContentType)
	require.NoError(t, err)

	err = exporter.tracesPartialSuccessHandler([]byte{0xFF}, protobufContentType)
	require.Error(t, err)
	require.Contains(t, err.Error(), "error parsing protobuf response")

	err = exporter.tracesPartialSuccessHandler(nil, protobufContentType)
	require.NoError(t, err)

	err = exporter.tracesPartialSuccessHandler(protoBytes, "unknown/content-type")
	require.NoError(t, err)
}

func TestLogsPartialSuccessHandler(t *testing.T) {
	exporter, err := newLogzioExporter(&Config{}, exportertest.NewNopSettings())
	require.NoError(t, err)
	// Create a valid ExportResponse with PartialSuccess information
	exportResponse := plogotlp.NewExportResponse()
	partial := exportResponse.PartialSuccess()
	partial.SetErrorMessage("Partial success error")
	partial.SetRejectedLogRecords(5)

	protoBytes, err := exportResponse.MarshalProto()
	require.NoError(t, err)

	logger, logs := observer.New(zap.WarnLevel)
	exporter.logger = zap.New(logger)

	err = exporter.logsPartialSuccessHandler(protoBytes, protobufContentType)
	require.NoError(t, err)

	warnLogs := logs.FilterLevelExact(zap.WarnLevel).All()
	require.Len(t, warnLogs, 1)
	require.Contains(t, warnLogs[0].Message, "Partial success response")
	require.Equal(t, "Partial success error", warnLogs[0].ContextMap()["message"])
	require.Equal(t, int64(5), warnLogs[0].ContextMap()["dropped_log_records"])

	// Now test with invalid protoBytes
	err = exporter.logsPartialSuccessHandler([]byte{0xFF}, protobufContentType)
	require.Error(t, err)
	require.Contains(t, err.Error(), "error parsing protobuf response")

	// Test with nil protoBytes
	err = exporter.logsPartialSuccessHandler(nil, protobufContentType)
	require.NoError(t, err)

	// Test with unknown content type
	err = exporter.logsPartialSuccessHandler(protoBytes, "unknown/content-type")
	require.NoError(t, err)
}

func TestReadResponseStatus(t *testing.T) {
	tests := []struct {
		name           string
		response       *http.Response
		expectedStatus *status.Status
	}{
		{
			name: "Valid status message",
			response: &http.Response{
				StatusCode: http.StatusBadRequest,
				Body: io.NopCloser(bytes.NewReader(func() []byte {
					statusProto := &status.Status{
						Code:    int32(codes.InvalidArgument),
						Message: "Invalid argument",
					}
					statusBytes, _ := proto.Marshal(statusProto)
					return statusBytes
				}())),
				Header: http.Header{"Content-Type": []string{protobufContentType}},
				ContentLength: int64(len(func() []byte {
					statusProto := &status.Status{
						Code:    int32(codes.InvalidArgument),
						Message: "Invalid argument",
					}
					statusBytes, _ := proto.Marshal(statusProto)
					return statusBytes
				}())),
			},
			expectedStatus: &status.Status{
				Code:    int32(codes.InvalidArgument),
				Message: "Invalid argument",
			},
		},
		{
			name: "Invalid status message",
			response: &http.Response{
				StatusCode:    http.StatusBadRequest,
				Body:          io.NopCloser(bytes.NewReader([]byte("invalid"))),
				Header:        http.Header{"Content-Type": []string{protobufContentType}},
				ContentLength: int64(len([]byte("invalid"))),
			},
			expectedStatus: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respStatus := readResponseStatus(tt.response)
			if tt.expectedStatus != nil {
				require.NotNil(t, respStatus)
				require.Equal(t, tt.expectedStatus.Message, respStatus.Message)
			} else {
				require.Nil(t, respStatus)
			}
		})
	}
}

func TestReadResponseBody(t *testing.T) {
	tests := []struct {
		name           string
		response       *http.Response
		expectedOutput []byte
		expectedError  error
	}{
		{
			name: "Empty body",
			response: &http.Response{
				ContentLength: 0,
				Body:          io.NopCloser(bytes.NewReader(nil)),
			},
			expectedOutput: nil,
			expectedError:  nil,
		},
		{
			name: "Valid body",
			response: &http.Response{
				ContentLength: 5,
				Body:          io.NopCloser(bytes.NewReader([]byte("hello"))),
			},
			expectedOutput: []byte("hello"),
			expectedError:  nil,
		},
		{
			name: "Body larger than max read",
			response: &http.Response{
				ContentLength: 100000,
				Body:          io.NopCloser(bytes.NewReader(make([]byte, 100000))),
			},
			expectedOutput: make([]byte, maxHTTPResponseReadBytes),
			expectedError:  nil,
		},
		{
			name: "Unexpected EOF",
			response: &http.Response{
				ContentLength: -1,
				Body:          io.NopCloser(bytes.NewReader([]byte("partial"))),
			},
			expectedOutput: []byte("partial"),
			expectedError:  nil,
		},
		{
			name: "Error reading body",
			response: &http.Response{
				ContentLength: 5,
				Body:          io.NopCloser(&errorReader{}),
			},
			expectedOutput: nil,
			expectedError:  errors.New("read error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := readResponseBody(tt.response)
			if tt.expectedError != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedOutput, output)
		})
	}
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}
