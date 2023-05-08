// Copyright The OpenTelemetry Authors
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
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
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
	var headers []http.Header

	return &http.Client{
		Transport: testRoundTripper(func(req *http.Request) *http.Response {
			code := codes[index%len(codes)]
			body := bodies[index%len(bodies)]
			index++

			headers = append(headers, req.Header)

			return &http.Response{
				StatusCode: code,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
				Header:     make(http.Header),
			}
		}),
	}, &headers
}

func createMetricsData(numberOfDataPoints int) pmetric.Metrics {

	doubleVal := 1234.5678
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k0", "v0")
	rm.Resource().Attributes().PutStr("k1", "v1")

	for i := 0; i < numberOfDataPoints; i++ {
		tsUnix := time.Unix(int64(i), int64(i)*time.Millisecond.Nanoseconds())

		ilm := rm.ScopeMetrics().AppendEmpty()
		metric := ilm.Metrics().AppendEmpty()
		metric.SetName("gauge_double_with_dims")
		doublePt := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		doublePt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
		doublePt.SetDoubleValue(doubleVal)
		doublePt.Attributes().PutStr("k/n0", "vn0")
		doublePt.Attributes().PutStr("k/n1", "vn1")
		doublePt.Attributes().PutStr("k/r0", "vr0")
		doublePt.Attributes().PutStr("k/r1", "vr1")
	}

	return metrics
}

func createTraceData(numberOfTraces int) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("resource", "R1")
	ils := rs.ScopeSpans().AppendEmpty()
	ils.Spans().EnsureCapacity(numberOfTraces)
	for i := 0; i < numberOfTraces; i++ {
		span := ils.Spans().AppendEmpty()
		span.SetName("root")
		span.SetStartTimestamp(pcommon.Timestamp((i + 1) * 1e9))
		span.SetEndTimestamp(pcommon.Timestamp((i + 2) * 1e9))
		span.SetTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})
		span.SetSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1})
		span.TraceState().FromRaw("foo")
		if i%2 == 0 {
			span.SetParentSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
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

// these runes are used to generate long log messages that will compress down to a number of bytes we can rely on for testing.
var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789wersgdgr43q3zvbcgv65ew 346xx$gt5/kuopo89.nytqasdfghjklpoiuy")

func repeatableString(length int) string {
	b := make([]rune, length)
	for i := range b {
		l := i % len(letterRunes)
		b[i] = letterRunes[l]
	}
	return string(b)
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
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr(splunk.DefaultNameLabel, fmt.Sprintf("%d_%d_%d", i, j, k))
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(splunk.DefaultIndexLabel, "myindex")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
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
	body, err := io.ReadAll(r.Body)

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

func runMetricsExport(cfg *Config, metrics pmetric.Metrics, expectedBatchesNum int, t *testing.T) ([]receivedRequest, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	factory := NewFactory()
	cfg.HTTPClientSettings.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.Token = "1234-1234"

	rr := make(chan receivedRequest)
	capture := CapturingData{testing: t, receivedRequest: rr, statusCode: 200, checkCompression: !cfg.DisableCompression}
	s := &http.Server{
		Handler:           &capture,
		ReadHeaderTimeout: 20 * time.Second,
	}
	defer s.Close()
	go func() {
		if e := s.Serve(listener); e != http.ErrServerClosed {
			require.NoError(t, e)
		}
	}()

	params := exportertest.NewNopCreateSettings()
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
			if len(requests) == expectedBatchesNum {
				return requests, nil
			}
		case <-time.After(5 * time.Second):
			if len(requests) == 0 {
				err = errors.New("timeout")
			}
			return requests, err
		}
	}
}

func runTraceExport(testConfig *Config, traces ptrace.Traces, expectedBatchesNum int, t *testing.T) ([]receivedRequest, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.DisableCompression = testConfig.DisableCompression
	cfg.MaxContentLengthTraces = testConfig.MaxContentLengthTraces
	cfg.Token = "1234-1234"

	rr := make(chan receivedRequest)
	capture := CapturingData{testing: t, receivedRequest: rr, statusCode: 200, checkCompression: !cfg.DisableCompression}
	s := &http.Server{
		Handler:           &capture,
		ReadHeaderTimeout: 20 * time.Second,
	}
	defer s.Close()
	go func() {
		if e := s.Serve(listener); e != http.ErrServerClosed {
			require.NoError(t, e)
		}
	}()

	params := exportertest.NewNopCreateSettings()
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
			if len(requests) == expectedBatchesNum {
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
				return requests, nil
			}
		case <-time.After(5 * time.Second):
			if len(requests) == 0 {
				return nil, errors.New("timeout")
			}

			return requests, err
		}
	}
}

