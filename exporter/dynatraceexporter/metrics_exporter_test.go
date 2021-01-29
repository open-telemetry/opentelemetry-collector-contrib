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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
)

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
	md.ResourceMetrics().Resize(2)
	rm := md.ResourceMetrics().At(0)

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Resize(2)
	ilm := ilms.At(0)

	metrics := ilm.Metrics()
	metrics.Resize(8)

	badNameMetric := metrics.At(0)
	badNameMetric.SetName("")

	noneMetric := metrics.At(1)
	noneMetric.SetName("none")

	intGaugeMetric := metrics.At(2)
	intGaugeMetric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.IntGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoints.Resize(1)
	intGaugeDataPoint := intGaugeDataPoints.At(0)
	intGaugeDataPoint.SetValue(10)
	intGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	intSumMetric := metrics.At(3)
	intSumMetric.SetDataType(pdata.MetricDataTypeIntSum)
	intSumMetric.SetName("int_sum")
	intSum := intSumMetric.IntSum()
	intSumDataPoints := intSum.DataPoints()
	intSumDataPoints.Resize(1)
	intSumDataPoint := intSumDataPoints.At(0)
	intSumDataPoint.SetValue(10)
	intSumDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	intHistogramMetric := metrics.At(4)
	intHistogramMetric.SetDataType(pdata.MetricDataTypeIntHistogram)
	intHistogramMetric.SetName("double_histogram")
	intHistogram := intHistogramMetric.IntHistogram()
	intHistogramDataPoints := intHistogram.DataPoints()
	intHistogramDataPoints.Resize(1)
	intHistogramDataPoint := intHistogramDataPoints.At(0)
	intHistogramDataPoint.SetCount(2)
	intHistogramDataPoint.SetSum(19)
	intHistogramDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	doubleGaugeMetric := metrics.At(5)
	doubleGaugeMetric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	doubleGaugeMetric.SetName("double_gauge")
	doubleGauge := doubleGaugeMetric.DoubleGauge()
	doubleGaugeDataPoints := doubleGauge.DataPoints()
	doubleGaugeDataPoints.Resize(1)
	doubleGaugeDataPoint := doubleGaugeDataPoints.At(0)
	doubleGaugeDataPoint.SetValue(10.1)
	doubleGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	doubleSumMetric := metrics.At(6)
	doubleSumMetric.SetDataType(pdata.MetricDataTypeDoubleSum)
	doubleSumMetric.SetName("double_sum")
	doubleSum := doubleSumMetric.DoubleSum()
	doubleSumDataPoints := doubleSum.DataPoints()
	doubleSumDataPoints.Resize(1)
	doubleSumDataPoint := doubleSumDataPoints.At(0)
	doubleSumDataPoint.SetValue(10.1)
	doubleSumDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	doubleHistogramMetric := metrics.At(7)
	doubleHistogramMetric.SetDataType(pdata.MetricDataTypeDoubleHistogram)
	doubleHistogramMetric.SetName("double_histogram")
	doubleHistogram := doubleHistogramMetric.DoubleHistogram()
	doubleHistogramDataPoints := doubleHistogram.DataPoints()
	doubleHistogramDataPoints.Resize(1)

	doubleHistogramDataPoint := doubleHistogramDataPoints.At(0)
	doubleHistogramDataPoint.SetCount(2)
	doubleHistogramDataPoint.SetSum(10.1)
	doubleHistogramDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

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
		name                  string
		fields                fields
		args                  args
		wantDroppedTimeSeries int
		wantErr               bool
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
		wantErr:               false,
		wantDroppedTimeSeries: 1,
	}

	t.Run(test.name, func(t *testing.T) {
		e := &exporter{
			logger: test.fields.logger,
			cfg:    test.fields.cfg,
			client: test.fields.client,
		}
		gotDroppedTimeSeries, err := e.PushMetricsData(test.args.ctx, test.args.md)
		if (err != nil) != test.wantErr {
			t.Errorf("exporter.PushMetricsData() error = %v, wantErr %v", err, test.wantErr)
			return
		}
		if gotDroppedTimeSeries != test.wantDroppedTimeSeries {
			t.Errorf("exporter.PushMetricsData() = %v, want %v", gotDroppedTimeSeries, test.wantDroppedTimeSeries)
		}
	})

	if wantBody := "prefix.int_gauge 10 100\nprefix.int_sum 10 100\nprefix.double_histogram gauge,min=9.5,max=9.5,sum=19,count=2 100\nprefix.double_gauge 10.1 100\nprefix.double_sum 10.1 100\nprefix.double_histogram gauge,min=5.05,max=5.05,sum=10.1,count=2 100"; sent != wantBody {
		t.Errorf("exporter.PushMetricsData():ResponseBody = %v, want %v", sent, wantBody)
	}
}

