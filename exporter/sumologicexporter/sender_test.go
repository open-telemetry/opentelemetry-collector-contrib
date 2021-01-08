// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sumologicexporter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type senderTest struct {
	srv *httptest.Server
	exp *sumologicexporter
	s   *sender
}

func prepareSenderTest(t *testing.T, cb []func(w http.ResponseWriter, req *http.Request)) *senderTest {
	reqCounter := 0
	// generate a test server so we can capture and inspect the request
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if len(cb) > 0 && assert.Greater(t, len(cb), reqCounter) {
			cb[reqCounter](w, req)
			reqCounter++
		}
	}))

	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: testServer.URL,
			Timeout:  defaultTimeout,
		},
		LogFormat:          "text",
		MetricFormat:       "carbon2",
		Client:             "otelcol",
		MaxRequestBodySize: 20_971_520,
	}
	exp, err := initExporter(cfg)
	require.NoError(t, err)

	f, err := newFilter([]string{})
	require.NoError(t, err)

	c, err := newCompressor(NoCompression)
	require.NoError(t, err)

	pf, err := newPrometheusFormatter()
	require.NoError(t, err)

	return &senderTest{
		srv: testServer,
		exp: exp,
		s: newSender(
			cfg,
			&http.Client{
				Timeout: cfg.HTTPClientSettings.Timeout,
			},
			f,
			sourceFormats{
				host:     getTestSourceFormat(t, "source_host"),
				category: getTestSourceFormat(t, "source_category"),
				name:     getTestSourceFormat(t, "source_name"),
			},
			c,
			pf,
		),
	}
}

func extractBody(t *testing.T, req *http.Request) string {
	buf := new(strings.Builder)
	_, err := io.Copy(buf, req.Body)
	require.NoError(t, err)
	return buf.String()
}

func exampleLog() []pdata.LogRecord {
	buffer := make([]pdata.LogRecord, 1)
	buffer[0] = pdata.NewLogRecord()
	buffer[0].Body().SetStringVal("Example log")

	return buffer
}

func exampleTwoLogs() []pdata.LogRecord {
	buffer := make([]pdata.LogRecord, 2)
	buffer[0] = pdata.NewLogRecord()
	buffer[0].Body().SetStringVal("Example log")
	buffer[0].Attributes().InsertString("key1", "value1")
	buffer[0].Attributes().InsertString("key2", "value2")
	buffer[1] = pdata.NewLogRecord()
	buffer[1].Body().SetStringVal("Another example log")
	buffer[1].Attributes().InsertString("key1", "value1")
	buffer[1].Attributes().InsertString("key2", "value2")

	return buffer
}

func exampleTwoDifferentLogs() []pdata.LogRecord {
	buffer := make([]pdata.LogRecord, 2)
	buffer[0] = pdata.NewLogRecord()
	buffer[0].Body().SetStringVal("Example log")
	buffer[0].Attributes().InsertString("key1", "value1")
	buffer[0].Attributes().InsertString("key2", "value2")
	buffer[1] = pdata.NewLogRecord()
	buffer[1].Body().SetStringVal("Another example log")
	buffer[1].Attributes().InsertString("key3", "value3")
	buffer[1].Attributes().InsertString("key4", "value4")

	return buffer
}

func TestSendLogs(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log\nAnother example log", body)
			assert.Equal(t, "key1=value, key2=value2", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.logBuffer = exampleTwoLogs()

	_, err := test.s.sendLogs(context.Background(), fields{"key1": "value", "key2": "value2"})
	assert.NoError(t, err)
}

func TestSendLogsSplit(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Another example log", body)
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.MaxRequestBodySize = 10
	test.s.logBuffer = exampleTwoLogs()

	_, err := test.s.sendLogs(context.Background(), fields{})
	assert.NoError(t, err)
}
func TestSendLogsSplitFailedOne(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Another example log", body)
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.MaxRequestBodySize = 10
	test.s.config.LogFormat = TextFormat
	test.s.logBuffer = exampleTwoLogs()

	dropped, err := test.s.sendLogs(context.Background(), fields{})
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, test.s.logBuffer[0:1], dropped)
}

