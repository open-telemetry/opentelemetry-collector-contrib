// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusremotewrite

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

// Test_validateMetrics checks validateMetrics return true if a type and temporality combination is valid, false
// otherwise.
func Test_validateMetrics(t *testing.T) {

	// define a single test
	type combTest struct {
		name   string
		metric pdata.Metric
		want   bool
	}

	tests := []combTest{}

	// append true cases
	for k, validMetric := range validMetrics1 {
		name := "valid_" + k

		tests = append(tests, combTest{
			name,
			validMetric,
			true,
		})
	}

	for k, invalidMetric := range invalidMetrics {
		name := "invalid_" + k

		tests = append(tests, combTest{
			name,
			invalidMetric,
			false,
		})
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateMetrics(tt.metric)
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
		metric pdata.Metric
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
		addSample(tsMap, nil, nil, pdata.NewMetric())
		assert.Exactly(t, tsMap, map[string]*prompb.TimeSeries{})
	})
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addSample(tt.orig, &tt.testCase[0].sample, tt.testCase[0].labels, tt.testCase[0].metric)
			addSample(tt.orig, &tt.testCase[1].sample, tt.testCase[1].labels, tt.testCase[1].metric)
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
		metric pdata.Metric
		want   string
	}{
		{
			"int64_signature",
			promLbs1,
			validMetrics1[validIntGauge],
			validMetrics1[validIntGauge].DataType().String() + lb1Sig,
		},
		{
			"histogram_signature",
			promLbs2,
			validMetrics1[validHistogram],
			validMetrics1[validHistogram].DataType().String() + lb2Sig,
		},
		{
			"unordered_signature",
			getPromLabels(label22, value22, label21, value21),
			validMetrics1[validHistogram],
			validMetrics1[validHistogram].DataType().String() + lb2Sig,
		},
		// descriptor type cannot be nil, as checked by validateMetrics
		{
			"nil_case",
			nil,
			validMetrics1[validHistogram],
			validMetrics1[validHistogram].DataType().String(),
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualValues(t, tt.want, timeSeriesSignature(tt.metric, &tt.lbs))
		})
	}
}

// Test_createLabelSet checks resultant label names are sanitized and label in extra overrides label in labels if
// collision happens. It does not check whether labels are not sorted
func Test_createLabelSet(t *testing.T) {
	tests := []struct {
		name           string
		resource       pdata.Resource
		orig           pdata.AttributeMap
		externalLabels map[string]string
		extras         []string
		want           []prompb.Label
	}{
		{
			"labels_clean",
			getResource(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, label31, value31, label32, value32),
		},
		{
			"labels_with_resource",
			getResource("job", "prometheus", "instance", "127.0.0.1:8080"),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, label31, value31, label32, value32, "job", "prometheus", "instance", "127.0.0.1:8080"),
		},
		{
			"labels_duplicate_in_extras",
			getResource(),
			lbs1,
			map[string]string{},
			[]string{label11, value31},
			getPromLabels(label11, value31, label12, value12),
		},
		{
			"labels_dirty",
			getResource(),
			lbs1Dirty,
			map[string]string{},
			[]string{label31 + dirty1, value31, label32, value32},
			getPromLabels(label11+"_", value11, "key_"+label12, value12, label31+"_", value31, label32, value32),
		},
		{
			"no_original_case",
			getResource(),
			pdata.NewAttributeMap(),
			nil,
			[]string{label31, value31, label32, value32},
			getPromLabels(label31, value31, label32, value32),
		},
		{
			"empty_extra_case",
			getResource(),
			lbs1,
			map[string]string{},
			[]string{"", ""},
			getPromLabels(label11, value11, label12, value12, "", ""),
		},
		{
			"single_left_over_case",
			getResource(),
			lbs1,
			map[string]string{},
			[]string{label31, value31, label32},
			getPromLabels(label11, value11, label12, value12, label31, value31),
		},
		{
			"valid_external_labels",
			getResource(),
			lbs1,
			exlbs1,
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, label41, value41, label31, value31, label32, value32),
		},
		{
			"overwritten_external_labels",
			getResource(),
			lbs1,
			exlbs2,
			[]string{label31, value31, label32, value32},
			getPromLabels(label11, value11, label12, value12, label31, value31, label32, value32),
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, createAttributes(tt.resource, tt.orig, tt.externalLabels, tt.extras...))
		})
	}
}

