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

package splunkhecexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

var requestTimeRegex = regexp.MustCompile(`time":(\d+)`)

type testRoundTripper func(req *http.Request) *http.Response

func (t testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t(req), nil
}

func newTestClient(respCode int, respBody string) (*http.Client, *[]http.Header) {
	return newTestClientWithPresetResponses([]int{respCode}, []string{respBody})
}

func newTestClientWithPresetResponses(codes []int, bodies []string) (*http.Client, *[]http.Header) {
	index := 0
	headers := make([]http.Header, 0)

	return &http.Client{
		Transport: testRoundTripper(func(req *http.Request) *http.Response {
			code := codes[index%len(codes)]
			body := bodies[index%len(bodies)]
			index++

			headers = append(headers, req.Header)

			return &http.Response{
				StatusCode: code,
				Body:       ioutil.NopCloser(bytes.NewBufferString(body)),
				Header:     make(http.Header),
			}
		}),
	}, &headers
}

func createMetricsData(numberOfDataPoints int) pmetric.Metrics {

	doubleVal := 1234.5678
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().InsertString("k0", "v0")
	rm.Resource().Attributes().InsertString("k1", "v1")

	for i := 0; i < numberOfDataPoints; i++ {
		tsUnix := time.Unix(int64(i), int64(i)*time.Millisecond.Nanoseconds())

		ilm := rm.ScopeMetrics().AppendEmpty()
		metric := ilm.Metrics().AppendEmpty()
		metric.SetName("gauge_double_with_dims")
		metric.SetDataType(pmetric.MetricDataTypeGauge)
		doublePt := metric.Gauge().DataPoints().AppendEmpty()
		doublePt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
		doublePt.SetDoubleVal(doubleVal)
		doublePt.Attributes().InsertString("k/n0", "vn0")
		doublePt.Attributes().InsertString("k/n1", "vn1")
		doublePt.Attributes().InsertString("k/r0", "vr0")
		doublePt.Attributes().InsertString("k/r1", "vr1")
	}

	return metrics
}

func createTraceData(numberOfTraces int) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().InsertString("resource", "R1")
	ils := rs.ScopeSpans().AppendEmpty()
	ils.Spans().EnsureCapacity(numberOfTraces)
	for i := 0; i < numberOfTraces; i++ {
		span := ils.Spans().AppendEmpty()
		span.SetName("root")
		span.SetStartTimestamp(pcommon.Timestamp((i + 1) * 1e9))
		span.SetEndTimestamp(pcommon.Timestamp((i + 2) * 1e9))
		span.SetTraceID(pcommon.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
		span.SetSpanID(pcommon.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
		span.SetTraceState("foo")
		if i%2 == 0 {
			span.SetParentSpanID(pcommon.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
			span.Status().SetCode(ptrace.StatusCodeOk)
			span.Status().SetMessage("ok")
		}
	}

	return traces
}

func createLogData(numResources int, numLibraries int, numRecords int) plog.Logs {
	return createLogDataWithCustomLibraries(numResources, make([]string, numLibraries), repeat(numRecords, numLibraries))
}

func repeat(what int, times int) []int {
	var result = make([]int, times)
	for i := range result {
		result[i] = what
	}
	return result
}

func createLogDataWithCustomLibraries(numResources int, libraries []string, numRecords []int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().EnsureCapacity(numResources)
	for i := 0; i < numResources; i++ {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.ScopeLogs().EnsureCapacity(len(libraries))
		for j := 0; j < len(libraries); j++ {
			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName(libraries[j])
			sl.LogRecords().EnsureCapacity(numRecords[j])
			for k := 0; k < numRecords[j]; k++ {
				ts := pcommon.Timestamp(int64(k) * time.Millisecond.Nanoseconds())
				logRecord := sl.LogRecords().AppendEmpty()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(splunk.DefaultNameLabel, fmt.Sprintf("%d_%d_%d", i, j, k))
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(splunk.DefaultIndexLabel, "myindex")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
			}
		}
	}

	return logs
}

type receivedRequest struct {
	body    []byte
	headers http.Header
}

type CapturingData struct {
	testing          *testing.T
	receivedRequest  chan receivedRequest
	statusCode       int
	checkCompression bool
}

func (c *CapturingData) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)

	if c.checkCompression {
		if len(body) > minCompressionLen && r.Header.Get("Content-Encoding") != "gzip" {
			c.testing.Fatal("No compression")
		}
	}

	if err != nil {
		panic(err)
	}
	go func() {
		c.receivedRequest <- receivedRequest{body, r.Header}
	}()
	w.WriteHeader(c.statusCode)
}