func runLogExport(cfg *Config, ld plog.Logs, expectedBatchesNum int, t *testing.T) ([]receivedRequest, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	cfg.HTTPClientSettings.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.Token = "1234-1234"

	rr := make(chan receivedRequest)
	capture := CapturingData{testing: t, receivedRequest: rr, statusCode: 200, checkCompression: !cfg.DisableCompression}
	s := &http.Server{
		Handler:           &capture,
		ReadHeaderTimeout: 20 * time.Second,
	}
	defer s.Close()
	go func() {
		if e := s.Serve(listener); e != http.ErrServerClosed {
			require.NoError(t, e)
		}
	}()

	params := exportertest.NewNopCreateSettings()
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
			if len(requests) == expectedBatchesNum {
				return requests, nil
			}
		case <-time.After(5 * time.Second):
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
			name:   "100 events, make sure that we produce more than one compressed batch",
			traces: createTraceData(100),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthTraces = minCompressionLen + 500
				return cfg
			}(),
			want: wantType{
				// just test that the test has 2 batches, don't test its contents.
				batches:    [][]string{{""}, {""}},
				numBatches: 2,
				compressed: true,
			},
		}, {
			name:   "100 events, make sure that we produce only one compressed batch when MaxContentLengthTraces is 0",
			traces: createTraceData(100),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthTraces = 0
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"start_time":1`, `"start_time":2`, `"start_time":3`, `"start_time":4`, `"start_time":7`, `"start_time":8`, `"start_time":9`, `"start_time":20`, `"start_time":40`, `"start_time":85`, `"start_time":98`, `"start_time":99`},
				},
				numBatches: 1,
				compressed: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runTraceExport(test.conf, test.traces, test.want.numBatches, t)

			require.NoError(t, err)
			require.Len(t, got, test.want.numBatches, "expected exact number of batches")

			for i := 0; i < test.want.numBatches; i++ {
				require.NotZero(t, got[i])
				if test.conf.MaxContentLengthTraces != 0 {
					require.True(t, int(test.conf.MaxContentLengthTraces) > len(got[i].body))
				}
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
		wantErr    string
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
			name: "1 log event long enough to trigger compression",
			logs: func() plog.Logs {
				l := createLogData(1, 1, 1)
				l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr(strings.Repeat("a", 1800))
				return l
			}(),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = 1750
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`},
				},
				numBatches: 1,
				compressed: true,
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
			name: "150 events, make sure that we produce more than one compressed batch",
			logs: createLogData(1, 1, 150),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = minCompressionLen + 150
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`, `"otel.log.name":"0_0_90"`},
					{`"otel.log.name":"0_0_110"`, `"otel.log.name":"0_0_149"`},
				},
				numBatches: 2,
				compressed: true,
			},
		},
		{
			name: "150 events, make sure that we produce only one compressed batch when MaxContentLengthLogs is 0",
			logs: createLogData(1, 1, 150),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = 0
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`, `"otel.log.name":"0_0_90"`, `"otel.log.name":"0_0_110"`, `"otel.log.name":"0_0_149"`},
				},
				numBatches: 1,
				compressed: true,
			},
		},
		{
			name: "one event with 1340 bytes, then one triggering compression (going over 1500 bytes) and bypassing the max length, moving to a separate batch",
			logs: func() plog.Logs {
				firstLog := createLogData(1, 1, 2)
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr(repeatableString(1340))
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Body().SetStr(repeatableString(2800000))
				return firstLog
			}(),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = 10000 // small so we can reproduce without allocating big logs.
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`}, {`"otel.log.name":"0_0_1"`},
				},
				numBatches: 2,
				compressed: true,
			},
		},
		{
			name: "one event that is so large we cannot send it",
			logs: func() plog.Logs {
				firstLog := createLogData(1, 1, 1)
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr(repeatableString(500000))
				return firstLog
			}(),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = 1800 // small so we can reproduce without allocating big logs.
				return cfg
			}(),
			want: wantType{
				batches:    [][]string{},
				numBatches: 0,
				compressed: true,
				wantErr:    "timeout", // our server will time out waiting for the data.
			},
		},
		{
			name: "two events with 2000 bytes, one with 2000 bytes, then one with 20000 bytes",
			logs: func() plog.Logs {
				firstLog := createLogData(1, 1, 3)
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr(repeatableString(2000))
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Body().SetStr(repeatableString(2000))
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(2).Body().SetStr(repeatableString(20000))
				return firstLog
			}(),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxEventSize = 20000 // small so we can reproduce without allocating big logs.
				cfg.DisableCompression = true
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`, `"otel.log.name":"0_0_1"`},
				},
				numBatches: 1,
			},
		},
		{
			name: "two events with 2000 bytes, one with 1000 bytes, then one with 4200 bytes",
			logs: func() plog.Logs {
				firstLog := createLogData(1, 1, 5)
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr(repeatableString(2000))
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Body().SetStr(repeatableString(2000))
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(2).Body().SetStr(repeatableString(1000))
				firstLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(3).Body().SetStr(repeatableString(4200))
				return firstLog
			}(),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxEventSize = 10000 // small so we can reproduce without allocating big logs.
				cfg.MaxContentLengthLogs = 5000
				cfg.DisableCompression = true
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"otel.log.name":"0_0_0"`, `"otel.log.name":"0_0_1"`},
					{`"otel.log.name":"0_0_2"`},
					{`"otel.log.name":"0_0_3"`, `"otel.log.name":"0_0_4"`},
				},
				numBatches: 3,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runLogExport(test.conf, test.logs, test.want.numBatches, t)

			if test.want.wantErr != "" {
				require.EqualError(t, err, test.want.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.want.numBatches, len(got))

			for i := 0; i < test.want.numBatches; i++ {
				require.NotZero(t, got[i])
				if test.conf.MaxContentLengthLogs != 0 {
					require.True(t, int(test.conf.MaxContentLengthLogs) > len(got[i].body))
				}
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

func TestReceiveRaw(t *testing.T) {
	tests := []struct {
		name string
		conf *Config
		logs plog.Logs
		text string
	}{
		{
			name: "single raw event",
			logs: createLogData(1, 1, 1),
			conf: func() *Config {
				conf := createDefaultConfig().(*Config)
				conf.ExportRaw = true
				return conf
			}(),
			text: "mylog\n",
		},
		{
			name: "single raw event as bytes",
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetEmptyBytes().FromRaw([]byte("mybytes"))
				return logs
			}(),
			conf: func() *Config {
				conf := createDefaultConfig().(*Config)
				conf.ExportRaw = true
				return conf
			}(),
			text: "bXlieXRlcw==\n",
		},
		{
			name: "single raw event as number",
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetDouble(64.345)
				return logs
			}(),
			conf: func() *Config {
				conf := createDefaultConfig().(*Config)
				conf.ExportRaw = true
				return conf
			}(),
			text: "64.345\n",
		},
		{
			name: "five raw events",
			logs: createLogData(1, 1, 5),
			conf: func() *Config {
				conf := createDefaultConfig().(*Config)
				conf.ExportRaw = true
				return conf
			}(),
			text: "mylog\nmylog\nmylog\nmylog\nmylog\n",
		},
		{
			name: "log with array body",
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				_ = logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetEmptySlice().FromRaw([]any{1, "foo", true})
				return logs
			}(),
			conf: func() *Config {
				conf := createDefaultConfig().(*Config)
				conf.ExportRaw = true
				return conf
			}(),
			text: "[1,\"foo\",true]\n",
		},
		{
			name: "log with map body",
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetEmptyMap().PutStr("foo", "bar")
				return logs
			}(),
			conf: func() *Config {
				conf := createDefaultConfig().(*Config)
				conf.ExportRaw = true
				return conf
			}(),
			text: "{\"foo\":\"bar\"}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runLogExport(test.conf, test.logs, 1, t)
			require.NoError(t, err)
			req := got[0]
			assert.Equal(t, test.text, string(req.body))
		})
	}
}

func TestReceiveMetrics(t *testing.T) {
	md := createMetricsData(3)
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.DisableCompression = true
	actual, err := runMetricsExport(cfg, md, 1, t)
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
					{``, ``},
					{``, ``},
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
			name:    "200 events, make sure that we produce more than one compressed batch",
			metrics: createMetricsData(100),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthMetrics = minCompressionLen + 150
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"time":1.001`, `"time":2.002`, `"time":3.003`, `"time":4.004`, `"time":5.005`, `"time":6.006`},
					{`"time":85.085`, `"time":99.099`},
				},
				numBatches: 2,
				compressed: true,
			},
		},
		{
			name:    "200 events, make sure that we produce only one compressed batch when MaxContentLengthMetrics is 0",
			metrics: createMetricsData(100),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthMetrics = 0
				return cfg
			}(),
			want: wantType{
				batches: [][]string{
					{`"time":1.001`, `"time":2.002`, `"time":3.003`, `"time":4.004`, `"time":5.005`, `"time":6.006`, `"time":85.085`, `"time":99.099`},
				},
				numBatches: 1,
				compressed: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := runMetricsExport(test.conf, test.metrics, test.want.numBatches, t)

			require.NoError(t, err)
			require.Len(t, got, test.want.numBatches)

			for i := 0; i < test.want.numBatches; i++ {
				require.NotZero(t, got[i])
				if test.conf.MaxContentLengthMetrics != 0 {
					require.True(t, int(test.conf.MaxContentLengthMetrics) > len(got[i].body))
				}
				if test.want.compressed {
					validateCompressedContains(t, test.want.batches[i], got[i].body)
				} else {
					found := false

					for _, expected := range test.want.batches[i] {
						if strings.Contains(string(got[i].body), expected) {
							found = true
							break
						}
					}
					assert.True(t, found, "%s did not match any expected batch", string(got[i].body))
				}
			}
		})
	}
}