// Tes_getPromMetricName checks if OTLP metric names are converted to Cortex metric names correctly.
// Test cases are empty namespace, monotonic metrics that require a total suffix, and metric names that contains
// invalid characters.
func Test_getPromMetricName(t *testing.T) {
	tests := []struct {
		name   string
		metric pdata.Metric
		ns     string
		want   string
	}{
		{
			"empty_case",
			invalidMetrics[empty],
			ns1,
			"test_ns_",
		},
		{
			"normal_case",
			validMetrics1[validDoubleGauge],
			ns1,
			"test_ns_" + validDoubleGauge,
		},
		{
			"empty_namespace",
			validMetrics1[validDoubleGauge],
			"",
			validDoubleGauge,
		},
		{
			// Ensure removed functionality stays removed.
			// See https://github.com/open-telemetry/opentelemetry-collector/pull/2993 for context
			"no_counter_suffix",
			validMetrics1[validIntSum],
			ns1,
			"test_ns_" + validIntSum,
		},
		{
			"dirty_string",
			validMetrics2[validIntGaugeDirty],
			"7" + ns1,
			"key_7test_ns__" + validIntGauge + "_",
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, getPromMetricName(tt.metric, tt.ns))
		})
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
		histogram *pdata.HistogramDataPoint
		expected  []prompb.Exemplar
	}{
		{
			"with_exemplars",
			getHistogramDataPointWithExemplars(tnow, floatVal1, traceIDKey, traceIDValue1),
			[]prompb.Exemplar{
				{
					Value:     floatVal1,
					Timestamp: timestamp.FromTime(tnow),
					Labels:    []prompb.Label{getLabel(traceIDKey, traceIDValue1)},
				},
			},
		},
		{
			"without_exemplar",
			getHistogramDataPoint(),
			nil,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := getPromExemplars(*tt.histogram)
			assert.Exactly(t, tt.expected, requests)
		})
	}
}

