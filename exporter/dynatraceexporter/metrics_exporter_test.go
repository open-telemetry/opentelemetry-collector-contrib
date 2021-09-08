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

package dynatraceexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
)

var testTimestamp = pdata.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano())

func Test_exporter_PushMetricsData(t *testing.T) {
	sent := "not sent"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := ioutil.ReadAll(r.Body)
		sent = string(bodyBytes)

		response := metricsResponse{
			Ok:      0,
			Invalid: 0,
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))
	defer ts.Close()

	md := pdata.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(2)
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()

	badNameMetric := metrics.AppendEmpty()
	badNameMetric.SetName("")

	noneMetric := metrics.AppendEmpty()
	noneMetric.SetName("none")

	intGaugeMetric := metrics.AppendEmpty()
	intGaugeMetric.SetDataType(pdata.MetricDataTypeGauge)
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.Gauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.SetIntVal(10)
	intGaugeDataPoint.SetTimestamp(testTimestamp)

	intSumMetric := metrics.AppendEmpty()
	intSumMetric.SetDataType(pdata.MetricDataTypeSum)
	intSumMetric.SetName("int_sum")
	intSum := intSumMetric.Sum()
	intSumDataPoints := intSum.DataPoints()
	intSumDataPoint := intSumDataPoints.AppendEmpty()
	intSumDataPoint.SetIntVal(10)
	intSumDataPoint.SetTimestamp(testTimestamp)

	doubleGaugeMetric := metrics.AppendEmpty()
	doubleGaugeMetric.SetDataType(pdata.MetricDataTypeGauge)
	doubleGaugeMetric.SetName("double_gauge")
	doubleGauge := doubleGaugeMetric.Gauge()
	doubleGaugeDataPoints := doubleGauge.DataPoints()
	doubleGaugeDataPoint := doubleGaugeDataPoints.AppendEmpty()
	doubleGaugeDataPoint.SetDoubleVal(10.1)
	doubleGaugeDataPoint.SetTimestamp(testTimestamp)

	doubleSumMetric := metrics.AppendEmpty()
	doubleSumMetric.SetDataType(pdata.MetricDataTypeSum)
	doubleSumMetric.SetName("double_sum")
	doubleSum := doubleSumMetric.Sum()
	doubleSumDataPoints := doubleSum.DataPoints()
	doubleSumDataPoint := doubleSumDataPoints.AppendEmpty()
	doubleSumDataPoint.SetDoubleVal(10.1)
	doubleSumDataPoint.SetTimestamp(testTimestamp)

	doubleHistogramMetric := metrics.AppendEmpty()
	doubleHistogramMetric.SetDataType(pdata.MetricDataTypeHistogram)
	doubleHistogramMetric.SetName("double_histogram")
	doubleHistogram := doubleHistogramMetric.Histogram()
	doubleHistogramDataPoints := doubleHistogram.DataPoints()
	doubleHistogramDataPoint := doubleHistogramDataPoints.AppendEmpty()
	doubleHistogramDataPoint.SetCount(2)
	doubleHistogramDataPoint.SetSum(10.1)
	doubleHistogramDataPoint.SetTimestamp(testTimestamp)

	type fields struct {
		logger *zap.Logger
		cfg    *config.Config
		client *http.Client
	}
	type args struct {
		ctx context.Context
		md  pdata.Metrics
	}
	test := struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		name: "Send metric data",
		fields: fields{
			logger: zap.NewNop(),
			cfg: &config.Config{
				APIToken:           "token",
				HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
				Prefix:             "prefix",
				Tags:               []string{},
			},
			client: ts.Client(),
		},
		args: args{
			ctx: context.Background(),
			md:  md,
		},
		wantErr: false,
	}

	t.Run(test.name, func(t *testing.T) {
		e := &exporter{
			logger: test.fields.logger,
			cfg:    test.fields.cfg,
			client: test.fields.client,
		}
		err := e.PushMetricsData(test.args.ctx, test.args.md)
		if (err != nil) != test.wantErr {
			t.Errorf("exporter.PushMetricsData() error = %v, wantErr %v", err, test.wantErr)
			return
		}
	})

	if wantBody := "prefix.int_gauge gauge,10 1626438600000\nprefix.int_sum gauge,10 1626438600000\nprefix.double_gauge gauge,10.1 1626438600000\nprefix.double_sum gauge,10.1 1626438600000\nprefix.double_histogram gauge,min=5.05,max=5.05,sum=10.1,count=2 1626438600000"; sent != wantBody {
		t.Errorf("exporter.PushMetricsData():ResponseBody = %v, want %v", sent, wantBody)
	}
}

func Test_exporter_PushMetricsData_EmptyPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Server should not be called")
	}))
	defer ts.Close()

	md := pdata.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(2)
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()
	noneMetric := metrics.AppendEmpty()
	noneMetric.SetName("none")

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}
	err := e.PushMetricsData(context.Background(), md)
	if err != nil {
		t.Errorf("exporter.PushMetricsData() error = %v", err)
		return
	}
}

