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
	"regexp"
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
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
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()
	previousMetrics := ilm.Metrics()

	e := &exporter{
		logger: zap.NewNop(),
		cfg: &config.Config{
			APIToken:           "token",
			Prefix:             "prefix",
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		defaultDimensions: dimensions.NewNormalizedDimensionList(),
		staticDimensions:  dimensions.NewNormalizedDimensionList(dimensions.NewDimension("dt.metrics.source", "opentelemetry")),
		prev:              ttlmap.New(cSweepIntervalSeconds, cMaxAgeSeconds),
	}
	e.client = ts.Client()

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

	prevIntSumCumulativeMetric := previousMetrics.AppendEmpty()
	prevIntSumCumulativeMetric.SetDataType(pdata.MetricDataTypeSum)
	prevIntSumCumulativeMetric.SetName("int_sum_cumulative")
	prevIntSumCumulative := prevIntSumCumulativeMetric.Sum()
	prevIntSumCumulative.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	prevIntSumCumulativeDataPoints := prevIntSumCumulative.DataPoints()
	prevIntSumCumulativeDataPoint := prevIntSumCumulativeDataPoints.AppendEmpty()
	prevIntSumCumulativeDataPoint.SetIntVal(8)
	prevIntSumCumulativeDataPoint.SetTimestamp(testTimestamp)

	intSumCumulativeMetric := metrics.AppendEmpty()
	intSumCumulativeMetric.SetDataType(pdata.MetricDataTypeSum)
	intSumCumulativeMetric.SetName("int_sum_cumulative")
	intSumCumulative := intSumCumulativeMetric.Sum()
	intSumCumulative.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	intSumCumulativeDataPoints := intSumCumulative.DataPoints()
	intSumCumulativeDataPoint := intSumCumulativeDataPoints.AppendEmpty()
	intSumCumulativeDataPoint.SetIntVal(10)
	intSumCumulativeDataPoint.SetTimestamp(testTimestamp)

	gaugeMetric := metrics.AppendEmpty()
	gaugeMetric.SetDataType(pdata.MetricDataTypeGauge)
	gaugeMetric.SetName("double_gauge")
	gauge := gaugeMetric.Gauge()
	gaugeDataPoints := gauge.DataPoints()
	gaugeDataPoint := gaugeDataPoints.AppendEmpty()
	gaugeDataPoint.SetDoubleVal(10.1)
	gaugeDataPoint.SetTimestamp(testTimestamp)

	sumMetric := metrics.AppendEmpty()
	sumMetric.SetDataType(pdata.MetricDataTypeSum)
	sumMetric.SetName("double_sum")
	sum := sumMetric.Sum()
	sumDataPoints := sum.DataPoints()
	sumDataPoint := sumDataPoints.AppendEmpty()
	sumDataPoint.SetDoubleVal(10.1)
	sumDataPoint.SetTimestamp(testTimestamp)

	prevDblSumCumulativeMetric := previousMetrics.AppendEmpty()
	prevDblSumCumulativeMetric.SetDataType(pdata.MetricDataTypeSum)
	prevDblSumCumulativeMetric.SetName("dbl_sum_cumulative")
	prevDblSumCumulative := prevDblSumCumulativeMetric.Sum()
	prevDblSumCumulative.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	prevDblSumCumulativeDataPoints := prevDblSumCumulative.DataPoints()
	prevDblSumCumulativeDataPoint := prevDblSumCumulativeDataPoints.AppendEmpty()
	prevDblSumCumulativeDataPoint.SetDoubleVal(10.0)
	prevDblSumCumulativeDataPoint.SetTimestamp(testTimestamp)

	dblSumCumulativeMetric := metrics.AppendEmpty()
	dblSumCumulativeMetric.SetDataType(pdata.MetricDataTypeSum)
	dblSumCumulativeMetric.SetName("dbl_sum_cumulative")
	dblSumCumulative := dblSumCumulativeMetric.Sum()
	dblSumCumulative.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	dblSumCumulativeDataPoints := dblSumCumulative.DataPoints()
	dblSumCumulativeDataPoint := dblSumCumulativeDataPoints.AppendEmpty()
	dblSumCumulativeDataPoint.SetDoubleVal(10.5)
	dblSumCumulativeDataPoint.SetTimestamp(testTimestamp)

	doubleHistogramMetric := metrics.AppendEmpty()
	doubleHistogramMetric.SetDataType(pdata.MetricDataTypeHistogram)
	doubleHistogramMetric.SetName("double_histogram")
	doubleHistogram := doubleHistogramMetric.Histogram()
	doubleHistogramDataPoints := doubleHistogram.DataPoints()
	doubleHistogramDataPoint := doubleHistogramDataPoints.AppendEmpty()
	doubleHistogramDataPoint.SetCount(2)
	doubleHistogramDataPoint.SetSum(9.5)
	doubleHistogramDataPoint.SetExplicitBounds([]float64{0, 2, 4, 8})
	doubleHistogramDataPoint.SetBucketCounts([]uint64{0, 1, 0, 1, 0})
	doubleHistogramDataPoint.SetTimestamp(testTimestamp)

	minBucketHistogramMetric := metrics.AppendEmpty()
	minBucketHistogramMetric.SetDataType(pdata.MetricDataTypeHistogram)
	minBucketHistogramMetric.SetName("min_bucket_histogram")
	minBucketHistogram := minBucketHistogramMetric.Histogram()
	minBucketHistogramDataPoints := minBucketHistogram.DataPoints()
	minBucketHistogramDataPoint := minBucketHistogramDataPoints.AppendEmpty()
	minBucketHistogramDataPoint.SetCount(2)
	minBucketHistogramDataPoint.SetSum(3)
	minBucketHistogramDataPoint.SetExplicitBounds([]float64{0, 2, 4, 8})
	minBucketHistogramDataPoint.SetBucketCounts([]uint64{1, 0, 0, 1, 0})
	minBucketHistogramDataPoint.SetTimestamp(testTimestamp)

	maxBucketHistogramMetric := metrics.AppendEmpty()
	maxBucketHistogramMetric.SetDataType(pdata.MetricDataTypeHistogram)
	maxBucketHistogramMetric.SetName("max_bucket_histogram")
	maxBucketHistogram := maxBucketHistogramMetric.Histogram()
	maxBucketHistogramDataPoints := maxBucketHistogram.DataPoints()
	maxBucketHistogramDataPoint := maxBucketHistogramDataPoints.AppendEmpty()
	maxBucketHistogramDataPoint.SetCount(2)
	maxBucketHistogramDataPoint.SetSum(20)
	maxBucketHistogramDataPoint.SetExplicitBounds([]float64{0, 2, 4, 8})
	maxBucketHistogramDataPoint.SetBucketCounts([]uint64{0, 1, 0, 0, 1})
	maxBucketHistogramDataPoint.SetTimestamp(testTimestamp)

	err := e.PushMetricsData(context.Background(), md)
	if err != nil {
		t.Errorf("exporter.PushMetricsData() error = %v", err)
		return
	}

	expected := `prefix.int_gauge,dt.metrics.source=opentelemetry gauge,10 1626438600000
prefix.int_sum,dt.metrics.source=opentelemetry count,delta=10 1626438600000
prefix.int_sum_cumulative,dt.metrics.source=opentelemetry count,delta=2 1626438600000
prefix.double_gauge,dt.metrics.source=opentelemetry gauge,10.1 1626438600000
prefix.double_sum,dt.metrics.source=opentelemetry count,delta=10.1 1626438600000
prefix.dbl_sum_cumulative,dt.metrics.source=opentelemetry count,delta=0.5 1626438600000
prefix.double_histogram,dt.metrics.source=opentelemetry gauge,min=0,max=8,sum=9.5,count=2 1626438600000
prefix.min_bucket_histogram,dt.metrics.source=opentelemetry gauge,min=0,max=8,sum=3,count=2 1626438600000
prefix.max_bucket_histogram,dt.metrics.source=opentelemetry gauge,min=0,max=8,sum=20,count=2 1626438600000`
	assert.Equal(t, expected, sent)
}