var (
	time1   = uint64(time.Now().UnixNano())
	time2   = uint64(time.Now().UnixNano() - 5)
	time3   = uint64(time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC).UnixNano())
	msTime1 = int64(time1 / uint64(int64(time.Millisecond)/int64(time.Nanosecond)))
	msTime2 = int64(time2 / uint64(int64(time.Millisecond)/int64(time.Nanosecond)))
	msTime3 = int64(time3 / uint64(int64(time.Millisecond)/int64(time.Nanosecond)))

	label11       = "test_label11"
	value11       = "test_value11"
	label12       = "test_label12"
	value12       = "test_value12"
	label21       = "test_label21"
	value21       = "test_value21"
	label22       = "test_label22"
	value22       = "test_value22"
	label31       = "test_label31"
	value31       = "test_value31"
	label32       = "test_label32"
	value32       = "test_value32"
	label41       = "__test_label41__"
	value41       = "test_value41"
	dirty1        = "%"
	dirty2        = "?"
	traceIDValue1 = "traceID-value1"
	traceIDKey    = "trace_id"

	intVal1   int64 = 1
	intVal2   int64 = 2
	floatVal1       = 1.0
	floatVal2       = 2.0
	floatVal3       = 3.0

	lbs1      = getAttributes(label11, value11, label12, value12)
	lbs2      = getAttributes(label21, value21, label22, value22)
	lbs1Dirty = getAttributes(label11+dirty1, value11, dirty2+label12, value12)

	exlbs1 = map[string]string{label41: value41}
	exlbs2 = map[string]string{label11: value41}

	promLbs1 = getPromLabels(label11, value11, label12, value12)
	promLbs2 = getPromLabels(label21, value21, label22, value22)

	lb1Sig = "-" + label11 + "-" + value11 + "-" + label12 + "-" + value12
	lb2Sig = "-" + label21 + "-" + value21 + "-" + label22 + "-" + value22
	ns1    = "test_ns"

	twoPointsSameTs = map[string]*prompb.TimeSeries{
		"Gauge" + "-" + label11 + "-" + value11 + "-" + label12 + "-" + value12: getTimeSeries(getPromLabels(label11, value11, label12, value12),
			getSample(float64(intVal1), msTime1),
			getSample(float64(intVal2), msTime2)),
	}
	twoPointsDifferentTs = map[string]*prompb.TimeSeries{
		"Gauge" + "-" + label11 + "-" + value11 + "-" + label12 + "-" + value12: getTimeSeries(getPromLabels(label11, value11, label12, value12),
			getSample(float64(intVal1), msTime1)),
		"Gauge" + "-" + label21 + "-" + value21 + "-" + label22 + "-" + value22: getTimeSeries(getPromLabels(label21, value21, label22, value22),
			getSample(float64(intVal1), msTime2)),
	}
	tsWithSamplesAndExemplars = map[string]*prompb.TimeSeries{
		lb1Sig: getTimeSeriesWithSamplesAndExemplars(getPromLabels(label11, value11, label12, value12),
			[]prompb.Sample{getSample(float64(intVal1), msTime1)},
			[]prompb.Exemplar{getExemplar(floatVal2, msTime1)}),
	}
	tsWithInfiniteBoundExemplarValue = map[string]*prompb.TimeSeries{
		lb1Sig: getTimeSeriesWithSamplesAndExemplars(getPromLabels(label11, value11, label12, value12),
			[]prompb.Sample{getSample(float64(intVal1), msTime1)},
			[]prompb.Exemplar{getExemplar(math.MaxFloat64, msTime1)}),
	}
	tsWithoutSampleAndExemplar = map[string]*prompb.TimeSeries{
		lb1Sig: getTimeSeries(getPromLabels(label11, value11, label12, value12),
			nil...),
	}
	bounds  = []float64{0.1, 0.5, 0.99}
	buckets = []uint64{1, 2, 3}

	quantileBounds = []float64{0.15, 0.9, 0.99}
	quantileValues = []float64{7, 8, 9}
	quantiles      = getQuantiles(quantileBounds, quantileValues)

	validIntGauge    = "valid_IntGauge"
	validDoubleGauge = "valid_DoubleGauge"
	validIntSum      = "valid_IntSum"
	validSum         = "valid_Sum"
	validHistogram   = "valid_Histogram"
	validSummary     = "valid_Summary"
	suffixedCounter  = "valid_IntSum_total"

	validIntGaugeDirty = "*valid_IntGauge$"

	unmatchedBoundBucketHist = "unmatchedBoundBucketHist"

	// valid metrics as input should not return error
	validMetrics1 = map[string]pdata.Metric{
		validIntGauge:    getIntGaugeMetric(validIntGauge, lbs1, intVal1, time1),
		validDoubleGauge: getDoubleGaugeMetric(validDoubleGauge, lbs1, floatVal1, time1),
		validIntSum:      getIntSumMetric(validIntSum, lbs1, intVal1, time1),
		suffixedCounter:  getIntSumMetric(suffixedCounter, lbs1, intVal1, time1),
		validSum:         getSumMetric(validSum, lbs1, floatVal1, time1),
		validHistogram:   getHistogramMetric(validHistogram, lbs1, time1, floatVal1, uint64(intVal1), bounds, buckets),
		validSummary:     getSummaryMetric(validSummary, lbs1, time1, floatVal1, uint64(intVal1), quantiles),
	}
	validMetrics2 = map[string]pdata.Metric{
		validIntGauge:            getIntGaugeMetric(validIntGauge, lbs2, intVal2, time2),
		validDoubleGauge:         getDoubleGaugeMetric(validDoubleGauge, lbs2, floatVal2, time2),
		validIntSum:              getIntSumMetric(validIntSum, lbs2, intVal2, time2),
		validSum:                 getSumMetric(validSum, lbs2, floatVal2, time2),
		validHistogram:           getHistogramMetric(validHistogram, lbs2, time2, floatVal2, uint64(intVal2), bounds, buckets),
		validSummary:             getSummaryMetric(validSummary, lbs2, time2, floatVal2, uint64(intVal2), quantiles),
		validIntGaugeDirty:       getIntGaugeMetric(validIntGaugeDirty, lbs1, intVal1, time1),
		unmatchedBoundBucketHist: getHistogramMetric(unmatchedBoundBucketHist, pdata.NewAttributeMap(), 0, 0, 0, []float64{0.1, 0.2, 0.3}, []uint64{1, 2}),
	}

	empty = "empty"

	// Category 1: type and data field doesn't match
	emptyGauge     = "emptyGauge"
	emptySum       = "emptySum"
	emptyHistogram = "emptyHistogram"
	emptySummary   = "emptySummary"

	// Category 2: invalid type and temporality combination
	emptyCumulativeSum       = "emptyCumulativeSum"
	emptyCumulativeHistogram = "emptyCumulativeHistogram"

	// different metrics that will not pass validate metrics and will cause the exporter to return an error
	invalidMetrics = map[string]pdata.Metric{
		empty:                    pdata.NewMetric(),
		emptyGauge:               getEmptyGaugeMetric(emptyGauge),
		emptySum:                 getEmptySumMetric(emptySum),
		emptyHistogram:           getEmptyHistogramMetric(emptyHistogram),
		emptySummary:             getEmptySummaryMetric(emptySummary),
		emptyCumulativeSum:       getEmptyCumulativeSumMetric(emptyCumulativeSum),
		emptyCumulativeHistogram: getEmptyCumulativeHistogramMetric(emptyCumulativeHistogram),
	}
	staleNaNIntGauge    = "staleNaNIntGauge"
	staleNaNDoubleGauge = "staleNaNDoubleGauge"
	staleNaNIntSum      = "staleNaNIntSum"
	staleNaNSum         = "staleNaNSum"
	staleNaNHistogram   = "staleNaNHistogram"
	staleNaNSummary     = "staleNaNSummary"

	// staleNaN metrics as input should have the staleness marker flag
	staleNaNMetrics = map[string]pdata.Metric{
		staleNaNIntGauge:    getIntGaugeMetric(staleNaNIntGauge, lbs1, intVal1, time1),
		staleNaNDoubleGauge: getDoubleGaugeMetric(staleNaNDoubleGauge, lbs1, floatVal1, time1),
		staleNaNIntSum:      getIntSumMetric(staleNaNIntSum, lbs1, intVal1, time1),
		staleNaNSum:         getSumMetric(staleNaNSum, lbs1, floatVal1, time1),
		staleNaNHistogram:   getHistogramMetric(staleNaNHistogram, lbs1, time1, floatVal2, uint64(intVal2), bounds, buckets),
		staleNaNSummary:     getSummaryMetric(staleNaNSummary, lbs2, time2, floatVal2, uint64(intVal2), quantiles),
	}
)