func TestSendLogsSplitFailedAll(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(404)

			body := extractBody(t, req)
			assert.Equal(t, "Another example log", body)
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.MaxRequestBodySize = 10
	test.s.config.LogFormat = TextFormat
	test.s.logBuffer = exampleTwoLogs()

	dropped, err := test.s.sendLogs(context.Background(), fields{})
	assert.EqualError(
		t,
		err,
		"[error during sending data: 500 Internal Server Error; error during sending data: 404 Not Found]",
	)
	assert.Equal(t, test.s.logBuffer[0:2], dropped)
}

func TestSendLogsJson(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `{"key1":"value1","key2":"value2","log":"Example log"}
{"key1":"value1","key2":"value2","log":"Another example log"}`
			assert.Equal(t, expected, body)
			assert.Equal(t, "key=value", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.LogFormat = JSONFormat
	test.s.logBuffer = exampleTwoLogs()

	_, err := test.s.sendLogs(context.Background(), fields{"key": "value"})
	assert.NoError(t, err)
}

func TestSendLogsJsonSplit(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `{"key1":"value1","key2":"value2","log":"Example log"}`, body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `{"key1":"value1","key2":"value2","log":"Another example log"}`, body)
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.LogFormat = JSONFormat
	test.s.config.MaxRequestBodySize = 10
	test.s.logBuffer = exampleTwoLogs()

	_, err := test.s.sendLogs(context.Background(), fields{})
	assert.NoError(t, err)
}

func TestSendLogsJsonSplitFailedOne(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			assert.Equal(t, `{"key1":"value1","key2":"value2","log":"Example log"}`, body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `{"key1":"value1","key2":"value2","log":"Another example log"}`, body)
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.LogFormat = JSONFormat
	test.s.config.MaxRequestBodySize = 10
	test.s.logBuffer = exampleTwoLogs()

	dropped, err := test.s.sendLogs(context.Background(), fields{})
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, test.s.logBuffer[0:1], dropped)
}

func TestSendLogsJsonSplitFailedAll(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			assert.Equal(t, `{"key1":"value1","key2":"value2","log":"Example log"}`, body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(404)

			body := extractBody(t, req)
			assert.Equal(t, `{"key1":"value1","key2":"value2","log":"Another example log"}`, body)
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.LogFormat = JSONFormat
	test.s.config.MaxRequestBodySize = 10
	test.s.logBuffer = exampleTwoLogs()

	dropped, err := test.s.sendLogs(context.Background(), fields{})
	assert.EqualError(
		t,
		err,
		"[error during sending data: 500 Internal Server Error; error during sending data: 404 Not Found]",
	)
	assert.Equal(t, test.s.logBuffer[0:2], dropped)
}

func TestSendLogsUnexpectedFormat(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.LogFormat = "dummy"
	test.s.logBuffer = exampleTwoLogs()

	_, err := test.s.sendLogs(context.Background(), fields{})
	assert.Error(t, err)
}

func TestOverrideSourceName(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Test source name/test_name", req.Header.Get("X-Sumo-Name"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.sources.name = getTestSourceFormat(t, "Test source name/%{key1}")
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), fields{"key1": "test_name"})
	assert.NoError(t, err)
}

func TestOverrideSourceCategory(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Test source category/test_name", req.Header.Get("X-Sumo-Category"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.sources.category = getTestSourceFormat(t, "Test source category/%{key1}")
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), fields{"key1": "test_name"})
	assert.NoError(t, err)
}

func TestOverrideSourceHost(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Test source host/test_name", req.Header.Get("X-Sumo-Host"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.sources.host = getTestSourceFormat(t, "Test source host/%{key1}")
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), fields{"key1": "test_name"})
	assert.NoError(t, err)
}