func runMetricsExport(cfg *Config, metrics pmetric.Metrics, t *testing.T) ([]receivedRequest, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	factory := NewFactory()
	cfg.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.Token = "1234-1234"

	rr := make(chan receivedRequest)
	capture := CapturingData{testing: t, receivedRequest: rr, statusCode: 200, checkCompression: !cfg.DisableCompression}
	s := &http.Server{
		Handler: &capture,
	}
	go func() {
		panic(s.Serve(listener))
	}()

	params := componenttest.NewNopExporterCreateSettings()
	exporter, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, exporter.Shutdown(context.Background()))
	}()

	err = exporter.ConsumeMetrics(context.Background(), metrics)
	assert.NoError(t, err)
	var requests []receivedRequest
	for {
		select {
		case request := <-rr:
			requests = append(requests, request)
		case <-time.After(1 * time.Second):
			if len(requests) == 0 {
				err = errors.New("timeout")
			}
			return requests, err
		}
	}
}

func runTraceExport(testConfig *Config, traces ptrace.Traces, t *testing.T) ([]receivedRequest, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.DisableCompression = testConfig.DisableCompression
	cfg.MaxContentLengthTraces = testConfig.MaxContentLengthTraces
	cfg.Token = "1234-1234"

	rr := make(chan receivedRequest)
	capture := CapturingData{testing: t, receivedRequest: rr, statusCode: 200, checkCompression: !cfg.DisableCompression}
	s := &http.Server{
		Handler: &capture,
	}
	go func() {
		panic(s.Serve(listener))
	}()

	params := componenttest.NewNopExporterCreateSettings()
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, exporter.Shutdown(context.Background()))
	}()

	err = exporter.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)
	var requests []receivedRequest
	for {
		select {
		case request := <-rr:
			requests = append(requests, request)
		case <-time.After(1 * time.Second):
			if len(requests) == 0 {
				err = errors.New("timeout")
			}

			// sort the requests according to the traces we received, reordering them so we can assert on their size.
			sort.Slice(requests, func(i, j int) bool {
				imatch := requestTimeRegex.FindSubmatch(requests[i].body)
				jmatch := requestTimeRegex.FindSubmatch(requests[j].body)
				// no matches mean it's compressed, just leave as is
				if len(imatch) == 0 {
					return i < j
				}
				return string(imatch[1]) <= string(jmatch[1])
			})

			return requests, err
		}
	}
}

func runLogExport(cfg *Config, ld plog.Logs, t *testing.T) ([]receivedRequest, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	cfg.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.Token = "1234-1234"

	rr := make(chan receivedRequest)
	capture := CapturingData{testing: t, receivedRequest: rr, statusCode: 200, checkCompression: !cfg.DisableCompression}
	s := &http.Server{
		Handler: &capture,
	}
	go func() {
		panic(s.Serve(listener))
	}()

	params := componenttest.NewNopExporterCreateSettings()
	exporter, err := NewFactory().CreateLogsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, exporter.Shutdown(context.Background()))
	}()

	err = exporter.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)

	var requests []receivedRequest
	for {
		select {
		case request := <-rr:
			requests = append(requests, request)
		case <-time.After(1 * time.Second):
			if len(requests) == 0 {
				err = errors.New("timeout")
			}
			return requests, err
		}
	}
}

