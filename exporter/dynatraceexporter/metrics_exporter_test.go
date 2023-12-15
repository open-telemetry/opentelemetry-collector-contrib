// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynatraceexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

var testTimestamp = pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano())

func Test_exporter_PushMetricsData(t *testing.T) {
	sent := "not sent"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		sent = string(bodyBytes)

		response := metricsResponse{
			Ok:      0,
			Invalid: 0,
		}
		body, _ := json.Marshal(response)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	md := pmetric.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(2)
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.ScopeMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()

	badNameMetric := metrics.AppendEmpty()
	badNameMetric.SetName("")

	noneMetric := metrics.AppendEmpty()
	noneMetric.SetName("none")

	intGaugeMetric := metrics.AppendEmpty()
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.SetEmptyGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.SetIntValue(10)
	intGaugeDataPoint.SetTimestamp(testTimestamp)

	nmIntSumMetric := metrics.AppendEmpty()
	nmIntSumMetric.SetName("nonmonotonic_int_sum")
	nmIntSum := nmIntSumMetric.SetEmptySum()
	nmIntSumDataPoints := nmIntSum.DataPoints()
	nmIntSumDataPoint := nmIntSumDataPoints.AppendEmpty()
	nmIntSumDataPoint.SetIntValue(10)
	nmIntSumDataPoint.SetTimestamp(testTimestamp)

	mIntSumMetric := metrics.AppendEmpty()
	mIntSumMetric.SetName("monotonic_int_sum")
	mIntSum := mIntSumMetric.SetEmptySum()
	mIntSum.SetIsMonotonic(true)
	mIntSumDataPoints := mIntSum.DataPoints()
	mIntSumDataPoint := mIntSumDataPoints.AppendEmpty()
	mIntSumDataPoint.SetIntValue(10)
	mIntSumDataPoint.SetTimestamp(testTimestamp)

	doubleGaugeMetric := metrics.AppendEmpty()
	doubleGaugeMetric.SetName("double_gauge")
	doubleGauge := doubleGaugeMetric.SetEmptyGauge()
	doubleGaugeDataPoints := doubleGauge.DataPoints()
	doubleGaugeDataPoint := doubleGaugeDataPoints.AppendEmpty()
	doubleGaugeDataPoint.SetDoubleValue(10.1)
	doubleGaugeDataPoint.SetTimestamp(testTimestamp)

	nmDoubleSumMetric := metrics.AppendEmpty()
	nmDoubleSumMetric.SetName("nonmonotonic_double_sum")
	nmDoubleSum := nmDoubleSumMetric.SetEmptySum()
	nmDoubleSumDataPoints := nmDoubleSum.DataPoints()
	nmDoubleSumDataPoint := nmDoubleSumDataPoints.AppendEmpty()
	nmDoubleSumDataPoint.SetDoubleValue(10.1)
	nmDoubleSumDataPoint.SetTimestamp(testTimestamp)

	mDoubleSumMetric := metrics.AppendEmpty()
	mDoubleSumMetric.SetName("monotonic_double_sum")
	mDoubleSum := mDoubleSumMetric.SetEmptySum()
	mDoubleSum.SetIsMonotonic(true)
	mDoubleSumDataPoints := mDoubleSum.DataPoints()
	mDoubleSumDataPoint := mDoubleSumDataPoints.AppendEmpty()
	mDoubleSumDataPoint.SetDoubleValue(10.1)
	mDoubleSumDataPoint.SetTimestamp(testTimestamp)

	doubleHistogramMetric := metrics.AppendEmpty()
	doubleHistogramMetric.SetName("double_histogram")
	doubleHistogram := doubleHistogramMetric.SetEmptyHistogram()
	doubleHistogramDataPoints := doubleHistogram.DataPoints()
	doubleHistogramDataPoint := doubleHistogramDataPoints.AppendEmpty()
	doubleHistogramDataPoint.SetCount(2)
	doubleHistogramDataPoint.SetSum(10.1)
	doubleHistogramDataPoint.ExplicitBounds().FromRaw([]float64{0, 2, 4, 8})
	doubleHistogramDataPoint.BucketCounts().FromRaw([]uint64{0, 1, 0, 1, 0})
	doubleHistogramDataPoint.SetTimestamp(testTimestamp)
	doubleHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	type fields struct {
		settings component.TelemetrySettings
		cfg      *config.Config
		client   *http.Client
	}
	type args struct {
		ctx context.Context
		md  pmetric.Metrics
	}
	test := struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		name: "Send metric data",
		fields: fields{
			settings: componenttest.NewNopTelemetrySettings(),
			cfg: &config.Config{
				APIToken:           "token",
				HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
				Prefix:             "prefix",
				DefaultDimensions:  map[string]string{},
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
		e := &metricsExporter{
			settings: test.fields.settings,
			cfg:      test.fields.cfg,
			client:   test.fields.client,
		}
		err := e.PushMetricsData(test.args.ctx, test.args.md)
		if (err != nil) != test.wantErr {
			t.Errorf("exporter.PushMetricsData() error = %v, wantErr %v", err, test.wantErr)
			return
		}
	})

	wantLines := []string{
		"prefix.int_gauge gauge,10 1626438600000",
		"prefix.monotonic_int_sum count,delta=10 1626438600000",
		"prefix.nonmonotonic_int_sum gauge,10 1626438600000",
		"prefix.double_gauge gauge,10.1 1626438600000",
		"prefix.monotonic_double_sum count,delta=10.1 1626438600000",
		"prefix.nonmonotonic_double_sum gauge,10.1 1626438600000",
		"prefix.double_histogram gauge,min=0,max=8,sum=10.1,count=2 1626438600000",
	}

	// only succeeds if the two lists contain the same elements, ignoring their order
	assert.ElementsMatch(t, wantLines, strings.Split(sent, "\n"))
}

