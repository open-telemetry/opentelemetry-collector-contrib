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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

type testRoundTripper func(req *http.Request) *http.Response

func (t testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t(req), nil
}

func newTestClient(respCode int, respBody string) *http.Client {
	return &http.Client{
		Transport: testRoundTripper(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: respCode,
				Body:       ioutil.NopCloser(bytes.NewBufferString(respBody)),
				Header:     make(http.Header),
			}
		}),
	}
}

func createMetricsData(numberOfDataPoints int) pdata.Metrics {

	doubleVal := 1234.5678
	metrics := pdata.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().InsertString("k0", "v0")
	rm.Resource().Attributes().InsertString("k1", "v1")

	for i := 0; i < numberOfDataPoints; i++ {
		tsUnix := time.Unix(int64(i), int64(i)*time.Millisecond.Nanoseconds())

		ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
		metric := ilm.Metrics().AppendEmpty()
		metric.SetName("gauge_double_with_dims")
		metric.SetDataType(pdata.MetricDataTypeDoubleGauge)
		doublePt := metric.DoubleGauge().DataPoints().AppendEmpty()
		doublePt.SetTimestamp(pdata.TimestampFromTime(tsUnix))
		doublePt.SetValue(doubleVal)
		doublePt.LabelsMap().Insert("k/n0", "vn0")
		doublePt.LabelsMap().Insert("k/n1", "vn1")
		doublePt.LabelsMap().Insert("k/r0", "vr0")
		doublePt.LabelsMap().Insert("k/r1", "vr1")
	}

	return metrics
}

func createTraceData(numberOfTraces int) pdata.Traces {
	traces := pdata.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().InsertString("resource", "R1")
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()
	ils.Spans().Resize(numberOfTraces)
	for i := 0; i < numberOfTraces; i++ {
		span := ils.Spans().At(i)
		span.SetName("root")
		span.SetStartTimestamp(pdata.Timestamp((i + 1) * 1e9))
		span.SetEndTimestamp(pdata.Timestamp((i + 2) * 1e9))
		span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
		span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
		span.SetTraceState("foo")
		if i%2 == 0 {
			span.SetParentSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
			span.Status().SetCode(pdata.StatusCodeOk)
			span.Status().SetMessage("ok")
		}
	}

	return traces
}

func createLogData(numResources int, numLibraries int, numRecords int) pdata.Logs {
	logs := pdata.NewLogs()
	logs.ResourceLogs().Resize(numResources)
	for i := 0; i < numResources; i++ {
		rl := logs.ResourceLogs().At(i)
		rl.InstrumentationLibraryLogs().Resize(numLibraries)
		for j := 0; j < numLibraries; j++ {
			ill := rl.InstrumentationLibraryLogs().At(j)
			ill.Logs().Resize(numRecords)
			for k := 0; k < numRecords; k++ {
				ts := pdata.Timestamp(int64(k) * time.Millisecond.Nanoseconds())
				logRecord := ill.Logs().At(k)
				logRecord.SetName(fmt.Sprintf("%d_%d_%d", i, j, k))
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(splunk.IndexLabel, "myindex")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
			}
		}
	}

	return logs
}

type CapturingData struct {
	testing          *testing.T
	receivedRequest  chan []byte
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
		c.receivedRequest <- body
	}()
	w.WriteHeader(c.statusCode)
}

func runMetricsExport(disableCompression bool, numberOfDataPoints int, t *testing.T) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.DisableCompression = disableCompression
	cfg.Token = "1234-1234"

	receivedRequest := make(chan []byte)
	capture := CapturingData{testing: t, receivedRequest: receivedRequest, statusCode: 200, checkCompression: !cfg.DisableCompression}
	s := &http.Server{
		Handler: &capture,
	}
	go func() {
		panic(s.Serve(listener))
	}()

	params := component.ExporterCreateSettings{Logger: zap.NewNop()}
	exporter, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer exporter.Shutdown(context.Background())

	md := createMetricsData(numberOfDataPoints)

	err = exporter.ConsumeMetrics(context.Background(), md)
	assert.NoError(t, err)
	select {
	case request := <-receivedRequest:
		return string(request), nil
	case <-time.After(1 * time.Second):
		return "", errors.New("timeout")
	}
}

