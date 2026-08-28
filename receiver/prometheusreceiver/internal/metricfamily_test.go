// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"math"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type testMetadataStore map[string]scrape.MetricMetadata

func (tmc testMetadataStore) GetMetadata(familyName string) (scrape.MetricMetadata, bool) {
	lookup, ok := tmc[familyName]
	return lookup, ok
}

func (testMetadataStore) ListMetadata() []scrape.MetricMetadata { return nil }

func (testMetadataStore) SizeMetadata() int { return 0 }

func (tmc testMetadataStore) LengthMetadata() int {
	return len(tmc)
}

var mc = testMetadataStore{
	"counter": scrape.MetricMetadata{
		MetricFamily: "cr",
		Type:         model.MetricTypeCounter,
		Help:         "This is some help for a counter",
		Unit:         "By",
	},
	"counter_created": scrape.MetricMetadata{
		MetricFamily: "counter",
		Type:         model.MetricTypeCounter,
		Help:         "This is some help for a counter",
		Unit:         "By",
	},
	"gauge": scrape.MetricMetadata{
		MetricFamily: "ge",
		Type:         model.MetricTypeGauge,
		Help:         "This is some help for a gauge",
		Unit:         "1",
	},
	"gaugehistogram": scrape.MetricMetadata{
		MetricFamily: "gh",
		Type:         model.MetricTypeGaugeHistogram,
		Help:         "This is some help for a gauge histogram",
		Unit:         "?",
	},
	"histogram": scrape.MetricMetadata{
		MetricFamily: "hg",
		Type:         model.MetricTypeHistogram,
		Help:         "This is some help for a histogram",
		Unit:         "ms",
	},
	"histogram_with_created": scrape.MetricMetadata{
		MetricFamily: "histogram_with_created",
		Type:         model.MetricTypeHistogram,
		Help:         "This is some help for a histogram",
		Unit:         "ms",
	},
	"histogram_stale": scrape.MetricMetadata{
		MetricFamily: "hg_stale",
		Type:         model.MetricTypeHistogram,
		Help:         "This is some help for a histogram",
		Unit:         "ms",
	},
	"summary": scrape.MetricMetadata{
		MetricFamily: "s",
		Type:         model.MetricTypeSummary,
		Help:         "This is some help for a summary",
		Unit:         "ms",
	},
	"summary_with_created": scrape.MetricMetadata{
		MetricFamily: "summary_with_created",
		Type:         model.MetricTypeSummary,
		Help:         "This is some help for a summary",
		Unit:         "ms",
	},
	"summary_stale": scrape.MetricMetadata{
		MetricFamily: "s_stale",
		Type:         model.MetricTypeSummary,
		Help:         "This is some help for a summary",
		Unit:         "ms",
	},
	"unknown": scrape.MetricMetadata{
		MetricFamily: "u",
		Type:         model.MetricTypeUnknown,
		Help:         "This is some help for an unknown metric",
		Unit:         "?",
	},
}

