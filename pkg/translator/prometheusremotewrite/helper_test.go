// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"math"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func Test_isValidAggregationTemporality(t *testing.T) {
	l := pcommon.NewMap()

	tests := []struct {
		name   string
		metric pmetric.Metric
		want   bool
	}{
		{
			name: "summary",
			metric: func() pmetric.Metric {
				quantiles := pmetric.NewSummaryDataPointValueAtQuantileSlice()
				quantiles.AppendEmpty().SetValue(1)
				return getSummaryMetric("", l, 0, 0, 0, quantiles)
			}(),
			want: true,
		},
		{
			name:   "gauge",
			metric: getIntGaugeMetric("", l, 0, 0),
			want:   true,
		},
		{
			name:   "cumulative sum",
			metric: getIntSumMetric("", l, pmetric.AggregationTemporalityCumulative, 0, 0),
			want:   true,
		},
		{
			name: "cumulative histogram",
			metric: getHistogramMetric(
				"", l, pmetric.AggregationTemporalityCumulative, 0, 0, 0, []float64{}, []uint64{}),
			want: true,
		},
		{
			name: "cumulative exponential histogram",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				h := metric.SetEmptyExponentialHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				return metric
			}(),
			want: true,
		},
		{
			name:   "missing type",
			metric: pmetric.NewMetric(),
			want:   false,
		},
		{
			name:   "unspecified sum temporality",
			metric: getIntSumMetric("", l, pmetric.AggregationTemporalityUnspecified, 0, 0),
			want:   false,
		},
		{
			name:   "delta sum",
			metric: getIntSumMetric("", l, pmetric.AggregationTemporalityDelta, 0, 0),
			want:   false,
		},
		{
			name: "delta histogram",
			metric: getHistogramMetric(
				"", l, pmetric.AggregationTemporalityDelta, 0, 0, 0, []float64{}, []uint64{}),
			want: false,
		},
		{
			name: "delta exponential histogram",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				h := metric.SetEmptyExponentialHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				return metric
			}(),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidAggregationTemporality(tt.metric)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test_addSample checks addSample updates the map it receives correctly based on the sample and Label
// set it receives.
// Test cases are two samples belonging to the same TimeSeries,  two samples belong to different TimeSeries, and nil
// case.
func Test_addSample(t *testing.T) {
	type testCase struct {
		metric pmetric.Metric
		sample prompb.Sample
		labels []prompb.Label
	}

	tests := []struct {
		name     string
		orig     map[string]*prompb.TimeSeries
		testCase []testCase
		want     map[string]*prompb.TimeSeries
	}{
		{
			"two_points_same_ts_same_metric",
			map[string]*prompb.TimeSeries{},
			[]testCase{
				{validMetrics1[validDoubleGauge],
					getSample(floatVal1, msTime1),
					promLbs1,
				},
				{
					validMetrics1[validDoubleGauge],
					getSample(floatVal2, msTime2),
					promLbs1,
				},
			},
			twoPointsSameTs,
		},
		{
			"two_points_different_ts_same_metric",
			map[string]*prompb.TimeSeries{},
			[]testCase{
				{validMetrics1[validIntGauge],
					getSample(float64(intVal1), msTime1),
					promLbs1,
				},
				{validMetrics1[validIntGauge],
					getSample(float64(intVal1), msTime2),
					promLbs2,
				},
			},
			twoPointsDifferentTs,
		},
	}
	t.Run("empty_case", func(t *testing.T) {
		tsMap := map[string]*prompb.TimeSeries{}
		addSample(tsMap, nil, nil, "")
		assert.Exactly(t, tsMap, map[string]*prompb.TimeSeries{})
	})
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addSample(tt.orig, &tt.testCase[0].sample, tt.testCase[0].labels, tt.testCase[0].metric.Type().String())
			addSample(tt.orig, &tt.testCase[1].sample, tt.testCase[1].labels, tt.testCase[1].metric.Type().String())
			assert.Exactly(t, tt.want, tt.orig)
		})
	}
}