func runTraceExport(disableCompression bool, numberOfTraces int, t *testing.T) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.DisableCompression = disableCompression
	cfg.Token = "1234-1234"

	receivedRequest := make(chan []byte)
	capture := CapturingData{testing: t, receivedRequest: receivedRequest, statusCode: 200, checkCompression: !cfg.DisableCompression}
	s := &http.Server{
		Handler: &capture,
	}
	go func() {
		panic(s.Serve(listener))
	}()

	params := component.ExporterCreateSettings{Logger: zap.NewNop()}
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer exporter.Shutdown(context.Background())

	td := createTraceData(numberOfTraces)

	err = exporter.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)
	select {
	case request := <-receivedRequest:
		return string(request), nil
	case <-time.After(1 * time.Second):
		return "", errors.New("timeout")
	}
}

func runLogExport(cfg *Config, ld pdata.Logs, t *testing.T) ([][]byte, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	cfg.Endpoint = "http://" + listener.Addr().String() + "/services/collector"
	cfg.Token = "1234-1234"

	receivedRequest := make(chan []byte)
	capture := CapturingData{testing: t, receivedRequest: receivedRequest, statusCode: 200, checkCompression: !cfg.DisableCompression}
	s := &http.Server{
		Handler: &capture,
	}
	go func() {
		panic(s.Serve(listener))
	}()

	params := component.ExporterCreateSettings{Logger: zap.NewNop()}
	exporter, err := NewFactory().CreateLogsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer exporter.Shutdown(context.Background())

	err = exporter.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)

	var requests [][]byte
	for {
		select {
		case request := <-receivedRequest:
			requests = append(requests, request)
		case <-time.After(1 * time.Second):
			if len(requests) == 0 {
				err = errors.New("timeout")
			}
			return requests, err
		}
	}
}

func TestReceiveTraces(t *testing.T) {
	actual, err := runTraceExport(true, 3, t)
	assert.NoError(t, err)
	expected := `{"time":1,"host":"unknown","event":{"trace_id":"01010101010101010101010101010101","span_id":"0000000000000001","parent_span_id":"0102030405060708","name":"root","end_time":2000000000,"kind":"SPAN_KIND_UNSPECIFIED","status":{"message":"ok","code":"STATUS_CODE_OK"},"start_time":1000000000},"fields":{"resource":"R1"}}`
	expected += "\n"
	expected += `{"time":2,"host":"unknown","event":{"trace_id":"01010101010101010101010101010101","span_id":"0000000000000001","parent_span_id":"","name":"root","end_time":3000000000,"kind":"SPAN_KIND_UNSPECIFIED","status":{"message":"","code":"STATUS_CODE_UNSET"},"start_time":2000000000},"fields":{"resource":"R1"}}`
	expected += "\n"
	expected += `{"time":3,"host":"unknown","event":{"trace_id":"01010101010101010101010101010101","span_id":"0000000000000001","parent_span_id":"0102030405060708","name":"root","end_time":4000000000,"kind":"SPAN_KIND_UNSPECIFIED","status":{"message":"ok","code":"STATUS_CODE_OK"},"start_time":3000000000},"fields":{"resource":"R1"}}`
	expected += "\n"
	assert.Equal(t, expected, actual)
}