func TestLogsBuffer(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	assert.Equal(t, test.s.countLogs(), 0)
	logs := exampleTwoLogs()

	droppedLogs, err := test.s.batchLog(context.Background(), logs[0], fields{})
	require.NoError(t, err)
	assert.Nil(t, droppedLogs)
	assert.Equal(t, 1, test.s.countLogs())
	assert.Equal(t, []pdata.LogRecord{logs[0]}, test.s.logBuffer)

	droppedLogs, err = test.s.batchLog(context.Background(), logs[1], fields{})
	require.NoError(t, err)
	assert.Nil(t, droppedLogs)
	assert.Equal(t, 2, test.s.countLogs())
	assert.Equal(t, logs, test.s.logBuffer)

	test.s.cleanLogsBuffer()
	assert.Equal(t, 0, test.s.countLogs())
	assert.Equal(t, []pdata.LogRecord{}, test.s.logBuffer)
}

func TestInvalidEndpoint(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ":"
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), fields{})
	assert.EqualError(t, err, `parse ":": missing protocol scheme`)
}

func TestInvalidPostRequest(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ""
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), fields{})
	assert.EqualError(t, err, `Post "": unsupported protocol scheme ""`)
}

func TestLogsBufferOverflow(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ":"
	log := exampleLog()

	for test.s.countLogs() < maxBufferSize-1 {
		_, err := test.s.batchLog(context.Background(), log[0], fields{})
		require.NoError(t, err)
	}

	_, err := test.s.batchLog(context.Background(), log[0], fields{})
	assert.EqualError(t, err, `parse ":": missing protocol scheme`)
	assert.Equal(t, 0, test.s.countLogs())
}

func TestMetricsPipeline(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	err := test.s.send(context.Background(), MetricsPipeline, strings.NewReader(""), fields{})
	assert.EqualError(t, err, `current sender version doesn't support metrics`)
}

func TestInvalidPipeline(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	err := test.s.send(context.Background(), "invalidPipeline", strings.NewReader(""), fields{})
	assert.EqualError(t, err, `unexpected pipeline`)
}

func TestSendCompressGzip(t *testing.T) {
	test := prepareSenderTest(t, []func(res http.ResponseWriter, req *http.Request){
		func(res http.ResponseWriter, req *http.Request) {
			res.WriteHeader(200)
			res.Write([]byte(""))
			body := decodeGzip(t, req.Body)
			assert.Equal(t, "gzip", req.Header.Get("Content-Encoding"))
			assert.Equal(t, "Some example log", body)
		},
	})
	defer func() { test.srv.Close() }()

	test.s.config.CompressEncoding = "gzip"

	c, err := newCompressor("gzip")
	require.NoError(t, err)

	test.s.compressor = c
	reader := strings.NewReader("Some example log")

	err = test.s.send(context.Background(), LogsPipeline, reader, fields{})
	require.NoError(t, err)
}

func TestSendCompressDeflate(t *testing.T) {
	test := prepareSenderTest(t, []func(res http.ResponseWriter, req *http.Request){
		func(res http.ResponseWriter, req *http.Request) {
			res.WriteHeader(200)
			res.Write([]byte(""))
			body := decodeDeflate(t, req.Body)
			assert.Equal(t, "deflate", req.Header.Get("Content-Encoding"))
			assert.Equal(t, "Some example log", body)
		},
	})
	defer func() { test.srv.Close() }()

	test.s.config.CompressEncoding = "deflate"

	c, err := newCompressor("deflate")
	require.NoError(t, err)

	test.s.compressor = c
	reader := strings.NewReader("Some example log")

	err = test.s.send(context.Background(), LogsPipeline, reader, fields{})
	require.NoError(t, err)
}

func TestCompressionError(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.compressor = getTestCompressor(errors.New("read error"), nil)
	reader := strings.NewReader("Some example log")

	err := test.s.send(context.Background(), LogsPipeline, reader, fields{})
	assert.EqualError(t, err, "read error")
}

func TestInvalidContentEncoding(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.CompressEncoding = "test"
	reader := strings.NewReader("Some example log")

	err := test.s.send(context.Background(), LogsPipeline, reader, fields{})
	assert.EqualError(t, err, "invalid content encoding: test")
}