func Test_PushMetricsData_Histogram_NaN_Sum(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()
	histogram := ilm.Metrics().AppendEmpty()
	histogram.SetName("histogram_with_empty_sum")
	dp := histogram.SetEmptyHistogram().DataPoints().AppendEmpty()
	dp.SetSum(math.NaN())

	c := client{
		config:    NewFactory().CreateDefaultConfig().(*Config),
		logger:    zap.NewNop(),
		hecWorker: &mockHecWorker{},
	}

	permanentErrors := c.pushMetricsDataInBatches(context.Background(), metrics, map[string]string{})
	assert.NoError(t, permanentErrors)
}

func Test_PushMetricsData_Summary_NaN_Sum(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()
	summary := ilm.Metrics().AppendEmpty()
	summary.SetName("Summary_with_empty_sum")
	dp := summary.SetEmptySummary().DataPoints().AppendEmpty()
	dp.SetSum(math.NaN())

	c := client{
		config:    NewFactory().CreateDefaultConfig().(*Config),
		logger:    zap.NewNop(),
		hecWorker: &mockHecWorker{},
	}

	permanentErrors := c.pushMetricsDataInBatches(context.Background(), metrics, map[string]string{})
	assert.NoError(t, permanentErrors)
}

func TestReceiveMetricsWithCompression(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.MaxContentLengthMetrics = 1800
	request, err := runMetricsExport(cfg, createMetricsData(100), 1, t)
	assert.NoError(t, err)
	assert.Equal(t, "gzip", request[0].headers.Get("Content-Encoding"))
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
		Handler:           &capture,
		ReadHeaderTimeout: 20 * time.Second,
	}
	defer s.Close()
	go func() {
		if e := s.Serve(listener); e != http.ErrServerClosed {
			require.NoError(t, e)
		}
	}()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	// Disable QueueSettings to ensure that we execute the request when calling ConsumeTraces
	// otherwise we will not see the error.
	cfg.QueueSettings.Enabled = false
	// Disable retries to not wait too much time for the return error.
	cfg.RetrySettings.Enabled = false
	cfg.DisableCompression = true
	cfg.Token = "1234-1234"

	params := exportertest.NewNopCreateSettings()
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
	_, err := runLogExport(config, createLogData(1, 1, 0), 1, t)
	assert.Error(t, err)
}

