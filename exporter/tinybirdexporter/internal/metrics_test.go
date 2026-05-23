// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type sumEncoderMock struct {
	metrics []sumMetricSignal
}

func (e *sumEncoderMock) Encode(v any) error {
	e.metrics = append(e.metrics, v.(sumMetricSignal))
	return nil
}

type gaugeEncoderMock struct {
	metrics []gaugeMetricSignal
}

func (e *gaugeEncoderMock) Encode(v any) error {
	e.metrics = append(e.metrics, v.(gaugeMetricSignal))
	return nil
}

type histogramEncoderMock struct {
	metrics []histogramMetricSignal
}

func (e *histogramEncoderMock) Encode(v any) error {
	e.metrics = append(e.metrics, v.(histogramMetricSignal))
	return nil
}

type exponentialHistogramEncoderMock struct {
	metrics []exponentialHistogramMetricSignal
}

func (e *exponentialHistogramEncoderMock) Encode(v any) error {
	e.metrics = append(e.metrics, v.(exponentialHistogramMetricSignal))
	return nil
}

func TestConvertMetrics(t *testing.T) {
	type args struct {
		md                          pmetric.Metrics
		sumEncoder                  Encoder
		gaugeEncoder                Encoder
		histogramEncoder            Encoder
		exponentialHistogramEncoder Encoder
	}
	tests := []struct {
		name                     string
		args                     args
		wantSum                  []sumMetricSignal
		wantGauge                []gaugeMetricSignal
		wantHistogram            []histogramMetricSignal
		wantExponentialHistogram []exponentialHistogramMetricSignal
		wantErr                  bool
	}{
		{
			name: "empty metrics",
			args: args{
				md:                          pmetric.NewMetrics(),
				sumEncoder:                  &sumEncoderMock{metrics: []sumMetricSignal{}},
				gaugeEncoder:                &gaugeEncoderMock{metrics: []gaugeMetricSignal{}},
				histogramEncoder:            &histogramEncoderMock{metrics: []histogramMetricSignal{}},
				exponentialHistogramEncoder: &exponentialHistogramEncoderMock{metrics: []exponentialHistogramMetricSignal{}},
			},
			wantSum:                  []sumMetricSignal{},
			wantGauge:                []gaugeMetricSignal{},
			wantHistogram:            []histogramMetricSignal{},
			wantExponentialHistogram: []exponentialHistogramMetricSignal{},
		},
		{
			name: "sum metric without attributes",
			args: args{
				md: func() pmetric.Metrics {
					md := pmetric.NewMetrics()
					resourceMetrics := md.ResourceMetrics().AppendEmpty()
					resourceMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
					scopeMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeMetrics.Scope().SetName("test-scope")
					scopeMetrics.Scope().SetVersion("1.0.0")
					metric := scopeMetrics.Metrics().AppendEmpty()
					metric.SetName("test.sum")
					metric.SetDescription("Test sum metric")
					metric.SetUnit("count")
					sum := metric.SetEmptySum()
					sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
					sum.SetIsMonotonic(true)
					dp := sum.DataPoints().AppendEmpty()
					dp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000))
					dp.SetTimestamp(pcommon.Timestamp(1719158401000000000))
					dp.SetDoubleValue(42.5)
					return md
				}(),
				sumEncoder:                  &sumEncoderMock{metrics: []sumMetricSignal{}},
				gaugeEncoder:                &gaugeEncoderMock{metrics: []gaugeMetricSignal{}},
				histogramEncoder:            &histogramEncoderMock{metrics: []histogramMetricSignal{}},
				exponentialHistogramEncoder: &exponentialHistogramEncoderMock{metrics: []exponentialHistogramMetricSignal{}},
			},
			wantSum: []sumMetricSignal{
				{
					baseMetricSignal: baseMetricSignal{
						ResourceSchemaURL:  "https://opentelemetry.io/schemas/1.20.0",
						ResourceAttributes: map[string]string{},
						ServiceName:        "",
						ScopeName:          "test-scope",
						ScopeVersion:       "1.0.0",
						ScopeSchemaURL:     "https://opentelemetry.io/schemas/1.20.0",
						ScopeAttributes:    map[string]string{},
						MetricName:         "test.sum",
						MetricDescription:  "Test sum metric",
						MetricUnit:         "count",
						MetricAttributes:   map[string]string{},
						StartTimestamp:     "2024-06-23T16:00:00Z",
						Timestamp:          "2024-06-23T16:00:01Z",
						Flags:              0,
						exemplars: exemplars{
							ExemplarsFilteredAttributes: []map[string]string{},
							ExemplarsTimestamp:          []string{},
							ExemplarsValue:              []float64{},
							ExemplarsSpanID:             []string{},
							ExemplarsTraceID:            []string{},
						},
					},
					Value:                  42.5,
					AggregationTemporality: 2, // Cumulative
					IsMonotonic:            true,
				},
			},
			wantGauge:                []gaugeMetricSignal{},
			wantHistogram:            []histogramMetricSignal{},
			wantExponentialHistogram: []exponentialHistogramMetricSignal{},
		},
		{
			name: "gauge metric with attributes",
			args: args{
				md: func() pmetric.Metrics {
					md := pmetric.NewMetrics()
					resourceMetrics := md.ResourceMetrics().AppendEmpty()
					resourceMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := resourceMetrics.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")
					scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
					scopeMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeMetrics.Scope().SetName("test-scope")
					scopeMetrics.Scope().SetVersion("1.0.0")
					scopeMetrics.Scope().Attributes().PutStr("telemetry.sdk.name", "opentelemetry")
					metric := scopeMetrics.Metrics().AppendEmpty()
					metric.SetName("test.gauge")
					metric.SetDescription("Test gauge metric")
					metric.SetUnit("bytes")
					gauge := metric.SetEmptyGauge()
					dp := gauge.DataPoints().AppendEmpty()
					dp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000))
					dp.SetTimestamp(pcommon.Timestamp(1719158401000000000))
					dp.SetIntValue(1024)
					dp.Attributes().PutStr("host", "server-1")
					dp.Attributes().PutStr("region", "us-west")
					return md
				}(),
				sumEncoder:                  &sumEncoderMock{metrics: []sumMetricSignal{}},
				gaugeEncoder:                &gaugeEncoderMock{metrics: []gaugeMetricSignal{}},
				histogramEncoder:            &histogramEncoderMock{metrics: []histogramMetricSignal{}},
				exponentialHistogramEncoder: &exponentialHistogramEncoderMock{metrics: []exponentialHistogramMetricSignal{}},
			},
			wantSum: []sumMetricSignal{},
			wantGauge: []gaugeMetricSignal{
				{
					baseMetricSignal: baseMetricSignal{
						ResourceSchemaURL: "https://opentelemetry.io/schemas/1.20.0",
						ResourceAttributes: map[string]string{
							"service.name": "test-service",
							"environment":  "production",
						},
						ServiceName:    "test-service",
						ScopeName:      "test-scope",
						ScopeVersion:   "1.0.0",
						ScopeSchemaURL: "https://opentelemetry.io/schemas/1.20.0",
						ScopeAttributes: map[string]string{
							"telemetry.sdk.name": "opentelemetry",
						},
						MetricName:        "test.gauge",
						MetricDescription: "Test gauge metric",
						MetricUnit:        "bytes",
						MetricAttributes: map[string]string{
							"host":   "server-1",
							"region": "us-west",
						},
						StartTimestamp: "2024-06-23T16:00:00Z",
						Timestamp:      "2024-06-23T16:00:01Z",
						Flags:          0,
						exemplars: exemplars{
							ExemplarsFilteredAttributes: []map[string]string{},
							ExemplarsTimestamp:          []string{},
							ExemplarsValue:              []float64{},
							ExemplarsSpanID:             []string{},
							ExemplarsTraceID:            []string{},
						},
					},
					Value: 1024.0,
				},
			},
			wantHistogram:            []histogramMetricSignal{},
			wantExponentialHistogram: []exponentialHistogramMetricSignal{},
		},
		{
			name: "histogram metric with exemplars",
			args: args{
				md: func() pmetric.Metrics {
					md := pmetric.NewMetrics()
					resourceMetrics := md.ResourceMetrics().AppendEmpty()
					resourceMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := resourceMetrics.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
					scopeMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeMetrics.Scope().SetName("test-scope")
					scopeMetrics.Scope().SetVersion("1.0.0")
					metric := scopeMetrics.Metrics().AppendEmpty()
					metric.SetName("test.histogram")
					metric.SetDescription("Test histogram metric")
					metric.SetUnit("seconds")
					histogram := metric.SetEmptyHistogram()
					histogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					dp := histogram.DataPoints().AppendEmpty()
					dp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000))
					dp.SetTimestamp(pcommon.Timestamp(1719158401000000000))
					dp.SetCount(100)
					dp.SetSum(50.5)
					dp.BucketCounts().FromRaw([]uint64{10, 20, 30, 40})
					dp.ExplicitBounds().FromRaw([]float64{0.5, 1.0, 1.5})

					// Add exemplar
					exemplar := dp.Exemplars().AppendEmpty()
					exemplar.SetTimestamp(pcommon.Timestamp(1719158400500000000))
					exemplar.SetDoubleValue(1.2)
					exemplar.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					exemplar.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					exemplar.FilteredAttributes().PutStr("exemplar.type", "outlier")

					return md
				}(),
				sumEncoder:                  &sumEncoderMock{metrics: []sumMetricSignal{}},
				gaugeEncoder:                &gaugeEncoderMock{metrics: []gaugeMetricSignal{}},
				histogramEncoder:            &histogramEncoderMock{metrics: []histogramMetricSignal{}},
				exponentialHistogramEncoder: &exponentialHistogramEncoderMock{metrics: []exponentialHistogramMetricSignal{}},
			},
			wantSum:   []sumMetricSignal{},
			wantGauge: []gaugeMetricSignal{},
			wantHistogram: []histogramMetricSignal{
				{
					baseMetricSignal: baseMetricSignal{
						ResourceSchemaURL: "https://opentelemetry.io/schemas/1.20.0",
						ResourceAttributes: map[string]string{
							"service.name": "test-service",
						},
						ServiceName:       "test-service",
						ScopeName:         "test-scope",
						ScopeVersion:      "1.0.0",
						ScopeSchemaURL:    "https://opentelemetry.io/schemas/1.20.0",
						ScopeAttributes:   map[string]string{},
						MetricName:        "test.histogram",
						MetricDescription: "Test histogram metric",
						MetricUnit:        "seconds",
						MetricAttributes:  map[string]string{},
						StartTimestamp:    "2024-06-23T16:00:00Z",
						Timestamp:         "2024-06-23T16:00:01Z",
						Flags:             0,
						exemplars: exemplars{
							ExemplarsFilteredAttributes: []map[string]string{
								{
									"exemplar.type": "outlier",
								},
							},
							ExemplarsTimestamp: []string{"2024-06-23T16:00:00.5Z"},
							ExemplarsValue:     []float64{1.2},
							ExemplarsSpanID:    []string{"0102030405060708"},
							ExemplarsTraceID:   []string{"0102030405060708090a0b0c0d0e0f10"},
						},
					},
					Count:                  100,
					Sum:                    50.5,
					BucketCounts:           []uint64{10, 20, 30, 40},
					ExplicitBounds:         []float64{0.5, 1.0, 1.5},
					Min:                    nil,
					Max:                    nil,
					AggregationTemporality: 1, // Delta
				},
			},
			wantExponentialHistogram: []exponentialHistogramMetricSignal{},
		},
		{
			name: "exponential histogram metric",
			args: args{
				md: func() pmetric.Metrics {
					md := pmetric.NewMetrics()
					resourceMetrics := md.ResourceMetrics().AppendEmpty()
					resourceMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := resourceMetrics.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
					scopeMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeMetrics.Scope().SetName("test-scope")
					scopeMetrics.Scope().SetVersion("1.0.0")
					metric := scopeMetrics.Metrics().AppendEmpty()
					metric.SetName("test.exponential_histogram")
					metric.SetDescription("Test exponential histogram metric")
					metric.SetUnit("bytes")
					ehistogram := metric.SetEmptyExponentialHistogram()
					ehistogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
					dp := ehistogram.DataPoints().AppendEmpty()
					dp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000))
					dp.SetTimestamp(pcommon.Timestamp(1719158401000000000))
					dp.SetCount(200)
					dp.SetSum(100.0)
					dp.SetScale(2)
					dp.SetZeroCount(50)
					dp.Positive().SetOffset(1)
					dp.Positive().BucketCounts().FromRaw([]uint64{10, 20, 30})
					dp.Negative().SetOffset(-1)
					dp.Negative().BucketCounts().FromRaw([]uint64{5, 15, 25})
					dp.SetMin(0.1)
					dp.SetMax(10.0)
					return md
				}(),
				sumEncoder:                  &sumEncoderMock{metrics: []sumMetricSignal{}},
				gaugeEncoder:                &gaugeEncoderMock{metrics: []gaugeMetricSignal{}},
				histogramEncoder:            &histogramEncoderMock{metrics: []histogramMetricSignal{}},
				exponentialHistogramEncoder: &exponentialHistogramEncoderMock{metrics: []exponentialHistogramMetricSignal{}},
			},
			wantSum:       []sumMetricSignal{},
			wantGauge:     []gaugeMetricSignal{},
			wantHistogram: []histogramMetricSignal{},
			wantExponentialHistogram: []exponentialHistogramMetricSignal{
				{
					baseMetricSignal: baseMetricSignal{
						ResourceSchemaURL: "https://opentelemetry.io/schemas/1.20.0",
						ResourceAttributes: map[string]string{
							"service.name": "test-service",
						},
						ServiceName:       "test-service",
						ScopeName:         "test-scope",
						ScopeVersion:      "1.0.0",
						ScopeSchemaURL:    "https://opentelemetry.io/schemas/1.20.0",
						ScopeAttributes:   map[string]string{},
						MetricName:        "test.exponential_histogram",
						MetricDescription: "Test exponential histogram metric",
						MetricUnit:        "bytes",
						MetricAttributes:  map[string]string{},
						StartTimestamp:    "2024-06-23T16:00:00Z",
						Timestamp:         "2024-06-23T16:00:01Z",
						Flags:             0,
						exemplars: exemplars{
							ExemplarsFilteredAttributes: []map[string]string{},
							ExemplarsTimestamp:          []string{},
							ExemplarsValue:              []float64{},
							ExemplarsSpanID:             []string{},
							ExemplarsTraceID:            []string{},
						},
					},
					Count:                  200,
					Sum:                    100.0,
					Scale:                  2,
					ZeroCount:              50,
					PositiveOffset:         1,
					PositiveBucketCounts:   []uint64{10, 20, 30},
					NegativeOffset:         -1,
					NegativeBucketCounts:   []uint64{5, 15, 25},
					Min:                    &[]float64{0.1}[0],
					Max:                    &[]float64{10.0}[0],
					AggregationTemporality: 2, // Cumulative
				},
			},
		},
		{
			name: "mixed metric types",
			args: args{
				md: func() pmetric.Metrics {
					md := pmetric.NewMetrics()
					resourceMetrics := md.ResourceMetrics().AppendEmpty()
					resourceMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := resourceMetrics.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
					scopeMetrics.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeMetrics.Scope().SetName("test-scope")
					scopeMetrics.Scope().SetVersion("1.0.0")

					// Sum metric
					sumMetric := scopeMetrics.Metrics().AppendEmpty()
					sumMetric.SetName("test.sum")
					sumMetric.SetDescription("Test sum metric")
					sumMetric.SetUnit("count")
					sum := sumMetric.SetEmptySum()
					sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
					sum.SetIsMonotonic(true)
					sumDp := sum.DataPoints().AppendEmpty()
					sumDp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000))
					sumDp.SetTimestamp(pcommon.Timestamp(1719158401000000000))
					sumDp.SetIntValue(100)

					// Gauge metric
					gaugeMetric := scopeMetrics.Metrics().AppendEmpty()
					gaugeMetric.SetName("test.gauge")
					gaugeMetric.SetDescription("Test gauge metric")
					gaugeMetric.SetUnit("bytes")
					gauge := gaugeMetric.SetEmptyGauge()
					gaugeDp := gauge.DataPoints().AppendEmpty()
					gaugeDp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000))
					gaugeDp.SetTimestamp(pcommon.Timestamp(1719158401000000000))
					gaugeDp.SetDoubleValue(2048.5)

					return md
				}(),
				sumEncoder:                  &sumEncoderMock{metrics: []sumMetricSignal{}},
				gaugeEncoder:                &gaugeEncoderMock{metrics: []gaugeMetricSignal{}},
				histogramEncoder:            &histogramEncoderMock{metrics: []histogramMetricSignal{}},
				exponentialHistogramEncoder: &exponentialHistogramEncoderMock{metrics: []exponentialHistogramMetricSignal{}},
			},
			wantSum: []sumMetricSignal{
				{
					baseMetricSignal: baseMetricSignal{
						ResourceSchemaURL:  "https://opentelemetry.io/schemas/1.20.0",
						ResourceAttributes: map[string]string{"service.name": "test-service"},
						ServiceName:        "test-service",
						ScopeName:          "test-scope",
						ScopeVersion:       "1.0.0",
						ScopeSchemaURL:     "https://opentelemetry.io/schemas/1.20.0",
						ScopeAttributes:    map[string]string{},
						MetricName:         "test.sum",
						MetricDescription:  "Test sum metric",
						MetricUnit:         "count",
						MetricAttributes:   map[string]string{},
						StartTimestamp:     "2024-06-23T16:00:00Z",
						Timestamp:          "2024-06-23T16:00:01Z",
						Flags:              0,
						exemplars: exemplars{
							ExemplarsFilteredAttributes: []map[string]string{},
							ExemplarsTimestamp:          []string{},
							ExemplarsValue:              []float64{},
							ExemplarsSpanID:             []string{},
							ExemplarsTraceID:            []string{},
						},
					},
					Value:                  100.0,
					AggregationTemporality: 2, // Cumulative
					IsMonotonic:            true,
				},
			},
			wantGauge: []gaugeMetricSignal{
				{
					baseMetricSignal: baseMetricSignal{
						ResourceSchemaURL:  "https://opentelemetry.io/schemas/1.20.0",
						ResourceAttributes: map[string]string{"service.name": "test-service"},
						ServiceName:        "test-service",
						ScopeName:          "test-scope",
						ScopeVersion:       "1.0.0",
						ScopeSchemaURL:     "https://opentelemetry.io/schemas/1.20.0",
						ScopeAttributes:    map[string]string{},
						MetricName:         "test.gauge",
						MetricDescription:  "Test gauge metric",
						MetricUnit:         "bytes",
						MetricAttributes:   map[string]string{},
						StartTimestamp:     "2024-06-23T16:00:00Z",
						Timestamp:          "2024-06-23T16:00:01Z",
						Flags:              0,
						exemplars: exemplars{
							ExemplarsFilteredAttributes: []map[string]string{},
							ExemplarsTimestamp:          []string{},
							ExemplarsValue:              []float64{},
							ExemplarsSpanID:             []string{},
							ExemplarsTraceID:            []string{},
						},
					},
					Value: 2048.5,
				},
			},
			wantHistogram:            []histogramMetricSignal{},
			wantExponentialHistogram: []exponentialHistogramMetricSignal{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConvertMetrics(tt.args.md, tt.args.sumEncoder, tt.args.gaugeEncoder, tt.args.histogramEncoder, tt.args.exponentialHistogramEncoder); (err != nil) != tt.wantErr {
				t.Errorf("ConvertMetrics() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check sum metrics
			if !reflect.DeepEqual(tt.args.sumEncoder.(*sumEncoderMock).metrics, tt.wantSum) {
				t.Errorf("ConvertMetrics() sum metrics = %v, want %v", tt.args.sumEncoder.(*sumEncoderMock).metrics, tt.wantSum)
			}

			// Check gauge metrics
			if !reflect.DeepEqual(tt.args.gaugeEncoder.(*gaugeEncoderMock).metrics, tt.wantGauge) {
				t.Errorf("ConvertMetrics() gauge metrics = %v, want %v", tt.args.gaugeEncoder.(*gaugeEncoderMock).metrics, tt.wantGauge)
			}

			// Check histogram metrics
			if !reflect.DeepEqual(tt.args.histogramEncoder.(*histogramEncoderMock).metrics, tt.wantHistogram) {
				t.Errorf("ConvertMetrics() histogram metrics = %v, want %v", tt.args.histogramEncoder.(*histogramEncoderMock).metrics, tt.wantHistogram)
			}

			// Check exponential histogram metrics
			if !reflect.DeepEqual(tt.args.exponentialHistogramEncoder.(*exponentialHistogramEncoderMock).metrics, tt.wantExponentialHistogram) {
				t.Errorf("ConvertMetrics() exponential histogram metrics = %v, want %v", tt.args.exponentialHistogramEncoder.(*exponentialHistogramEncoderMock).metrics, tt.wantExponentialHistogram)
			}
		})
	}
}