func TestReceiveLogs(t *testing.T) {
	type wantType struct {
		batches    []string
		numBatches int
		compressed bool
	}

	// The test cases depend on the constant minCompressionLen = 1500.
	// If the constant changed, the test cases with want.compressed=true must be updated.
	require.Equal(t, minCompressionLen, 1500)

	tests := []struct {
		name string
		conf *Config
		logs pdata.Logs
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
				batches: []string{
					`{"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_0","service.name":"myapp"}}` + "\n" +
						`{"time":0.001,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_1","service.name":"myapp"}}` + "\n" +
						`{"time":0.002,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_2","service.name":"myapp"}}` + "\n" +
						`{"time":0.003,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_3","service.name":"myapp"}}` + "\n",
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
				batches: []string{
					`{"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_0","service.name":"myapp"}}` + "\n",
					`{"time":0.001,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_1","service.name":"myapp"}}` + "\n",
					`{"time":0.002,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_2","service.name":"myapp"}}` + "\n",
					`{"time":0.003,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_3","service.name":"myapp"}}` + "\n",
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
				batches: []string{
					`{"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_0","service.name":"myapp"}}` + "\n" +
						`{"time":0.001,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_1","service.name":"myapp"}}` + "\n",
					`{"time":0.002,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_2","service.name":"myapp"}}` + "\n" +
						`{"time":0.003,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_3","service.name":"myapp"}}` + "\n",
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
				batches: []string{
					`{"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_0","service.name":"myapp"}}` + "\n" +
						`{"time":0.001,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_1","service.name":"myapp"}}` + "\n" +
						`{"time":0.002,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_2","service.name":"myapp"}}` + "\n" +
						`{"time":0.003,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_3","service.name":"myapp"}}` + "\n" +
						`{"time":0.004,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_4","service.name":"myapp"}}` + "\n" +
						`{"time":0.005,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_5","service.name":"myapp"}}` + "\n" +
						`{"time":0.006,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_6","service.name":"myapp"}}` + "\n" +
						`{"time":0.007,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_7","service.name":"myapp"}}` + "\n" +
						`{"time":0.008,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_8","service.name":"myapp"}}` + "\n" +
						`{"time":0.009,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_9","service.name":"myapp"}}` + "\n",
				},
				numBatches: 1,
				compressed: true,
			},
		},
		{
			name: "2 compressed batches - 1832 bytes each, make sure the log size is more than minCompressionLen=1500 to trigger compression",
			logs: createLogData(1, 1, 18),
			conf: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MaxContentLengthLogs = 1916
				return cfg
			}(),
			want: wantType{
				batches: []string{
					`{"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_0","service.name":"myapp"}}` + "\n" +
						`{"time":0.001,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_1","service.name":"myapp"}}` + "\n" +
						`{"time":0.002,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_2","service.name":"myapp"}}` + "\n" +
						`{"time":0.003,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_3","service.name":"myapp"}}` + "\n" +
						`{"time":0.004,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_4","service.name":"myapp"}}` + "\n" +
						`{"time":0.005,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_5","service.name":"myapp"}}` + "\n" +
						`{"time":0.006,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_6","service.name":"myapp"}}` + "\n" +
						`{"time":0.007,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_7","service.name":"myapp"}}` + "\n" +
						`{"time":0.008,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_8","service.name":"myapp"}}` + "\n",
					`{"time":0.009,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_9","service.name":"myapp"}}` + "\n" +
						`{"time":0.01,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_10","service.name":"myapp"}}` + "\n" +
						`{"time":0.011,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_11","service.name":"myapp"}}` + "\n" +
						`{"time":0.012,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_12","service.name":"myapp"}}` + "\n" +
						`{"time":0.013,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_13","service.name":"myapp"}}` + "\n" +
						`{"time":0.014,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_14","service.name":"myapp"}}` + "\n" +
						`{"time":0.015,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_15","service.name":"myapp"}}` + "\n" +
						`{"time":0.016,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_16","service.name":"myapp"}}` + "\n" +
						`{"time":0.017,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom","host.name":"myhost","otlp.log.name":"0_0_17","service.name":"myapp"}}` + "\n",
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
					validateCompressedEqual(t, test.want.batches[i], got[i])
				} else {
					assert.Equal(t, test.want.batches[i], string(got[i]))
				}

			}
		})
	}
}

func TestReceiveMetrics(t *testing.T) {
	actual, err := runMetricsExport(true, 3, t)
	assert.NoError(t, err)
	expected := `{"host":"unknown","event":"metric","fields":{"k/n0":"vn0","k/n1":"vn1","k/r0":"vr0","k/r1":"vr1","k0":"v0","k1":"v1","metric_name:gauge_double_with_dims":1234.5678,"metric_type":"DoubleGauge"}}`
	expected += "\n"
	expected += `{"time":1.001,"host":"unknown","event":"metric","fields":{"k/n0":"vn0","k/n1":"vn1","k/r0":"vr0","k/r1":"vr1","k0":"v0","k1":"v1","metric_name:gauge_double_with_dims":1234.5678,"metric_type":"DoubleGauge"}}`
	expected += "\n"
	expected += `{"time":2.002,"host":"unknown","event":"metric","fields":{"k/n0":"vn0","k/n1":"vn1","k/r0":"vr0","k/r1":"vr1","k0":"v0","k1":"v1","metric_name:gauge_double_with_dims":1234.5678,"metric_type":"DoubleGauge"}}`
	expected += "\n"
	assert.Equal(t, expected, actual)
}

func TestReceiveTracesWithCompression(t *testing.T) {
	request, err := runTraceExport(false, 1000, t)
	assert.NoError(t, err)
	assert.NotEqual(t, "", request)
}