func TestInvalidMetrics(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	_, err := runMetricsExport(cfg, pmetric.NewMetrics(), 1, t)
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
	cfg.HTTPClientSettings.Endpoint = "ftp://example.com:134"
	cfg.Token = "1234-1234"
	params := exportertest.NewNopCreateSettings()
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
	_, err := jsoniter.Marshal(badEvent)
	assert.Error(t, err)
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
		config: NewFactory().CreateDefaultConfig().(*Config),
		logger: zaptest.NewLogger(t),
	}

	logs := plog.NewLogs()
	log := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	// Invalid log value
	log.Body().SetDouble(math.Inf(1))

	err := c.pushLogData(context.Background(), logs)

	assert.Error(t, err, "Permanent error: dropped log event: &{<nil> unknown    +Inf map[]}, error: splunk.Event.Event: unsupported value: +Inf")
}

func Test_pushLogData_PostError(t *testing.T) {
	c := client{
		config:    NewFactory().CreateDefaultConfig().(*Config),
		logger:    zaptest.NewLogger(t),
		hecWorker: &defaultHecWorker{url: &url.URL{Host: "in va lid"}},
	}

	// 2000 log records -> ~371888 bytes when JSON encoded.
	logs := createLogData(1, 1, 2000)

	// 0 -> unlimited size batch, true -> compression disabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 0, true
	err := c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	var logsErr consumererror.Logs
	assert.ErrorAs(t, err, &logsErr)
	assert.Equal(t, logs, logsErr.Data())

	// 0 -> unlimited size batch, false -> compression enabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 0, false
	err = c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.ErrorAs(t, err, &logsErr)
	assert.Equal(t, logs, logsErr.Data())

	// 200000 < 371888 -> multiple batches, true -> compression disabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 200000, true
	err = c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.ErrorAs(t, err, &logsErr)
	assert.Equal(t, logs, logsErr.Data())

	// 200000 < 371888 -> multiple batches, false -> compression enabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 200000, false
	err = c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.ErrorAs(t, err, &logsErr)
	assert.Equal(t, logs, logsErr.Data())
}

