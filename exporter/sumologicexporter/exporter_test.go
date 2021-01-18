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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func LogRecordsToLogs(records []pdata.LogRecord) pdata.Logs {
	logs := pdata.NewLogs()
	logs.ResourceLogs().Resize(1)
	logs.ResourceLogs().At(0).InstrumentationLibraryLogs().Resize(1)
	for _, record := range records {
		logs.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().Append(record)
	}

	return logs
}

func TestInitExporter(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
	})
	assert.NoError(t, err)
}

func TestInitExporterInvalidLogFormat(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "test_format",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
	})

	assert.EqualError(t, err, "unexpected log format: test_format")
}

func TestInitExporterInvalidMetricFormat(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:    "json",
		MetricFormat: "test_format",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
		CompressEncoding: "gzip",
	})

	assert.EqualError(t, err, "unexpected metric format: test_format")
}

func TestInitExporterInvalidCompressEncoding(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "test_format",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
	})

	assert.EqualError(t, err, "unexpected compression encoding: test_format")
}

func TestInitExporterInvalidEndpoint(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: defaultTimeout,
		},
	})

	assert.EqualError(t, err, "endpoint is not set")
}

func TestAllSuccess(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "", req.Header.Get("X-Sumo-Fields"))
		},
	})
	defer func() { test.srv.Close() }()

	logs := LogRecordsToLogs(exampleLog())

	_, err := test.exp.pushLogsData(context.Background(), logs)
	assert.NoError(t, err)
}

func TestResourceMerge(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "key1=original_value, key2=additional_value", req.Header.Get("X-Sumo-Fields"))
		},
	})
	defer func() { test.srv.Close() }()

	f, err := newFilter([]string{`key\d`})
	require.NoError(t, err)
	test.exp.filter = f

	logs := LogRecordsToLogs(exampleLog())
	logs.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes().InsertString("key1", "original_value")
	logs.ResourceLogs().At(0).Resource().Attributes().InsertString("key1", "overwrite_value")
	logs.ResourceLogs().At(0).Resource().Attributes().InsertString("key2", "additional_value")

	_, err = test.exp.pushLogsData(context.Background(), logs)
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

	logs := LogRecordsToLogs(exampleTwoLogs())

	dropped, err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, 2, dropped)

	partial, ok := err.(consumererror.PartialError)
	require.True(t, ok)
	assert.Equal(t, logs, partial.GetLogs())
}

func TestPartiallyFailed(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
			assert.Equal(t, "key1=value1, key2=value2", req.Header.Get("X-Sumo-Fields"))
		},
		func(w http.ResponseWriter, req *http.Request) {
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
	logs := LogRecordsToLogs(records)
	expected := LogRecordsToLogs(records[:1])

	dropped, err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, 1, dropped)

	partial, ok := err.(consumererror.PartialError)
	require.True(t, ok)
	assert.Equal(t, expected, partial.GetLogs())
}

func TestInvalidSourceFormats(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
		MetadataAttributes: []string{"[a-z"},
	})
	assert.EqualError(t, err, "error parsing regexp: missing closing ]: `[a-z`")
}

func TestInvalidHTTPCLient(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "test_endpoint",
			CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
				return nil, errors.New("roundTripperException")
			},
		},
	})
	assert.EqualError(t, err, "failed to create HTTP Client: roundTripperException")
}

func TestPushInvalidCompressor(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "", req.Header.Get("X-Sumo-Fields"))
		},
	})
	defer func() { test.srv.Close() }()

	logs := LogRecordsToLogs(exampleLog())

	test.exp.config.CompressEncoding = "invalid"

	_, err := test.exp.pushLogsData(context.Background(), logs)
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

	logs := LogRecordsToLogs(exampleLog())
	logs.ResourceLogs().Resize(maxBufferSize + 1)
	log := logs.ResourceLogs().At(0)

	for i := 0; i < maxBufferSize; i++ {
		logs.ResourceLogs().Append(log)
	}

	count, err := test.exp.pushLogsData(context.Background(), logs)
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, maxBufferSize, count)
}

