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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func LogRecordsToLogs(records []plog.LogRecord) plog.Logs {
	logs := plog.NewLogs()
	logsSlice := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	for _, record := range records {
		record.CopyTo(logsSlice.AppendEmpty())
	}

	return logs
}

type exporterTest struct {
	srv        *httptest.Server
	exp        *sumologicexporter
	reqCounter *int32
}

func createTestConfig() *Config {
	config := createDefaultConfig().(*Config)
	config.CompressEncoding = NoCompression
	config.LogFormat = TextFormat
	config.MaxRequestBodySize = 20_971_520
	config.MetricFormat = OTLPMetricFormat
	config.TraceFormat = OTLPTraceFormat
	return config
}

func createExporterCreateSettings() exporter.CreateSettings {
	return exporter.CreateSettings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}
}

// prepareExporterTest prepares an exporter test object using provided config
// and a slice of callbacks to be called for subsequent requests coming being
// sent to the server.
// The enclosed *httptest.Server is automatically closed on test cleanup.
func prepareExporterTest(t *testing.T, cfg *Config, cb []func(w http.ResponseWriter, req *http.Request), cfgOpts ...func(*Config)) *exporterTest {
	var reqCounter int32
	// generate a test server so we can capture and inspect the request
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c := int(atomic.LoadInt32(&reqCounter))
		if assert.Greaterf(t, len(cb), c, "Exporter sent more requests (%d) than the number of test callbacks defined: %d", c+1, len(cb)) {
			cb[c](w, req)
			atomic.AddInt32(&reqCounter, 1)
		}
	}))
	t.Cleanup(func() {
		testServer.Close()

		// Ensure we got all required requests
		assert.Eventuallyf(t, func() bool {
			return int(atomic.LoadInt32(&reqCounter)) == len(cb)
		}, 2*time.Second, 100*time.Millisecond,
			"HTTP server didn't receive all the expected requests; got: %d, expected: %d",
			atomic.LoadInt32(&reqCounter), len(cb),
		)
	})

	cfg.HTTPClientSettings.Endpoint = testServer.URL
	cfg.HTTPClientSettings.Auth = nil
	for _, cfgOpt := range cfgOpts {
		cfgOpt(cfg)
	}

	exp, err := initExporter(cfg, createExporterCreateSettings())
	require.NoError(t, err)

	require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))

	return &exporterTest{
		srv:        testServer,
		exp:        exp,
		reqCounter: &reqCounter,
	}
}

func TestInitExporter(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "otlp",
		CompressEncoding: "gzip",
		TraceFormat:      "otlp",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
	}, createExporterCreateSettings())
	assert.NoError(t, err)
}

func TestConfigureExporter(t *testing.T) {
	host := componenttest.NewNopHost()
	config := createDefaultConfig().(*Config)
	config.HTTPClientSettings.Endpoint = "http://test_endpoint"
	exporter, err := initExporter(config, createExporterCreateSettings())
	require.NoError(t, err)
	err = exporter.start(context.Background(), host)
	require.NoError(t, err)
	require.Equal(t, "http://test_endpoint/v1/logs", exporter.dataUrlLogs)
	require.Equal(t, "http://test_endpoint/v1/metrics", exporter.dataUrlMetrics)
	require.Equal(t, "http://test_endpoint/v1/traces", exporter.dataUrlTraces)
}

func TestAllSuccess(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "", req.Header.Get("X-Sumo-Fields"))
		},
	})

	logs := LogRecordsToLogs(exampleLog())

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.NoError(t, err)
}

