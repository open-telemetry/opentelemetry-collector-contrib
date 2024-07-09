// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"fmt"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var (
	now            = time.Now()
	nowMillis      = now.UnixNano() / int64(time.Millisecond)
	defaultSetting = PRWToMetricSettings{
		Logger:        *zap.NewNop(),
		TimeThreshold: 2,
	}
)

func TestIsValidCumulativeSuffix(t *testing.T) {
	type args struct {
		suffix string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "sum",
			args: args{
				suffix: "sum",
			},
			want: true,
		},
		{
			name: "count",
			args: args{
				suffix: "count",
			},
			want: true,
		},
		{
			name: "total",
			args: args{
				suffix: "total",
			},
			want: true,
		},
		{
			name: "foo",
			args: args{
				suffix: "bar",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, isValidCumulativeSuffix(tt.args.suffix), "IsValidCumulativeSuffix(%v)", tt.args.suffix)
		})
	}
}

func TestIsValidSuffix(t *testing.T) {
	type args struct {
		suffix string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "max",
			args: args{
				suffix: "max",
			},
			want: true,
		},
		{
			name: "sum",
			args: args{
				suffix: "sum",
			},
			want: true,
		},
		{
			name: "count",
			args: args{
				suffix: "count",
			},
			want: true,
		},
		{
			name: "total",
			args: args{
				suffix: "total",
			},
			want: true,
		},
		{
			name: "foo",
			args: args{
				suffix: "bar",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, isValidSuffix(tt.args.suffix), "IsValidSuffix(%v)", tt.args.suffix)
		})
	}
}

func TestGetMetricTypeAndUnit(t *testing.T) {
	tests := []struct {
		name         string
		metricName   string
		expectedType string
		expectedUnit string
	}{
		{
			name:         "Valid suffix and unit",
			metricName:   "http_request_duration_seconds_sum",
			expectedType: "sum",
			expectedUnit: "seconds",
		},
		{
			name:         "Valid unit only",
			metricName:   "http_request_duration_seconds",
			expectedType: "",
			expectedUnit: "seconds",
		},
		{
			name:         "Valid suffix only",
			metricName:   "http_request_count_total",
			expectedType: "total",
			expectedUnit: "",
		},
		{
			name:         "Invalid suffix and unit",
			metricName:   "http_request_duration_invalid",
			expectedType: "",
			expectedUnit: "",
		},
		{
			name:         "No suffix or unit",
			metricName:   "http_request_duration",
			expectedType: "",
			expectedUnit: "",
		},
		{
			name:         "Empty metric name",
			metricName:   "",
			expectedType: "",
			expectedUnit: "",
		},
		{
			name:         "Valid suffix without unit",
			metricName:   "http_request_count_sum",
			expectedType: "sum",
			expectedUnit: "",
		},
		{
			name:         "Valid unit without suffix",
			metricName:   "http_request_duration_bytes",
			expectedType: "",
			expectedUnit: "bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricType, unit := getMetricTypeAndUnit(tt.metricName)
			assert.Equal(t, tt.expectedType, metricType)
			assert.Equal(t, tt.expectedUnit, unit)
		})
	}
}

func Test_finalName(t *testing.T) {
	type args struct {
		labels []prompb.Label
	}
	tests := []struct {
		name    string
		args    args
		wantRet string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "test if __name__ label is set",
			args: args{
				labels: []prompb.Label{
					{
						Name:  nameLabel,
						Value: "foo",
					},
				},
			},
			wantRet: "foo",
			wantErr: assert.NoError,
		},
		{
			name: "test if __name__ label is not set",
			args: args{
				labels: []prompb.Label{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
			},
			wantRet: "",
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRet, err := getMetricName(tt.args.labels)
			if !tt.wantErr(t, err, fmt.Sprintf("finalName(%v)", tt.args.labels)) {
				return
			}
			assert.Equalf(t, tt.wantRet, gotRet, "finalName(%v)", tt.args.labels)
		})
	}
}