func TestAllMetricsSuccess(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `test_metric_data{test="test_value",test2="second_value"} 14500 1605534165000
gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()
	test.exp.config.MetricFormat = PrometheusFormat

	metrics := metricPairToMetrics([]metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	})

	_, err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestAllMetricsFailed(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			expected := `test_metric_data{test="test_value",test2="second_value"} 14500 1605534165000
gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()
	test.exp.config.MetricFormat = PrometheusFormat

	metrics := metricPairToMetrics([]metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	})

	dropped, err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, 2, dropped)

	partial, ok := err.(consumererror.PartialError)
	require.True(t, ok)
	assert.Equal(t, metrics, partial.GetMetrics())
}

func TestMetricsPartiallyFailed(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			expected := `test_metric_data{test="test_value",test2="second_value"} 14500 1605534165000`
			assert.Equal(t, expected, body)
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `gauge_metric_name{foo="bar",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()
	test.exp.config.MetricFormat = PrometheusFormat
	test.exp.config.MaxRequestBodySize = 1

	records := []metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	}
	metrics := metricPairToMetrics(records)
	expected := metricPairToMetrics(records[:1])

	dropped, err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, 1, dropped)

	partial, ok := err.(consumererror.PartialError)
	require.True(t, ok)
	assert.Equal(t, expected, partial.GetMetrics())
}

func TestPushMetricsInvalidCompressor(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, `Example log`, body)
			assert.Equal(t, "", req.Header.Get("X-Sumo-Fields"))
		},
	})
	defer func() { test.srv.Close() }()

	metrics := metricPairToMetrics([]metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	})

	test.exp.config.CompressEncoding = "invalid"

	_, err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.EqualError(t, err, "failed to initialize compressor: invalid format: invalid")
}

func TestMetricsDifferentMetadata(t *testing.T) {
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)

			body := extractBody(t, req)
			expected := `test_metric_data{test="test_value",test2="second_value",key1="value1"} 14500 1605534165000`
			assert.Equal(t, expected, body)
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
		func(w http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `gauge_metric_name{foo="bar",key2="value2",remote_name="156920",url="http://example_url"} 124 1608124661166
gauge_metric_name{foo="bar",key2="value2",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
	})
	defer func() { test.srv.Close() }()
	test.exp.config.MetricFormat = PrometheusFormat
	test.exp.config.MaxRequestBodySize = 1

	f, err := newFilter([]string{`key\d`})
	require.NoError(t, err)
	test.exp.filter = f

	records := []metricPair{
		exampleIntMetric(),
		exampleIntGaugeMetric(),
	}

	records[0].attributes.InsertString("key1", "value1")
	records[1].attributes.InsertString("key2", "value2")

	metrics := metricPairToMetrics(records)
	expected := metricPairToMetrics(records[:1])

	dropped, err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, 1, dropped)

	partial, ok := err.(consumererror.PartialError)
	require.True(t, ok)
	assert.Equal(t, expected, partial.GetMetrics())
}

func TestPushMetricsFailedBatch(t *testing.T) {
	t.Skip("Skip test due to prometheus format complexity. Execution can take over 30s")
	test := prepareSenderTest(t, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(500)
			body := extractBody(t, req)

			expected := fmt.Sprintf(
				"%s%s",
				strings.Repeat("test_metric_data{test=\"test_value\",test2=\"second_value\"} 14500 1605534165000\n", maxBufferSize-1),
				`test_metric_data{test="test_value",test2="second_value"} 14500 1605534165000`,
			)

			assert.Equal(t, expected, body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(200)
			body := extractBody(t, req)

			assert.Equal(t, `test_metric_data{test="test_value",test2="second_value"} 14500 1605534165000`, body)
		},
	})
	defer func() { test.srv.Close() }()
	test.exp.config.MetricFormat = PrometheusFormat
	test.exp.config.MaxRequestBodySize = 1024 * 1024 * 1024 * 1024

	metrics := metricPairToMetrics([]metricPair{exampleIntMetric()})
	metrics.ResourceMetrics().Resize(maxBufferSize + 1)
	metric := metrics.ResourceMetrics().At(0)

	for i := 0; i < maxBufferSize; i++ {
		metrics.ResourceMetrics().Append(metric)
	}

	count, err := test.exp.pushMetricsData(context.Background(), metrics)
	assert.EqualError(t, err, "error during sending data: 500 Internal Server Error")
	assert.Equal(t, maxBufferSize, count)
}
