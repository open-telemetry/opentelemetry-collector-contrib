// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter/internal/metadata"
)

func logRecordsToLogs(records []plog.LogRecord) plog.Logs {
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
	config.Compression = NoCompression
	config.LogFormat = TextFormat
	config.MaxRequestBodySize = 20_971_520
	config.MetricFormat = OTLPMetricFormat
	return config
}

// prepareExporterTest prepares an exporter test object using provided config
// and a slice of callbacks to be called for subsequent requests coming being
// sent to the server.
// The enclosed *httptest.Server is automatically closed on test cleanup.
func prepareExporterTest(t *testing.T, cfg *Config, cb []func(w http.ResponseWriter, req *http.Request)) *exporterTest {
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

	cfg.Endpoint = testServer.URL
	cfg.Auth = nil

	exp, err := initExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)

	require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))

	return &exporterTest{
		srv:        testServer,
		exp:        exp,
		reqCounter: &reqCounter,
	}
}

func TestAllSuccess(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Empty(t, req.Header.Get("X-Sumo-Fields"))
		},
	})

	logs := logRecordsToLogs(exampleLog())
	logs.MarkReadOnly()

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
				return config
			},
			callbacks: []func(w http.ResponseWriter, req *http.Request){
				func(_ http.ResponseWriter, req *http.Request) {
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

				logs := logRecordsToLogs(buffer)
				logs.ResourceLogs().At(0).Resource().Attributes().PutStr("res_attr1", "1")
				logs.ResourceLogs().At(0).Resource().Attributes().PutStr("res_attr2", "2")
				logs.MarkReadOnly()
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
			w.WriteHeader(http.StatusInternalServerError)

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

	logs.MarkReadOnly()

	logsExpected := plog.NewLogs()
	logsSlice.CopyTo(logsExpected.ResourceLogs().AppendEmpty())

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error")

	var partial consumererror.Logs
	require.ErrorAs(t, err, &partial)
	assert.Equal(t, logsExpected, partial.Data())
}

func TestPartiallyFailed(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
			// No resource attributes for those logs hence no fields
			assert.Empty(t, req.Header.Get("X-Sumo-Fields"))
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)

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

	logs.MarkReadOnly()

	logsExpected := plog.NewLogs()
	logsSlice2.CopyTo(logsExpected.ResourceLogs().AppendEmpty())

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error")

	var partial consumererror.Logs
	require.ErrorAs(t, err, &partial)
	assert.Equal(t, logsExpected, partial.Data())
}

func TestInvalidHTTPClient(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "test_endpoint"
	clientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			MinVersion: "invalid",
		},
	}
	exp, err := initExporter(&Config{
		ClientConfig: clientConfig,
	}, exportertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)

	assert.EqualError(t,
		exp.start(context.Background(), componenttest.NewNopHost()),
		"failed to create HTTP Client: failed to load TLS config: invalid TLS min_version: unsupported TLS version: \"invalid\"",
	)
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
		logs.MarkReadOnly()

		return logs
	}

	callbacks := []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
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
		func(_ http.ResponseWriter, req *http.Request) {
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
				func(_ http.ResponseWriter, req *http.Request) {
					body := extractBody(t, req)
					assert.Equal(t, tc.expectedBody, body)
					assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
				},
			})
			test.exp.config.MetricFormat = PrometheusFormat

			metric := metricAndAttributesToPdataMetrics(tc.metricFunc())
			metric.MarkReadOnly()

			err := test.exp.pushMetricsData(context.Background(), metric)
			assert.NoError(t, err)
		})
	}
}

func TestAllMetricsOTLP(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
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
					w.WriteHeader(http.StatusInternalServerError)

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
				metrics.MarkReadOnly()
				return metrics
			},
			expectedError: "failed sending data: status: 500 Internal Server Error",
		},
		{
			name: "sent together when metrics under different resources",
			callbacks: []func(w http.ResponseWriter, req *http.Request){
				func(w http.ResponseWriter, req *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)

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
			require.ErrorAs(t, err, &partial)
			// TODO fix
			// assert.Equal(t, metrics, partial.GetMetrics())
		})
	}
}