func Test_pushLogData_ShouldAddResponseTo400Error(t *testing.T) {
	config := NewFactory().CreateDefaultConfig().(*Config)
	url := &url.URL{Scheme: "http", Host: "splunk"}
	splunkClient := client{
		config: config,
		logger: zaptest.NewLogger(t),
	}
	logs := createLogData(1, 1, 1)

	responseBody := `some error occurred`

	// An HTTP client that returns status code 400 and response body responseBody.
	httpClient, _ := newTestClient(400, responseBody)
	splunkClient.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())}
	// Sending logs using the client.
	err := splunkClient.pushLogData(context.Background(), logs)
	require.True(t, consumererror.IsPermanent(err), "Expecting permanent error")
	require.Contains(t, err.Error(), "HTTP/0.0 400")
	// The returned error should contain the response body responseBody.
	assert.Contains(t, err.Error(), responseBody)

	// An HTTP client that returns some other status code other than 400 and response body responseBody.
	httpClient, _ = newTestClient(500, responseBody)
	splunkClient.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())}
	// Sending logs using the client.
	err = splunkClient.pushLogData(context.Background(), logs)
	require.False(t, consumererror.IsPermanent(err), "Expecting non-permanent error")
	require.Contains(t, err.Error(), "HTTP 500")
	// The returned error should not contain the response body responseBody.
	assert.NotContains(t, err.Error(), responseBody)
}

func Test_pushLogData_ShouldReturnUnsentLogsOnly(t *testing.T) {
	config := NewFactory().CreateDefaultConfig().(*Config)
	url := &url.URL{Scheme: "http", Host: "splunk"}
	c := client{
		config: config,
		logger: zaptest.NewLogger(t),
	}

	// Just two records
	logs := createLogData(2, 1, 1)

	// Each record is about 200 bytes, so the 250-byte buffer will fit only one at a time
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 250, true

	// The first record is to be sent successfully, the second one should not
	httpClient, _ := newTestClientWithPresetResponses([]int{200, 400}, []string{"OK", "NOK"})
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())}

	err := c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.IsType(t, consumererror.Logs{}, err)

	// Only the record that was not successfully sent should be returned
	var logsErr consumererror.Logs
	require.ErrorAs(t, err, &logsErr)
	assert.Equal(t, 1, logsErr.Data().ResourceLogs().Len())
	assert.Equal(t, logs.ResourceLogs().At(1), logsErr.Data().ResourceLogs().At(0))
}