func TestMetricGroupData_toDistributionUnitTest(t *testing.T) {
	type scrape struct {
		at         int64
		value      float64
		metric     string
		extraLabel labels.Label
	}
	tests := []struct {
		name                string
		metricName          string
		labels              labels.Labels
		scrapes             []*scrape
		want                func() pmetric.HistogramDataPoint
		wantErr             bool
		intervalStartTimeMs int64
	}{
		{
			name:                "histogram with no startTimestamp",
			metricName:          "histogram",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			scrapes: []*scrape{
				{at: 11, value: 66, metric: "histogram_count"},
				{at: 11, value: 1004.78, metric: "histogram_sum"},
				{at: 11, value: 33, metric: "histogram_bucket", extraLabel: labels.Label{Name: "le", Value: "0.75"}},
				{at: 11, value: 55, metric: "histogram_bucket", extraLabel: labels.Label{Name: "le", Value: "2.75"}},
				{at: 11, value: 66, metric: "histogram_bucket", extraLabel: labels.Label{Name: "le", Value: "+Inf"}},
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetCount(66)
				point.SetSum(1004.78)
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				point.ExplicitBounds().FromRaw([]float64{0.75, 2.75})
				point.BucketCounts().FromRaw([]uint64{33, 22, 11})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name:                "histogram with startTimestamp from _created",
			metricName:          "histogram_with_created",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A"}),
			scrapes: []*scrape{
				{at: 11, value: 66, metric: "histogram_with_created_count"},
				{at: 11, value: 1004.78, metric: "histogram_with_created_sum"},
				{at: 11, value: 600.78, metric: "histogram_with_created_created"},
				{
					at:         11,
					value:      33,
					metric:     "histogram_with_created_bucket",
					extraLabel: labels.Label{Name: "le", Value: "0.75"},
				},
				{
					at:         11,
					value:      55,
					metric:     "histogram_with_created_bucket",
					extraLabel: labels.Label{Name: "le", Value: "2.75"},
				},
				{
					at:         11,
					value:      66,
					metric:     "histogram_with_created_bucket",
					extraLabel: labels.Label{Name: "le", Value: "+Inf"},
				},
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetCount(66)
				point.SetSum(1004.78)

				// the time in milliseconds -> nanoseconds.
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond))
				point.SetStartTimestamp(timestampFromFloat64(600.78))

				point.ExplicitBounds().FromRaw([]float64{0.75, 2.75})
				point.BucketCounts().FromRaw([]uint64{33, 22, 11})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				return point
			},
		},
		{
			name:                "histogram that is stale",
			metricName:          "histogram_stale",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			scrapes: []*scrape{
				{at: 11, value: math.Float64frombits(value.StaleNaN), metric: "histogram_stale_count"},
				{at: 11, value: math.Float64frombits(value.StaleNaN), metric: "histogram_stale_sum"},
				{at: 11, value: math.Float64frombits(value.StaleNaN), metric: "histogram_bucket", extraLabel: labels.Label{Name: "le", Value: "0.75"}},
				{at: 11, value: math.Float64frombits(value.StaleNaN), metric: "histogram_bucket", extraLabel: labels.Label{Name: "le", Value: "2.75"}},
				{at: 11, value: math.Float64frombits(value.StaleNaN), metric: "histogram_bucket", extraLabel: labels.Label{Name: "le", Value: "+Inf"}},
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				point.ExplicitBounds().FromRaw([]float64{0.75, 2.75})
				point.BucketCounts().FromRaw([]uint64{0, 0, 0})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name:                "histogram with inconsistent timestamps",
			metricName:          "histogram_inconsistent_ts",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "le": "0.75", "b": "B"}),
			scrapes: []*scrape{
				{at: 11, value: math.Float64frombits(value.StaleNaN), metric: "histogram_stale_count"},
				{at: 12, value: math.Float64frombits(value.StaleNaN), metric: "histogram_stale_sum"},
				{at: 13, value: math.Float64frombits(value.StaleNaN), metric: "value"},
			},
			wantErr: true,
		},
		{
			name:                "histogram without buckets",
			metricName:          "histogram",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			scrapes: []*scrape{
				{at: 11, value: 66, metric: "histogram_count"},
				{at: 11, value: 1004.78, metric: "histogram_sum"},
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetCount(66)
				point.SetSum(1004.78)
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				point.BucketCounts().FromRaw([]uint64{66})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := newMetricFamily(tt.metricName, mc, zap.NewNop(), false, false)
			for i, tv := range tt.scrapes {
				var lbls labels.Labels
				if tv.extraLabel.Name != "" {
					lbls = labels.NewBuilder(tt.labels).Set(tv.extraLabel.Name, tv.extraLabel.Value).Labels()
				} else {
					lbls = tt.labels.Copy()
				}
				sRef, _ := getSeriesRefWithoutScopeLabels(nil, lbls, mp.mtype)
				err := mp.addSeries(sRef, tv.metric, lbls, tv.at, tv.value)
				if tt.wantErr {
					if i != 0 {
						require.Error(t, err)
					}
				} else {
					require.NoError(t, err)
				}
			}
			if tt.wantErr {
				// Don't check the result if we got an error
				return
			}

			require.Len(t, mp.groups, 1)

			sl := pmetric.NewMetricSlice()
			mp.appendMetric(sl, false)

			require.Equal(t, 1, sl.Len(), "Exactly one metric expected")
			metric := sl.At(0)
			require.Equal(t, mc[tt.metricName].Help, metric.Description(), "Expected help metadata in metric description")
			require.Equal(t, mc[tt.metricName].Unit, metric.Unit(), "Expected unit metadata in metric")

			hdpL := metric.Histogram().DataPoints()
			require.Equal(t, 1, hdpL.Len(), "Exactly one point expected")
			got := hdpL.At(0)
			want := tt.want()
			require.Equal(t, want, got, "Expected the points to be equal")
		})
	}
}