func TestLogsResourceAttributesSentAsFields(t *testing.T) {
	testcases := []struct {
		name       string
		configFunc func() *Config
		callbacks  []func(w http.ResponseWriter, req *http.Request)
		logsFunc   func() plog.Logs
	}{
		{
			name: "text",
			configFunc: func() *Config {
				config := createTestConfig()
				config.LogFormat = TextFormat
				// config.MetadataAttributes = []string{".*"}
				return config
			},
			callbacks: []func(w http.ResponseWriter, req *http.Request){
				func(w http.ResponseWriter, req *http.Request) {
					// b, err := httputil.DumpRequest(req, true)
					// assert.NoError(t, err)
					// fmt.Printf("body:\n%s\n", string(b))
					body := extractBody(t, req)
					assert.Equal(t, "Example log\nAnother example log", body)
					assert.Equal(t, "res_attr1=1, res_attr2=2", req.Header.Get("X-Sumo-Fields"))
				},
			},
			logsFunc: func() plog.Logs {
				buffer := make([]plog.LogRecord, 2)
				buffer[0] = plog.NewLogRecord()
				buffer[0].Body().SetStr("Example log")
				buffer[0].Attributes().PutStr("key1", "value1")
				buffer[0].Attributes().PutStr("key2", "value2")
				buffer[1] = plog.NewLogRecord()
				buffer[1].Body().SetStr("Another example log")
				buffer[1].Attributes().PutStr("key1", "value1")
				buffer[1].Attributes().PutStr("key2", "value2")
				buffer[1].Attributes().PutStr("key3", "value3")

				logs := LogRecordsToLogs(buffer)
				logs.ResourceLogs().At(0).Resource().Attributes().PutStr("res_attr1", "1")
				logs.ResourceLogs().At(0).Resource().Attributes().PutStr("res_attr2", "2")
				return logs
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.configFunc()
			test := prepareExporterTest(t, cfg, tc.callbacks)

			logs := tc.logsFunc()
			assert.NoError(t, test.exp.pushLogsData(context.Background(), logs))
			assert.EqualValues(t, len(tc.callbacks), atomic.LoadInt32(test.reqCounter))
		})
	}
}

func TestAllFailed(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			assert.Equal(t, "Example log\nAnother example log", body)
			assert.Empty(t, req.Header.Get("X-Sumo-Fields"))
		},
	})

	logs := plog.NewLogs()
	logsSlice := logs.ResourceLogs().AppendEmpty()
	logsRecords1 := logsSlice.ScopeLogs().AppendEmpty().LogRecords()
	logsRecords1.AppendEmpty().Body().SetStr("Example log")

	logsRecords2 := logsSlice.ScopeLogs().AppendEmpty().LogRecords()
	logsRecords2.AppendEmpty().Body().SetStr("Another example log")

	logsExpected := plog.NewLogs()
	logsSlice.CopyTo(logsExpected.ResourceLogs().AppendEmpty())

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error")

	var partial consumererror.Logs
	require.True(t, errors.As(err, &partial))
	assert.Equal(t, logsExpected, partial.Data())
}

func TestPartiallyFailed(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
			// No resource attributes for those logs hence no fields
			assert.Empty(t, req.Header.Get("X-Sumo-Fields"))
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			assert.Equal(t, "Another example log", body)
			// No resource attributes for those logs hence no fields
			assert.Empty(t, req.Header.Get("X-Sumo-Fields"))
		},
	})

	logs := plog.NewLogs()
	logsSlice1 := logs.ResourceLogs().AppendEmpty()
	logsRecords1 := logsSlice1.ScopeLogs().AppendEmpty().LogRecords()
	logsRecords1.AppendEmpty().Body().SetStr("Example log")
	logsSlice2 := logs.ResourceLogs().AppendEmpty()
	logsRecords2 := logsSlice2.ScopeLogs().AppendEmpty().LogRecords()
	logsRecords2.AppendEmpty().Body().SetStr("Another example log")

	logsExpected := plog.NewLogs()
	logsSlice2.CopyTo(logsExpected.ResourceLogs().AppendEmpty())

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error")

	var partial consumererror.Logs
	require.True(t, errors.As(err, &partial))
	assert.Equal(t, logsExpected, partial.Data())
}

func TestInvalidHTTPCLient(t *testing.T) {
	exp, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "otlp",
		CompressEncoding: "gzip",
		TraceFormat:      "otlp",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "test_endpoint",
			CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
				return nil, errors.New("roundTripperException")
			},
		},
	}, createExporterCreateSettings())
	assert.NoError(t, err)

	assert.EqualError(t,
		exp.start(context.Background(), componenttest.NewNopHost()),
		"failed to create HTTP Client: roundTripperException",
	)
}

func TestPushInvalidCompressor(t *testing.T) {
	// Expect no requests
	test := prepareExporterTest(t, createTestConfig(), nil)
	test.exp.config.CompressEncoding = "invalid"

	logs := LogRecordsToLogs(exampleLog())

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed to initialize compressor: invalid format: invalid")
}