func Test_pushLogData_ShouldAddHeadersForProfilingData(t *testing.T) {
	config := NewFactory().CreateDefaultConfig().(*Config)
	url := &url.URL{Scheme: "http", Host: "splunk"}
	c := client{
		config: config,
		logger: zaptest.NewLogger(t),
	}

	logs := createLogDataWithCustomLibraries(1, []string{"otel.logs", "otel.profiling"}, []int{10, 20})
	var headers *[]http.Header

	httpClient, headers := newTestClient(200, "OK")
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())}

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
	config := NewFactory().CreateDefaultConfig().(*Config)
	url := &url.URL{Scheme: "http", Host: "splunk"}
	c := client{
		config: config,
		logger: zaptest.NewLogger(b),
	}

	httpClient, _ := newTestClient(200, "OK")
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())}

	c.config.MaxContentLengthLogs = bufSize
	logs := createLogDataWithCustomLibraries(numResources, []string{"otel.logs", "otel.profiling"}, []int{numNonProfiling, numProfiling})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := c.pushLogData(context.Background(), logs)
		require.NoError(b, err)
	}
}

func Test_pushLogData_Small_MaxContentLength(t *testing.T) {
	config := NewFactory().CreateDefaultConfig().(*Config)
	c := client{
		config:    config,
		logger:    zaptest.NewLogger(t),
		hecWorker: &defaultHecWorker{&url.URL{Scheme: "http", Host: "splunk"}, http.DefaultClient, buildHTTPHeaders(config, component.NewDefaultBuildInfo())},
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
		name               string
		allowProfilingData bool
		allowLogData       bool
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

			numBatches := 1
			if test.allowLogData && test.allowProfilingData {
				numBatches = 2
			}

			requests, err := runLogExport(cfg, logs, numBatches, t)
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
	got := c.subLogs(logs, _0_0_0, nil)

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
	got = c.subLogs(logs, _0_1_2, nil)

	assert.Equal(t, 7, got.LogRecordCount())

	// The name of the leftmost log record should be 0_1_2.
	val, _ = got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "0_1_2", val.AsString())
	// The name of the rightmost log record should be 1_1_2.
	val, _ = got.ResourceLogs().At(1).ScopeLogs().At(1).LogRecords().At(2).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "1_1_2", val.AsString())

	// Logs subset from rightmost index (resource 1, library 1, log 2).
	_1_1_2 := &index{resource: 1, library: 1, record: 2} //revive:disable-line:var-naming
	got = c.subLogs(logs, _1_1_2, nil)

	// Number of logs in subset should be 1.
	assert.Equal(t, 1, got.LogRecordCount())

	// The name of the sole log record should be 1_1_2.
	val, _ = got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get(splunk.DefaultNameLabel)
	assert.Equal(t, "1_1_2", val.AsString())

	// Now see how profiling and log data are merged
	logs = createLogDataWithCustomLibraries(2, []string{"otel.logs", "otel.profiling"}, []int{10, 10})
	slice := &index{resource: 1, library: 0, record: 5}
	profSlice := &index{resource: 0, library: 1, record: 8}

	got = c.subLogs(logs, slice, profSlice)

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

	p, err := io.ReadAll(z)
	require.NoError(t, err)
	for _, e := range expected {
		assert.Contains(t, string(p), e)
	}

}

func BenchmarkPushLogRecords(b *testing.B) {
	logs := createLogData(1, 1, 1)
	c := client{
		config:    NewFactory().CreateDefaultConfig().(*Config),
		logger:    zap.NewNop(),
		hecWorker: &mockHecWorker{},
	}

	state := makeBlankBufferState(4096, true, 4096)
	for n := 0; n < b.N; n++ {
		permanentErrs, sendingErr := c.pushLogRecords(context.Background(), logs.ResourceLogs(), state, map[string]string{})
		assert.NoError(b, sendingErr)
		for _, permanentErr := range permanentErrs {
			assert.NoError(b, permanentErr)
		}
	}
}