func TestReceiveMetricsWithCompression(t *testing.T) {
	request, err := runMetricsExport(false, 1000, t)
	assert.NoError(t, err)
	assert.NotEqual(t, "", request)
}

func TestErrorReceived(t *testing.T) {
	receivedRequest := make(chan []byte)
	capture := CapturingData{receivedRequest: receivedRequest, statusCode: 500}
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

	params := component.ExporterCreateSettings{Logger: zap.NewNop()}
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer exporter.Shutdown(context.Background())

	td := createTraceData(3)

	err = exporter.ConsumeTraces(context.Background(), td)
	select {
	case <-receivedRequest:
	case <-time.After(5 * time.Second):
		t.Fatal("Should have received request")
	}
	assert.EqualError(t, err, "HTTP 500 \"Internal Server Error\"")
}

func TestInvalidTraces(t *testing.T) {
	_, err := runTraceExport(false, 0, t)
	assert.Error(t, err)
}

func TestInvalidLogs(t *testing.T) {
	config := NewFactory().CreateDefaultConfig().(*Config)
	config.DisableCompression = false
	_, err := runLogExport(config, createLogData(1, 1, 0), t)
	assert.Error(t, err)
}

func TestInvalidMetrics(t *testing.T) {
	_, err := runMetricsExport(false, 0, t)
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
	params := component.ExporterCreateSettings{Logger: zap.NewNop()}
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer exporter.Shutdown(context.Background())

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
	assert.EqualError(t, err, "Permanent error: json: unsupported value: +Inf")
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
		logs     pdata.Logs
		requires func(*testing.T, pdata.Logs)
	}{
		{
			name: func(disable bool) string {
				return "COMPRESSION " + map[bool]string{true: "DISABLED ", false: "ENABLED "}[disable] + "nil ResourceLogs"
			},
			logs: pdata.NewLogs(),
			requires: func(t *testing.T, logs pdata.Logs) {
				require.Zero(t, logs.ResourceLogs().Len())
			},
		},
		{
			name: func(disable bool) string {
				return "COMPRESSION " + map[bool]string{true: "DISABLED ", false: "ENABLED "}[disable] + "nil InstrumentationLogs"
			},
			logs: func() pdata.Logs {
				logs := pdata.NewLogs()
				logs.ResourceLogs().AppendEmpty()
				return logs
			}(),
			requires: func(t *testing.T, logs pdata.Logs) {
				require.Equal(t, logs.ResourceLogs().Len(), 1)
				require.Zero(t, logs.ResourceLogs().At(0).InstrumentationLibraryLogs().Len())
			},
		},
		{
			name: func(disable bool) string {
				return "COMPRESSION " + map[bool]string{true: "DISABLED ", false: "ENABLED "}[disable] + "nil LogRecords"
			},
			logs: func() pdata.Logs {
				logs := pdata.NewLogs()
				logs.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty()
				return logs
			}(),
			requires: func(t *testing.T, logs pdata.Logs) {
				require.Equal(t, logs.ResourceLogs().Len(), 1)
				require.Equal(t, logs.ResourceLogs().At(0).InstrumentationLibraryLogs().Len(), 1)
				require.Zero(t, logs.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().Len())
			},
		},
	}

	c := client{
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
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
		config: &Config{},
	}

	logs := pdata.NewLogs()
	log := logs.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty().Logs().AppendEmpty()
	// Invalid log value
	log.Body().SetDoubleVal(math.Inf(1))

	err := c.pushLogData(context.Background(), logs)

	assert.Contains(t, err.Error(), "json: unsupported value: +Inf")
}

func Test_pushLogData_PostError(t *testing.T) {
	c := client{
		url: &url.URL{Host: "in va lid"},
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
	}

	// 2000 log records -> ~371888 bytes when JSON encoded.
	logs := createLogData(1, 1, 2000)

	// 0 -> unlimited size batch, true -> compression disabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 0, true
	err := c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.IsType(t, consumererror.Logs{}, err)
	assert.Equal(t, (err.(consumererror.Logs)).GetLogs(), logs)

	// 0 -> unlimited size batch, false -> compression enabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 0, false
	err = c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.IsType(t, consumererror.Logs{}, err)
	assert.Equal(t, (err.(consumererror.Logs)).GetLogs(), logs)

	// 200000 < 371888 -> multiple batches, true -> compression disabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 200000, true
	err = c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.IsType(t, consumererror.Logs{}, err)
	assert.Equal(t, (err.(consumererror.Logs)).GetLogs(), logs)

	// 200000 < 371888 -> multiple batches, false -> compression enabled.
	c.config.MaxContentLengthLogs, c.config.DisableCompression = 200000, false
	err = c.pushLogData(context.Background(), logs)
	require.Error(t, err)
	assert.IsType(t, consumererror.Logs{}, err)
	assert.Equal(t, (err.(consumererror.Logs)).GetLogs(), logs)
}