func TestPushFailedBatch(t *testing.T) {
	t.Skip()

	t.Parallel()

	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)
			body := extractBody(t, req)

			expected := fmt.Sprintf(
				"%s%s",
				strings.Repeat("Example log\n", maxBufferSize-1),
				"Example log",
			)

			assert.Equal(t, expected, body)
			assert.Equal(t, "", req.Header.Get("X-Sumo-Fields"))
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(200)
			body := extractBody(t, req)

			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "", req.Header.Get("X-Sumo-Fields"))
		},
	})

	logs := LogRecordsToLogs(exampleLog())
	logs.ResourceLogs().EnsureCapacity(maxBufferSize + 1)
	log := logs.ResourceLogs().At(0)
	rLogs := logs.ResourceLogs()

	for i := 0; i < maxBufferSize; i++ {
		log.CopyTo(rLogs.AppendEmpty())
	}

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error")
}

func TestPushOTLPLogsClearTimestamp(t *testing.T) {
	createLogs := func() plog.Logs {
		exampleLogs := exampleLog()
		exampleLogs[0].SetTimestamp(12345)
		logs := LogRecordsToLogs(exampleLogs)
		return logs
	}

	testcases := []struct {
		name         string
		configFunc   func() *Config
		expectedBody string
	}{
		{
			name: "enabled",
			configFunc: func() *Config {
				config := createTestConfig()
				config.ClearLogsTimestamp = true
				config.LogFormat = OTLPLogFormat
				return config
			},
			expectedBody: "\n\x1b\n\x00\x12\x17\n\x00\x12\x13*\r\n\vExample logJ\x00R\x00",
		},
		{
			name: "disabled",
			configFunc: func() *Config {
				config := createTestConfig()
				config.ClearLogsTimestamp = false
				config.LogFormat = OTLPLogFormat
				return config
			},
			expectedBody: "\n$\n\x00\x12 \n\x00\x12\x1c\t90\x00\x00\x00\x00\x00\x00*\r\n\vExample logJ\x00R\x00",
		},
		{
			name: "default does clear the timestamp",
			configFunc: func() *Config {
				config := createTestConfig()
				// Don't set the clear timestamp config value
				config.LogFormat = OTLPLogFormat
				return config
			},
			expectedBody: "\n\x1b\n\x00\x12\x17\n\x00\x12\x13*\r\n\vExample logJ\x00R\x00",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			expectedRequests := []func(w http.ResponseWriter, req *http.Request){
				func(w http.ResponseWriter, req *http.Request) {
					body := extractBody(t, req)
					assert.Equal(t, tc.expectedBody, body)
				},
			}
			test := prepareExporterTest(t, tc.configFunc(), expectedRequests)

			err := test.exp.pushLogsData(context.Background(), createLogs())
			assert.NoError(t, err)
		})
	}
}

func TestPushLogs_DontRemoveSourceAttributes(t *testing.T) {
	createLogs := func() plog.Logs {
		logs := plog.NewLogs()
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		logsSlice := resourceLogs.ScopeLogs().AppendEmpty().LogRecords()

		logRecords := make([]plog.LogRecord, 2)
		logRecords[0] = plog.NewLogRecord()
		logRecords[0].Body().SetStr("Example log aaaaaaaaaaaaaaaaaaaaaa 1")
		logRecords[0].CopyTo(logsSlice.AppendEmpty())
		logRecords[1] = plog.NewLogRecord()
		logRecords[1].Body().SetStr("Example log aaaaaaaaaaaaaaaaaaaaaa 2")
		logRecords[1].CopyTo(logsSlice.AppendEmpty())

		resourceAttrs := resourceLogs.Resource().Attributes()
		resourceAttrs.PutStr("hostname", "my-host-name")
		resourceAttrs.PutStr("hosttype", "my-host-type")
		resourceAttrs.PutStr("_sourceCategory", "my-source-category")
		resourceAttrs.PutStr("_sourceHost", "my-source-host")
		resourceAttrs.PutStr("_sourceName", "my-source-name")

		return logs
	}

	callbacks := []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log aaaaaaaaaaaaaaaaaaaaaa 1", body)
			assert.Equal(t, "hostname=my-host-name, hosttype=my-host-type", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "my-source-category", req.Header.Get("X-Sumo-Category"))
			assert.Equal(t, "my-source-host", req.Header.Get("X-Sumo-Host"))
			assert.Equal(t, "my-source-name", req.Header.Get("X-Sumo-Name"))
			for k, v := range req.Header {
				t.Logf("request #1 header: %v=%v", k, v)
			}
		},
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log aaaaaaaaaaaaaaaaaaaaaa 2", body)
			assert.Equal(t, "hostname=my-host-name, hosttype=my-host-type", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "my-source-category", req.Header.Get("X-Sumo-Category"))
			assert.Equal(t, "my-source-host", req.Header.Get("X-Sumo-Host"))
			assert.Equal(t, "my-source-name", req.Header.Get("X-Sumo-Name"))
			for k, v := range req.Header {
				t.Logf("request #2 header: %v=%v", k, v)
			}
		},
	}

	config := createTestConfig()
	config.LogFormat = TextFormat
	config.MaxRequestBodySize = 32

	test := prepareExporterTest(t, config, callbacks)
	assert.NoError(t, test.exp.pushLogsData(context.Background(), createLogs()))
}