func Test_SumMetrics(t *testing.T) {
	type args struct {
		monotonic   bool
		temporality pmetric.AggregationTemporality
		valueType   string // either 'double' or 'int'
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Monotonic Delta sum (int)",
			args: args{
				true,
				pmetric.AggregationTemporalityDelta,
				"int",
			},
			want: []string{
				"prefix.metric_name count,delta=10 1626438600000",
				"prefix.metric_name count,delta=20 1626438600000",
			},
		},
		{
			name: "Non-monotonic Delta sum (int)",
			args: args{
				false,
				pmetric.AggregationTemporalityDelta,
				"int",
			},
			want: []string{"nothing sent"},
		},
		{
			name: "Monotonic Cumulative sum (int)",
			args: args{
				true,
				pmetric.AggregationTemporalityCumulative,
				"int",
			},
			want: []string{"prefix.metric_name count,delta=10 1626438600000"},
		},
		{
			name: "Non-monotonic Cumulative sum (int)",
			args: args{
				false,
				pmetric.AggregationTemporalityCumulative,
				"int",
			},
			want: []string{
				"prefix.metric_name gauge,10 1626438600000",
				"prefix.metric_name gauge,20 1626438600000",
			},
		},
		{
			name: "Monotonic Delta sum (double)",
			args: args{
				true,
				pmetric.AggregationTemporalityDelta,
				"double",
			},
			want: []string{
				"prefix.metric_name count,delta=10.1 1626438600000",
				"prefix.metric_name count,delta=20.2 1626438600000",
			},
		},
		{
			name: "Non-monotonic Delta sum (double)",
			args: args{
				false,
				pmetric.AggregationTemporalityDelta,
				"double",
			},
			want: []string{"nothing sent"},
		},
		{
			name: "Monotonic Cumulative sum (double)",
			args: args{
				true,
				pmetric.AggregationTemporalityCumulative,
				"double",
			},
			want: []string{"prefix.metric_name count,delta=10.1 1626438600000"},
		},
		{
			name: "Non-monotonic Cumulative sum (double)",
			args: args{
				false,
				pmetric.AggregationTemporalityCumulative,
				"double",
			},
			want: []string{
				"prefix.metric_name gauge,10.1 1626438600000",
				"prefix.metric_name gauge,20.2 1626438600000",
			},
		},
	}

	// server setup:
	sent := "nothing sent"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		sent = string(bodyBytes)

		response := metricsResponse{
			Ok:      0,
			Invalid: 0,
		}
		body, _ := json.Marshal(response)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset the export buffer for the HTTP client
			sent = "nothing sent"

			prevPts := ttlmap.New(cSweepIntervalSeconds, cMaxAgeSeconds)

			// set up the exporter
			exp := &metricsExporter{
				settings: componenttest.NewNopTelemetrySettings(),
				cfg: &config.Config{
					APIToken:           "token",
					HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
					Prefix:             "prefix",
				},
				client:  ts.Client(),
				prevPts: prevPts,
			}

			metrics := pmetric.NewMetrics()
			resourceMetric := metrics.ResourceMetrics().AppendEmpty()
			scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()
			metric := scopeMetric.Metrics().AppendEmpty()
			metric.SetName("metric_name")
			sum := metric.SetEmptySum()
			sum.SetAggregationTemporality(tt.args.temporality)
			sum.SetIsMonotonic(tt.args.monotonic)

			dataPoint1 := sum.DataPoints().AppendEmpty()
			dataPoint1.SetTimestamp(testTimestamp)

			dataPoint2 := sum.DataPoints().AppendEmpty()
			dataPoint2.SetTimestamp(testTimestamp)

			switch tt.args.valueType {
			case "int":
				dataPoint1.SetIntValue(10)
				dataPoint2.SetIntValue(20)
			case "double":
				dataPoint1.SetDoubleValue(10.1)
				dataPoint2.SetDoubleValue(20.2)
			default:
				t.Fatalf("valueType can only be 'int' or 'double' but was '%s'", tt.args.valueType)
			}

			err := exp.PushMetricsData(context.Background(), metrics)
			assert.NoError(t, err)

			assert.ElementsMatch(t, tt.want, strings.Split(sent, "\n"))
		})
	}
}

