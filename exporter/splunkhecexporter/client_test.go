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
	"compress/gzip"
	"context"
	"errors"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func createMetricsData(numberOfDataPoints int) pdata.Metrics {

	doubleVal := 1234.5678
	metrics := pdata.NewMetrics()
	rm := pdata.NewResourceMetrics()
	rm.InitEmpty()
	rm.Resource().Attributes().InsertString("k0", "v0")
	rm.Resource().Attributes().InsertString("k1", "v1")
	metrics.ResourceMetrics().Append(rm)

	for i := 0; i < numberOfDataPoints; i++ {
		tsUnix := time.Unix(int64(i), int64(i)*time.Millisecond.Nanoseconds())

		ilm := pdata.NewInstrumentationLibraryMetrics()
		ilm.InitEmpty()
		metric := pdata.NewMetric()
		metric.InitEmpty()
		metric.SetName("gauge_double_with_dims")
		metric.SetDataType(pdata.MetricDataTypeDoubleGauge)
		metric.DoubleGauge().InitEmpty()
		doublePt := pdata.NewDoubleDataPoint()
		doublePt.InitEmpty()
		doublePt.SetTimestamp(pdata.TimestampUnixNano(tsUnix.UnixNano()))
		doublePt.SetValue(doubleVal)
		doublePt.LabelsMap().Insert("k/n0", "vn0")
		doublePt.LabelsMap().Insert("k/n1", "vn1")
		doublePt.LabelsMap().Insert("k/r0", "vr0")
		doublePt.LabelsMap().Insert("k/r1", "vr1")
		metric.DoubleGauge().DataPoints().Append(doublePt)
		ilm.Metrics().Append(metric)
		rm.InstrumentationLibraryMetrics().Append(ilm)
	}

	return metrics
}

func createTraceData(numberOfTraces int) pdata.Traces {
	traces := pdata.NewTraces()
	rs := pdata.NewResourceSpans()
	rs.InitEmpty()
	traces.ResourceSpans().Append(rs)
	ils := pdata.NewInstrumentationLibrarySpans()
	ils.InitEmpty()
	rs.InstrumentationLibrarySpans().Append(ils)
	rs.Resource().Attributes().InsertString("resource", "R1")
	for i := 0; i < numberOfTraces; i++ {
		span := pdata.NewSpan()
		span.InitEmpty()
		span.SetName("root")
		span.SetStartTime(pdata.TimestampUnixNano((i + 1) * 1e9))
		span.SetEndTime(pdata.TimestampUnixNano((i + 2) * 1e9))
		span.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
		span.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
		span.SetTraceState("foo")
		if i%2 == 0 {
			span.SetParentSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
			span.Status().InitEmpty()
			span.Status().SetCode(pdata.StatusCodeOk)
			span.Status().SetMessage("ok")

		}
		ils.Spans().Append(span)
	}

	return traces
}

func createLogData(numberOfLogs int) pdata.Logs {
	logs := pdata.NewLogs()
	rl := pdata.NewResourceLogs()
	rl.InitEmpty()
	logs.ResourceLogs().Append(rl)
	ill := pdata.NewInstrumentationLibraryLogs()
	ill.InitEmpty()
	rl.InstrumentationLibraryLogs().Append(ill)

	for i := 0; i < numberOfLogs; i++ {
		ts := pdata.TimestampUnixNano(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := pdata.NewLogRecord()
		logRecord.InitEmpty()
		logRecord.Body().SetStringVal("mylog")
		logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
		logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
		logRecord.Attributes().InsertString(splunk.IndexLabel, "myindex")
		logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
		logRecord.Attributes().InsertString("custom", "custom")
		logRecord.SetTimestamp(ts)

		ill.Logs().Append(logRecord)
	}

	return logs
}

type CapturingData struct {
	testing          *testing.T
	receivedRequest  chan string
	statusCode       int
	checkCompression bool
}

func (c *CapturingData) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.checkCompression {
		if r.Header.Get("Content-Encoding") != "gzip" {
			c.testing.Fatal("No compression")
		}
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	go func() {
		c.receivedRequest <- string(body)
	}()
	w.WriteHeader(c.statusCode)
}

func runMetricsExport(disableCompression bool, numberOfDataPoints int, t *testing.T) (string, error) {
	receivedRequest := make(chan string)
	capture := CapturingData{testing: t, receivedRequest: receivedRequest, statusCode: 200, checkCompression: !disableCompression}
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
	cfg.DisableCompression = disableCompression
	cfg.Token = "1234-1234"

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer exporter.Shutdown(context.Background())

	md := createMetricsData(numberOfDataPoints)

	err = exporter.ConsumeMetrics(context.Background(), md)
	assert.NoError(t, err)
	select {
	case request := <-receivedRequest:
		return request, nil
	case <-time.After(1 * time.Second):
		return "", errors.New("Timeout")
	}
}

func runTraceExport(disableCompression bool, numberOfTraces int, t *testing.T) (string, error) {
	receivedRequest := make(chan string)
	capture := CapturingData{testing: t, receivedRequest: receivedRequest, statusCode: 200, checkCompression: !disableCompression}
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
	cfg.DisableCompression = disableCompression
	cfg.Token = "1234-1234"

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer exporter.Shutdown(context.Background())

	td := createTraceData(numberOfTraces)

	err = exporter.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)
	select {
	case request := <-receivedRequest:
		return request, nil
	case <-time.After(1 * time.Second):
		return "", errors.New("Timeout")
	}
}

