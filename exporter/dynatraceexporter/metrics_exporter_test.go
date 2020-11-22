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

	rm := pdata.NewResourceMetrics()
	erm := pdata.NewResourceMetrics()

	rm.InitEmpty()

	md.ResourceMetrics().Append(rm)
	md.ResourceMetrics().Append(erm)

	ilm := pdata.NewInstrumentationLibraryMetrics()
	eilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.InitEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Append(ilm)
	ilms.Append(eilm)

	emptyMetric := pdata.NewMetric()

	badNameMetric := pdata.NewMetric()
	badNameMetric.InitEmpty()
	badNameMetric.SetName("")

	noneMetric := pdata.NewMetric()
	noneMetric.InitEmpty()
	noneMetric.SetName("none")

	intGaugeMetric := pdata.NewMetric()
	intGaugeMetric.InitEmpty()

	intGaugeMetric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.IntGauge()
	intGauge.InitEmpty()
	intGaugeDataPoints := intGauge.DataPoints()

	intGaugeDataPoint := pdata.NewIntDataPoint()
	intGaugeDataPoint.InitEmpty()
	intGaugeDataPoint.SetValue(10)
	intGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100000))
	intGaugeDataPoints.Append(intGaugeDataPoint)

	intSumMetric := pdata.NewMetric()
	intSumMetric.InitEmpty()

	intSumMetric.SetDataType(pdata.MetricDataTypeIntSum)
	intSumMetric.SetName("int_sum")
	intSum := intSumMetric.IntSum()
	intSum.InitEmpty()
	intSumDataPoints := intSum.DataPoints()

	intSumDataPoint := pdata.NewIntDataPoint()
	intSumDataPoint.InitEmpty()
	intSumDataPoint.SetValue(10)
	intSumDataPoint.SetTimestamp(pdata.TimestampUnixNano(100000))
	intSumDataPoints.Append(intSumDataPoint)

	intHistogramMetric := pdata.NewMetric()
	intHistogramMetric.InitEmpty()

	intHistogramMetric.SetDataType(pdata.MetricDataTypeIntHistogram)
	intHistogramMetric.SetName("double_histogram")
	intHistogram := intHistogramMetric.IntHistogram()
	intHistogram.InitEmpty()
	intHistogramDataPoints := intHistogram.DataPoints()

	intHistogramDataPoint := pdata.NewIntHistogramDataPoint()
	intHistogramDataPoint.InitEmpty()
	intHistogramDataPoint.SetCount(2)
	intHistogramDataPoint.SetSum(19)
	intHistogramDataPoint.SetTimestamp(pdata.TimestampUnixNano(100000))
	intHistogramDataPoints.Append(intHistogramDataPoint)

	doubleGaugeMetric := pdata.NewMetric()
	doubleGaugeMetric.InitEmpty()

	doubleGaugeMetric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	doubleGaugeMetric.SetName("double_gauge")
	doubleGauge := doubleGaugeMetric.DoubleGauge()
	doubleGauge.InitEmpty()
	doubleGaugeDataPoints := doubleGauge.DataPoints()

	doubleGaugeDataPoint := pdata.NewDoubleDataPoint()
	doubleGaugeDataPoint.InitEmpty()
	doubleGaugeDataPoint.SetValue(10.1)
	doubleGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100000))
	doubleGaugeDataPoints.Append(doubleGaugeDataPoint)

	doubleSumMetric := pdata.NewMetric()
	doubleSumMetric.InitEmpty()

	doubleSumMetric.SetDataType(pdata.MetricDataTypeDoubleSum)
	doubleSumMetric.SetName("double_sum")
	doubleSum := doubleSumMetric.DoubleSum()
	doubleSum.InitEmpty()
	doubleSumDataPoints := doubleSum.DataPoints()

	doubleSumDataPoint := pdata.NewDoubleDataPoint()
	doubleSumDataPoint.InitEmpty()
	doubleSumDataPoint.SetValue(10.1)
	doubleSumDataPoint.SetTimestamp(pdata.TimestampUnixNano(100000))
	doubleSumDataPoints.Append(doubleSumDataPoint)

	doubleHistogramMetric := pdata.NewMetric()
	doubleHistogramMetric.InitEmpty()

	doubleHistogramMetric.SetDataType(pdata.MetricDataTypeDoubleHistogram)
	doubleHistogramMetric.SetName("double_histogram")
	doubleHistogram := doubleHistogramMetric.DoubleHistogram()
	doubleHistogram.InitEmpty()
	doubleHistogramDataPoints := doubleHistogram.DataPoints()

	doubleHistogramDataPoint := pdata.NewDoubleHistogramDataPoint()
	doubleHistogramDataPoint.InitEmpty()
	doubleHistogramDataPoint.SetCount(2)
	doubleHistogramDataPoint.SetSum(10.1)
	doubleHistogramDataPoint.SetTimestamp(pdata.TimestampUnixNano(100000))
	doubleHistogramDataPoints.Append(doubleHistogramDataPoint)

	metrics := ilm.Metrics()
	metrics.Append(intGaugeMetric)
	metrics.Append(intSumMetric)
	metrics.Append(intHistogramMetric)
	metrics.Append(doubleGaugeMetric)
	metrics.Append(doubleSumMetric)
	metrics.Append(doubleHistogramMetric)
	metrics.Append(emptyMetric)
	metrics.Append(badNameMetric)
	metrics.Append(noneMetric)

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

	if wantBody := "prefix.int_gauge 10 100\nprefix.int_sum 10 100\nprefix.double_histogram gauge,min=9.5,max=9.5,sum=19,count=2 100\nprefix.double_gauge 10.1 100\nprefix.double_sum 10.1 100\nprefix.double_histogram gauge,min=5.05,max=5.05,sum=10.1,count=2 100\n"; sent != wantBody {
		t.Errorf("exporter.PushMetricsData():ResponseBody = %v, want %v", sent, wantBody)
	}
}