func TestMetricGroupData_toNHCBDistributionUnitTest(t *testing.T) {
	tests := []struct {
		name                string
		metricName          string
		labels              labels.Labels
		integerHistogram    *histogram.Histogram
		floatHistogram      *histogram.FloatHistogram
		want                func() pmetric.HistogramDataPoint
		wantErr             bool
		intervalStartTimeMs int64
	}{
		{
			name:                "integer NHCB",
			metricName:          "histogram",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			integerHistogram: &histogram.Histogram{
				Schema:          -53,
				Count:           180,
				Sum:             100.5,
				CustomValues:    []float64{1.0, 2.0, 5.0, 10.0},
				PositiveSpans:   []histogram.Span{{Offset: 0, Length: 5}},
				PositiveBuckets: []int64{10, 15, 20, 5, 0},
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetCount(180)
				point.SetSum(100.5)
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond))
				point.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 5.0, 10.0})
				point.BucketCounts().FromRaw([]uint64{10, 25, 45, 50, 50})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name:                "integer NHCB without explicit bounds",
			metricName:          "histogram",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			integerHistogram: &histogram.Histogram{
				Schema:          histogram.CustomBucketsSchema,
				Count:           42,
				Sum:             123.5,
				PositiveSpans:   []histogram.Span{{Offset: 0, Length: 1}},
				PositiveBuckets: []int64{42},
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetCount(42)
				point.SetSum(123.5)
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond))
				point.BucketCounts().FromRaw([]uint64{42})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name:                "integer NHCB without explicit bounds with zero count",
			metricName:          "histogram",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			integerHistogram: &histogram.Histogram{
				Schema: histogram.CustomBucketsSchema,
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetSum(0)
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond))
				point.BucketCounts().FromRaw([]uint64{0})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name:                "integer NHCB that is stale",
			metricName:          "histogram",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			integerHistogram: &histogram.Histogram{
				Schema:       -53,
				Sum:          math.Float64frombits(value.StaleNaN),
				CustomValues: []float64{1.0, 2.0, 5.0, 10.0},
				Count:        0,
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond))
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				point.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 5.0, 10.0})
				point.BucketCounts().FromRaw([]uint64{0, 0, 0, 0, 0})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name:                "integer NHCB without explicit bounds that is stale",
			metricName:          "histogram",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			integerHistogram: &histogram.Histogram{
				Schema: histogram.CustomBucketsSchema,
				Sum:    math.Float64frombits(value.StaleNaN),
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond))
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				point.BucketCounts().FromRaw([]uint64{0})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name:                "float NHCB",
			metricName:          "histogram",
			intervalStartTimeMs: 12,
			labels:              labels.FromMap(map[string]string{"a": "A"}),
			floatHistogram: &histogram.FloatHistogram{
				Schema:          -53,
				Count:           50.0,
				Sum:             125.25,
				CustomValues:    []float64{0.5, 2.0},
				PositiveBuckets: []float64{15.0, 20.0, 15.0},
				PositiveSpans:   []histogram.Span{{Offset: 0, Length: 3}},
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetCount(50)
				point.SetSum(125.25)
				point.SetTimestamp(pcommon.Timestamp(12 * time.Millisecond))
				point.ExplicitBounds().FromRaw([]float64{0.5, 2.0})
				point.BucketCounts().FromRaw([]uint64{15, 20, 15})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				return point
			},
		},
		{
			name:                "float NHCB without explicit bounds",
			metricName:          "histogram",
			intervalStartTimeMs: 12,
			labels:              labels.FromMap(map[string]string{"a": "A"}),
			floatHistogram: &histogram.FloatHistogram{
				Schema:          histogram.CustomBucketsSchema,
				Count:           42,
				Sum:             123.5,
				PositiveSpans:   []histogram.Span{{Offset: 0, Length: 1}},
				PositiveBuckets: []float64{42},
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetCount(42)
				point.SetSum(123.5)
				point.SetTimestamp(pcommon.Timestamp(12 * time.Millisecond))
				point.BucketCounts().FromRaw([]uint64{42})
				point.Attributes().PutStr("a", "A")
				return point
			},
		},
		{
			name:                "float NHCB without explicit bounds with zero count",
			metricName:          "histogram",
			intervalStartTimeMs: 12,
			labels:              labels.FromMap(map[string]string{"a": "A"}),
			floatHistogram: &histogram.FloatHistogram{
				Schema: histogram.CustomBucketsSchema,
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetSum(0)
				point.SetTimestamp(pcommon.Timestamp(12 * time.Millisecond))
				point.BucketCounts().FromRaw([]uint64{0})
				point.Attributes().PutStr("a", "A")
				return point
			},
		},
		{
			name:                "float NHCB without explicit bounds that is stale",
			metricName:          "histogram",
			intervalStartTimeMs: 12,
			labels:              labels.FromMap(map[string]string{"a": "A"}),
			floatHistogram: &histogram.FloatHistogram{
				Schema: histogram.CustomBucketsSchema,
				Sum:    math.Float64frombits(value.StaleNaN),
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetTimestamp(pcommon.Timestamp(12 * time.Millisecond))
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				point.BucketCounts().FromRaw([]uint64{0})
				point.Attributes().PutStr("a", "A")
				return point
			},
		},
		{
			name:                "integer NHCB with negative boundaries",
			metricName:          "histogram",
			intervalStartTimeMs: 30,
			labels:              labels.FromMap(map[string]string{"a": "A"}),
			integerHistogram: &histogram.Histogram{
				Schema:          -53,
				Count:           16,
				Sum:             10.0,
				CustomValues:    []float64{-5.0, 0.0, 5.0},
				PositiveSpans:   []histogram.Span{{Offset: 0, Length: 4}},
				PositiveBuckets: []int64{5, 5, 5, 1},
			},
			want: func() pmetric.HistogramDataPoint {
				point := pmetric.NewHistogramDataPoint()
				point.SetCount(16)
				point.SetSum(10.0)
				point.SetTimestamp(pcommon.Timestamp(30 * time.Millisecond))
				point.ExplicitBounds().FromRaw([]float64{-5.0, 0.0, 5.0})
				point.BucketCounts().FromRaw([]uint64{5, 10, 15, 16})
				point.Attributes().PutStr("a", "A")
				return point
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := newMetricFamily(tt.metricName, mc, zap.NewNop(), false, false)
			sRef, _ := getSeriesRefWithoutScopeLabels(nil, tt.labels, mp.mtype)

			err := mp.addNHCBSeries(sRef, tt.metricName, tt.labels, tt.intervalStartTimeMs, tt.integerHistogram, tt.floatHistogram)
			require.NoError(t, err)

			require.Len(t, mp.groups, 1)

			sl := pmetric.NewMetricSlice()
			mp.appendMetric(sl, false)

			require.Equal(t, 1, sl.Len(), "Exactly one metric expected")
			metric := sl.At(0)

			hdpL := metric.Histogram().DataPoints()
			require.Equal(t, 1, hdpL.Len(), "Exactly one point expected")
			got := hdpL.At(0)
			want := tt.want()
			require.Equal(t, want, got, "Expected the points to be equal")
		})
	}
}

