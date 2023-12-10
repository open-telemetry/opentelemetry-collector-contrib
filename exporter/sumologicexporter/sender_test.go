// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type senderTest struct {
	srv *httptest.Server
	exp *sumologicexporter
	s   *sender
}

func prepareSenderTest(t *testing.T, cb []func(w http.ResponseWriter, req *http.Request)) *senderTest {
	reqCounter := &atomic.Int32{}
	// generate a test server so we can capture and inspect the request
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if len(cb) == 0 {
			return
		}

		if c := int(reqCounter.Load()); assert.Greater(t, len(cb), c) {
			cb[c](w, req)
			reqCounter.Add(1)
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
	exp, err := initExporter(cfg, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	f, err := newFilter([]string{})
	require.NoError(t, err)

	c, err := newCompressor(NoCompression)
	require.NoError(t, err)

	pf := newPrometheusFormatter()

	gf := newGraphiteFormatter(DefaultGraphiteTemplate)

	err = exp.start(context.Background(), componenttest.NewNopHost())
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
				host:     getTestSourceFormat("source_host"),
				category: getTestSourceFormat("source_category"),
				name:     getTestSourceFormat("source_name"),
			},
			c,
			pf,
			gf,
		),
	}
}

func extractBody(t *testing.T, req *http.Request) string {
	buf := new(strings.Builder)
	_, err := io.Copy(buf, req.Body)
	require.NoError(t, err)
	return buf.String()
}

func exampleLog() []plog.LogRecord {
	buffer := make([]plog.LogRecord, 1)
	buffer[0] = plog.NewLogRecord()
	buffer[0].Body().SetStr("Example log")

	return buffer
}

func exampleTwoLogs() []plog.LogRecord {
	buffer := make([]plog.LogRecord, 2)
	buffer[0] = plog.NewLogRecord()
	buffer[0].Body().SetStr("Example log")
	buffer[0].Attributes().PutStr("key1", "value1")
	buffer[0].Attributes().PutStr("key2", "value2")
	buffer[1] = plog.NewLogRecord()
	buffer[1].Body().SetStr("Another example log")
	buffer[1].Attributes().PutStr("key1", "value1")
	buffer[1].Attributes().PutStr("key2", "value2")

	return buffer
}

func exampleTwoDifferentLogs() []plog.LogRecord {
	buffer := make([]plog.LogRecord, 2)
	buffer[0] = plog.NewLogRecord()
	buffer[0].Body().SetStr("Example log")
	buffer[0].Attributes().PutStr("key1", "value1")
	buffer[0].Attributes().PutStr("key2", "value2")
	buffer[1] = plog.NewLogRecord()
	buffer[1].Body().SetStr("Another example log")
	buffer[1].Attributes().PutStr("key3", "value3")
	buffer[1].Attributes().PutStr("key4", "value4")

	return buffer
}

func exampleMultitypeLogs() []plog.LogRecord {
	buffer := make([]plog.LogRecord, 2)

	attVal := pcommon.NewValueMap()
	attMap := attVal.Map()
	attMap.PutStr("lk1", "lv1")
	attMap.PutInt("lk2", 13)

	buffer[0] = plog.NewLogRecord()
	attVal.CopyTo(buffer[0].Body())

	buffer[0].Attributes().PutStr("key1", "value1")
	buffer[0].Attributes().PutStr("key2", "value2")

	buffer[1] = plog.NewLogRecord()

	attVal = pcommon.NewValueSlice()
	attArr := attVal.Slice()
	strVal := pcommon.NewValueEmpty()
	strVal.SetStr("lv2")
	intVal := pcommon.NewValueEmpty()
	intVal.SetInt(13)

	strTgt := attArr.AppendEmpty()
	strVal.CopyTo(strTgt)
	intTgt := attArr.AppendEmpty()
	intVal.CopyTo(intTgt)

	attVal.CopyTo(buffer[1].Body())
	buffer[1].Attributes().PutStr("key1", "value1")
	buffer[1].Attributes().PutStr("key2", "value2")

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

	_, err := test.s.sendLogs(context.Background(), fieldsFromMap(map[string]string{"key1": "value", "key2": "value2"}))
	assert.NoError(t, err)
}