func TestReceiveTracesBatches(t *testing.T) {
	type wantType struct {
		batches    [][]string
		numBatches int
		compressed bool
	}

	// The test cases depend on the constant minCompressionLen = 1500.
	// If the constant changed, the test cases with want.compressed=true must be updated.
	require.Equal(t, minCompressionLen, 1500)

	tests := []struct {
		name   string
		conf   *Config
		traces ptrace.Traces
		want   wantType
	}{
		{
			name:   "all trace events in payload when max content length unknown (configured max content length 0)",
			traces: createTraceData(4),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthTraces = 0
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"start_time":1`,
						`"start_time":2`,
						`start_time":3`,
						`start_time":4`},
				},
				numBatches: 1,
			},
		},
		{
			name:   "1 trace event per payload (configured max content length is same as event size)",
			traces: createTraceData(4),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthTraces = 320
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"start_time":1`},
					{`"start_time":2`},
					{`"start_time":3`},
					{`"start_time":4`},
				},
				numBatches: 4,
			},
		},
		{
			name:   "2 trace events per payload (configured max content length is twice event size)",
			traces: createTraceData(4),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthTraces = 640
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"start_time":1`, `"start_time":2`},
					{`"start_time":3`, `"start_time":4`},
				},
				numBatches: 2,
			},
		},
		{
			name:   "1 compressed batch of 2037 bytes, make sure the event size is more than minCompressionLen=1500 to trigger compression",
			traces: createTraceData(10),
			conf: func() *Config {
				return NewFactory().CreateDefaultConfig().(*Config)
			}(),
			want: wantType{
				batches: [][]string{
					{`"start_time":1`, `"start_time":2`, `"start_time":3`, `"start_time":4`, `"start_time":7`, `"start_time":8`, `"start_time":9`},
				},
				numBatches: 1,
				compressed: true,
			},
		},
		{
			name:   "2 compressed batches - 1832 bytes each, make sure the log size is more than minCompressionLen=1500 to trigger compression",
			traces: createTraceData(22),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthTraces = 3520
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"start_time":1`, `"start_time":2`, `"start_time":5`, `"start_time":6`, `"start_time":7`, `"start_time":8`, `"start_time":9`, `"start_time":10`, `"start_time":11`},
					{`"start_time":15`, `"start_time":16`, `"start_time":17`, `"start_time":18`, `"start_time":19`, `"start_time":20`, `"start_time":21`},
				},
				numBatches: 2,
				compressed: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runTraceExport(test.conf, test.traces, t)

			require.NoError(t, err)
			require.Len(t, got, test.want.numBatches)

			for i := 0; i < test.want.numBatches; i++ {
				require.NotZero(t, got[i])
				if test.want.compressed {
					validateCompressedContains(t, test.want.batches[i], got[i].body)
				} else {
					for _, expected := range test.want.batches[i] {
						assert.Contains(t, string(got[i].body), expected)
					}
				}
			}
		})
	}
}

func TestReceiveLogs(t *testing.T) {
	type wantType struct {
		batches    [][]string
		numBatches int
		compressed bool
	}

	// The test cases depend on the constant minCompressionLen = 1500.
	// If the constant changed, the test cases with want.compressed=true must be updated.
	require.Equal(t, minCompressionLen, 1500)

	tests := []struct {
		name string
		conf *Config
		logs plog.Logs
		want wantType
	}{
		{
			name: "all log events in payload when max content length unknown (configured max content length 0)",
			logs: createLogData(1, 1, 4),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = 0
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`,
						`"otel.log.name":"0_0_1"`,
						`otel.log.name":"0_0_2`,
						`otel.log.name":"0_0_3`},
				},
				numBatches: 1,
			},
		},
		{
			name: "1 log event per payload (configured max content length is same as event size)",
			logs: createLogData(1, 1, 4),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = 300
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`},
					{`"otel.log.name":"0_0_1"`},
					{`"otel.log.name":"0_0_2"`},
					{`"otel.log.name":"0_0_3"`},
				},
				numBatches: 4,
			},
		},
		{
			name: "2 log events per payload (configured max content length is twice event size)",
			logs: createLogData(1, 1, 4),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = 448
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`, `"otel.log.name":"0_0_1"`},
					{`"otel.log.name":"0_0_2"`, `"otel.log.name":"0_0_3"`},
				},
				numBatches: 2,
			},
		},
		{
			name: "1 compressed batch of 2037 bytes, make sure the event size is more than minCompressionLen=1500 to trigger compression",
			logs: createLogData(1, 1, 10),
			conf: func() *Config {
				return NewFactory().CreateDefaultConfig().(*Config)
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`, `"otel.log.name":"0_0_1"`, `"otel.log.name":"0_0_5"`, `"otel.log.name":"0_0_6"`, `"otel.log.name":"0_0_7"`, `"otel.log.name":"0_0_8"`, `"otel.log.name":"0_0_9"`},
				},
				numBatches: 1,
				compressed: true,
			},
		},
		{
			name: "2 compressed batches - 1832 bytes each, make sure the log size is more than minCompressionLen=1500 to trigger compression",
			logs: createLogData(1, 1, 22),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = 1916
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`, `"otel.log.name":"0_0_1"`, `"otel.log.name":"0_0_5"`, `"otel.log.name":"0_0_6"`, `"otel.log.name":"0_0_7"`, `"otel.log.name":"0_0_8"`, `"otel.log.name":"0_0_9"`, `"otel.log.name":"0_0_10"`, `"otel.log.name":"0_0_11"`},
					{`"otel.log.name":"0_0_15"`, `"otel.log.name":"0_0_16"`, `"otel.log.name":"0_0_17"`, `"otel.log.name":"0_0_18"`, `"otel.log.name":"0_0_19"`, `"otel.log.name":"0_0_20"`, `"otel.log.name":"0_0_21"`},
				},
				numBatches: 2,
				compressed: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runLogExport(test.conf, test.logs, t)

			require.NoError(t, err)
			require.Len(t, got, test.want.numBatches)

			for i := 0; i < test.want.numBatches; i++ {
				require.NotZero(t, got[i])
				if test.want.compressed {
					validateCompressedContains(t, test.want.batches[i], got[i].body)
				} else {
					for _, expected := range test.want.batches[i] {
						assert.Contains(t, string(got[i].body), expected)
					}
				}
			}
		})
	}
}