func TestAllMetricsSuccess(t *testing.T) {
	testcases := []struct {
		name         string
		expectedBody string
		metricFunc   func() (pmetric.Metric, pcommon.Map)
	}{
		{
			name:         "sum",
			expectedBody: `test.metric.data{test="test_value",test2="second_value"} 14500 1605534165000`,
			metricFunc:   exampleIntMetric,
		},
		{
			name: "gauge",
			expectedBody: `gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`,
			metricFunc: exampleIntGaugeMetric,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
				func(w http.ResponseWriter, req *http.Request) {
					body := extractBody(t, req)
					assert.Equal(t, tc.expectedBody, body)
					assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
				},
			})
			test.exp.config.MetricFormat = PrometheusFormat

			metric := metricAndAttributesToPdataMetrics(tc.metricFunc())

			err := test.exp.pushMetricsData(context.Background(), metric)
			assert.NoError(t, err)
		})
	}
}

func TestAllMetricsOTLP(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)

			md, err := (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics([]byte(body))
			assert.NoError(t, err)
			assert.NotNil(t, md)

			//nolint:lll
			expected := "\nf\n/\n\x14\n\x04test\x12\f\n\ntest_value\n\x17\n\x05test2\x12\x0e\n\fsecond_value\x123\n\x00\x12/\n\x10test.metric.data\x1a\x05bytes:\x14\n\x12\x19\x00\x12\x94\v\xd1\x00H\x161\xa48\x00\x00\x00\x00\x00\x00\n\xc2\x01\n\x0e\n\f\n\x03foo\x12\x05\n\x03bar\x12\xaf\x01\n\x00\x12\xaa\x01\n\x11gauge_metric_name*\x94\x01\nH\x19\x80GX\xef\xdb4Q\x161|\x00\x00\x00\x00\x00\x00\x00:\x17\n\vremote_name\x12\b\n\x06156920:\x1b\n\x03url\x12\x14\n\x12http://example_url\nH\x19\x80\x11\xf3*\xdc4Q\x161\xf5\x00\x00\x00\x00\x00\x00\x00:\x17\n\vremote_name\x12\b\n\x06156955:\x1b\n\x03url\x12\x14\n\x12http://another_url"
			assert.Equal(t, expected, body)
			assert.Equal(t, "application/x-protobuf", req.Header.Get("Content-Type"))
		},
	})
	test.exp.config.MetricFormat = OTLPMetricFormat

	metricSum, attrsSum := exampleIntMetric()
	metricGauge, attrsGauge := exampleIntGaugeMetric()
	metrics := metricPairToMetrics(
		metricPair{
			attributes: attrsSum,
			metric:     metricSum,
		},
		metricPair{
			attributes: attrsGauge,
			metric:     metricGauge,
		},
	)

	err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestAllMetricsFailed(t *testing.T) {
	testcases := []struct {
		name          string
		callbacks     []func(w http.ResponseWriter, req *http.Request)
		metricFunc    func() pmetric.Metrics
		expectedError string
	}{
		{
			name: "sent together when metrics under the same resource",
			callbacks: []func(w http.ResponseWriter, req *http.Request){
				func(w http.ResponseWriter, req *http.Request) {
					w.WriteHeader(500)

					body := extractBody(t, req)
					expected := `test.metric.data{test="test_value",test2="second_value"} 14500 1605534165000
gauge_metric_name{test="test_value",test2="second_value",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{test="test_value",test2="second_value",remote_name="156955",url="http://another_url"} 245 1608124662166`
					assert.Equal(t, expected, body)
					assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
				},
			},
			metricFunc: func() pmetric.Metrics {
				metricSum, attrs := exampleIntMetric()
				metricGauge, _ := exampleIntGaugeMetric()
				metrics := metricAndAttrsToPdataMetrics(
					attrs,
					metricSum, metricGauge,
				)
				return metrics
			},
			expectedError: "failed sending data: status: 500 Internal Server Error",
		},
		{
			name: "sent together when metrics under different resources",
			callbacks: []func(w http.ResponseWriter, req *http.Request){
				func(w http.ResponseWriter, req *http.Request) {
					w.WriteHeader(500)

					body := extractBody(t, req)
					expected := `test.metric.data{test="test_value",test2="second_value"} 14500 1605534165000
gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
					assert.Equal(t, expected, body)
					assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
				},
			},
			metricFunc: func() pmetric.Metrics {
				metricSum, attrsSum := exampleIntMetric()
				metricGauge, attrsGauge := exampleIntGaugeMetric()
				metrics := metricPairToMetrics(
					metricPair{
						attributes: attrsSum,
						metric:     metricSum,
					},
					metricPair{
						attributes: attrsGauge,
						metric:     metricGauge,
					},
				)
				return metrics
			},
			expectedError: "failed sending data: status: 500 Internal Server Error",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			test := prepareExporterTest(t, createTestConfig(), tc.callbacks)
			test.exp.config.MetricFormat = PrometheusFormat

			metrics := tc.metricFunc()
			err := test.exp.pushMetricsData(context.Background(), metrics)

			assert.EqualError(t, err, tc.expectedError)

			var partial consumererror.Metrics
			require.True(t, errors.As(err, &partial))
			// TODO fix
			// assert.Equal(t, metrics, partial.GetMetrics())
		})
	}
}

func TestPushMetricsInvalidCompressor(t *testing.T) {
	metrics := metricAndAttributesToPdataMetrics(exampleIntMetric())

	// Expect no requests
	test := prepareExporterTest(t, createTestConfig(), nil)
	test.exp.config.CompressEncoding = "invalid"

	err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.EqualError(t, err, "failed to initialize compressor: invalid format: invalid")
}

func TestLogsTextFormatMetadataFilterWithDroppedAttribute(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "key2=value2", req.Header.Get("X-Sumo-Fields"))
		},
	})
	test.exp.config.LogFormat = TextFormat
	test.exp.config.DropRoutingAttribute = "key1"

	logs := LogRecordsToLogs(exampleLog())
	logs.ResourceLogs().At(0).Resource().Attributes().PutStr("key1", "value1")
	logs.ResourceLogs().At(0).Resource().Attributes().PutStr("key2", "value2")

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.NoError(t, err)
}

func TestMetricsPrometheusFormatMetadataFilter(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `test.metric.data{test="test_value",test2="second_value",key1="value1",key2="value2"} 14500 1605534165000`
			assert.Equal(t, expected, body)
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
	})
	test.exp.config.MetricFormat = PrometheusFormat

	metrics := metricAndAttributesToPdataMetrics(exampleIntMetric())

	attrs := metrics.ResourceMetrics().At(0).Resource().Attributes()
	attrs.PutStr("key1", "value1")
	attrs.PutStr("key2", "value2")

	err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestMetricsPrometheusWithDroppedRoutingAttribute(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `test.metric.data{test="test_value",test2="second_value",key1="value1",key2="value2"} 14500 1605534165000`
			assert.Equal(t, expected, body)
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
	})
	test.exp.config.MetricFormat = PrometheusFormat
	test.exp.config.DropRoutingAttribute = "http_listener_v2_path_custom"

	metrics := metricAndAttributesToPdataMetrics(exampleIntMetric())

	attrs := metrics.ResourceMetrics().At(0).Resource().Attributes()
	attrs.PutStr("key1", "value1")
	attrs.PutStr("key2", "value2")
	attrs.PutStr("http_listener_v2_path_custom", "prometheus.metrics")

	err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestTracesWithDroppedAttribute(t *testing.T) {
	// Prepare data to compare (trace without routing attribute)
	traces := exampleTrace()
	traces.ResourceSpans().At(0).Resource().Attributes().PutStr("key2", "value2")
	tracesMarshaler = ptrace.ProtoMarshaler{}
	bytes, err := tracesMarshaler.MarshalTraces(traces)
	require.NoError(t, err)

	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, string(bytes), body)
		},
	})
	test.exp.config.DropRoutingAttribute = "key1"

	// add routing attribute and check if after marshalling it's different
	traces.ResourceSpans().At(0).Resource().Attributes().PutStr("key1", "value1")
	bytesWithAttribute, err := tracesMarshaler.MarshalTraces(traces)
	require.NoError(t, err)
	require.NotEqual(t, bytes, bytesWithAttribute)

	err = test.exp.pushTracesData(context.Background(), traces)
	assert.NoError(t, err)
}

func Benchmark_ExporterPushLogs(b *testing.B) {
	createConfig := func() *Config {
		config := createDefaultConfig().(*Config)
		config.CompressEncoding = GZIPCompression
		config.MetricFormat = PrometheusFormat
		config.LogFormat = TextFormat
		config.HTTPClientSettings.Auth = nil
		return config
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	}))
	b.Cleanup(func() { testServer.Close() })

	cfg := createConfig()
	cfg.HTTPClientSettings.Endpoint = testServer.URL

	exp, err := initExporter(cfg, createExporterCreateSettings())
	require.NoError(b, err)
	require.NoError(b, exp.start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, exp.shutdown(context.Background()))
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg := sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				err := exp.pushLogsData(context.Background(), LogRecordsToLogs(exampleNLogs(128)))
				if err != nil {
					b.Logf("Failed pushing logs: %v", err)
				}
				wg.Done()
			}()
		}

		wg.Wait()
	}
}

func TestSendEmptyLogsOTLP(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		// No request is sent
	})

	logs := plog.NewLogs()

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.NoError(t, err)
}

func TestSendEmptyMetricsOTLP(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		// No request is sent
	})
	test.exp.config.MetricFormat = OTLPMetricFormat

	metrics := metricPairToMetrics()

	err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestSendEmptyTraces(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		// No request is sent
	})

	traces := ptrace.NewTraces()

	err := test.exp.pushTracesData(context.Background(), traces)
	assert.NoError(t, err)
}

func TestGetSignalUrl(t *testing.T) {
	testCases := []struct {
		description  string
		signalType   component.Type
		cfg          Config
		endpointUrl  string
		expected     string
		errorMessage string
	}{
		{
			description: "no change if log format not otlp",
			signalType:  component.DataTypeLogs,
			cfg:         Config{LogFormat: TextFormat},
			endpointUrl: "http://localhost",
			expected:    "http://localhost",
		},
		{
			description: "no change if metric format not otlp",
			signalType:  component.DataTypeMetrics,
			cfg:         Config{MetricFormat: PrometheusFormat},
			endpointUrl: "http://localhost",
			expected:    "http://localhost",
		},
		{
			description: "always add suffix for traces if not present",
			signalType:  component.DataTypeTraces,
			endpointUrl: "http://localhost",
			expected:    "http://localhost/v1/traces",
		},
		{
			description: "always add suffix for logs if not present",
			signalType:  component.DataTypeLogs,
			cfg:         Config{LogFormat: OTLPLogFormat},
			endpointUrl: "http://localhost",
			expected:    "http://localhost/v1/logs",
		},
		{
			description: "always add suffix for metrics if not present",
			signalType:  component.DataTypeMetrics,
			cfg:         Config{MetricFormat: OTLPMetricFormat},
			endpointUrl: "http://localhost",
			expected:    "http://localhost/v1/metrics",
		},
		{
			description: "no change if suffix already present",
			signalType:  component.DataTypeTraces,
			endpointUrl: "http://localhost/v1/traces",
			expected:    "http://localhost/v1/traces",
		},
		{
			description:  "error if url invalid",
			signalType:   component.DataTypeTraces,
			endpointUrl:  ":",
			errorMessage: `parse ":": missing protocol scheme`,
		},
		{
			description:  "error if signal type is unknown",
			signalType:   "unknown",
			endpointUrl:  "http://localhost",
			errorMessage: `unknown signal type: unknown`,
		},
	}
	for _, tC := range testCases {
		testCase := tC
		t.Run(tC.description, func(t *testing.T) {
			actual, err := getSignalURL(&testCase.cfg, testCase.endpointUrl, testCase.signalType)
			if testCase.errorMessage != "" {
				require.Error(t, err)
				require.EqualError(t, err, testCase.errorMessage)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, testCase.expected, actual)
		})
	}
}