func TestSendLogsMultitype(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `{"lk1":"lv1","lk2":13}
["lv2",13]`
			assert.Equal(t, expected, body)
			assert.Equal(t, "key1=value, key2=value2", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.logBuffer = exampleMultitypeLogs()

	_, err := test.s.sendLogs(context.Background(), fieldsFromMap(map[string]string{"key1": "value", "key2": "value2"}))
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

	_, err := test.s.sendLogs(context.Background(), newFields(pcommon.NewMap()))
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

	dropped, err := test.s.sendLogs(context.Background(), newFields(pcommon.NewMap()))
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

	dropped, err := test.s.sendLogs(context.Background(), newFields(pcommon.NewMap()))
	assert.EqualError(
		t,
		err,
		"error during sending data: 500 Internal Server Error\nerror during sending data: 404 Not Found",
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

	_, err := test.s.sendLogs(context.Background(), fieldsFromMap(map[string]string{"key": "value"}))
	assert.NoError(t, err)
}

func TestSendLogsJsonMultitype(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `{"key1":"value1","key2":"value2","log":{"lk1":"lv1","lk2":13}}
{"key1":"value1","key2":"value2","log":["lv2",13]}`
			assert.Equal(t, expected, body)
			assert.Equal(t, "key=value", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.LogFormat = JSONFormat
	test.s.logBuffer = exampleMultitypeLogs()

	_, err := test.s.sendLogs(context.Background(), fieldsFromMap(map[string]string{"key": "value"}))
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

	_, err := test.s.sendLogs(context.Background(), newFields(pcommon.NewMap()))
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

	dropped, err := test.s.sendLogs(context.Background(), newFields(pcommon.NewMap()))
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

	dropped, err := test.s.sendLogs(context.Background(), newFields(pcommon.NewMap()))
	assert.EqualError(
		t,
		err,
		"error during sending data: 500 Internal Server Error\nerror during sending data: 404 Not Found",
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
	logs := exampleTwoLogs()
	test.s.logBuffer = logs

	dropped, err := test.s.sendLogs(context.Background(), newFields(pcommon.NewMap()))
	assert.Error(t, err)
	assert.Equal(t, logs, dropped)
}

func TestOverrideSourceName(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Test source name/test_name", req.Header.Get("X-Sumo-Name"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.sources.name = getTestSourceFormat("Test source name/%{key1}")
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), fieldsFromMap(map[string]string{"key1": "test_name"}))
	assert.NoError(t, err)
}

func TestOverrideSourceCategory(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Test source category/test_name", req.Header.Get("X-Sumo-Category"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.sources.category = getTestSourceFormat("Test source category/%{key1}")
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), fieldsFromMap(map[string]string{"key1": "test_name"}))
	assert.NoError(t, err)
}

func TestOverrideSourceHost(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Test source host/test_name", req.Header.Get("X-Sumo-Host"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.sources.host = getTestSourceFormat("Test source host/%{key1}")
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), fieldsFromMap(map[string]string{"key1": "test_name"}))
	assert.NoError(t, err)
}

func TestLogsBuffer(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	assert.Equal(t, test.s.countLogs(), 0)
	logs := exampleTwoLogs()

	droppedLogs, err := test.s.batchLog(context.Background(), logs[0], newFields(pcommon.NewMap()))
	require.NoError(t, err)
	assert.Nil(t, droppedLogs)
	assert.Equal(t, 1, test.s.countLogs())
	assert.Equal(t, []plog.LogRecord{logs[0]}, test.s.logBuffer)

	droppedLogs, err = test.s.batchLog(context.Background(), logs[1], newFields(pcommon.NewMap()))
	require.NoError(t, err)
	assert.Nil(t, droppedLogs)
	assert.Equal(t, 2, test.s.countLogs())
	assert.Equal(t, logs, test.s.logBuffer)

	test.s.cleanLogsBuffer()
	assert.Equal(t, 0, test.s.countLogs())
	assert.Equal(t, []plog.LogRecord{}, test.s.logBuffer)
}

func TestInvalidEndpoint(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ":"
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), newFields(pcommon.NewMap()))
	assert.EqualError(t, err, `parse ":": missing protocol scheme`)
}

func TestInvalidPostRequest(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ""
	test.s.logBuffer = exampleLog()

	_, err := test.s.sendLogs(context.Background(), newFields(pcommon.NewMap()))
	assert.EqualError(t, err, `Post "": unsupported protocol scheme ""`)
}

