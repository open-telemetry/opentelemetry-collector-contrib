// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type exporterTest struct {
	srv        *httptest.Server
	exp        *sumologicexporter
	reqCounter *int32
}

func createTestConfig() *Config {
	config := createDefaultConfig().(*Config)
	config.ClientConfig.Compression = configcompression.Type(NoCompression)
	config.LogFormat = TextFormat
	config.MaxRequestBodySize = 20_971_520
	config.MetricFormat = OTLPMetricFormat
	return config
}

func logRecordsToLogs(records []plog.LogRecord) plog.Logs {
	logs := plog.NewLogs()
	logsSlice := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	for _, record := range records {
		tgt := logsSlice.AppendEmpty()
		record.CopyTo(tgt)
	}

	return logs
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

	cfg.ClientConfig.Endpoint = testServer.URL
	cfg.ClientConfig.Auth = nil

	exp, err := initExporter(cfg, createExporterCreateSettings().TelemetrySettings)
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
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		ClientConfig: confighttp.ClientConfig{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
	}, componenttest.NewNopTelemetrySettings())
	assert.NoError(t, err)
}

func TestAllSuccess(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "", req.Header.Get("X-Sumo-Fields"))
		},
	})
	defer func() { test.srv.Close() }()

	logs := logRecordsToLogs(exampleLog())

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.NoError(t, err)
}

func TestResourceMerge(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "key1=original_value, key2=additional_value", req.Header.Get("X-Sumo-Fields"))
		},
	})
	defer func() { test.srv.Close() }()

	f, err := newFilter([]string{`key\d`})
	require.NoError(t, err)
	test.exp.filter = f

	logs := logRecordsToLogs(exampleLog())
	logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("key1", "original_value")
	logs.ResourceLogs().At(0).Resource().Attributes().PutStr("key1", "overwrite_value")
	logs.ResourceLogs().At(0).Resource().Attributes().PutStr("key2", "additional_value")

	err = test.exp.pushLogsData(context.Background(), logs)
	assert.NoError(t, err)
}

func TestAllFailed(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			assert.Equal(t, "Example log\nAnother example log", body)
			assert.Equal(t, "", req.Header.Get("X-Sumo-Fields"))
		},
	})
	defer func() { test.srv.Close() }()

	logs := logRecordsToLogs(exampleTwoLogs())

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error")

	var partial consumererror.Logs
	require.True(t, errors.As(err, &partial))
	assert.Equal(t, logs, partial.Data())
}

func TestPartiallyFailed(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
			assert.Equal(t, "key1=value1, key2=value2", req.Header.Get("X-Sumo-Fields"))
		},
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Another example log", body)
			assert.Equal(t, "key3=value3, key4=value4", req.Header.Get("X-Sumo-Fields"))
		},
	})
	defer func() { test.srv.Close() }()

	f, err := newFilter([]string{`key\d`})
	require.NoError(t, err)
	test.exp.filter = f

	records := exampleTwoDifferentLogs()
	logs := logRecordsToLogs(records)
	expected := logRecordsToLogs(records[:1])

	err = test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error")

	var partial consumererror.Logs
	require.True(t, errors.As(err, &partial))
	assert.Equal(t, expected, partial.Data())
}

func TestInvalidSourceFormats(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		ClientConfig: confighttp.ClientConfig{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
		MetadataAttributes: []string{"[a-z"},
	}, componenttest.NewNopTelemetrySettings())
	assert.EqualError(t, err, "error parsing regexp: missing closing ]: `[a-z`")
}

func TestInvalidHTTPCLient(t *testing.T) {
	se, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "test_endpoint",
			CustomRoundTripper: func(_ http.RoundTripper) (http.RoundTripper, error) {
				return nil, errors.New("roundTripperException")
			},
		},
	}, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, se)

	err = se.start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, "failed to create HTTP Client: roundTripperException")
}

func TestPushInvalidCompressor(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "", req.Header.Get("X-Sumo-Fields"))
		},
	})
	defer func() { test.srv.Close() }()

	logs := logRecordsToLogs(exampleLog())

	test.exp.config.CompressEncoding = "invalid"

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed to initialize compressor: invalid format: invalid")
}

func TestPushFailedBatch(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
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
	defer func() { test.srv.Close() }()

	logs := logRecordsToLogs(exampleLog())
	logs.ResourceLogs().EnsureCapacity(maxBufferSize + 1)
	log := logs.ResourceLogs().At(0)

	for i := 0; i < maxBufferSize; i++ {
		tgt := logs.ResourceLogs().AppendEmpty()
		log.CopyTo(tgt)
	}

	err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error")
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
				metrics.MarkReadOnly()
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

func TestGetSignalUrl(t *testing.T) {
	testCases := []struct {
		description  string
		signalType   component.Type
		cfg          Config
		endpointURL  string
		expected     string
		errorMessage string
	}{
		{
			description: "no change if log format not otlp",
			signalType:  component.DataTypeLogs,
			cfg:         Config{LogFormat: TextFormat},
			endpointURL: "http://localhost",
			expected:    "http://localhost",
		},
		{
			description: "no change if metric format not otlp",
			signalType:  component.DataTypeMetrics,
			cfg:         Config{MetricFormat: PrometheusFormat},
			endpointURL: "http://localhost",
			expected:    "http://localhost",
		},
		{
			description: "always add suffix for traces if not present",
			signalType:  component.DataTypeTraces,
			endpointURL: "http://localhost",
			expected:    "http://localhost/v1/traces",
		},
		{
			description: "no change if suffix already present",
			signalType:  component.DataTypeTraces,
			endpointURL: "http://localhost/v1/traces",
			expected:    "http://localhost/v1/traces",
		},
		{
			description:  "error if url invalid",
			signalType:   component.DataTypeTraces,
			endpointURL:  ":",
			errorMessage: `parse ":": missing protocol scheme`,
		},
		{
			description:  "error if signal type is unknown",
			signalType:   component.MustNewType("unknown"),
			endpointURL:  "http://localhost",
			errorMessage: `unknown signal type: unknown`,
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
