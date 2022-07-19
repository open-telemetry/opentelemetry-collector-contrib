// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logzioexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

const (
	testService   = "testService"
	testHost      = "testHost"
	testOperation = "testOperation"
)

var (
	resourceAttributes1  = map[string]interface{}{"resource-attr": "resource-attr-val-1"}
	TestLogTime          = time.Now()
	TestLogTimeUnixMilli = TestLogTime.UnixMilli()
	TestLogTimestamp     = pcommon.NewTimestampFromTime(TestLogTime)
)

// Resource Attributes
func initResourceAttributes1(dest pcommon.Map) {
	pcommon.NewMapFromRaw(resourceAttributes1).CopyTo(dest)
}

// Resources
func initResource1(r pcommon.Resource) {
	initResourceAttributes1(r.Attributes())
}

// Logs

func fillLogOne(log plog.LogRecord) {
	log.SetTimestamp(TestLogTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetSeverityNumber(plog.SeverityNumberINFO)
	log.SetSeverityText("Info")
	log.SetSpanID(pcommon.NewSpanID([8]byte{0x01, 0x02, 0x04, 0x08}))
	log.SetTraceID(pcommon.NewTraceID([16]byte{0x08, 0x04, 0x02, 0x01}))

	attrs := log.Attributes()
	attrs.InsertString("app", "server")
	attrs.InsertDouble("instance_num", 1)

	// nested body map
	attVal := pcommon.NewValueMap()
	attNestedVal := pcommon.NewValueMap()

	attMap := attVal.MapVal()
	attMap.InsertDouble("23", 45)
	attMap.InsertString("foo", "bar")
	attMap.InsertString("message", "hello there")
	attNestedMap := attNestedVal.MapVal()
	attNestedMap.InsertString("string", "v1")
	attNestedMap.InsertDouble("number", 499)
	attMap.Insert("nested", attNestedVal)
	attVal.CopyTo(log.Body())

}

func fillLogTwo(log plog.LogRecord) {
	log.SetTimestamp(TestLogTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetSeverityNumber(plog.SeverityNumberINFO)
	log.SetSeverityText("Info")

	attrs := log.Attributes()
	attrs.InsertString("customer", "acme")
	attrs.InsertDouble("number", 64)
	attrs.InsertBool("bool", true)
	attrs.InsertString("env", "dev")
	log.Body().SetStringVal("something happened")
}
func fillLogNoTimestamp(log plog.LogRecord) {
	log.SetDroppedAttributesCount(1)
	log.SetSeverityNumber(plog.SeverityNumberINFO)
	log.SetSeverityText("Info")

	attrs := log.Attributes()
	attrs.InsertString("customer", "acme")
	attrs.InsertDouble("number", 64)
	attrs.InsertBool("bool", true)
	attrs.InsertString("env", "dev")
	log.Body().SetStringVal("something happened")
}

func GenerateLogsOneEmptyTimestamp() plog.Logs {
	ld := GenerateLogsOneEmptyLogRecord()
	logs := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	fillLogOne(logs.At(0))
	fillLogNoTimestamp(logs.AppendEmpty())
	return ld
}

func GenerateLogsOneEmptyResourceLogs() plog.Logs {
	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty()
	return ld
}

func GenerateLogsNoLogRecords() plog.Logs {
	ld := GenerateLogsOneEmptyResourceLogs()
	initResource1(ld.ResourceLogs().At(0).Resource())
	return ld
}

func GenerateLogsOneEmptyLogRecord() plog.Logs {
	ld := GenerateLogsNoLogRecords()
	rs0 := ld.ResourceLogs().At(0)
	rs0.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	return ld
}

func GenerateLogsManyLogRecordsSameResource(count int) plog.Logs {
	ld := GenerateLogsOneEmptyLogRecord()
	logs := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	logs.EnsureCapacity(count)
	for i := 0; i < count; i++ {
		var l plog.LogRecord
		if i < logs.Len() {
			l = logs.At(i)
		} else {
			l = logs.AppendEmpty()
		}

		if i%2 == 0 {
			fillLogOne(l)
		} else {
			fillLogTwo(l)
		}
	}
	return ld
}

func testLogsExporter(ld plog.Logs, t *testing.T, cfg *Config) error {
	var err error
	params := componenttest.NewNopExporterCreateSettings()
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
		s.SetTraceID(pcommon.NewTraceID([16]byte{byte(i), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}))
		s.SetSpanID(pcommon.NewSpanID([8]byte{byte(i), 0, 0, 0, 0, 0, 0, 2}))
		for j := 0; j < 5; j++ {
			s.Attributes().Insert(fmt.Sprintf("k%d", j), pcommon.NewValueString(fmt.Sprintf("v%d", j)))
		}
		s.SetKind(ptrace.SpanKindServer)
	}
	return td
}

func newTestTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	s := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	s.SetName(testOperation)
	s.SetTraceID(pcommon.NewTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}))
	s.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 2}))
	s.SetKind(ptrace.SpanKindServer)
	return td
}

func testTracesExporter(td ptrace.Traces, t *testing.T, cfg *Config) error {
	params := componenttest.NewNopExporterCreateSettings()
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
			Region:           "",
			Token:            "token",
			ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: server.URL,
			},
		}
		td := newTestTracesWithAttributes()
		ld := GenerateLogsManyLogRecordsSameResource(10)
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
	params := componenttest.NewNopExporterCreateSettings()
	_, err := newLogzioTracesExporter(nil, params)
	assert.Error(tester, err, "Null exporter config should produce error")
}

func TestNullExporterConfig(tester *testing.T) {
	params := componenttest.NewNopExporterCreateSettings()
	_, err := newLogzioExporter(nil, params)
	assert.Error(tester, err, "Null exporter config should produce error")
}

func TestNullTokenConfig(tester *testing.T) {
	cfg := Config{
		Region: "eu",
	}
	params := componenttest.NewNopExporterCreateSettings()
	_, err := createTracesExporter(context.Background(), params, &cfg)
	assert.Error(tester, err, "Empty token should produce error")
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
		recordedRequests, _ = ioutil.ReadAll(req.Body)
		rw.WriteHeader(http.StatusOK)
	}))
	cfg := Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		Token:            "token",
		Region:           "",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint:    server.URL,
			Compression: configcompression.Gzip,
		},
	}
	defer server.Close()
	td := newTestTraces()
	res := td.ResourceSpans().At(0).Resource()
	res.Attributes().UpsertString(conventions.AttributeServiceName, testService)
	res.Attributes().UpsertString(conventions.AttributeHostName, testHost)
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
		recordedRequests, _ = ioutil.ReadAll(req.Body)
		rw.WriteHeader(http.StatusOK)
	}))
	cfg := Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		Token:            "token",
		Region:           "",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint:    server.URL,
			Compression: configcompression.Gzip,
		},
	}
	defer server.Close()
	ld := GenerateLogsManyLogRecordsSameResource(2)
	res := ld.ResourceLogs().At(0).Resource()
	res.Attributes().UpsertString(conventions.AttributeServiceName, testService)
	res.Attributes().UpsertString(conventions.AttributeHostName, testHost)
	err := testLogsExporter(ld, tester, &cfg)
	require.NoError(tester, err)
	var jsonLog map[string]interface{}
	decoded, _ := gUnzipData(recordedRequests)
	requests := strings.Split(string(decoded), "\n")
	assert.NoError(tester, json.Unmarshal([]byte(requests[0]), &jsonLog))
	assert.Equal(tester, testHost, jsonLog["host.name"])
	assert.Equal(tester, testService, jsonLog["service.name"])

}