func TestReceiveMetrics(t *testing.T) {
	md := createMetricsData(3)
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.DisableCompression = true
	actual, err := runMetricsExport(cfg, md, t)
	assert.Len(t, actual, 1)
	assert.NoError(t, err)
	msg := string(actual[0].body)
	assert.Contains(t, msg, "\"event\":\"metric\"")
	assert.Contains(t, msg, "\"time\":1.001")
	assert.Contains(t, msg, "\"time\":2.002")
}

func TestReceiveBatchedMetrics(t *testing.T) {
	type wantType struct {
		batches    [][]string
		numBatches int
		compressed bool
	}

	// The test cases depend on the constant minCompressionLen = 1500.
	// If the constant changed, the test cases with want.compressed=true must be updated.
	require.Equal(t, minCompressionLen, 1500)

	tests := []struct {
		name    string
		conf    *Config
		metrics pmetric.Metrics
		want    wantType
	}{
		{
			name:    "all metrics events in payload when max content length unknown (configured max content length 0)",
			metrics: createMetricsData(4),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthMetrics = 0
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"k1":"v1"`, `"time":1.001`, `"time":2.002`, `"time":3.003`},
				},
				numBatches: 1,
			},
		},
		{
			name:    "1 metric event per payload (configured max content length is same as event size)",
			metrics: createMetricsData(4),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthMetrics = 300
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"k1":"v1"`},
					{`"time":1.001`},
					{`"time":2.002`},
					{`"time":3.003`},
				},
				numBatches: 4,
			},
		},
		{
			name:    "2 metric events per payload (configured max content length is twice event size)",
			metrics: createMetricsData(4),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthMetrics = 448
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"k1":"v1"`, `"time":1.001`},
					{`"time":2.002`, `"time":3.003`},
				},
				numBatches: 2,
			},
		},
		{
			name:    "1 compressed batch of 2037 bytes, make sure the event size is more than minCompressionLen=1500 to trigger compression",
			metrics: createMetricsData(10),
			conf: func() *Config {
				return NewFactory().CreateDefaultConfig().(*Config)
			}(),
			want: wantType{
				batches: [][]string{
					{`"k1":"v1"`, `"time":1.001`, `"time":2.002`, `"time":3.003`, `"time":4.004`, `"time":5.005`, `"time":6.006`},
				},
				numBatches: 1,
				compressed: true,
			},
		},
		{
			name:    "2 compressed batches - 2211 bytes each, make sure the event size is more than minCompressionLen=1500 to trigger compression",
			metrics: createMetricsData(22),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthMetrics = 2211
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"k1":"v1"`, `"time":1.001`, `"time":2.002`, `"time":3.003`, `"time":4.004`, `"time":5.005`, `"time":7.007`, `"time":8.008`, `"time":9.009`, `"time":10.01`},
					{`"time":11.011`, `"time":15.015`, `"time":16.016`, `"time":17.017`, `"time":18.018`, `"time":19.019`, `"time":20.02`, `"time":21.021`},
				},
				numBatches: 2,
				compressed: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runMetricsExport(test.conf, test.metrics, t)

			require.NoError(t, err)
			require.Len(t, got, test.want.numBatches)

			for i := 0; i < test.want.numBatches; i++ {
				require.NotZero(t, got[i])
				if test.want.compressed {
					validateCompressedContains(t, test.want.batches[i], got[i].body)
				} else {
					for _, expected := range test.want.batches[i] {
						assert.Contains(t, string(got[i].body), expected)
					}
				}
			}
		})
	}
}

func TestReceiveMetricsWithCompression(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	request, err := runMetricsExport(cfg, createMetricsData(1000), t)
	assert.NoError(t, err)
	assert.NotEqual(t, "", request)
}