func runLogExport(disableCompression bool, numberOfLogs int, t *testing.T) (string, error) {
	receivedRequest := make(chan string)
	capture := CapturingData{testing: t, receivedRequest: receivedRequest, statusCode: 200, checkCompression: !disableCompression}
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
	cfg.DisableCompression = disableCompression
	cfg.Token = "1234-1234"

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	defer exporter.Shutdown(context.Background())

	ld := createLogData(numberOfLogs)

	err = exporter.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)
	select {
	case request := <-receivedRequest:
		return request, nil
	case <-time.After(1 * time.Second):
		return "", errors.New("Timeout")
	}
}

func TestReceiveTraces(t *testing.T) {
	actual, err := runTraceExport(true, 3, t)
	assert.NoError(t, err)
	expected := `{"time":1,"host":"unknown","event":{"trace_id":"01010101010101010101010101010101","span_id":"0000000000000001","parent_span_id":"0102030405060708","name":"root","end_time":2000000000,"kind":"SPAN_KIND_UNSPECIFIED","status":{"message":"ok","code":"STATUS_CODE_OK"},"start_time":1000000000},"fields":{"resource":"R1"}}`
	expected += "\n\r\n\r\n"
	expected += `{"time":2,"host":"unknown","event":{"trace_id":"01010101010101010101010101010101","span_id":"0000000000000001","parent_span_id":"","name":"root","end_time":3000000000,"kind":"SPAN_KIND_UNSPECIFIED","status":{"message":"","code":""},"start_time":2000000000},"fields":{"resource":"R1"}}`
	expected += "\n\r\n\r\n"
	expected += `{"time":3,"host":"unknown","event":{"trace_id":"01010101010101010101010101010101","span_id":"0000000000000001","parent_span_id":"0102030405060708","name":"root","end_time":4000000000,"kind":"SPAN_KIND_UNSPECIFIED","status":{"message":"ok","code":"STATUS_CODE_OK"},"start_time":3000000000},"fields":{"resource":"R1"}}`
	expected += "\n\r\n\r\n"
	assert.Equal(t, expected, actual)
}

func TestReceiveLogs(t *testing.T) {
	actual, err := runLogExport(true, 3, t)
	assert.NoError(t, err)
	expected := `{"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom"}}`
	expected += "\n\r\n\r\n"
	expected += `{"time":0.001,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom"}}`
	expected += "\n\r\n\r\n"
	expected += `{"time":0.002,"host":"myhost","source":"myapp","sourcetype":"myapp-type","index":"myindex","event":"mylog","fields":{"custom":"custom"}}`
	expected += "\n\r\n\r\n"
	assert.Equal(t, expected, actual)
}

func TestReceiveMetrics(t *testing.T) {
	actual, err := runMetricsExport(true, 3, t)
	assert.NoError(t, err)
	expected := `{"host":"unknown","event":"metric","fields":{"k/n0":"vn0","k/n1":"vn1","k/r0":"vr0","k/r1":"vr1","k0":"v0","k1":"v1","metric_name:gauge_double_with_dims":1234.5678}}`
	expected += "\n\r\n\r\n"
	expected += `{"time":1.001,"host":"unknown","event":"metric","fields":{"k/n0":"vn0","k/n1":"vn1","k/r0":"vr0","k/r1":"vr1","k0":"v0","k1":"v1","metric_name:gauge_double_with_dims":1234.5678}}`
	expected += "\n\r\n\r\n"
	expected += `{"time":2.002,"host":"unknown","event":"metric","fields":{"k/n0":"vn0","k/n1":"vn1","k/r0":"vr0","k/r1":"vr1","k0":"v0","k1":"v1","metric_name:gauge_double_with_dims":1234.5678}}`
	expected += "\n\r\n\r\n"
	assert.Equal(t, expected, actual)
}

func TestReceiveTracesWithCompression(t *testing.T) {
	request, err := runTraceExport(false, 1000, t)
	assert.NoError(t, err)
	assert.NotEqual(t, "", request)
}

func TestReceiveLogsWithCompression(t *testing.T) {
	request, err := runLogExport(false, 1000, t)
	assert.NoError(t, err)
	assert.NotEqual(t, "", request)
}

func TestReceiveMetricsWithCompression(t *testing.T) {
	request, err := runMetricsExport(false, 1000, t)
	assert.NoError(t, err)
	assert.NotEqual(t, "", request)
}

func TestErrorReceived(t *testing.T) {
	receivedRequest := make(chan string)
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

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
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
	_, err := runLogExport(false, 0, t)
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
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
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