func Test_exporter_PushMetricsData_EmptyPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Server should not be called")
	}))
	defer ts.Close()

	md := pdata.NewMetrics()
	md.ResourceMetrics().Resize(2)
	rm := md.ResourceMetrics().At(0)

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Resize(2)
	ilm := ilms.At(0)

	metrics := ilm.Metrics()
	metrics.Resize(1)

	noneMetric := metrics.At(0)
	noneMetric.SetName("none")

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}
	gotDroppedTimeSeries, err := e.PushMetricsData(context.Background(), md)
	if err != nil {
		t.Errorf("exporter.PushMetricsData() error = %v", err)
		return
	}
	if gotDroppedTimeSeries != md.MetricCount() {
		t.Errorf("Expected %d metrics to be reported dropped, got %d", md.MetricCount(), gotDroppedTimeSeries)
	}
}

func Test_exporter_PushMetricsData_isDisabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Server should not be called")
	}))
	defer ts.Close()

	md := pdata.NewMetrics()
	md.ResourceMetrics().Resize(2)
	rm := md.ResourceMetrics().At(0)

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Resize(2)
	ilm := ilms.At(0)

	metrics := ilm.Metrics()
	metrics.Resize(1)

	metric := metrics.At(0)
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.SetName("int_gauge")
	intGauge := metric.IntGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoints.Resize(1)
	intGaugeDataPoint := intGaugeDataPoints.At(0)
	intGaugeDataPoint.SetValue(10)
	intGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client:     ts.Client(),
		isDisabled: true,
	}
	gotDroppedTimeSeries, err := e.PushMetricsData(context.Background(), md)
	if err != nil {
		t.Errorf("exporter.PushMetricsData() error = %v", err)
		return
	}
	if gotDroppedTimeSeries != md.MetricCount() {
		t.Errorf("Expected %d metrics to be reported dropped, got %d", md.MetricCount(), gotDroppedTimeSeries)
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
	invalid, err := e.send(context.Background(), []string{})
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
	_, err := e.send(context.Background(), []string{})
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
	_, err := e.send(context.Background(), []string{})
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
	_, err := e.send(context.Background(), []string{})
	if !consumererror.IsPermanent(err) {
		t.Errorf("Expected error to be permanent %v", err)
		return
	}
	if !e.isDisabled {
		t.Error("Expected exporter to be disabled")
		return
	}
}

func Test_exporter_PushMetricsData_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	ts.Close()

	md := pdata.NewMetrics()
	md.ResourceMetrics().Resize(2)
	rm := md.ResourceMetrics().At(0)

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Resize(2)
	ilm := ilms.At(0)

	metrics := ilm.Metrics()
	metrics.Resize(1)

	intGaugeMetric := metrics.At(0)
	intGaugeMetric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.IntGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoints.Resize(1)
	intGaugeDataPoint := intGaugeDataPoints.At(0)
	intGaugeDataPoint.SetValue(10)
	intGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

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
		name                  string
		fields                fields
		args                  args
		wantDroppedTimeSeries int
		wantErr               bool
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
		wantErr:               true,
		wantDroppedTimeSeries: 1,
	}

	t.Run(test.name, func(t *testing.T) {
		e := &exporter{
			logger: test.fields.logger,
			cfg:    test.fields.cfg,
			client: test.fields.client,
		}
		gotDroppedTimeSeries, err := e.PushMetricsData(test.args.ctx, test.args.md)
		if (err != nil) != test.wantErr {
			t.Errorf("exporter.PushMetricsData() error = %v, wantErr %v", err, test.wantErr)
			return
		}
		if gotDroppedTimeSeries != test.wantDroppedTimeSeries {
			t.Errorf("exporter.PushMetricsData() = %v, want %v", gotDroppedTimeSeries, test.wantDroppedTimeSeries)
		}
	})
}

func Test_normalizeMetricName(t *testing.T) {
	type args struct {
		prefix string
		name   string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Normalize name with prefix",
			args: args{
				name:   "metric_name",
				prefix: "prefix",
			},
			want:    "prefix.metric_name",
			wantErr: false,
		},
		{
			name: "Normalize name without prefix",
			args: args{
				name:   "metric_name",
				prefix: "",
			},
			want:    "metric_name",
			wantErr: false,
		},
		{
			name: "Normalize name with trailing _",
			args: args{
				name:   "metric_name_",
				prefix: "",
			},
			want:    "metric_name",
			wantErr: false,
		},
		{
			name: "Normalize name with invalid chars, prefix, and trailing invalid chars _",
			args: args{
				name:   "^*&(metric_name^&*(_",
				prefix: "prefix",
			},
			want:    "prefix._metric_name",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMetricName(tt.args.prefix, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeMetricName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("normalizeMetricName() = %v, want %v", got, tt.want)
			}
		})
	}
}