func Test_exporter_PushMetricsData_dimensions(t *testing.T) {
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
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
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
	intGaugeDataPoint.Attributes().Insert("from", pdata.NewAttributeValueString("metric"))
	intGaugeDataPoint.Attributes().Insert("dt.metrics.source", pdata.NewAttributeValueString("metric"))

	e := newMetricsExporter(component.ExporterCreateSettings{Logger: zap.NewNop()}, &config.Config{
		APIToken:           "token",
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		DefaultDimensions: map[string]string{
			"dim":               "default",
			"from":              "default",
			"dt.metrics.source": "default",
			"default_tag":       "default",
		},
		Tags: []string{"tag_key=tag_value", "badlyformatted", "default_tag=tag"},
	})
	e.client = ts.Client()

	err := e.PushMetricsData(context.Background(), md)
	if err != nil {
		t.Errorf("exporter.PushMetricsData() error = %v", err)
		return
	}

	assert.Regexp(t, regexp.MustCompile("dt.metrics.source=opentelemetry"), sent, "Must contain dt.metrics.source dimension")
	assert.NotRegexp(t, regexp.MustCompile("dt.metrics.source=default"), sent, "Default must not override static metric source")
	assert.NotRegexp(t, regexp.MustCompile("dt.metrics.source=metric"), sent, "Metric must not override static metric source")

	assert.Regexp(t, regexp.MustCompile("dim=default"), sent, "Must contain default dimensions")
	assert.Regexp(t, regexp.MustCompile("default_tag=default"), sent, "Default should override tag")

	assert.Regexp(t, regexp.MustCompile("tag_key=tag_value"), sent, "Must contain tags")
	assert.NotRegexp(t, regexp.MustCompile("default_tag=tag"), sent, "Tag should not override default")

	assert.NotRegexp(t, regexp.MustCompile("badlyformatted"), sent, "Must skip badly formatted tags")

	assert.Regexp(t, regexp.MustCompile("from=metric"), sent, "Must contain dimension from metric")
	assert.NotRegexp(t, regexp.MustCompile("from=default"), sent, "Default dimension must not override metric dimension")
}

func Test_exporter_PushMetricsData_EmptyPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Server should not be called")
	}))
	defer ts.Close()

	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()

	badNameMetric := metrics.AppendEmpty()
	badNameMetric.SetName("")

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
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()
	metric := metrics.AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeGauge)
	metric.SetName("int_gauge")
	intGauge := metric.Gauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.SetIntVal(10)
	intGaugeDataPoint.SetTimestamp(pdata.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

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
			DefaultDimensions:  make(map[string]string),
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
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
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
				DefaultDimensions:  make(map[string]string),
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

	exp := newMetricsExporter(component.ExporterCreateSettings{Logger: zap.NewNop()}, config)

	err := exp.start(context.Background(), componenttest.NewNopHost())
	if err == nil {
		t.Errorf("Expected error when creating a metrics exporter with invalid HTTP Client Settings")
		return
	}
}
