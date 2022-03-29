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
	"encoding/hex"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

var (
	time1   = uint64(time.Now().UnixNano())
	time2   = uint64(time.Now().UnixNano() - 5)
	msTime1 = int64(time1 / uint64(int64(time.Millisecond)/int64(time.Nanosecond)))
	msTime2 = int64(time2 / uint64(int64(time.Millisecond)/int64(time.Nanosecond)))

	label11            = "test_label11"
	value11            = "test_value11"
	label12            = "test_label12"
	value12            = "test_value12"
	label21            = "test_label21"
	value21            = "test_value21"
	label22            = "test_label22"
	value22            = "test_value22"
	label31            = "test_label31"
	value31            = "test_value31"
	label32            = "test_label32"
	value32            = "test_value32"
	label41            = "__test_label41__"
	value41            = "test_value41"
	dirty1             = "%"
	dirty2             = "?"
	traceIDValue1      = "4303853f086f4f8c86cf198b6551df84"
	spanIDValue1       = "e5513c32795c41b9"
	colliding1         = "test.colliding"
	colliding2         = "test/colliding"
	collidingSanitized = "test_colliding"
	keyWith129Runes    = "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"
	// because of the special characters, this has 132 bytes and 128 runes
	keyWith128Runes = "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii世界"
	// 64 + trace id + span id = 129 characters
	keyWith64Runes = "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"

	intVal1   int64 = 1
	intVal2   int64 = 2
	floatVal1       = 1.0
	floatVal2       = 2.0

	lbs1         = getAttributes(label11, value11, label12, value12)
	lbs2         = getAttributes(label21, value21, label22, value22)
	lbs1Dirty    = getAttributes(label11+dirty1, value11, dirty2+label12, value12)
	lbsColliding = getAttributes(colliding1, value11, colliding2, value12)

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
		unmatchedBoundBucketHist: getHistogramMetric(unmatchedBoundBucketHist, pdata.NewMap(), 0, 0, 0, []float64{0.1, 0.2, 0.3}, []uint64{1, 2}),
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
)

// OTLP metrics
// attributes must come in pairs
func getAttributes(labels ...string) pdata.Map {
	attributeMap := pdata.NewMap()
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

func getHistogramDataPointWithExemplars(t *testing.T, time time.Time, value float64, traceID string, spanID string, attributeKey string, attributeValue string) *pdata.HistogramDataPoint {
	h := pdata.NewHistogramDataPoint()

	e := h.Exemplars().AppendEmpty()
	e.SetDoubleVal(value)
	e.SetTimestamp(pdata.NewTimestampFromTime(time))
	e.FilteredAttributes().Insert(attributeKey, pdata.NewValueString(attributeValue))

	if traceID != "" {
		var traceIDBytes [16]byte
		traceIDBytesSlice, err := hex.DecodeString(traceID)
		require.NoErrorf(t, err, "error decoding trace id: %v", err)
		copy(traceIDBytes[:], traceIDBytesSlice)
		e.SetTraceID(pdata.NewTraceID(traceIDBytes))
	}
	if spanID != "" {
		var spanIDBytes [8]byte
		spanIDBytesSlice, err := hex.DecodeString(spanID)
		require.NoErrorf(t, err, "error decoding span id: %v", err)
		copy(spanIDBytes[:], spanIDBytesSlice)
		e.SetSpanID(pdata.NewSpanID(spanIDBytes))
	}

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

func getEmptyGaugeMetric(name string) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeGauge)
	return metric
}

func getIntGaugeMetric(name string, attributes pdata.Map, value int64, ts uint64) pdata.Metric {
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

func getDoubleGaugeMetric(name string, attributes pdata.Map, value float64, ts uint64) pdata.Metric {
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

func getIntSumMetric(name string, attributes pdata.Map, value int64, ts uint64) pdata.Metric {
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

func getSumMetric(name string, attributes pdata.Map, value float64, ts uint64) pdata.Metric {
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

func getHistogramMetric(name string, attributes pdata.Map, ts uint64, sum float64, count uint64, bounds []float64, buckets []uint64) pdata.Metric {
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

func getSummaryMetric(name string, attributes pdata.Map, ts uint64, sum float64, count uint64, quantiles pdata.ValueAtQuantileSlice) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSummary)
	dp := metric.Summary().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetCount(count)
	dp.SetSum(sum)
	attributes.Range(func(k string, v pdata.Value) bool {
		dp.Attributes().Upsert(k, v)
		return true
	})

	dp.SetTimestamp(pdata.Timestamp(ts))

	quantiles.CopyTo(dp.QuantileValues())
	quantiles.At(0).Quantile()

	return metric
}

func getResource(resources map[string]pdata.Value) pdata.Resource {
	resource := pdata.NewResource()

	for k, v := range resources {
		resource.Attributes().Upsert(k, v)
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