func Test_exporter_PushMetricsData_EmptyPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Server should not be called")
	}))
	defer ts.Close()

	md := pmetric.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(2)
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.ScopeMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()
	noneMetric := metrics.AppendEmpty()
	noneMetric.SetName("none")

	e := &metricsExporter{
		settings: componenttest.NewNopTelemetrySettings(),
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

	md := pmetric.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(2)
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.ScopeMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName("int_gauge")
	intGauge := metric.SetEmptyGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.SetIntValue(10)
	intGaugeDataPoint.SetTimestamp(testTimestamp)

	e := &metricsExporter{
		settings: componenttest.NewNopTelemetrySettings(),
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
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	e := &metricsExporter{
		settings: componenttest.NewNopTelemetrySettings(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}
	err := e.send(context.Background(), []string{""})
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
		_, _ = w.Write([]byte{})
	}))
	defer ts.Close()

	e := &metricsExporter{
		settings: componenttest.NewNopTelemetrySettings(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}
	err := e.send(context.Background(), []string{""})
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
		_, _ = w.Write([]byte{})
	}))
	defer ts.Close()

	e := &metricsExporter{
		settings: componenttest.NewNopTelemetrySettings(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}
	err := e.send(context.Background(), []string{""})
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
		_, _ = w.Write([]byte{})
	}))
	defer ts.Close()

	e := &metricsExporter{
		settings: componenttest.NewNopTelemetrySettings(),
		cfg: &config.Config{
			APIToken:           "token",
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
			Prefix:             "prefix",
			DefaultDimensions:  map[string]string{},
		},
		client: ts.Client(),
	}
	err := e.send(context.Background(), []string{""})
	if !consumererror.IsPermanent(err) {
		t.Errorf("Expected error to be permanent %v", err)
		return
	}
	if e.isDisabled {
		t.Error("Expected exporter to not be disabled")
		return
	}
}

func Test_exporter_send_TooManyRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte{})
	}))
	defer ts.Close()

	e := &metricsExporter{
		settings: componenttest.NewNopTelemetrySettings(),
		cfg: &config.Config{
			APIToken:           "token",
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
			Prefix:             "prefix",
			DefaultDimensions:  map[string]string{},
		},
		client: ts.Client(),
	}
	err := e.send(context.Background(), []string{""})

	assert.True(t, consumererror.IsPermanent(err), "Expected error to be permanent %v", err)
	assert.False(t, e.isDisabled, "Expected exporter to not be disabled")
}

func Test_exporter_send_MiscellaneousErrorCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusExpectationFailed)
		_, _ = w.Write([]byte{})
	}))
	defer ts.Close()

	e := &metricsExporter{
		settings: componenttest.NewNopTelemetrySettings(),
		cfg: &config.Config{
			APIToken:           "token",
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
			Prefix:             "prefix",
			DefaultDimensions:  map[string]string{},
		},
		client: ts.Client(),
	}
	err := e.send(context.Background(), []string{""})

	assert.ErrorContains(t, err, "417 Expectation Failed")
	assert.True(t, consumererror.IsPermanent(err), "Expected error to be permanent %v", err)
	assert.False(t, e.isDisabled, "Expected exporter to not be disabled")
}