// Test_timeSeries checks timeSeriesSignature returns consistent and unique signatures for a distinct label set and
// metric type combination.
func Test_timeSeriesSignature(t *testing.T) {
	tests := []struct {
		name   string
		lbs    []prompb.Label
		metric pmetric.Metric
		want   string
	}{
		{
			"int64_signature",
			promLbs1,
			validMetrics1[validIntGauge],
			validMetrics1[validIntGauge].Type().String() + lb1Sig,
		},
		{
			"histogram_signature",
			promLbs2,
			validMetrics1[validHistogram],
			validMetrics1[validHistogram].Type().String() + lb2Sig,
		},
		{
			"unordered_signature",
			getPromLabels(label22, value22, label21, value21),
			validMetrics1[validHistogram],
			validMetrics1[validHistogram].Type().String() + lb2Sig,
		},
		// descriptor type cannot be nil, as checked by validateAggregationTemporality
		{
			"nil_case",
			nil,
			validMetrics1[validHistogram],
			validMetrics1[validHistogram].Type().String(),
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualValues(t, tt.want, timeSeriesSignature(tt.metric.Type().String(), &tt.lbs))
		})
	}
}

// Test_createLabelSet checks resultant label names are sanitized and label in extra overrides label in labels if
// collision happens. It does not check whether labels are not sorted
func Test_createLabelSet(t *testing.T) {
	tests := []struct {
		name           string
		resource       pcommon.Resource
		orig           pcommon.Map
		externalLabels map[string]string
		extras         []string
		want           []prompb.Label
	}{
		{
			"labels_clean",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, label31, value31, label32, value32),
		},
		{
			"labels_with_resource",
			func() pcommon.Resource {
				res := pcommon.NewResource()
				res.Attributes().PutStr("service.name", "prometheus")
				res.Attributes().PutStr("service.instance.id", "127.0.0.1:8080")
				return res
			}(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, label31, value31, label32, value32, "job", "prometheus", "instance", "127.0.0.1:8080"),
		},
		{
			"labels_with_nonstring_resource",
			func() pcommon.Resource {
				res := pcommon.NewResource()
				res.Attributes().PutInt("service.name", 12345)
				res.Attributes().PutBool("service.instance.id", true)
				return res
			}(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, label31, value31, label32, value32, "job", "12345", "instance", "true"),
		},
		{
			"labels_duplicate_in_extras",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{label11, value31},
			getPromLabels(label11, value31, label12, value12),
		},
		{
			"labels_dirty",
			pcommon.NewResource(),
			lbs1Dirty,
			map[string]string{},
			[]string{label31 + dirty1, value31, label32, value32},
			getPromLabels(label11+"_", value11, "key_"+label12, value12, label31+"_", value31, label32, value32),
		},
		{
			"no_original_case",
			pcommon.NewResource(),
			pcommon.NewMap(),
			nil,
			[]string{label31, value31, label32, value32},
			getPromLabels(label31, value31, label32, value32),
		},
		{
			"empty_extra_case",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{"", ""},
			getPromLabels(label11, value11, label12, value12, "", ""),
		},
		{
			"single_left_over_case",
			pcommon.NewResource(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32},
			getPromLabels(label11, value11, label12, value12, label31, value31),
		},
		{
			"valid_external_labels",
			pcommon.NewResource(),
			lbs1,
			exlbs1,
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, label41, value41, label31, value31, label32, value32),
		},
		{
			"overwritten_external_labels",
			pcommon.NewResource(),
			lbs1,
			exlbs2,
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, label31, value31, label32, value32),
		},
		{
			"colliding attributes",
			pcommon.NewResource(),
			lbsColliding,
			nil,
			[]string{label31, value31, label32, value32},
			getPromLabels(collidingSanitized, value11+";"+value12, label31, value31, label32, value32),
		},
		{
			"sanitize_labels_starts_with_underscore",
			pcommon.NewResource(),
			lbs3,
			exlbs1,
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, "key"+label51, value51, label41, value41, label31, value31, label32, value32),
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, createAttributes(tt.resource, tt.orig, tt.externalLabels, tt.extras...))
		})
	}
}

func BenchmarkCreateAttributes(b *testing.B) {
	r := pcommon.NewResource()
	ext := map[string]string{}

	m := pcommon.NewMap()
	m.PutStr("test-string-key2", "test-value-2")
	m.PutStr("test-string-key1", "test-value-1")
	m.PutInt("test-int-key", 123)
	m.PutBool("test-bool-key", true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		createAttributes(r, m, ext)
	}
}

