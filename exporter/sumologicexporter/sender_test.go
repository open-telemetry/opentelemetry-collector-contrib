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
		Client:             "otelcol",
		MaxRequestBodySize: 20_971_520,
	}

	f, err := newFilter([]string{})
	require.NoError(t, err)

	return &senderTest{
		srv: testServer,
		s: newSender(
			context.Background(),
			cfg,
			&http.Client{
				Timeout: cfg.HTTPClientSettings.Timeout,
			},
			f,
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

func TestSend(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log\nAnother example log", body)
			assert.Equal(t, "test_metadata", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.buffer = exampleTwoLogs()

	_, err := test.s.sendLogs("test_metadata")
	assert.NoError(t, err)
}

func TestSendSplit(t *testing.T) {
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
	test.s.buffer = exampleTwoLogs()

	_, err := test.s.sendLogs("test_metadata")
	assert.NoError(t, err)
}
func TestSendSplitFailedOne(t *testing.T) {
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
	test.s.buffer = exampleTwoLogs()

	dropped, err := test.s.sendLogs("test_metadata")
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, test.s.buffer[0:1], dropped)
}

func TestSendSplitFailedAll(t *testing.T) {
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
	test.s.buffer = exampleTwoLogs()

	dropped, err := test.s.sendLogs("test_metadata")
	assert.EqualError(
		t,
		err,
		"[error during sending data: 500 Internal Server Error; error during sending data: 404 Not Found]",
	)
	assert.Equal(t, test.s.buffer[0:2], dropped)
}

func TestSendJson(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `{"key1":"value1","key2":"value2","log":"Example log"}
{"key1":"value1","key2":"value2","log":"Another example log"}`
			assert.Equal(t, expected, body)
			assert.Equal(t, "test_metadata", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.LogFormat = JSONFormat
	test.s.buffer = exampleTwoLogs()

	_, err := test.s.sendLogs("test_metadata")
	assert.NoError(t, err)
}

func TestSendJsonSplit(t *testing.T) {
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
	test.s.buffer = exampleTwoLogs()

	_, err := test.s.sendLogs("test_metadata")
	assert.NoError(t, err)
}

func TestSendJsonSplitFailedOne(t *testing.T) {
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
	test.s.buffer = exampleTwoLogs()

	dropped, err := test.s.sendLogs("test_metadata")
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, test.s.buffer[0:1], dropped)
}

func TestSendJsonSplitFailedAll(t *testing.T) {
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
	test.s.buffer = exampleTwoLogs()

	dropped, err := test.s.sendLogs("test_metadata")
	assert.EqualError(
		t,
		err,
		"[error during sending data: 500 Internal Server Error; error during sending data: 404 Not Found]",
	)
	assert.Equal(t, test.s.buffer[0:2], dropped)
}

func TestSendUnexpectedFormat(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.LogFormat = "dummy"
	test.s.buffer = exampleTwoLogs()

	_, err := test.s.sendLogs("test_metadata")
	assert.Error(t, err)
}

func TestOverrideSourceName(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Test source name", req.Header.Get("X-Sumo-Name"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.config.SourceName = "Test source name"
	test.s.buffer = exampleLog()

	_, err := test.s.sendLogs("test_metadata")
	assert.NoError(t, err)
}

func TestOverrideSourceCategory(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Test source category", req.Header.Get("X-Sumo-Category"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.config.SourceCategory = "Test source category"
	test.s.buffer = exampleLog()

	_, err := test.s.sendLogs("test_metadata")
	assert.NoError(t, err)
}

func TestOverrideSourceHost(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Test source host", req.Header.Get("X-Sumo-Host"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.config.SourceHost = "Test source host"
	test.s.buffer = exampleLog()

	_, err := test.s.sendLogs("test_metadata")
	assert.NoError(t, err)
}

func TestBuffer(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	assert.Equal(t, test.s.count(), 0)
	logs := exampleTwoLogs()

	droppedLogs, err := test.s.batch(logs[0], "")
	require.NoError(t, err)
	assert.Nil(t, droppedLogs)
	assert.Equal(t, 1, test.s.count())
	assert.Equal(t, []pdata.LogRecord{logs[0]}, test.s.buffer)

	droppedLogs, err = test.s.batch(logs[1], "")
	require.NoError(t, err)
	assert.Nil(t, droppedLogs)
	assert.Equal(t, 2, test.s.count())
	assert.Equal(t, logs, test.s.buffer)

	test.s.cleanBuffer()
	assert.Equal(t, 0, test.s.count())
	assert.Equal(t, []pdata.LogRecord{}, test.s.buffer)
}

func TestInvalidEndpoint(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ":"
	test.s.buffer = exampleLog()

	_, err := test.s.sendLogs("test_metadata")
	assert.EqualError(t, err, `parse ":": missing protocol scheme`)
}

func TestInvalidPostRequest(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ""
	test.s.buffer = exampleLog()

	_, err := test.s.sendLogs("test_metadata")
	assert.EqualError(t, err, `Post "": unsupported protocol scheme ""`)
}

func TestBufferOverflow(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ":"
	log := exampleLog()

	for test.s.count() < maxBufferSize-1 {
		_, err := test.s.batch(log[0], "test_metadata")
		require.NoError(t, err)
	}

	_, err := test.s.batch(log[0], "test_metadata")
	assert.EqualError(t, err, `parse ":": missing protocol scheme`)
	assert.Equal(t, 0, test.s.count())
}

func TestMetricsPipeline(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	err := test.s.send(MetricsPipeline, strings.NewReader(""), "")
	assert.EqualError(t, err, `current sender version doesn't support metrics`)
}

func TestInvalidPipeline(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	err := test.s.send("invalidPipeline", strings.NewReader(""), "")
	assert.EqualError(t, err, `unexpected pipeline`)
}