func Test_exporter_PushMetricsData_isDisabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Server should not be called")
	}))
	defer ts.Close()

	md := pdata.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(2)
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()
	metric := metrics.AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.SetName("int_gauge")
	intGauge := metric.Gauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.SetIntVal(10)
	intGaugeDataPoint.SetTimestamp(testTimestamp)

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client:     ts.Client(),
		isDisabled: true,
	}
	err := e.PushMetricsData(context.Background(), md)
	if err != nil {
		t.Errorf("exporter.PushMetricsData() error = %v", err)
		return
	}
}

func Test_exporter_send_BadRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		body, _ := json.Marshal(metricsResponse{
			Ok:      0,
			Invalid: 10,
		})
		w.Write(body)
	}))
	defer ts.Close()

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}
	invalid, err := e.send(context.Background(), []string{""})
	if invalid != 10 {
		t.Errorf("Expected 10 lines to be reported invalid")
		return
	}
	if consumererror.IsPermanent(err) {
		t.Errorf("Expected error to not be permanent %v", err)
		return
	}
	if e.isDisabled {
		t.Error("Expected exporter to not be disabled")
		return
	}
}

func Test_exporter_send_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte{})
	}))
	defer ts.Close()

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}
	_, err := e.send(context.Background(), []string{""})
	if !consumererror.IsPermanent(err) {
		t.Errorf("Expected error to be permanent %v", err)
		return
	}
	if !e.isDisabled {
		t.Error("Expected exporter to be disabled")
		return
	}
}

func Test_exporter_send_TooLarge(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte{})
	}))
	defer ts.Close()

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}
	_, err := e.send(context.Background(), []string{""})
	if !consumererror.IsPermanent(err) {
		t.Errorf("Expected error to be permanent %v", err)
		return
	}
	if e.isDisabled {
		t.Error("Expected exporter not to be disabled")
		return
	}
}

func Test_exporter_send_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte{})
	}))
	defer ts.Close()

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			APIToken:           "token",
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
			Prefix:             "prefix",
			Tags:               []string{},
		},
		client: ts.Client(),
	}
	_, err := e.send(context.Background(), []string{""})
	if !consumererror.IsPermanent(err) {
		t.Errorf("Expected error to be permanent %v", err)
		return
	}
	if !e.isDisabled {
		t.Error("Expected exporter to be disabled")
		return
	}
}

func Test_exporter_send_chunking(t *testing.T) {
	sentChunks := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		body, _ := json.Marshal(metricsResponse{
			Ok:      0,
			Invalid: 1,
		})
		w.Write(body)
		sentChunks++
	}))
	defer ts.Close()

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}

	batch := make([]string, 1001)

	for i := 0; i < 1001; i++ {
		batch[i] = fmt.Sprintf("%d", i)
	}

	invalid, err := e.send(context.Background(), batch)
	if sentChunks != 2 {
		t.Errorf("Expected batch to be sent in 2 chunks")
	}
	if invalid != 2 {
		t.Errorf("Expected 2 lines to be reported invalid")
		return
	}
	if consumererror.IsPermanent(err) {
		t.Errorf("Expected error to not be permanent %v", err)
		return
	}
	if e.isDisabled {
		t.Error("Expected exporter to not be disabled")
		return
	}
}

func Test_exporter_PushMetricsData_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	ts.Close()

	md := pdata.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(2)
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()
	intGaugeMetric := metrics.AppendEmpty()
	intGaugeMetric.SetDataType(pdata.MetricDataTypeGauge)
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.Gauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.SetIntVal(10)
	intGaugeDataPoint.SetTimestamp(testTimestamp)

	type fields struct {
		logger *zap.Logger
		cfg    *config.Config
		client *http.Client
	}
	type args struct {
		ctx context.Context
		md  pdata.Metrics
	}
	test := struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		name: "When the client errors, all timeseries are assumed to be dropped",
		fields: fields{
			logger: zap.NewNop(),
			cfg: &config.Config{
				APIToken:           "token",
				HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
				Prefix:             "prefix",
				Tags:               []string{},
			},
			client: ts.Client(),
		},
		args: args{
			ctx: context.Background(),
			md:  md,
		},
		wantErr: true,
	}

	t.Run(test.name, func(t *testing.T) {
		e := &exporter{
			logger: test.fields.logger,
			cfg:    test.fields.cfg,
			client: test.fields.client,
		}
		err := e.PushMetricsData(test.args.ctx, test.args.md)
		if (err != nil) != test.wantErr {
			t.Errorf("exporter.PushMetricsData() error = %v, wantErr %v", err, test.wantErr)
			return
		}
	})
}

func Test_exporter_start_InvalidHTTPClientSettings(t *testing.T) {
	config := &config.Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "localhost:9090",
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: "/non/existent",
				},
			},
		},
	}

	exp := newMetricsExporter(componenttest.NewNopExporterCreateSettings(), config)

	err := exp.start(context.Background(), componenttest.NewNopHost())
	if err == nil {
		t.Errorf("Expected error when creating a metrics exporter with invalid HTTP Client Settings")
		return
	}
}