func TestPrwConfig_FromTimeSeries(t *testing.T) {
	type args struct {
		ts []prompb.TimeSeries
	}
	tests := []struct {
		name     string
		settings PRWToMetricSettings
		args     args
		want     pmetric.Metrics
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "sum",
			settings: defaultSetting,
			args: args{
				[]prompb.TimeSeries{
					{
						Labels: []prompb.Label{
							{
								Name:  nameLabel,
								Value: value71,
							},
							{
								Name:  label12,
								Value: value12,
							},
						},
						Samples: []prompb.Sample{{Value: 1.0, Timestamp: nowMillis}},
					},
				},
			},
			want: getMetrics(getSumMetric(value71, getAttributes("key_name", value71, label12, value12), pmetric.AggregationTemporalityCumulative, func(point pmetric.NumberDataPoint) {
				point.SetDoubleValue(1.)
			}, uint64(time.Now().UnixNano()), func(metric *pmetric.Metric) {
				metric.SetUnit("")
				metric.Sum().SetIsMonotonic(true)
			})),
			wantErr: assert.NoError,
		},
		{
			name:     "count",
			settings: defaultSetting,
			args: args{
				[]prompb.TimeSeries{
					{
						Labels: []prompb.Label{
							{
								Name:  nameLabel,
								Value: value61,
							},
							{
								Name:  label12,
								Value: value12,
							},
						},
						Samples: []prompb.Sample{{Value: 2.0, Timestamp: nowMillis}},
					},
				},
			},
			want: getMetrics(getSumMetric(value61, getAttributes("key_name", value61, label12, value12), pmetric.AggregationTemporalityCumulative, func(point pmetric.NumberDataPoint) {
				point.SetDoubleValue(2.)
			}, uint64(time.Now().UnixNano()), func(metric *pmetric.Metric) {
				metric.SetUnit("")
				metric.Sum().SetIsMonotonic(true)
			})),
			wantErr: assert.NoError,
		},
		{
			name:     "bytes",
			settings: defaultSetting,
			args: args{
				[]prompb.TimeSeries{
					{
						Labels: []prompb.Label{
							{
								Name:  nameLabel,
								Value: value81,
							},
							{
								Name:  label12,
								Value: value12,
							},
						},
						Samples: []prompb.Sample{{Value: 2.0, Timestamp: nowMillis}},
					},
				},
			},
			want: getMetrics(getDoubleGaugeMetric(value81, getAttributes("key_name", value81, label12, value12), 2., uint64(time.Now().UnixNano()), func(metric *pmetric.Metric) {
				metric.SetUnit("bytes")
			})),
			wantErr: assert.NoError,
		},
		{
			name:     "count - old",
			settings: defaultSetting,
			args: args{
				[]prompb.TimeSeries{
					{
						Labels: []prompb.Label{
							{
								Name:  nameLabel,
								Value: value61,
							},
							{
								Name:  label12,
								Value: value12,
							},
						},
						Samples: []prompb.Sample{{Value: 0.0, Timestamp: now.Add(-time.Hour*24).UnixNano() / int64(time.Millisecond)}},
					},
				},
			},
			want:    getMetrics(getNoneMetric(value61)),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMetrics, err := FromTimeSeries(tt.args.ts, tt.settings)
			for i := 0; i < gotMetrics.ResourceMetrics().Len(); i++ {
				rm := gotMetrics.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						gotMetric := sm.Metrics().At(k)
						testMetric := tt.want.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
						if !tt.wantErr(t, err, fmt.Sprintf("FromTimeSeries(%v)", tt.args.ts)) {
							return
						}
						if (gotMetric.Sum() != pmetric.Sum{}) {
							for i := 0; i < gotMetric.Sum().DataPoints().Len(); i++ {
								gotMetric.Sum().DataPoints().At(i).SetTimestamp(pcommon.NewTimestampFromTime(testMetric.Sum().DataPoints().At(i).Timestamp().AsTime()))
							}
						}
						if (gotMetric.Summary() != pmetric.Summary{}) {
							for i := 0; i < gotMetric.Summary().DataPoints().Len(); i++ {
								gotMetric.Summary().DataPoints().At(i).SetTimestamp(pcommon.NewTimestampFromTime(testMetric.Summary().DataPoints().At(i).Timestamp().AsTime()))
							}
						}
						if (gotMetric.Gauge() != pmetric.Gauge{}) {
							for i := 0; i < gotMetric.Gauge().DataPoints().Len(); i++ {
								gotMetric.Gauge().DataPoints().At(i).SetTimestamp(pcommon.NewTimestampFromTime(testMetric.Gauge().DataPoints().At(i).Timestamp().AsTime()))
							}
						}
						if (gotMetric.Histogram() != pmetric.Histogram{}) {
							for i := 0; i < gotMetric.Histogram().DataPoints().Len(); i++ {
								gotMetric.Histogram().DataPoints().At(i).SetTimestamp(pcommon.NewTimestampFromTime(testMetric.Histogram().DataPoints().At(i).Timestamp().AsTime()))
							}
						}
						assert.Equalf(t, testMetric, gotMetric, "FromTimeSeries(%v)", tt.args.ts)
					}
				}
			}
			assert.Equalf(t, tt.want, gotMetrics, "FromTimeSeries(%v)", tt.args.ts)
		})
	}
}