func TestErrorReceived(t *testing.T) {
	rr := make(chan receivedRequest)
	capture := CapturingData{receivedRequest: rr, statusCode: 500}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	s := &http.Server{
		Handler: &capture,
	}
	go func() {
		panic(s.Serve(listener))
	}()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	// Disable QueueSettings to ensure that we execute the request when calling ConsumeTraces
	// otherwise we will not see the error.
	cfg.QueueSettings.Enabled = false
	// Disable retries to not wait too much time for the return error.
	cfg.RetrySettings.Enabled = false
	cfg.DisableCompression = true
	cfg.Token = "1234-1234"

	params := componenttest.NewNopExporterCreateSettings()
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, exporter.Shutdown(context.Background()))
	}()

	td := createTraceData(3)

	err = exporter.ConsumeTraces(context.Background(), td)
	select {
	case <-rr:
	case <-time.After(5 * time.Second):
		t.Fatal("Should have received request")
	}
	assert.EqualError(t, err, "HTTP 500 \"Internal Server Error\"")
}

func TestInvalidLogs(t *testing.T) {
	config := NewFactory().CreateDefaultConfig().(*Config)
	config.DisableCompression = false
	_, err := runLogExport(config, createLogData(1, 1, 0), t)
	assert.Error(t, err)
}

func TestInvalidMetrics(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	_, err := runMetricsExport(cfg, pmetric.NewMetrics(), t)
	assert.Error(t, err)
}

func TestInvalidURL(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Disable queuing to ensure that we execute the request when calling ConsumeTraces
	// otherwise we will not see the error.
	cfg.QueueSettings.Enabled = false
	// Disable retries to not wait too much time for the return error.
	cfg.RetrySettings.Enabled = false
	cfg.Endpoint = "ftp://example.com:134"
	cfg.Token = "1234-1234"
	params := componenttest.NewNopExporterCreateSettings()
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, exporter.Shutdown(context.Background()))
	}()
	td := createTraceData(2)

	err = exporter.ConsumeTraces(context.Background(), td)
	assert.EqualError(t, err, "Post \"ftp://example.com:134/services/collector\": unsupported protocol scheme \"ftp\"")
}

type badJSON struct {
	Foo float64 `json:"foo"`
}

func TestInvalidJson(t *testing.T) {
	badEvent := badJSON{
		Foo: math.Inf(1),
	}
	syncPool := sync.Pool{New: func() interface{} {
		return gzip.NewWriter(nil)
	}}
	evs := []*splunk.Event{
		{
			Event: badEvent,
		},
		nil,
	}
	reader, _, err := encodeBodyEvents(&syncPool, evs, false)
	assert.Error(t, err, reader)
}

func TestStartAlwaysReturnsNil(t *testing.T) {
	c := client{}
	err := c.start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
}

func TestInvalidJsonClient(t *testing.T) {
	badEvent := badJSON{
		Foo: math.Inf(1),
	}
	evs := []*splunk.Event{
		{
			Event: badEvent,
		},
		nil,
	}
	c := client{
		url: nil,
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: &Config{},
	}
	err := c.sendSplunkEvents(context.Background(), evs)
	assert.EqualError(t, err, "Permanent error: splunk.Event.Event: splunkhecexporter.badJSON.Foo: unsupported value: +Inf")
}

func TestInvalidURLClient(t *testing.T) {
	c := client{
		url: &url.URL{Host: "in va lid"},
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: &Config{},
	}
	err := c.sendSplunkEvents(context.Background(), []*splunk.Event{})
	assert.EqualError(t, err, "Permanent error: parse \"//in%20va%20lid\": invalid URL escape \"%20\"")
}

func Test_pushLogData_nil_Logs(t *testing.T) {
	tests := []struct {
		name     func(bool) string
		logs     plog.Logs
		requires func(*testing.T, plog.Logs)
	}{
		{
			name: func(disable bool) string {
				return "COMPRESSION " + map[bool]string{true: "DISABLED ", false: "ENABLED "}[disable] + "nil ResourceLogs"
			},
			logs: plog.NewLogs(),
			requires: func(t *testing.T, logs plog.Logs) {
				require.Zero(t, logs.ResourceLogs().Len())
			},
		},
		{
			name: func(disable bool) string {
				return "COMPRESSION " + map[bool]string{true: "DISABLED ", false: "ENABLED "}[disable] + "nil InstrumentationLogs"
			},
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				logs.ResourceLogs().AppendEmpty()
				return logs
			}(),
			requires: func(t *testing.T, logs plog.Logs) {
				require.Equal(t, logs.ResourceLogs().Len(), 1)
				require.Zero(t, logs.ResourceLogs().At(0).ScopeLogs().Len())
			},
		},
		{
			name: func(disable bool) string {
				return "COMPRESSION " + map[bool]string{true: "DISABLED ", false: "ENABLED "}[disable] + "nil LogRecords"
			},
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
				return logs
			}(),
			requires: func(t *testing.T, logs plog.Logs) {
				require.Equal(t, logs.ResourceLogs().Len(), 1)
				require.Equal(t, logs.ResourceLogs().At(0).ScopeLogs().Len(), 1)
				require.Zero(t, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
			},
		},
	}

	c := client{
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
		logger: zaptest.NewLogger(t),
	}

	for _, test := range tests {
		for _, disabled := range []bool{true, false} {
			t.Run(test.name(disabled), func(t *testing.T) {
				test.requires(t, test.logs)
				err := c.pushLogData(context.Background(), test.logs)
				assert.NoError(t, err)
			})
		}
	}

}