func TestLogsBufferOverflow(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ":"
	log := exampleLog()
	flds := newFields(pcommon.NewMap())

	for test.s.countLogs() < maxBufferSize-1 {
		_, err := test.s.batchLog(context.Background(), log[0], flds)
		require.NoError(t, err)
	}

	_, err := test.s.batchLog(context.Background(), log[0], flds)
	assert.EqualError(t, err, `parse ":": missing protocol scheme`)
	assert.Equal(t, 0, test.s.countLogs())
}

func TestInvalidMetricFormat(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.MetricFormat = "invalid"

	err := test.s.send(context.Background(), MetricsPipeline, strings.NewReader(""), newFields(pcommon.NewMap()))
	assert.EqualError(t, err, `unsupported metrics format: invalid`)
}

func TestInvalidPipeline(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	err := test.s.send(context.Background(), "invalidPipeline", strings.NewReader(""), newFields(pcommon.NewMap()))
	assert.EqualError(t, err, `unexpected pipeline`)
}

func TestSendCompressGzip(t *testing.T) {
	test := prepareSenderTest(t, []func(res http.ResponseWriter, req *http.Request){
		func(res http.ResponseWriter, req *http.Request) {
			res.WriteHeader(200)
			_, err := res.Write([]byte(""))
			require.NoError(t, err)
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

	err = test.s.send(context.Background(), LogsPipeline, reader, newFields(pcommon.NewMap()))
	require.NoError(t, err)
}

func TestSendCompressDeflate(t *testing.T) {
	test := prepareSenderTest(t, []func(res http.ResponseWriter, req *http.Request){
		func(res http.ResponseWriter, req *http.Request) {
			res.WriteHeader(200)
			_, err := res.Write([]byte(""))
			require.NoError(t, err)
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

	err = test.s.send(context.Background(), LogsPipeline, reader, newFields(pcommon.NewMap()))
	require.NoError(t, err)
}

func TestCompressionError(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.compressor = getTestCompressor(errors.New("read error"), nil)
	reader := strings.NewReader("Some example log")

	err := test.s.send(context.Background(), LogsPipeline, reader, newFields(pcommon.NewMap()))
	assert.EqualError(t, err, "read error")
}

func TestInvalidContentEncoding(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.CompressEncoding = "test"
	reader := strings.NewReader("Some example log")

	err := test.s.send(context.Background(), LogsPipeline, reader, newFields(pcommon.NewMap()))
	assert.EqualError(t, err, "invalid content encoding: test")
}

func TestSendMetrics(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `test_metric_data{test="test_value",test2="second_value"} 14500 1605534165000
gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()
	flds := fieldsFromMap(map[string]string{
		"key1": "value",
		"key2": "value2",
	})

	test.s.config.MetricFormat = PrometheusFormat
	test.s.metricBuffer = []metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	}
	_, err := test.s.sendMetrics(context.Background(), flds)
	assert.NoError(t, err)
}

func TestSendMetricsSplit(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `test_metric_data{test="test_value",test2="second_value"} 14500 1605534165000`
			assert.Equal(t, expected, body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.MaxRequestBodySize = 10
	test.s.config.MetricFormat = PrometheusFormat
	test.s.metricBuffer = []metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	}

	_, err := test.s.sendMetrics(context.Background(), newFields(pcommon.NewMap()))
	assert.NoError(t, err)
}

func TestSendMetricsSplitFailedOne(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			expected := `test_metric_data{test="test_value",test2="second_value"} 14500 1605534165000`
			assert.Equal(t, expected, body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.MaxRequestBodySize = 10
	test.s.config.MetricFormat = PrometheusFormat
	test.s.metricBuffer = []metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	}

	dropped, err := test.s.sendMetrics(context.Background(), newFields(pcommon.NewMap()))
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, test.s.metricBuffer[0:1], dropped)
}

func TestSendMetricsSplitFailedAll(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			expected := `test_metric_data{test="test_value",test2="second_value"} 14500 1605534165000`
			assert.Equal(t, expected, body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(404)

			body := extractBody(t, req)
			expected := `gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.MaxRequestBodySize = 10
	test.s.config.MetricFormat = PrometheusFormat
	test.s.metricBuffer = []metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	}

	dropped, err := test.s.sendMetrics(context.Background(), newFields(pcommon.NewMap()))
	assert.EqualError(
		t,
		err,
		"error during sending data: 500 Internal Server Error\nerror during sending data: 404 Not Found",
	)
	assert.Equal(t, test.s.metricBuffer[0:2], dropped)
}

func TestSendMetricsUnexpectedFormat(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
		},
	})
	defer func() { test.srv.Close() }()
	test.s.config.MetricFormat = "invalid"
	metrics := []metricPair{
		exampleIntMetric(),
	}
	test.s.metricBuffer = metrics

	dropped, err := test.s.sendMetrics(context.Background(), newFields(pcommon.NewMap()))
	assert.EqualError(t, err, "unexpected metric format: invalid")
	assert.Equal(t, dropped, metrics)
}

func TestMetricsBuffer(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	assert.Equal(t, test.s.countMetrics(), 0)
	metrics := []metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	}

	droppedMetrics, err := test.s.batchMetric(context.Background(), metrics[0], newFields(pcommon.NewMap()))
	require.NoError(t, err)
	assert.Nil(t, droppedMetrics)
	assert.Equal(t, 1, test.s.countMetrics())
	assert.Equal(t, metrics[0:1], test.s.metricBuffer)

	droppedMetrics, err = test.s.batchMetric(context.Background(), metrics[1], newFields(pcommon.NewMap()))
	require.NoError(t, err)
	assert.Nil(t, droppedMetrics)
	assert.Equal(t, 2, test.s.countMetrics())
	assert.Equal(t, metrics, test.s.metricBuffer)

	test.s.cleanMetricBuffer()
	assert.Equal(t, 0, test.s.countMetrics())
	assert.Equal(t, []metricPair{}, test.s.metricBuffer)
}

func TestMetricsBufferOverflow(t *testing.T) {
	t.Skip("Skip test due to prometheus format complexity. Execution can take over 30s")
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.HTTPClientSettings.Endpoint = ":"
	test.s.config.MetricFormat = PrometheusFormat
	test.s.config.MaxRequestBodySize = 1024 * 1024 * 1024 * 1024
	metric := exampleIntMetric()
	flds := newFields(pcommon.NewMap())

	for test.s.countMetrics() < maxBufferSize-1 {
		_, err := test.s.batchMetric(context.Background(), metric, flds)
		require.NoError(t, err)
	}

	_, err := test.s.batchMetric(context.Background(), metric, flds)
	assert.EqualError(t, err, `parse ":": missing protocol scheme`)
	assert.Equal(t, 0, test.s.countMetrics())
}

func TestSendCarbon2Metrics(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `test=test_value test2=second_value _unit=m/s escape_me=:invalid_ metric=true metric=test.metric.data unit=bytes  14500 1605534165
foo=bar metric=gauge_metric_name  124 1608124661
foo=bar metric=gauge_metric_name  245 1608124662`
			assert.Equal(t, expected, body)
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/vnd.sumologic.carbon2", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()

	test.s.config.MetricFormat = Carbon2Format
	test.s.metricBuffer = []metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	}

	flds := fieldsFromMap(map[string]string{
		"key1": "value",
		"key2": "value2",
	})

	test.s.metricBuffer[0].attributes.PutStr("unit", "m/s")
	test.s.metricBuffer[0].attributes.PutStr("escape me", "=invalid\n")
	test.s.metricBuffer[0].attributes.PutBool("metric", true)

	_, err := test.s.sendMetrics(context.Background(), flds)
	assert.NoError(t, err)
}

func TestSendGraphiteMetrics(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `test_metric_data.true.m/s 14500 1605534165
gauge_metric_name.. 124 1608124661
gauge_metric_name.. 245 1608124662`
			assert.Equal(t, expected, body)
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/vnd.sumologic.graphite", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()

	gf := newGraphiteFormatter("%{_metric_}.%{metric}.%{unit}")
	test.s.graphiteFormatter = gf

	test.s.config.MetricFormat = GraphiteFormat
	test.s.metricBuffer = []metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	}

	flds := fieldsFromMap(map[string]string{
		"key1": "value",
		"key2": "value2",
	})

	test.s.metricBuffer[0].attributes.PutStr("unit", "m/s")
	test.s.metricBuffer[0].attributes.PutBool("metric", true)

	_, err := test.s.sendMetrics(context.Background(), flds)
	assert.NoError(t, err)
}