func TestMetricGroupData_toNHCBDistributionRejectsNonzeroSumWithZeroCount(t *testing.T) {
	tests := []struct {
		name             string
		integerHistogram *histogram.Histogram
		floatHistogram   *histogram.FloatHistogram
	}{
		{
			name: "integer NHCB",
			integerHistogram: &histogram.Histogram{
				Schema: histogram.CustomBucketsSchema,
				Sum:    123.5,
			},
		},
		{
			name: "float NHCB",
			floatHistogram: &histogram.FloatHistogram{
				Schema: histogram.CustomBucketsSchema,
				Sum:    123.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := newMetricFamily("histogram", mc, zap.NewNop(), false, false)
			lbls := labels.FromMap(map[string]string{"a": "A"})
			sRef, _ := getSeriesRefWithoutScopeLabels(nil, lbls, mp.mtype)

			err := mp.addNHCBSeries(sRef, "histogram", lbls, 11, tt.integerHistogram, tt.floatHistogram)
			require.NoError(t, err)

			sl := pmetric.NewMetricSlice()
			mp.appendMetric(sl, false)
			require.Zero(t, sl.Len())
		})
	}
}

func TestMetricGroupData_toExponentialDistributionUnitTest(t *testing.T) {
	type scrape struct {
		at         int64
		metric     string
		extraLabel labels.Label

		// Only one kind of value should be set.
		value            float64
		integerHistogram *histogram.Histogram
		floatHistogram   *histogram.FloatHistogram // TODO: add tests for float histograms.
	}
	tests := []struct {
		name                string
		metricName          string
		labels              labels.Labels
		scrapes             []*scrape
		want                func() pmetric.ExponentialHistogramDataPoint
		wantErr             bool
		intervalStartTimeMs int64
	}{
		{
			name:                "integer histogram without startTimestamp",
			metricName:          "request_duration_seconds",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			scrapes: []*scrape{
				{
					at:     11,
					metric: "request_duration_seconds",
					integerHistogram: &histogram.Histogram{
						CounterResetHint: histogram.UnknownCounterReset,
						Schema:           1,
						ZeroThreshold:    0.42,
						ZeroCount:        1,
						Count:            66,
						Sum:              1004.78,
						PositiveSpans:    []histogram.Span{{Offset: 1, Length: 2}, {Offset: 3, Length: 1}},
						PositiveBuckets:  []int64{33, -30, 26}, // Delta encoded counts: 33, 3=(33-30), 30=(3+27) -> 65
						NegativeSpans:    []histogram.Span{{Offset: 0, Length: 1}},
						NegativeBuckets:  []int64{1}, // Delta encoded counts: 1
					},
				},
			},
			want: func() pmetric.ExponentialHistogramDataPoint {
				point := pmetric.NewExponentialHistogramDataPoint()
				point.SetCount(66)
				point.SetSum(1004.78)
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				point.SetScale(1)
				point.SetZeroThreshold(0.42)
				point.SetZeroCount(1)
				point.Positive().SetOffset(0)
				point.Positive().BucketCounts().FromRaw([]uint64{33, 3, 0, 0, 0, 29})
				point.Negative().SetOffset(-1)
				point.Negative().BucketCounts().FromRaw([]uint64{1})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name:                "integer histogram with startTimestamp from _created",
			metricName:          "request_duration_seconds",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A"}),
			scrapes: []*scrape{
				{
					at:     11,
					metric: "request_duration_seconds",
					integerHistogram: &histogram.Histogram{
						CounterResetHint: histogram.UnknownCounterReset,
						Schema:           1,
						ZeroThreshold:    0.42,
						ZeroCount:        1,
						Count:            66,
						Sum:              1004.78,
						PositiveSpans:    []histogram.Span{{Offset: 1, Length: 2}, {Offset: 3, Length: 1}},
						PositiveBuckets:  []int64{33, -30, 26}, // Delta encoded counts: 33, 3=(33-30), 30=(3+27) -> 65
						NegativeSpans:    []histogram.Span{{Offset: 0, Length: 1}},
						NegativeBuckets:  []int64{1}, // Delta encoded counts: 1
					},
				},
				{
					at:     11,
					metric: "request_duration_seconds_created",
					value:  600.78,
				},
			},
			want: func() pmetric.ExponentialHistogramDataPoint {
				point := pmetric.NewExponentialHistogramDataPoint()
				point.SetCount(66)
				point.SetSum(1004.78)
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				point.SetStartTimestamp(timestampFromFloat64(600.78))        // the time in milliseconds -> nanoseconds.
				point.SetScale(1)
				point.SetZeroThreshold(0.42)
				point.SetZeroCount(1)
				point.Positive().SetOffset(0)
				point.Positive().BucketCounts().FromRaw([]uint64{33, 3, 0, 0, 0, 29})
				point.Negative().SetOffset(-1)
				point.Negative().BucketCounts().FromRaw([]uint64{1})
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				return point
			},
		},
		{
			name:                "integer histogram that is stale",
			metricName:          "request_duration_seconds",
			intervalStartTimeMs: 11,
			labels:              labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			scrapes: []*scrape{
				{
					at:     11,
					metric: "request_duration_seconds",
					integerHistogram: &histogram.Histogram{
						Sum: math.Float64frombits(value.StaleNaN),
					},
				},
			},
			want: func() pmetric.ExponentialHistogramDataPoint {
				point := pmetric.NewExponentialHistogramDataPoint()
				point.SetTimestamp(pcommon.Timestamp(11 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := newMetricFamily(tt.metricName, mc, zap.NewNop(), true, false)
			for i, tv := range tt.scrapes {
				var lbls labels.Labels
				if tv.extraLabel.Name != "" {
					lbls = labels.NewBuilder(tt.labels).Set(tv.extraLabel.Name, tv.extraLabel.Value).Labels()
				} else {
					lbls = tt.labels.Copy()
				}

				var err error
				switch {
				case tv.integerHistogram != nil:
					mp.mtype = pmetric.MetricTypeExponentialHistogram
					sRef, _ := getSeriesRefWithoutScopeLabels(nil, lbls, mp.mtype)
					err = mp.addExponentialHistogramSeries(sRef, tv.metric, lbls, tv.at, tv.integerHistogram, nil)
				case tv.floatHistogram != nil:
					mp.mtype = pmetric.MetricTypeExponentialHistogram
					sRef, _ := getSeriesRefWithoutScopeLabels(nil, lbls, mp.mtype)
					err = mp.addExponentialHistogramSeries(sRef, tv.metric, lbls, tv.at, nil, tv.floatHistogram)
				default:
					sRef, _ := getSeriesRefWithoutScopeLabels(nil, lbls, mp.mtype)
					err = mp.addSeries(sRef, tv.metric, lbls, tv.at, tv.value)
				}
				if tt.wantErr {
					if i != 0 {
						require.Error(t, err)
					}
				} else {
					require.NoError(t, err)
				}
			}
			if tt.wantErr {
				// Don't check the result if we got an error
				return
			}

			require.Len(t, mp.groups, 1)

			sl := pmetric.NewMetricSlice()
			mp.appendMetric(sl, false)

			require.Equal(t, 1, sl.Len(), "Exactly one metric expected")
			metric := sl.At(0)
			require.Equal(t, mc[tt.metricName].Help, metric.Description(), "Expected help metadata in metric description")
			require.Equal(t, mc[tt.metricName].Unit, metric.Unit(), "Expected unit metadata in metric")

			hdpL := metric.ExponentialHistogram().DataPoints()
			require.Equal(t, 1, hdpL.Len(), "Exactly one point expected")
			got := hdpL.At(0)
			want := tt.want()
			require.Equal(t, want, got, "Expected the points to be equal")
		})
	}
}

func TestMetricGroupData_toSummaryUnitTest(t *testing.T) {
	type scrape struct {
		at     int64
		value  float64
		metric string
	}

	type labelsScrapes struct {
		labels  labels.Labels
		scrapes []*scrape
	}
	tests := []struct {
		name          string
		labelsScrapes []*labelsScrapes
		want          func() pmetric.SummaryDataPoint
		wantErr       bool
	}{
		{
			name: "summary",
			labelsScrapes: []*labelsScrapes{
				{
					labels: labels.FromMap(map[string]string{"a": "A", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 10, metric: "summary_count"},
						{at: 14, value: 15, metric: "summary_sum"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.0", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 8, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.75", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 33.7, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.50", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 27, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.90", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 56, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.99", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 82, metric: "value"},
					},
				},
			},
			want: func() pmetric.SummaryDataPoint {
				point := pmetric.NewSummaryDataPoint()
				point.SetCount(10)
				point.SetSum(15)
				qtL := point.QuantileValues()
				qn0 := qtL.AppendEmpty()
				qn0.SetQuantile(0)
				qn0.SetValue(8)
				qn50 := qtL.AppendEmpty()
				qn50.SetQuantile(.5)
				qn50.SetValue(27)
				qn75 := qtL.AppendEmpty()
				qn75.SetQuantile(.75)
				qn75.SetValue(33.7)
				qn90 := qtL.AppendEmpty()
				qn90.SetQuantile(.9)
				qn90.SetValue(56)
				qn99 := qtL.AppendEmpty()
				qn99.SetQuantile(.99)
				qn99.SetValue(82)
				point.SetTimestamp(pcommon.Timestamp(14 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name: "summary_with_created",
			labelsScrapes: []*labelsScrapes{
				{
					labels: labels.FromMap(map[string]string{"a": "A", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 10, metric: "summary_with_created_count"},
						{at: 14, value: 15, metric: "summary_with_created_sum"},
						{at: 14, value: 150, metric: "summary_with_created_created"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.0", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 8, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.75", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 33.7, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.50", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 27, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.90", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 56, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.99", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 82, metric: "value"},
					},
				},
			},
			want: func() pmetric.SummaryDataPoint {
				point := pmetric.NewSummaryDataPoint()
				point.SetCount(10)
				point.SetSum(15)
				qtL := point.QuantileValues()
				qn0 := qtL.AppendEmpty()
				qn0.SetQuantile(0)
				qn0.SetValue(8)
				qn50 := qtL.AppendEmpty()
				qn50.SetQuantile(.5)
				qn50.SetValue(27)
				qn75 := qtL.AppendEmpty()
				qn75.SetQuantile(.75)
				qn75.SetValue(33.7)
				qn90 := qtL.AppendEmpty()
				qn90.SetQuantile(.9)
				qn90.SetValue(56)
				qn99 := qtL.AppendEmpty()
				qn99.SetQuantile(.99)
				qn99.SetValue(82)

				// the time in milliseconds -> nanoseconds.
				point.SetTimestamp(pcommon.Timestamp(14 * time.Millisecond))
				point.SetStartTimestamp(timestampFromFloat64(150))

				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name: "summary_stale",
			labelsScrapes: []*labelsScrapes{
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.0", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 10, metric: "summary_stale_count"},
						{at: 14, value: 12, metric: "summary_stale_sum"},
						{at: 14, value: 8, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.75", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 10, metric: "summary_stale_count"},
						{at: 14, value: 1004.78, metric: "summary_stale_sum"},
						{at: 14, value: 33.7, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.50", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 10, metric: "summary_stale_count"},
						{at: 14, value: 13, metric: "summary_stale_sum"},
						{at: 14, value: 27, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.90", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: 10, metric: "summary_stale_count"},
						{at: 14, value: 14, metric: "summary_stale_sum"},
						{at: 14, value: 56, metric: "value"},
					},
				},
				{
					labels: labels.FromMap(map[string]string{"a": "A", "quantile": "0.99", "b": "B"}),
					scrapes: []*scrape{
						{at: 14, value: math.Float64frombits(value.StaleNaN), metric: "summary_stale_count"},
						{at: 14, value: math.Float64frombits(value.StaleNaN), metric: "summary_stale_sum"},
						{at: 14, value: math.Float64frombits(value.StaleNaN), metric: "value"},
					},
				},
			},
			want: func() pmetric.SummaryDataPoint {
				point := pmetric.NewSummaryDataPoint()
				qtL := point.QuantileValues()
				qn0 := qtL.AppendEmpty()
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				qn0.SetQuantile(0)
				qn0.SetValue(0)
				qn50 := qtL.AppendEmpty()
				qn50.SetQuantile(.5)
				qn50.SetValue(0)
				qn75 := qtL.AppendEmpty()
				qn75.SetQuantile(.75)
				qn75.SetValue(0)
				qn90 := qtL.AppendEmpty()
				qn90.SetQuantile(.9)
				qn90.SetValue(0)
				qn99 := qtL.AppendEmpty()
				qn99.SetQuantile(.99)
				qn99.SetValue(0)
				point.SetTimestamp(pcommon.Timestamp(14 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name: "summary with inconsistent timestamps",
			labelsScrapes: []*labelsScrapes{
				{
					labels: labels.FromMap(map[string]string{"a": "A", "b": "B"}),
					scrapes: []*scrape{
						{at: 11, value: 10, metric: "summary_count"},
						{at: 14, value: 15, metric: "summary_sum"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := newMetricFamily(tt.name, mc, zap.NewNop(), false, false)
			for _, lbs := range tt.labelsScrapes {
				for i, scrape := range lbs.scrapes {
					lb := lbs.labels.Copy()
					sRef, _ := getSeriesRefWithoutScopeLabels(nil, lb, mp.mtype)
					err := mp.addSeries(sRef, scrape.metric, lb, scrape.at, scrape.value)
					if tt.wantErr {
						// The first scrape won't have an error
						if i != 0 {
							require.Error(t, err)
						}
					} else {
						require.NoError(t, err)
					}
				}
			}
			if tt.wantErr {
				// Don't check the result if we got an error
				return
			}

			require.Len(t, mp.groups, 1)

			sl := pmetric.NewMetricSlice()
			mp.appendMetric(sl, false)

			require.Equal(t, 1, sl.Len(), "Exactly one metric expected")
			metric := sl.At(0)
			require.Equal(t, mc[tt.name].Help, metric.Description(), "Expected help metadata in metric description")
			require.Equal(t, mc[tt.name].Unit, metric.Unit(), "Expected unit metadata in metric")

			sdpL := metric.Summary().DataPoints()
			require.Equal(t, 1, sdpL.Len(), "Exactly one point expected")
			got := sdpL.At(0)
			want := tt.want()
			require.Equal(t, want, got, "Expected the points to be equal")
		})
	}
}

func TestMetricGroupData_toNumberDataUnitTest(t *testing.T) {
	type scrape struct {
		at     int64
		value  float64
		metric string
	}
	tests := []struct {
		name                     string
		metricKind               string
		labels                   labels.Labels
		scrapes                  []*scrape
		intervalStartTimestampMs int64
		want                     func() pmetric.NumberDataPoint
	}{
		{
			metricKind:               "counter",
			name:                     "counter:: startTimestampMs from _created",
			intervalStartTimestampMs: 11,
			labels:                   labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			scrapes: []*scrape{
				{at: 13, value: 33.7, metric: "value"},
				{at: 13, value: 150, metric: "value_created"},
			},
			want: func() pmetric.NumberDataPoint {
				point := pmetric.NewNumberDataPoint()
				point.SetDoubleValue(150)

				// the time in milliseconds -> nanoseconds.
				point.SetTimestamp(pcommon.Timestamp(13 * time.Millisecond))

				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			metricKind:               "counter_created",
			name:                     "counter:: startTimestampMs from _created",
			intervalStartTimestampMs: 11,
			labels:                   labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			scrapes: []*scrape{
				{at: 13, value: 33.7, metric: "counter"},
				{at: 13, value: 150, metric: "counter_created"},
			},
			want: func() pmetric.NumberDataPoint {
				point := pmetric.NewNumberDataPoint()
				point.SetDoubleValue(33.7)

				// the time in milliseconds -> nanoseconds.
				point.SetTimestamp(pcommon.Timestamp(13 * time.Millisecond))
				point.SetStartTimestamp(timestampFromFloat64(150))

				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			metricKind:               "counter",
			name:                     "counter:: startTimestampMs of 11",
			intervalStartTimestampMs: 11,
			labels:                   labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			scrapes: []*scrape{
				{at: 13, value: 33.7, metric: "value"},
			},
			want: func() pmetric.NumberDataPoint {
				point := pmetric.NewNumberDataPoint()
				point.SetDoubleValue(33.7)
				point.SetTimestamp(pcommon.Timestamp(13 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
		{
			name:                     "counter:: startTimestampMs of 0",
			metricKind:               "counter",
			intervalStartTimestampMs: 0,
			labels:                   labels.FromMap(map[string]string{"a": "A", "b": "B"}),
			scrapes: []*scrape{
				{at: 28, value: 99.9, metric: "value"},
			},
			want: func() pmetric.NumberDataPoint {
				point := pmetric.NewNumberDataPoint()
				point.SetDoubleValue(99.9)
				point.SetTimestamp(pcommon.Timestamp(28 * time.Millisecond)) // the time in milliseconds -> nanoseconds.
				attributes := point.Attributes()
				attributes.PutStr("a", "A")
				attributes.PutStr("b", "B")
				return point
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := newMetricFamily(tt.metricKind, mc, zap.NewNop(), false, false)
			for _, tv := range tt.scrapes {
				lb := tt.labels.Copy()
				sRef, _ := getSeriesRefWithoutScopeLabels(nil, lb, mp.mtype)
				require.NoError(t, mp.addSeries(sRef, tv.metric, lb, tv.at, tv.value))
			}

			require.Len(t, mp.groups, 1)

			sl := pmetric.NewMetricSlice()
			mp.appendMetric(sl, false)

			require.Equal(t, 1, sl.Len(), "Exactly one metric expected")
			metric := sl.At(0)
			require.Equal(t, mc[tt.metricKind].Help, metric.Description(), "Expected help metadata in metric description")
			require.Equal(t, mc[tt.metricKind].Unit, metric.Unit(), "Expected unit metadata in metric")

			ndpL := metric.Sum().DataPoints()
			require.Equal(t, 1, ndpL.Len(), "Exactly one point expected")
			got := ndpL.At(0)
			want := tt.want()
			require.Equal(t, want, got, "Expected the points to be equal")
		})
	}
}

func TestConvertExemplar(t *testing.T) {
	const (
		validTraceID = "0102030405060708090a0b0c0d0e0f10"
		validSpanID  = "0102030405060708"
	)
	validTraceIDBytes := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	validSpanIDBytes := [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	tests := []struct {
		name   string
		labels labels.Labels
		want   func() pmetric.Exemplar
	}{
		{
			name: "valid ids are converted",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "trace_id", Value: validTraceID},
				labels.Label{Name: "span_id", Value: validSpanID},
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.SetTraceID(validTraceIDBytes)
				e.SetSpanID(validSpanIDBytes)
				return e
			},
		},
		// Each trace: / span: case below only sets the ID it's testing (the other
		// is either absent or a known-valid ID), so a copy-paste slip between the
		// two near-identical branches in convertExemplar shows up as a failure in
		// the right ID's own test, not silently masked by both branches always
		// being exercised together with matching validity.
		{
			name: "trace: undersized id is not zero padded and is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "trace_id", Value: "174137cab66dc880"}, // 16 hex chars / 8 bytes
				labels.Label{Name: "span_id", Value: validSpanID},
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("trace_id", "174137cab66dc880")
				e.SetSpanID(validSpanIDBytes)
				return e
			},
		},
		{
			name: "trace: oversized id is not truncated and is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "trace_id", Value: "10a47365b8aa04e08291fab9deca84db6170"}, // 36 hex chars / 18 bytes
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("trace_id", "10a47365b8aa04e08291fab9deca84db6170")
				return e
			},
		},
		{
			name: "trace: odd length id is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "trace_id", Value: "174137cab66dc88"}, // 15 hex chars, odd length
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("trace_id", "174137cab66dc88")
				return e
			},
		},
		{
			name: "trace: non-hex id is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "trace_id", Value: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"}, // 32 chars, not hex
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("trace_id", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
				return e
			},
		},
		{
			name: "trace: all-zero id is not a valid id and is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "trace_id", Value: "00000000000000000000000000000000"}, // 32 hex chars, all zero
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("trace_id", "00000000000000000000000000000000")
				return e
			},
		},
		{
			name: "span: undersized id is not zero padded and is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "trace_id", Value: validTraceID},
				labels.Label{Name: "span_id", Value: "dfa4597a9d"}, // 10 hex chars / 5 bytes
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("span_id", "dfa4597a9d")
				e.SetTraceID(validTraceIDBytes)
				return e
			},
		},
		{
			name: "span: oversized id is not truncated and is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "span_id", Value: "719cee4a669fd7d109ff"}, // 20 hex chars / 10 bytes
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("span_id", "719cee4a669fd7d109ff")
				return e
			},
		},
		{
			name: "span: odd length id is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "span_id", Value: "dfa4597a9"}, // 9 hex chars, odd length
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("span_id", "dfa4597a9")
				return e
			},
		},
		{
			name: "span: non-hex id is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "span_id", Value: "zzzzzzzzzzzzzzzz"}, // 16 chars, not hex
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("span_id", "zzzzzzzzzzzzzzzz")
				return e
			},
		},
		{
			name: "span: all-zero id is not a valid id and is kept as an attribute",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "trace_id", Value: validTraceID},
				labels.Label{Name: "span_id", Value: "0000000000000000"}, // 16 hex chars, all zero
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				e.FilteredAttributes().PutStr("span_id", "0000000000000000")
				e.SetTraceID(validTraceIDBytes)
				return e
			},
		},
		{
			name: "empty value is dropped entirely, matching absent label",
			labels: labels.New(
				labels.Label{Name: "foo", Value: "bar"},
				labels.Label{Name: "trace_id", Value: ""},
				labels.Label{Name: "span_id", Value: ""},
			),
			want: func() pmetric.Exemplar {
				e := pmetric.NewExemplar()
				e.SetTimestamp(timestampFromMs(11))
				e.SetDoubleValue(1)
				e.FilteredAttributes().PutStr("foo", "bar")
				return e
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pe := exemplar.Exemplar{
				Value:  1,
				Ts:     11,
				Labels: tt.labels,
			}
			got := pmetric.NewExemplar()
			convertExemplar(pe, got)
			require.Equal(t, tt.want(), got)
		})
	}
}