func Test_pushLogData_ShouldAddResponseTo400Error(t *testing.T) {
	splunkClient := client{
		url: &url.URL{Scheme: "http", Host: "splunk"},
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
	}
	logs := createLogData(1, 1, 1)

	responseBody := `some error occurred`

	// An HTTP client that returns status code 400 and response body responseBody.
	splunkClient.client = newTestClient(400, responseBody)
	// Sending logs using the client.
	err := splunkClient.pushLogData(context.Background(), logs)
	// TODO: Uncomment after consumererror.Logs implements method Unwrap.
	//require.True(t, consumererror.IsPermanent(err), "Expecting permanent error")
	require.Contains(t, err.Error(), "HTTP/0.0 400")
	// The returned error should contain the response body responseBody.
	assert.Contains(t, err.Error(), responseBody)

	// An HTTP client that returns some other status code other than 400 and response body responseBody.
	splunkClient.client = newTestClient(500, responseBody)
	// Sending logs using the client.
	err = splunkClient.pushLogData(context.Background(), logs)
	// TODO: Uncomment after consumererror.Logs implements method Unwrap.
	//require.False(t, consumererror.IsPermanent(err), "Expecting non-permanent error")
	require.Contains(t, err.Error(), "HTTP 500")
	// The returned error should not contain the response body responseBody.
	assert.NotContains(t, err.Error(), responseBody)
}

func Test_pushLogData_Small_MaxContentLength(t *testing.T) {
	c := client{
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		config: NewFactory().CreateDefaultConfig().(*Config),
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

func TestSubLogs(t *testing.T) {
	// Creating 12 logs (2 resources x 2 libraries x 3 records)
	logs := createLogData(2, 2, 3)

	// Logs subset from leftmost index (resource 0, library 0, record 0).
	_0_0_0 := &logIndex{resource: 0, library: 0, record: 0} //revive:disable-line:var-naming
	got := subLogs(&logs, _0_0_0)

	// Number of logs in subset should equal original logs.
	assert.Equal(t, logs.LogRecordCount(), got.LogRecordCount())

	// The name of the leftmost log record should be 0_0_0.
	assert.Equal(t, "0_0_0", got.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Name())
	// The name of the rightmost log record should be 1_1_2.
	assert.Equal(t, "1_1_2", got.ResourceLogs().At(1).InstrumentationLibraryLogs().At(1).Logs().At(2).Name())

	// Logs subset from some mid index (resource 0, library 1, log 2).
	_0_1_2 := &logIndex{resource: 0, library: 1, record: 2} //revive:disable-line:var-naming
	got = subLogs(&logs, _0_1_2)

	assert.Equal(t, 7, got.LogRecordCount())

	// The name of the leftmost log record should be 0_1_2.
	assert.Equal(t, "0_1_2", got.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Name())
	// The name of the rightmost log record should be 1_1_2.
	assert.Equal(t, "1_1_2", got.ResourceLogs().At(1).InstrumentationLibraryLogs().At(1).Logs().At(2).Name())

	// Logs subset from rightmost index (resource 1, library 1, log 2).
	_1_1_2 := &logIndex{resource: 1, library: 1, record: 2} //revive:disable-line:var-naming
	got = subLogs(&logs, _1_1_2)

	// Number of logs in subset should be 1.
	assert.Equal(t, 1, got.LogRecordCount())

	// The name of the sole log record should be 1_1_2.
	assert.Equal(t, "1_1_2", got.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Name())
}

// validateCompressedEqual validates that GZipped `got` contains `expected` string
func validateCompressedEqual(t *testing.T, expected string, got []byte) {
	z, err := gzip.NewReader(bytes.NewReader(got))
	require.NoError(t, err)
	defer z.Close()

	p, err := ioutil.ReadAll(z)
	require.NoError(t, err)

	assert.Equal(t, expected, string(p))
}