func Test_pushLogData_InvalidLog(t *testing.T) {
	c := client{
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
		logger: zaptest.NewLogger(t),
	}

	logs := plog.NewLogs()
	log := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	// Invalid log value
	log.Body().SetDoubleVal(math.Inf(1))

	err := c.pushLogData(context.Background(), logs)

	assert.Error(t, err, "Permanent error: dropped log event: &{<nil> unknown    +Inf map[]}, error: splunk.Event.Event: unsupported value: +Inf")
}

func Test_pushLogData_PostError(t *testing.T) {
	c := client{
		url: &url.URL{Host: "in va lid"},
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
		logger: zaptest.NewLogger(t),
	}

	// 2000 log records -> ~371888 bytes when JSON encoded.
	logs := createLogData(1, 1, 2000)

	// 0 -> unlimited size batch, true -> compression disabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 0, true
	err := c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	var logsErr consumererror.Logs
	assert.ErrorAs(t, err, &logsErr)
	assert.Equal(t, logs, logsErr.GetLogs())

	// 0 -> unlimited size batch, false -> compression enabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 0, false
	err = c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.ErrorAs(t, err, &logsErr)
	assert.Equal(t, logs, logsErr.GetLogs())

	// 200000 < 371888 -> multiple batches, true -> compression disabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 200000, true
	err = c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.ErrorAs(t, err, &logsErr)
	assert.Equal(t, logs, logsErr.GetLogs())

	// 200000 < 371888 -> multiple batches, false -> compression enabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 200000, false
	err = c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.ErrorAs(t, err, &logsErr)
	assert.Equal(t, logs, logsErr.GetLogs())
}

func Test_pushLogData_ShouldAddResponseTo400Error(t *testing.T) {
	splunkClient := client{
		url: &url.URL{Scheme: "http", Host: "splunk"},
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
		logger: zaptest.NewLogger(t),
	}
	logs := createLogData(1, 1, 1)

	responseBody := `some error occurred`

	// An HTTP client that returns status code 400 and response body responseBody.
	splunkClient.client, _ = newTestClient(400, responseBody)
	// Sending logs using the client.
	err := splunkClient.pushLogData(context.Background(), logs)
	// TODO: Uncomment after consumererror.Logs implements method Unwrap.
	// require.True(t, consumererror.IsPermanent(err), "Expecting permanent error")
	require.Contains(t, err.Error(), "HTTP/0.0 400")
	// The returned error should contain the response body responseBody.
	assert.Contains(t, err.Error(), responseBody)

	// An HTTP client that returns some other status code other than 400 and response body responseBody.
	splunkClient.client, _ = newTestClient(500, responseBody)
	// Sending logs using the client.
	err = splunkClient.pushLogData(context.Background(), logs)
	// TODO: Uncomment after consumererror.Logs implements method Unwrap.
	// require.False(t, consumererror.IsPermanent(err), "Expecting non-permanent error")
	require.Contains(t, err.Error(), "HTTP 500")
	// The returned error should not contain the response body responseBody.
	assert.NotContains(t, err.Error(), responseBody)
}

func Test_pushLogData_ShouldReturnUnsentLogsOnly(t *testing.T) {
	config := NewFactory().CreateDefaultConfig().(*Config)
	c := client{
		url: &url.URL{Scheme: "http", Host: "splunk"},
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: config,
		logger: zaptest.NewLogger(t),
	}

	// Just two records
	logs := createLogData(2, 1, 1)

	// Each record is about 200 bytes, so the 250-byte buffer will fit only one at a time
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 250, true

	// The first record is to be sent successfully, the second one should not
	c.client, _ = newTestClientWithPresetResponses([]int{200, 400}, []string{"OK", "NOK"})

	err := c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.IsType(t, consumererror.Logs{}, err)

	// Only the record that was not successfully sent should be returned
	var logsErr consumererror.Logs
	require.ErrorAs(t, err, &logsErr)
	assert.Equal(t, 1, logsErr.GetLogs().ResourceLogs().Len())
	assert.Equal(t, logs.ResourceLogs().At(1), logsErr.GetLogs().ResourceLogs().At(0))
}

