// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"encoding/hex"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	label51            = "_test_label51"
	value51            = "test_value51"
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
	lbs3         = getAttributes(label11, value11, label12, value12, label51, value51)
	lbs1Dirty    = getAttributes(label11+dirty1, value11, dirty2+label12, value12)
	lbsColliding = getAttributes(colliding1, value11, colliding2, value12)

	exlbs1 = map[string]string{label41: value41}
	exlbs2 = map[string]string{label11: value41}

	promLbs1 = getPromLabels(label11, value11, label12, value12)
	promLbs2 = getPromLabels(label21, value21, label22, value22)

	lb1Sig = "-" + label11 + "-" + value11 + "-" + label12 + "-" + value12
	lb2Sig = "-" + label21 + "-" + value21 + "-" + label22 + "-" + value22

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

	validIntGauge    = "valid_IntGauge"
	validDoubleGauge = "valid_DoubleGauge"
	validHistogram   = "valid_Histogram"

	// valid metrics as input should not return error
	validMetrics1 = map[string]pmetric.Metric{
		validIntGauge:    getIntGaugeMetric(validIntGauge, lbs1, intVal1, time1),
		validDoubleGauge: getDoubleGaugeMetric(validDoubleGauge, lbs1, floatVal1, time1),
		validHistogram: getHistogramMetric(
			validHistogram,
			lbs1,
			pmetric.AggregationTemporalityCumulative,
			time1,
			floatVal1,
			uint64(intVal1),
			[]float64{0.1, 0.5, 0.99}, []uint64{1, 2, 3},
		),
	}
)

// OTLP metrics
// attributes must come in pairs
func getAttributes(labels ...string) pcommon.Map {
	attributeMap := pcommon.NewMap()
	for i := 0; i < len(labels); i += 2 {
		attributeMap.PutStr(labels[i], labels[i+1])
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

func getHistogramDataPointWithExemplars(t *testing.T, time time.Time, value float64, traceID string, spanID string, attributeKey string, attributeValue string) pmetric.HistogramDataPoint {
	h := pmetric.NewHistogramDataPoint()

	e := h.Exemplars().AppendEmpty()
	e.SetDoubleValue(value)
	e.SetTimestamp(pcommon.NewTimestampFromTime(time))
	e.FilteredAttributes().PutStr(attributeKey, attributeValue)

	if traceID != "" {
		var traceIDBytes [16]byte
		traceIDBytesSlice, err := hex.DecodeString(traceID)
		require.NoErrorf(t, err, "error decoding trace id: %v", err)
		copy(traceIDBytes[:], traceIDBytesSlice)
		e.SetTraceID(traceIDBytes)
	}
	if spanID != "" {
		var spanIDBytes [8]byte
		spanIDBytesSlice, err := hex.DecodeString(spanID)
		require.NoErrorf(t, err, "error decoding span id: %v", err)
		copy(spanIDBytes[:], spanIDBytesSlice)
		e.SetSpanID(spanIDBytes)
	}

	return h
}

func getIntGaugeMetric(name string, attributes pcommon.Map, value int64, ts uint64) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetIntValue(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pcommon.Timestamp(0))
	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getDoubleGaugeMetric(name string, attributes pcommon.Map, value float64, ts uint64) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetDoubleValue(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pcommon.Timestamp(0))
	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getIntSumMetric(
	name string,
	attributes pcommon.Map,
	temporality pmetric.AggregationTemporality,
	value int64,
	ts uint64,
) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptySum().SetAggregationTemporality(temporality)
	dp := metric.Sum().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetIntValue(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pcommon.Timestamp(0))
	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getHistogramMetric(
	name string,
	attributes pcommon.Map,
	temporality pmetric.AggregationTemporality,
	ts uint64,
	sum float64,
	count uint64,
	bounds []float64,
	buckets []uint64,
) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptyHistogram().SetAggregationTemporality(temporality)
	dp := metric.Histogram().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetCount(count)
	dp.SetSum(sum)
	dp.BucketCounts().FromRaw(buckets)
	dp.ExplicitBounds().FromRaw(bounds)
	attributes.CopyTo(dp.Attributes())

	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getSummaryMetric(name string, attributes pcommon.Map, ts uint64, sum float64, count uint64, quantiles pmetric.SummaryDataPointValueAtQuantileSlice) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	dp := metric.SetEmptySummary().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetCount(count)
	dp.SetSum(sum)
	attributes.Range(func(k string, v pcommon.Value) bool {
		v.CopyTo(dp.Attributes().PutEmpty(k))
		return true
	})

	dp.SetTimestamp(pcommon.Timestamp(ts))

	quantiles.CopyTo(dp.QuantileValues())
	quantiles.At(0).Quantile()

	return metric
}

func getBucketBoundsData(values []float64) []bucketBoundsData {
	b := make([]bucketBoundsData, len(values))

	for i, value := range values {
		b[i] = bucketBoundsData{sig: lb1Sig, bound: value}
	}

	return b
}