func TestMetricsPrometheusFormatMetadataFilter(t *testing.T) {
	test := prepareExporterTest(t, createTestConfig(), []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
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

	metrics.MarkReadOnly()

	err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.NoError(t, err)
}

func Benchmark_ExporterPushLogs(b *testing.B) {
	createConfig := func() *Config {
		config := createDefaultConfig().(*Config)
		config.MetricFormat = PrometheusFormat
		config.LogFormat = TextFormat
		config.Auth = nil
		config.Compression = configcompression.TypeGzip
		return config
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
	}))
	b.Cleanup(func() { testServer.Close() })

	cfg := createConfig()
	cfg.Endpoint = testServer.URL

	exp, err := initExporter(cfg, exportertest.NewNopSettings(metadata.Type))
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
				logs := logRecordsToLogs(exampleNLogs(128))
				logs.MarkReadOnly()
				err := exp.pushLogsData(context.Background(), logs)
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
	logs.MarkReadOnly()

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

func TestGetSignalURL(t *testing.T) {
	testCases := []struct {
		description  string
		signalType   pipeline.Signal
		cfg          Config
		endpointURL  string
		expected     string
		errorMessage string
	}{
		{
			description: "no change if log format not otlp",
			signalType:  pipeline.SignalLogs,
			cfg:         Config{LogFormat: TextFormat},
			endpointURL: "http://localhost",
			expected:    "http://localhost",
		},
		{
			description: "no change if metric format not otlp",
			signalType:  pipeline.SignalMetrics,
			cfg:         Config{MetricFormat: PrometheusFormat},
			endpointURL: "http://localhost",
			expected:    "http://localhost",
		},
		{
			description: "always add suffix for traces if not present",
			signalType:  pipeline.SignalTraces,
			endpointURL: "http://localhost",
			expected:    "http://localhost/v1/traces",
		},
		{
			description: "always add suffix for logs if not present",
			signalType:  pipeline.SignalLogs,
			cfg:         Config{LogFormat: OTLPLogFormat},
			endpointURL: "http://localhost",
			expected:    "http://localhost/v1/logs",
		},
		{
			description: "always add suffix for metrics if not present",
			signalType:  pipeline.SignalMetrics,
			cfg:         Config{MetricFormat: OTLPMetricFormat},
			endpointURL: "http://localhost",
			expected:    "http://localhost/v1/metrics",
		},
		{
			description: "no change if suffix already present",
			signalType:  pipeline.SignalTraces,
			endpointURL: "http://localhost/v1/traces",
			expected:    "http://localhost/v1/traces",
		},
		{
			description:  "error if url invalid",
			signalType:   pipeline.SignalTraces,
			endpointURL:  ":",
			errorMessage: `parse ":": missing protocol scheme`,
		},
		{
			description:  "error if signal type is unknown",
			signalType:   pipeline.Signal{},
			endpointURL:  "http://localhost",
			errorMessage: `unknown signal type: `,
		},
	}
	for _, tC := range testCases {
		testCase := tC
		t.Run(tC.description, func(t *testing.T) {
			actual, err := getSignalURL(&testCase.cfg, testCase.endpointURL, testCase.signalType)
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

func TestNChars(t *testing.T) {
	s := nchars('*', 10)
	require.Equal(t, "**********", s)
	s = nchars(' ', 2)
	require.Equal(t, "  ", s)
}

func TestSanitizeURL(t *testing.T) {
	testCases := []struct {
		description string
		urlString   string
		expected    string
	}{
		{
			description: "sanitized logs url",
			urlString:   "https://collectors.au.sumologic.com/receiver/v1/otlp/xxxxx/v1/logs",
			expected:    "https://collectors.au.sumologic.com/receiver/v1/otlp/*****/v1/logs",
		},
		{
			description: "sanitized metrics url",
			urlString:   "https://collectors.au.sumologic.com/receiver/v1/otlp/xxxx==/v1/metrics",
			expected:    "https://collectors.au.sumologic.com/receiver/v1/otlp/******/v1/metrics",
		},
		{
			description: "sanitized traces url",
			urlString:   "https://collectors.au.sumologic.com/receiver/v1/otlp/xxxx==/v1/traces",
			expected:    "https://collectors.au.sumologic.com/receiver/v1/otlp/******/v1/traces",
		},
		{
			description: "no sanitization required",
			urlString:   "https://collectors.au.sumologic.com/receiver/v1/xxxx==/v1/traces",
			expected:    "https://collectors.au.sumologic.com/receiver/v1/xxxx==/v1/traces",
		},
		{
			description: "no sanitization required with otlp/ appearing after v1/",
			urlString:   "https://collectors.au.sumologic.com/receiver/v1/v1/xxxx==/otlp/traces",
			expected:    "https://collectors.au.sumologic.com/receiver/v1/v1/xxxx==/otlp/traces",
		},
	}
	for _, tC := range testCases {
		testCase := tC
		t.Run(tC.description, func(t *testing.T) {
			actual := sanitizeURL(testCase.urlString)
			require.Equal(t, testCase.expected, actual)
		})
	}
}