func Test_exporter_PushMetricsData_EmptyPayload(t *testing.T) {
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

	rm := pdata.NewResourceMetrics()
	erm := pdata.NewResourceMetrics()

	rm.InitEmpty()

	md.ResourceMetrics().Append(rm)
	md.ResourceMetrics().Append(erm)

	ilm := pdata.NewInstrumentationLibraryMetrics()
	eilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.InitEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Append(ilm)
	ilms.Append(eilm)

	noneMetric := pdata.NewMetric()
	noneMetric.InitEmpty()
	noneMetric.SetName("none")

	metrics := ilm.Metrics()
	metrics.Append(noneMetric)

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
		wantDroppedTimeSeries: 0,
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

	if sent != "not sent" {
		t.Errorf("empty payload should not be sent: %s", sent)
	}
}

func Test_exporter_PushMetricsData_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		response := metricsResponse{
			Ok:      0,
			Invalid: 1,
			Error:   "unauthorized",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))
	defer ts.Close()

	md := pdata.NewMetrics()

	rm := pdata.NewResourceMetrics()
	erm := pdata.NewResourceMetrics()

	rm.InitEmpty()

	md.ResourceMetrics().Append(rm)
	md.ResourceMetrics().Append(erm)

	ilm := pdata.NewInstrumentationLibraryMetrics()
	eilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.InitEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Append(ilm)
	ilms.Append(eilm)

	intGaugeMetric := pdata.NewMetric()
	intGaugeMetric.InitEmpty()

	intGaugeMetric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.IntGauge()
	intGauge.InitEmpty()
	intGaugeDataPoints := intGauge.DataPoints()

	intGaugeDataPoint := pdata.NewIntDataPoint()
	intGaugeDataPoint.InitEmpty()
	intGaugeDataPoint.SetValue(10)
	intGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100000))
	intGaugeDataPoints.Append(intGaugeDataPoint)
	metrics := ilm.Metrics()

	metrics.Append(intGaugeMetric)

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
		wantErr:               true,
		wantDroppedTimeSeries: 0,
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

func Test_exporter_PushMetricsData_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	ts.Close()

	md := pdata.NewMetrics()

	rm := pdata.NewResourceMetrics()
	erm := pdata.NewResourceMetrics()

	rm.InitEmpty()

	md.ResourceMetrics().Append(rm)
	md.ResourceMetrics().Append(erm)

	ilm := pdata.NewInstrumentationLibraryMetrics()
	eilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.InitEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Append(ilm)
	ilms.Append(eilm)

	intGaugeMetric := pdata.NewMetric()
	intGaugeMetric.InitEmpty()

	intGaugeMetric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.IntGauge()
	intGauge.InitEmpty()
	intGaugeDataPoints := intGauge.DataPoints()

	intGaugeDataPoint := pdata.NewIntDataPoint()
	intGaugeDataPoint.InitEmpty()
	intGaugeDataPoint.SetValue(10)
	intGaugeDataPoint.SetTimestamp(pdata.TimestampUnixNano(100000))
	intGaugeDataPoints.Append(intGaugeDataPoint)
	metrics := ilm.Metrics()

	metrics.Append(intGaugeMetric)

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
		wantErr:               true,
		wantDroppedTimeSeries: 0,
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
			want:    "prefix.metric_name",
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