// Test_addExemplars checks addExemplars updates the map it receives correctly based on the exemplars and bucket bounds data it receives.
func Test_addExemplars(t *testing.T) {
	type testCase struct {
		exemplars    []prompb.Exemplar
		bucketBounds []bucketBoundsData
	}

	tests := []struct {
		name     string
		orig     map[string]*prompb.TimeSeries
		testCase []testCase
		want     map[string]*prompb.TimeSeries
	}{
		{
			"timeSeries_is_empty",
			map[string]*prompb.TimeSeries{},
			[]testCase{
				{
					[]prompb.Exemplar{getExemplar(float64(intVal1), msTime1)},
					getBucketBoundsData([]float64{1, 2, 3}),
				},
			},
			map[string]*prompb.TimeSeries{},
		},
		{
			"timeSeries_without_sample",
			tsWithoutSampleAndExemplar,
			[]testCase{
				{
					[]prompb.Exemplar{getExemplar(float64(intVal1), msTime1)},
					getBucketBoundsData([]float64{1, 2, 3}),
				},
			},
			tsWithoutSampleAndExemplar,
		},
		{
			"exemplar_value_less_than_bucket_bound",
			map[string]*prompb.TimeSeries{
				lb1Sig: getTimeSeries(getPromLabels(label11, value11, label12, value12),
					getSample(float64(intVal1), msTime1)),
			},
			[]testCase{
				{
					[]prompb.Exemplar{getExemplar(floatVal2, msTime1)},
					getBucketBoundsData([]float64{1, 2, 3}),
				},
			},
			tsWithSamplesAndExemplars,
		},
		{
			"infinite_bucket_bound",
			map[string]*prompb.TimeSeries{
				lb1Sig: getTimeSeries(getPromLabels(label11, value11, label12, value12),
					getSample(float64(intVal1), msTime1)),
			},
			[]testCase{
				{
					[]prompb.Exemplar{getExemplar(math.MaxFloat64, msTime1)},
					getBucketBoundsData([]float64{1, math.Inf(1)}),
				},
			},
			tsWithInfiniteBoundExemplarValue,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addExemplars(tt.orig, tt.testCase[0].exemplars, tt.testCase[0].bucketBounds)
			assert.Exactly(t, tt.want, tt.orig)
		})
	}
}

// Test_getPromExemplars checks if exemplars is not nul and return the prometheus exemplars.
func Test_getPromExemplars(t *testing.T) {
	tnow := time.Now()
	tests := []struct {
		name      string
		histogram pmetric.HistogramDataPoint
		expected  []prompb.Exemplar
	}{
		{
			"with_exemplars",
			getHistogramDataPointWithExemplars(t, tnow, floatVal1, traceIDValue1, spanIDValue1, label11, value11),
			[]prompb.Exemplar{
				{
					Value:     floatVal1,
					Timestamp: timestamp.FromTime(tnow),
					Labels:    []prompb.Label{getLabel(traceIDKey, traceIDValue1), getLabel(spanIDKey, spanIDValue1), getLabel(label11, value11)},
				},
			},
		},
		{
			"with_exemplars_without_trace_or_span",
			getHistogramDataPointWithExemplars(t, tnow, floatVal1, "", "", label11, value11),
			[]prompb.Exemplar{
				{
					Value:     floatVal1,
					Timestamp: timestamp.FromTime(tnow),
					Labels:    []prompb.Label{getLabel(label11, value11)},
				},
			},
		},
		{
			"too_many_runes_drops_labels",
			getHistogramDataPointWithExemplars(t, tnow, floatVal1, "", "", keyWith129Runes, ""),
			[]prompb.Exemplar{
				{
					Value:     floatVal1,
					Timestamp: timestamp.FromTime(tnow),
				},
			},
		},
		{
			"runes_at_limit_bytes_over_keeps_labels",
			getHistogramDataPointWithExemplars(t, tnow, floatVal1, "", "", keyWith128Runes, ""),
			[]prompb.Exemplar{
				{
					Value:     floatVal1,
					Timestamp: timestamp.FromTime(tnow),
					Labels:    []prompb.Label{getLabel(keyWith128Runes, "")},
				},
			},
		},
		{
			"too_many_runes_with_exemplar_drops_attrs_keeps_exemplar",
			getHistogramDataPointWithExemplars(t, tnow, floatVal1, traceIDValue1, spanIDValue1, keyWith64Runes, ""),
			[]prompb.Exemplar{
				{
					Value:     floatVal1,
					Timestamp: timestamp.FromTime(tnow),
					Labels:    []prompb.Label{getLabel(traceIDKey, traceIDValue1), getLabel(spanIDKey, spanIDValue1)},
				},
			},
		},
		{
			"without_exemplar",
			pmetric.NewHistogramDataPoint(),
			nil,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := getPromExemplars(tt.histogram)
			assert.Exactly(t, tt.expected, requests)
		})
	}
}