func Test_pushLogData_ShouldAddHeadersForProfilingData(t *testing.T) {
	c := client{
		url: &url.URL{Scheme: "http", Host: "splunk"},
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
		logger: zaptest.NewLogger(t),
	}

	logs := createLogDataWithCustomLibraries(1, []string{"otel.logs", "otel.profiling"}, []int{10, 20})
	var headers *[]http.Header

	c.client, headers = newTestClient(200, "OK")
	// A 300-byte buffer only fits one record (around 200 bytes), so each record will be sent separately
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 300, true

	err := c.pushLogData(context.Background(), logs)
	require.NoError(t, err)
	assert.Equal(t, 30, len(*headers))

	profilingCount, nonProfilingCount := 0, 0
	for i := range *headers {
		if (*headers)[i].Get(libraryHeaderName) == profilingLibraryName {
			profilingCount++
		} else {
			nonProfilingCount++
		}
	}

	assert.Equal(t, 20, profilingCount)
	assert.Equal(t, 10, nonProfilingCount)
}

func Benchmark_pushLogData_100_10_10_1024(b *testing.B) {
	benchPushLogData(b, 100, 10, 10, 1024)
}

func Benchmark_pushLogData_10_100_100_1024(b *testing.B) {
	benchPushLogData(b, 10, 100, 100, 1024)
}

func Benchmark_pushLogData_10_0_100_1024(b *testing.B) {
	benchPushLogData(b, 10, 0, 100, 1024)
}

func Benchmark_pushLogData_10_100_0_1024(b *testing.B) {
	benchPushLogData(b, 10, 100, 0, 1024)
}

func Benchmark_pushLogData_10_10_10_256(b *testing.B) {
	benchPushLogData(b, 10, 10, 10, 256)
}

func Benchmark_pushLogData_10_10_10_1024(b *testing.B) {
	benchPushLogData(b, 10, 10, 10, 1024)
}

func Benchmark_pushLogData_10_10_10_8K(b *testing.B) {
	benchPushLogData(b, 10, 10, 10, 8*1024)
}

func Benchmark_pushLogData_10_10_10_1M(b *testing.B) {
	benchPushLogData(b, 10, 10, 10, 1024*1024)
}
func Benchmark_pushLogData_10_1_1_1024(b *testing.B) {
	benchPushLogData(b, 10, 1, 1, 1024)
}

func benchPushLogData(b *testing.B, numResources int, numProfiling int, numNonProfiling int, bufSize uint) {
	c := client{
		url: &url.URL{Scheme: "http", Host: "splunk"},
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
		logger: zaptest.NewLogger(b),
	}

	c.client, _ = newTestClient(200, "OK")
	c.config.MaxContentLengthLogs = bufSize
	logs := createLogDataWithCustomLibraries(numResources, []string{"otel.logs", "otel.profiling"}, []int{numNonProfiling, numProfiling})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := c.pushLogData(context.Background(), logs)
		require.NoError(b, err)
	}
}

func Test_pushLogData_Small_MaxContentLength(t *testing.T) {
	c := client{
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
		logger: zaptest.NewLogger(t),
	}
	c.config.MaxContentLengthLogs = 1

	logs := createLogData(1, 1, 2000)

	for _, disable := range []bool{true, false} {
		c.config.DisableCompression = disable

		err := c.pushLogData(context.Background(), logs)
		require.Error(t, err)

		assert.True(t, consumererror.IsPermanent(err))
		assert.Contains(t, err.Error(), "dropped log event")
	}
}

func TestAllowedLogDataTypes(t *testing.T) {
	tests := []struct {
		name                 string
		allowProfilingData   bool
		allowLogData         bool
		wantProfilingRecords int
		wantLogRecords       int
	}{
		{
			name:               "both_allowed",
			allowProfilingData: true,
			allowLogData:       true,
		},
		{
			name:               "logs_allowed",
			allowProfilingData: false,
			allowLogData:       true,
		},
		{
			name:               "profiling_allowed",
			allowProfilingData: true,
			allowLogData:       false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logs := createLogDataWithCustomLibraries(1, []string{"otel.logs", "otel.profiling"}, []int{1, 1})
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			cfg.LogDataEnabled = test.allowLogData
			cfg.ProfilingDataEnabled = test.allowProfilingData

			requests, err := runLogExport(cfg, logs, t)
			assert.NoError(t, err)

			seenLogs := false
			seenProfiling := false
			for _, r := range requests {
				if r.headers.Get(libraryHeaderName) == profilingLibraryName {
					seenProfiling = true
				} else {
					seenLogs = true
				}
			}
			assert.Equal(t, test.allowLogData, seenLogs)
			assert.Equal(t, test.allowProfilingData, seenProfiling)
		})
	}
}