func getMetrics(metric pmetric.Metric) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	empty := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	pm := pmetric.NewMetric()
	pm.MoveTo(empty)
	metric.MoveTo(empty)
	return metrics
}

func TestPrwGaugeConvert(t *testing.T) {
	tests := []struct {
		name           string
		ts             writev2.TimeSeries
		metric         pmetric.Metric
		symbol         []string
		expected       []float64
		expectedLabels map[string]string
	}{
		{
			name: "Basic conversion",
			ts: writev2.TimeSeries{
				Samples: []writev2.Sample{
					{Value: 1.23, Timestamp: nowMillis},
					{Value: 4.56, Timestamp: nowMillis},
				},
				LabelsRefs: []uint32{1, 2, 3, 4},
			},
			metric:         pmetric.NewMetric(),
			symbol:         []string{"name", "label1", "value1", "label2", "value2"},
			expected:       []float64{1.23, 4.56},
			expectedLabels: map[string]string{"label1": "value1", "label2": "value2"},
		},
		{
			name: "Metric older than threshold",
			ts: writev2.TimeSeries{
				Samples: []writev2.Sample{
					{Value: 7.89, Timestamp: now.Add(-time.Hour*24).UnixNano() / int64(time.Millisecond)},
				},
				LabelsRefs: []uint32{1, 2},
			},
			metric:         pmetric.NewMetric(),
			symbol:         []string{"name", "label1", "value1"},
			expected:       []float64{},
			expectedLabels: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := prwGaugeConvert(tt.ts, &tt.metric, tt.symbol, defaultSetting)
			assert.NoError(t, err)
			dataPoints := tt.metric.Gauge().DataPoints()
			assert.Equal(t, len(tt.expected), dataPoints.Len())

			for i := 0; i < dataPoints.Len(); i++ {
				dp := dataPoints.At(i)
				if len(tt.expected) > 0 {
					assert.Equal(t, tt.expected[i], dp.DoubleValue())
				} else {
					assert.Equal(t, 0, dp.DoubleValue())
				}
				for k, v := range tt.expectedLabels {
					actualV, has := dp.Attributes().Get(k)
					assert.True(t, has)
					assert.Equal(t, v, actualV.Str())
				}
			}
		})
	}
}

func TestConvertExponentialBuckets(t *testing.T) {
	tests := []struct {
		name                   string
		histogram              writev2.Histogram
		expectedBucketCounts   []uint64
		expectedExplicitBounds []float64
	}{
		{
			name: "counts",
			histogram: writev2.Histogram{
				NegativeCounts: []float64{1, 2},
				NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 2}},
				PositiveCounts: []float64{3, 4},
				PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 2}},
			},
			expectedBucketCounts:   []uint64{1, 2, 3, 4},
			expectedExplicitBounds: []float64{-0, -1, 0, 1},
		},
		{
			name: "deltas",
			histogram: writev2.Histogram{
				NegativeDeltas: []int64{1, 1},
				NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 2}},
				PositiveDeltas: []int64{2, 2},
				PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 2}},
			},
			expectedBucketCounts:   []uint64{1, 2, 2, 4},
			expectedExplicitBounds: []float64{-0, -1, 0, 1},
		},
		{
			name: "multiple positive and negative buckets",
			histogram: writev2.Histogram{
				NegativeCounts: []float64{2, 3, 5},
				NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 3}},
				PositiveCounts: []float64{7, 11, 13},
				PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 3}},
			},
			expectedBucketCounts:   []uint64{2, 3, 5, 7, 11, 13},
			expectedExplicitBounds: []float64{-0, -1, -2, 0, 1, 2},
		},
		{
			name: "multiple positive and negative buckets",
			histogram: writev2.Histogram{
				NegativeCounts: []float64{3, 2, 1},
				NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 3}},
				PositiveCounts: []float64{1, 2, 3},
				PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 3}},
			},
			expectedBucketCounts:   []uint64{3, 2, 1, 1, 2, 3},
			expectedExplicitBounds: []float64{-0, -1, -2, 0, 1, 2},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			histogramMetric := pmetric.NewMetric().SetEmptyHistogram()

			dataPoint := histogramMetric.DataPoints().AppendEmpty()

			convertExponentialBuckets(test.histogram, dataPoint)
			assert.EqualValues(t, test.expectedBucketCounts, dataPoint.BucketCounts().AsRaw())
			assert.EqualValues(t, test.expectedExplicitBounds, dataPoint.ExplicitBounds().AsRaw())
		})
	}
}