func TestAddResourceTargetInfo(t *testing.T) {
	resourceAttrMap := map[string]interface{}{
		conventions.AttributeServiceName:       "service-name",
		conventions.AttributeServiceNamespace:  "service-namespace",
		conventions.AttributeServiceInstanceID: "service-instance-id",
	}
	resourceWithServiceAttrs := pcommon.NewResource()
	assert.NoError(t, resourceWithServiceAttrs.Attributes().FromRaw(resourceAttrMap))
	resourceWithServiceAttrs.Attributes().PutStr("resource_attr", "resource-attr-val-1")
	resourceWithOnlyServiceAttrs := pcommon.NewResource()
	assert.NoError(t, resourceWithOnlyServiceAttrs.Attributes().FromRaw(resourceAttrMap))
	for _, tc := range []struct {
		desc      string
		resource  pcommon.Resource
		settings  Settings
		timestamp pcommon.Timestamp
		expected  map[string]*prompb.TimeSeries
	}{
		{
			desc:     "empty resource",
			resource: pcommon.NewResource(),
			expected: map[string]*prompb.TimeSeries{},
		},
		{
			desc:     "disable target info metric",
			resource: testdata.GenerateMetricsNoLibraries().ResourceMetrics().At(0).Resource(),
			settings: Settings{DisableTargetInfo: true},
			expected: map[string]*prompb.TimeSeries{},
		},
		{
			desc:      "with resource",
			resource:  testdata.GenerateMetricsNoLibraries().ResourceMetrics().At(0).Resource(),
			timestamp: testdata.TestMetricStartTimestamp,
			expected: map[string]*prompb.TimeSeries{
				"info-__name__-target_info-resource_attr-resource-attr-val-1": {
					Labels: []prompb.Label{
						{
							Name:  "__name__",
							Value: "target_info",
						},
						{
							Name:  "resource_attr",
							Value: "resource-attr-val-1",
						},
					},
					Samples: []prompb.Sample{
						{
							Value:     1,
							Timestamp: 1581452772000,
						},
					},
				},
			},
		},
		{
			desc:      "with resource, with namespace",
			resource:  testdata.GenerateMetricsNoLibraries().ResourceMetrics().At(0).Resource(),
			timestamp: testdata.TestMetricStartTimestamp,
			settings:  Settings{Namespace: "foo"},
			expected: map[string]*prompb.TimeSeries{
				"info-__name__-foo_target_info-resource_attr-resource-attr-val-1": {
					Labels: []prompb.Label{
						{
							Name:  "__name__",
							Value: "foo_target_info",
						},
						{
							Name:  "resource_attr",
							Value: "resource-attr-val-1",
						},
					},
					Samples: []prompb.Sample{
						{
							Value:     1,
							Timestamp: 1581452772000,
						},
					},
				},
			},
		},
		{
			desc:      "with resource, with service attributes",
			resource:  resourceWithServiceAttrs,
			timestamp: testdata.TestMetricStartTimestamp,
			expected: map[string]*prompb.TimeSeries{
				"info-__name__-target_info-instance-service-instance-id-job-service-namespace/service-name-resource_attr-resource-attr-val-1": {
					Labels: []prompb.Label{
						{
							Name:  "__name__",
							Value: "target_info",
						},
						{
							Name:  "instance",
							Value: "service-instance-id",
						},
						{
							Name:  "job",
							Value: "service-namespace/service-name",
						},
						{
							Name:  "resource_attr",
							Value: "resource-attr-val-1",
						},
					},
					Samples: []prompb.Sample{
						{
							Value:     1,
							Timestamp: 1581452772000,
						},
					},
				},
			},
		},
		{
			desc:      "with resource, with only service attributes",
			resource:  resourceWithOnlyServiceAttrs,
			timestamp: testdata.TestMetricStartTimestamp,
			expected:  map[string]*prompb.TimeSeries{},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			tsMap := map[string]*prompb.TimeSeries{}
			addResourceTargetInfo(tc.resource, tc.settings, tc.timestamp, tsMap)
			assert.Exactly(t, tc.expected, tsMap)
		})
	}
}