func TestSubLogs(t *testing.T) {
	// Creating 12 logs (2 resources x 2 libraries x 3 records)
	logs := createLogData(2, 2, 3)

	c := client{
		config: NewFactory().CreateDefaultConfig().(*Config),
	}

	// Logs subset from leftmost index (resource 0, library 0, record 0).
	_0_0_0 := &index{resource: 0, library: 0, record: 0} //revive:disable-line:var-naming
	got := c.subLogs(&logs, _0_0_0, nil)

	// Number of logs in subset should equal original logs.
	assert.Equal(t, logs.LogRecordCount(), got.LogRecordCount())

	// The name of the leftmost log record should be 0_0_0.
	val, _ := got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "0_0_0", val.AsString())
	// The name of the rightmost log record should be 1_1_2.
	val, _ = got.ResourceLogs().At(1).ScopeLogs().At(1).LogRecords().At(2).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "1_1_2", val.AsString())

	// Logs subset from some mid index (resource 0, library 1, log 2).
	_0_1_2 := &index{resource: 0, library: 1, record: 2} //revive:disable-line:var-naming
	got = c.subLogs(&logs, _0_1_2, nil)

	assert.Equal(t, 7, got.LogRecordCount())

	// The name of the leftmost log record should be 0_1_2.
	val, _ = got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "0_1_2", val.AsString())
	// The name of the rightmost log record should be 1_1_2.
	val, _ = got.ResourceLogs().At(1).ScopeLogs().At(1).LogRecords().At(2).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "1_1_2", val.AsString())

	// Logs subset from rightmost index (resource 1, library 1, log 2).
	_1_1_2 := &index{resource: 1, library: 1, record: 2} //revive:disable-line:var-naming
	got = c.subLogs(&logs, _1_1_2, nil)

	// Number of logs in subset should be 1.
	assert.Equal(t, 1, got.LogRecordCount())

	// The name of the sole log record should be 1_1_2.
	val, _ = got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "1_1_2", val.AsString())

	// Now see how profiling and log data are merged
	logs = createLogDataWithCustomLibraries(2, []string{"otel.logs", "otel.profiling"}, []int{10, 10})
	slice := &index{resource: 1, library: 0, record: 5}
	profSlice := &index{resource: 0, library: 1, record: 8}

	got = c.subLogs(&logs, slice, profSlice)

	assert.Equal(t, 5+2+10, got.LogRecordCount())
	assert.Equal(t, "otel.logs", got.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Name())
	val, _ = got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "1_0_5", val.AsString())
	val, _ = got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(4).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "1_0_9", val.AsString())

	assert.Equal(t, "otel.profiling", got.ResourceLogs().At(1).ScopeLogs().At(0).Scope().Name())
	val, _ = got.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "0_1_8", val.AsString())
	val, _ = got.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(1).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "0_1_9", val.AsString())
	assert.Equal(t, "otel.profiling", got.ResourceLogs().At(2).ScopeLogs().At(0).Scope().Name())
	val, _ = got.ResourceLogs().At(2).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "1_1_0", val.AsString())
	val, _ = got.ResourceLogs().At(2).ScopeLogs().At(0).LogRecords().At(9).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "1_1_9", val.AsString())
}

// validateCompressedEqual validates that GZipped `got` contains `expected` strings
func validateCompressedContains(t *testing.T, expected []string, got []byte) {
	z, err := gzip.NewReader(bytes.NewReader(got))
	require.NoError(t, err)
	defer z.Close()

	p, err := ioutil.ReadAll(z)
	require.NoError(t, err)

	for _, e := range expected {
		assert.Contains(t, string(p), e)
	}

}

func BenchmarkPushLogRecords(b *testing.B) {
	logs := createLogData(1, 1, 1)
	c := client{
		url: &url.URL{Scheme: "http", Host: "splunk"},
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
		logger: zap.NewNop(),
	}
	sender := func(ctx context.Context, buffer *bytes.Buffer, headers map[string]string) error {
		return nil
	}
	state := makeBlankBufferState(4096)
	for n := 0; n < b.N; n++ {
		permanentErrs, sendingErr := c.pushLogRecords(context.Background(), logs.ResourceLogs(), &state, map[string]string{}, sender)
		assert.NoError(b, sendingErr)
		for _, permanentErr := range permanentErrs {
			assert.NoError(b, permanentErr)
		}
	}
}