func Test_exporter_send_chunking(t *testing.T) {
	sentChunks := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		body, _ := json.Marshal(metricsResponse{
			Ok:      0,
			Invalid: 1,
		})
		_, _ = w.Write(body)
		sentChunks++
	}))
	defer ts.Close()

	e := &metricsExporter{
		settings: componenttest.NewNopTelemetrySettings(),
		cfg: &config.Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ts.URL},
		},
		client: ts.Client(),
	}

	batch := make([]string, 1001)

	for i := 0; i < 1001; i++ {
		batch[i] = fmt.Sprintf("%d", i)
	}

	err := e.send(context.Background(), batch)
	if sentChunks != 2 {
		t.Errorf("Expected batch to be sent in 2 chunks")
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

	md := pmetric.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(2)
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.ScopeMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()
	intGaugeMetric := metrics.AppendEmpty()
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.SetEmptyGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.SetIntValue(10)
	intGaugeDataPoint.SetTimestamp(testTimestamp)
	type fields struct {
		logger *zap.Logger
		cfg    *config.Config
		client *http.Client
	}
	type args struct {
		ctx context.Context
		md  pmetric.Metrics
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
				DefaultDimensions:  map[string]string{},
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
		e := &metricsExporter{
			settings: componenttest.NewNopTelemetrySettings(),
			cfg:      test.fields.cfg,
			client:   test.fields.client,
		}
		err := e.PushMetricsData(test.args.ctx, test.args.md)
		if (err != nil) != test.wantErr {
			t.Errorf("exporter.PushMetricsData() error = %v, wantErr %v", err, test.wantErr)
			return
		}
	})
}

func Test_exporter_start_InvalidHTTPClientSettings(t *testing.T) {
	cfg := &config.Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "localhost:9090",
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: "/non/existent",
				},
			},
		},
	}

	exp := newMetricsExporter(exportertest.NewNopCreateSettings(), cfg)

	err := exp.start(context.Background(), componenttest.NewNopHost())
	if err == nil {
		t.Errorf("Expected error when creating a metrics exporter with invalid HTTP Client Settings")
		return
	}
}

func Test_exporter_new_with_tags(t *testing.T) {
	cfg := &config.Config{
		DefaultDimensions: map[string]string{"test_tag": "value"},
	}

	exp := newMetricsExporter(exportertest.NewNopCreateSettings(), cfg)

	assert.Equal(t, dimensions.NewNormalizedDimensionList(dimensions.NewDimension("test_tag", "value")), exp.defaultDimensions)
}

func Test_exporter_new_with_default_dimensions(t *testing.T) {
	cfg := &config.Config{
		DefaultDimensions: map[string]string{"test_dimension": "value"},
	}

	exp := newMetricsExporter(exportertest.NewNopCreateSettings(), cfg)

	assert.Equal(t, dimensions.NewNormalizedDimensionList(dimensions.NewDimension("test_dimension", "value")), exp.defaultDimensions)
}

func Test_exporter_new_with_default_dimensions_override_tag(t *testing.T) {
	cfg := &config.Config{
		Tags:              []string{"from=tag"},
		DefaultDimensions: map[string]string{"from": "default_dimensions"},
	}

	exp := newMetricsExporter(exportertest.NewNopCreateSettings(), cfg)

	assert.Equal(t, dimensions.NewNormalizedDimensionList(dimensions.NewDimension("from", "default_dimensions")), exp.defaultDimensions)
}

func Test_LineTooLong(t *testing.T) {
	numDims := 50_000 / 9
	dims := make(map[string]string, numDims)
	for i := 0; i < numDims; i++ {
		dims[fmt.Sprintf("dim%d", i)] = fmt.Sprintf("val%d", i)
	}

	md := pmetric.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(1)
	rm := md.ResourceMetrics().AppendEmpty()

	scms := rm.ScopeMetrics()
	scms.EnsureCapacity(1)
	scm := scms.AppendEmpty()

	metrics := scm.Metrics()
	intGaugeMetric := metrics.AppendEmpty()
	intGaugeMetric.SetName("int_gauge")
	intGauge := intGaugeMetric.SetEmptyGauge()
	intGaugeDataPoints := intGauge.DataPoints()
	intGaugeDataPoint := intGaugeDataPoints.AppendEmpty()
	intGaugeDataPoint.SetIntValue(10)
	intGaugeDataPoint.SetTimestamp(testTimestamp)
	exp := newMetricsExporter(exportertest.NewNopCreateSettings(), &config.Config{DefaultDimensions: dims})

	assert.Empty(t, exp.serializeMetrics(md))
}