// OTLP metrics
// attributes must come in pairs
func getAttributes(labels ...string) pdata.AttributeMap {
	attributeMap := pdata.NewAttributeMap()
	for i := 0; i < len(labels); i += 2 {
		attributeMap.UpsertString(labels[i], labels[i+1])
	}
	return attributeMap
}

// Prometheus TimeSeries
func getPromLabels(lbs ...string) []prompb.Label {
	pbLbs := prompb.Labels{
		Labels: []prompb.Label{},
	}
	for i := 0; i < len(lbs); i += 2 {
		pbLbs.Labels = append(pbLbs.Labels, getLabel(lbs[i], lbs[i+1]))
	}
	return pbLbs.Labels
}

func getLabel(name string, value string) prompb.Label {
	return prompb.Label{
		Name:  name,
		Value: value,
	}
}

func getSample(v float64, t int64) prompb.Sample {
	return prompb.Sample{
		Value:     v,
		Timestamp: t,
	}
}

func getTimeSeries(labels []prompb.Label, samples ...prompb.Sample) *prompb.TimeSeries {
	return &prompb.TimeSeries{
		Labels:  labels,
		Samples: samples,
	}
}

func getExemplar(v float64, t int64) prompb.Exemplar {
	return prompb.Exemplar{
		Value:     v,
		Timestamp: t,
		Labels:    []prompb.Label{getLabel(traceIDKey, traceIDValue1)},
	}
}

func getTimeSeriesWithSamplesAndExemplars(labels []prompb.Label, samples []prompb.Sample, exemplars []prompb.Exemplar) *prompb.TimeSeries {
	return &prompb.TimeSeries{
		Labels:    labels,
		Samples:   samples,
		Exemplars: exemplars,
	}
}

func getHistogramDataPointWithExemplars(time time.Time, value float64, attributeKey string, attributeValue string) *pdata.HistogramDataPoint {
	h := pdata.NewHistogramDataPoint()

	e := h.Exemplars().AppendEmpty()
	e.SetDoubleVal(value)
	e.SetTimestamp(pdata.NewTimestampFromTime(time))
	e.FilteredAttributes().Insert(attributeKey, pdata.NewAttributeValueString(attributeValue))

	return &h
}