func TestMostRecentTimestampInMetric(t *testing.T) {
	laterTimestamp := pcommon.NewTimestampFromTime(testdata.TestMetricTime.Add(1 * time.Minute))
	metricMultipleTimestamps := testdata.GenerateMetricsOneMetric().ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	// the first datapoint timestamp is at testdata.TestMetricTime
	metricMultipleTimestamps.Sum().DataPoints().At(1).SetTimestamp(laterTimestamp)
	for _, tc := range []struct {
		desc     string
		input    pmetric.Metric
		expected pcommon.Timestamp
	}{
		{
			desc:     "empty",
			input:    pmetric.NewMetric(),
			expected: pcommon.Timestamp(0),
		},
		{
			desc:     "multiple timestamps",
			input:    metricMultipleTimestamps,
			expected: laterTimestamp,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := mostRecentTimestampInMetric(tc.input)
			assert.Exactly(t, tc.expected, got)
		})
	}
}

func TestAddSingleSummaryDataPoint(t *testing.T) {
	ts := pcommon.Timestamp(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   func() map[string]*prompb.TimeSeries
	}{
		{
			name: "summary with start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_summary")
				metric.SetEmptySummary()

				dp := metric.Summary().DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)
				dp.SetStartTimestamp(ts)

				return metric
			},
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_summary" + countStr},
				}
				createdLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_summary" + createdSuffix},
				}
				sumLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_summary" + sumStr},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeSummary.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
					},
					timeSeriesSignature(pmetric.MetricTypeSummary.String(), &sumLabels): {
						Labels: sumLabels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
					},
					timeSeriesSignature(pmetric.MetricTypeSummary.String(), &createdLabels): {
						Labels: createdLabels,
						Samples: []prompb.Sample{
							{Value: float64(convertTimeStamp(ts))},
						},
					},
				}
			},
		},
		{
			name: "summary without start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_summary")
				metric.SetEmptySummary()

				dp := metric.Summary().DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)

				return metric
			},
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_summary" + countStr},
				}
				sumLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_summary" + sumStr},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeSummary.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
					},
					timeSeriesSignature(pmetric.MetricTypeSummary.String(), &sumLabels): {
						Labels: sumLabels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()

			got := make(map[string]*prompb.TimeSeries)
			for x := 0; x < metric.Summary().DataPoints().Len(); x++ {
				addSingleSummaryDataPoint(
					metric.Summary().DataPoints().At(x),
					pcommon.NewResource(),
					metric,
					Settings{
						ExportCreatedMetric: true,
					},
					got,
				)
			}
			assert.Equal(t, tt.want(), got)
		})
	}
}

func TestAddSingleHistogramDataPoint(t *testing.T) {
	ts := pcommon.Timestamp(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   func() map[string]*prompb.TimeSeries
	}{
		{
			name: "histogram with start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_hist")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				pt := metric.Histogram().DataPoints().AppendEmpty()
				pt.SetTimestamp(ts)
				pt.SetStartTimestamp(ts)

				return metric
			},
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist" + countStr},
				}
				createdLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist" + createdSuffix},
				}
				infLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist_bucket"},
					{Name: model.BucketLabel, Value: "+Inf"},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeHistogram.String(), &infLabels): {
						Labels: infLabels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
					},
					timeSeriesSignature(pmetric.MetricTypeHistogram.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
					},
					timeSeriesSignature(pmetric.MetricTypeHistogram.String(), &createdLabels): {
						Labels: createdLabels,
						Samples: []prompb.Sample{
							{Value: float64(convertTimeStamp(ts))},
						},
					},
				}
			},
		},
		{
			name: "histogram without start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_hist")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				pt := metric.Histogram().DataPoints().AppendEmpty()
				pt.SetTimestamp(ts)

				return metric
			},
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist" + countStr},
				}
				infLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist_bucket"},
					{Name: model.BucketLabel, Value: "+Inf"},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeHistogram.String(), &infLabels): {
						Labels: infLabels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
					},
					timeSeriesSignature(pmetric.MetricTypeHistogram.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()

			got := make(map[string]*prompb.TimeSeries)
			for x := 0; x < metric.Histogram().DataPoints().Len(); x++ {
				addSingleHistogramDataPoint(
					metric.Histogram().DataPoints().At(x),
					pcommon.NewResource(),
					metric,
					Settings{
						ExportCreatedMetric: true,
					},
					got,
				)
			}
			assert.Equal(t, tt.want(), got)
		})
	}
}