func getHistogramDataPoint() *pdata.HistogramDataPoint {
	h := pdata.NewHistogramDataPoint()

	return &h
}

func getQuantiles(bounds []float64, values []float64) pdata.ValueAtQuantileSlice {
	quantiles := pdata.NewValueAtQuantileSlice()
	quantiles.EnsureCapacity(len(bounds))

	for i := 0; i < len(bounds); i++ {
		quantile := quantiles.AppendEmpty()
		quantile.SetQuantile(bounds[i])
		quantile.SetValue(values[i])
	}

	return quantiles
}

func getTimeseriesMap(timeseries []*prompb.TimeSeries) map[string]*prompb.TimeSeries {
	tsMap := make(map[string]*prompb.TimeSeries)
	for i, v := range timeseries {
		tsMap[fmt.Sprintf("%s%d", "timeseries_name", i)] = v
	}
	return tsMap
}

func getEmptyGaugeMetric(name string) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeGauge)
	return metric
}

func getIntGaugeMetric(name string, attributes pdata.AttributeMap, value int64, ts uint64) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeGauge)
	dp := metric.Gauge().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetIntVal(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pdata.Timestamp(0))
	dp.SetTimestamp(pdata.Timestamp(ts))
	return metric
}

func getDoubleGaugeMetric(name string, attributes pdata.AttributeMap, value float64, ts uint64) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeGauge)
	dp := metric.Gauge().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetDoubleVal(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pdata.Timestamp(0))
	dp.SetTimestamp(pdata.Timestamp(ts))
	return metric
}

func getEmptySumMetric(name string) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSum)
	return metric
}

func getIntSumMetric(name string, attributes pdata.AttributeMap, value int64, ts uint64) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSum)
	metric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	dp := metric.Sum().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetIntVal(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pdata.Timestamp(0))
	dp.SetTimestamp(pdata.Timestamp(ts))
	return metric
}

func getEmptyCumulativeSumMetric(name string) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSum)
	metric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	return metric
}

func getSumMetric(name string, attributes pdata.AttributeMap, value float64, ts uint64) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSum)
	metric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	dp := metric.Sum().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetDoubleVal(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pdata.Timestamp(0))
	dp.SetTimestamp(pdata.Timestamp(ts))
	return metric
}

func getEmptyHistogramMetric(name string) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	return metric
}

func getEmptyCumulativeHistogramMetric(name string) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	metric.Histogram().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	return metric
}

func getHistogramMetric(name string, attributes pdata.AttributeMap, ts uint64, sum float64, count uint64, bounds []float64, buckets []uint64) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	metric.Histogram().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	dp := metric.Histogram().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetCount(count)
	dp.SetSum(sum)
	dp.SetBucketCounts(buckets)
	dp.SetExplicitBounds(bounds)
	attributes.CopyTo(dp.Attributes())

	dp.SetTimestamp(pdata.Timestamp(ts))
	return metric
}

func getEmptySummaryMetric(name string) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSummary)
	return metric
}

func getSummaryMetric(name string, attributes pdata.AttributeMap, ts uint64, sum float64, count uint64, quantiles pdata.ValueAtQuantileSlice) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSummary)
	dp := metric.Summary().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetCount(count)
	dp.SetSum(sum)
	attributes.Range(func(k string, v pdata.AttributeValue) bool {
		dp.Attributes().Upsert(k, v)
		return true
	})

	dp.SetTimestamp(pdata.Timestamp(ts))

	quantiles.CopyTo(dp.QuantileValues())
	quantiles.At(0).Quantile()

	return metric
}

func getResource(resources ...string) pdata.Resource {
	resource := pdata.NewResource()

	for i := 0; i < len(resources); i += 2 {
		resource.Attributes().Upsert(resources[i], pdata.NewAttributeValueString(resources[i+1]))
	}

	return resource
}

func getBucketBoundsData(values []float64) []bucketBoundsData {
	var b []bucketBoundsData

	for _, value := range values {
		b = append(b, bucketBoundsData{sig: lb1Sig, bound: value})
	}

	return b
}
