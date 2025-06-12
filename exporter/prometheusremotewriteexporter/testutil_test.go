// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var (
	time1   = uint64(time.Now().UnixNano())
	time2   = uint64(time.Now().UnixNano() - 5)
	time3   = uint64(time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC).UnixNano())
	msTime1 = int64(time1 / uint64(int64(time.Millisecond)/int64(time.Nanosecond)))
	msTime2 = int64(time2 / uint64(int64(time.Millisecond)/int64(time.Nanosecond)))
	msTime3 = int64(time3 / uint64(int64(time.Millisecond)/int64(time.Nanosecond)))

	label11 = "test_label11"
	value11 = "test_value11"
	label12 = "test_label12"
	value12 = "test_value12"
	label21 = "test_label21"
	value21 = "test_value21"
	label22 = "test_label22"
	value22 = "test_value22"

	intVal1      int64 = 1
	intVal2      int64 = 2
	floatValZero       = 0.0
	floatVal1          = 1.0
	floatVal2          = 2.0
	floatVal3          = 3.0

	lbs1    = getAttributes(label11, value11, label12, value12)
	lbs2    = getAttributes(label21, value21, label22, value22)
	bounds  = []float64{0.1, 0.5, 0.99}
	buckets = []uint64{1, 2, 3}

	quantileBounds = []float64{0.15, 0.9, 0.99}
	quantileValues = []float64{7, 8, 9}
	quantiles      = getQuantiles(quantileBounds, quantileValues)

	validIntGauge       = "valid_IntGauge"
	validDoubleGauge    = "valid_DoubleGauge"
	validIntSum         = "valid_IntSum"
	validSum            = "valid_Sum"
	validHistogram      = "valid_Histogram"
	validEmptyHistogram = "valid_empty_Histogram"
	validHistogramNoSum = "valid_Histogram_No_Sum"
	validSummary        = "valid_Summary"
	suffixedCounter     = "valid_IntSum_total"

	validIntGaugeDirty = "*valid_IntGauge$"

	unmatchedBoundBucketHist = "unmatchedBoundBucketHist"

	// valid metrics as input should not return error
	validMetrics1 = map[string]pmetric.Metric{
		validIntGauge:       getIntGaugeMetric(validIntGauge, lbs1, intVal1, time1),
		validDoubleGauge:    getDoubleGaugeMetric(validDoubleGauge, lbs1, floatVal1, time1),
		validIntSum:         getIntSumMetric(validIntSum, lbs1, intVal1, time1),
		suffixedCounter:     getIntSumMetric(suffixedCounter, lbs1, intVal1, time1),
		validSum:            getSumMetric(validSum, lbs1, floatVal1, time1),
		validHistogram:      getHistogramMetric(validHistogram, lbs1, time1, &floatVal1, uint64(intVal1), bounds, buckets),
		validHistogramNoSum: getHistogramMetric(validHistogramNoSum, lbs1, time1, nil, uint64(intVal1), bounds, buckets),
		validEmptyHistogram: getHistogramMetricEmptyDataPoint(validEmptyHistogram, lbs1, time1),
		validSummary:        getSummaryMetric(validSummary, lbs1, time1, floatVal1, uint64(intVal1), quantiles),
	}
	validMetrics2 = map[string]pmetric.Metric{
		validIntGauge:       getIntGaugeMetric(validIntGauge, lbs2, intVal2, time2),
		validDoubleGauge:    getDoubleGaugeMetric(validDoubleGauge, lbs2, floatVal2, time2),
		validIntSum:         getIntSumMetric(validIntSum, lbs2, intVal2, time2),
		validSum:            getSumMetric(validSum, lbs2, floatVal2, time2),
		validHistogram:      getHistogramMetric(validHistogram, lbs2, time2, &floatVal2, uint64(intVal2), bounds, buckets),
		validHistogramNoSum: getHistogramMetric(validHistogramNoSum, lbs2, time2, nil, uint64(intVal2), bounds, buckets),
		validEmptyHistogram: getHistogramMetricEmptyDataPoint(validEmptyHistogram, lbs2, time2),
		validSummary:        getSummaryMetric(validSummary, lbs2, time2, floatVal2, uint64(intVal2), quantiles),
		validIntGaugeDirty:  getIntGaugeMetric(validIntGaugeDirty, lbs1, intVal1, time1),
		unmatchedBoundBucketHist: getHistogramMetric(unmatchedBoundBucketHist, pcommon.NewMap(), 0, &floatValZero, 0,
			[]float64{0.1, 0.2, 0.3}, []uint64{1, 2}),
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
	invalidMetrics = map[string]pmetric.Metric{
		empty:                    pmetric.NewMetric(),
		emptyGauge:               getEmptyGaugeMetric(emptyGauge),
		emptySum:                 getEmptySumMetric(emptySum),
		emptyHistogram:           getEmptyHistogramMetric(emptyHistogram),
		emptySummary:             getEmptySummaryMetric(emptySummary),
		emptyCumulativeSum:       getEmptyCumulativeSumMetric(emptyCumulativeSum),
		emptyCumulativeHistogram: getEmptyCumulativeHistogramMetric(emptyCumulativeHistogram),
	}
	staleNaNIntGauge       = "staleNaNIntGauge"
	staleNaNDoubleGauge    = "staleNaNDoubleGauge"
	staleNaNIntSum         = "staleNaNIntSum"
	staleNaNSum            = "staleNaNSum"
	staleNaNHistogram      = "staleNaNHistogram"
	staleNaNEmptyHistogram = "staleNaNEmptyHistogram"
	staleNaNSummary        = "staleNaNSummary"

	// staleNaN metrics as input should have the staleness marker flag
	staleNaNMetrics = map[string]pmetric.Metric{
		staleNaNIntGauge:    getIntGaugeMetric(staleNaNIntGauge, lbs1, intVal1, time1),
		staleNaNDoubleGauge: getDoubleGaugeMetric(staleNaNDoubleGauge, lbs1, floatVal1, time1),
		staleNaNIntSum:      getIntSumMetric(staleNaNIntSum, lbs1, intVal1, time1),
		staleNaNSum:         getSumMetric(staleNaNSum, lbs1, floatVal1, time1),
		staleNaNHistogram:   getHistogramMetric(staleNaNHistogram, lbs1, time1, &floatVal2, uint64(intVal2), bounds, buckets),
		staleNaNEmptyHistogram: getHistogramMetric(staleNaNEmptyHistogram, lbs1, time1, &floatVal2, uint64(intVal2),
			[]float64{}, []uint64{}),
		staleNaNSummary: getSummaryMetric(staleNaNSummary, lbs2, time2, floatVal2, uint64(intVal2), quantiles),
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

func getSampleV2(v float64, t int64) writev2.Sample {
	return writev2.Sample{
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

func getTimeSeriesV2(labels []prompb.Label, samples ...writev2.Sample) (*writev2.TimeSeries, writev2.SymbolsTable) {
	symbolsTable := writev2.NewSymbolTable()
	buf := make([]uint32, 0, len(labels)*2)
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})

	var off uint32
	for _, label := range labels {
		off = symbolsTable.Symbolize(label.Name)
		buf = append(buf, off)
		off = symbolsTable.Symbolize(label.Value)
		buf = append(buf, off)
	}

	return &writev2.TimeSeries{
		LabelsRefs: buf,
		Samples:    samples,
	}, symbolsTable
}

func getMetricsFromMetricList(metricList ...pmetric.Metric) pmetric.Metrics {
	metrics := pmetric.NewMetrics()

	rm := metrics.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()
	ilm.Metrics().EnsureCapacity(len(metricList))
	for i := 0; i < len(metricList); i++ {
		metricList[i].CopyTo(ilm.Metrics().AppendEmpty())
	}

	return metrics
}

func getEmptyGaugeMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptyGauge()
	return metric
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

func getEmptySumMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptySum()
	return metric
}

func getIntSumMetric(name string, attributes pcommon.Map, value int64, ts uint64) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
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

func getEmptyCumulativeSumMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	return metric
}

func getSumMetric(name string, attributes pcommon.Map, value float64, ts uint64) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := metric.Sum().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetDoubleValue(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pcommon.Timestamp(0))
	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getEmptyHistogramMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptyHistogram()
	return metric
}

func getEmptyCumulativeHistogramMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	return metric
}

func getHistogramMetricEmptyDataPoint(name string, attributes pcommon.Map, ts uint64) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := metric.Histogram().DataPoints().AppendEmpty()
	attributes.CopyTo(dp.Attributes())
	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getExpHistogramMetric(
	name string,
	attributes pcommon.Map,
	ts uint64,
	sum *float64,
	count uint64,
	offset int32,
	bucketCounts []uint64,
) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := metric.ExponentialHistogram().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetCount(count)

	if sum != nil {
		dp.SetSum(*sum)
	}
	dp.Positive().SetOffset(offset)
	dp.Positive().BucketCounts().FromRaw(bucketCounts)
	attributes.CopyTo(dp.Attributes())

	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getHistogramMetric(name string, attributes pcommon.Map, ts uint64, sum *float64, count uint64, bounds []float64,
	buckets []uint64,
) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := metric.Histogram().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetCount(count)

	if sum != nil {
		dp.SetSum(*sum)
	}
	dp.BucketCounts().FromRaw(buckets)
	dp.ExplicitBounds().FromRaw(bounds)
	attributes.CopyTo(dp.Attributes())

	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getEmptySummaryMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetEmptySummary()
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
	for k, v := range attributes.All() {
		v.CopyTo(dp.Attributes().PutEmpty(k))
	}

	dp.SetTimestamp(pcommon.Timestamp(ts))

	quantiles.CopyTo(dp.QuantileValues())
	quantiles.At(0).Quantile()

	return metric
}

func getQuantiles(bounds []float64, values []float64) pmetric.SummaryDataPointValueAtQuantileSlice {
	quantiles := pmetric.NewSummaryDataPointValueAtQuantileSlice()
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

func getTimeseriesMapV2(timeseries []*writev2.TimeSeries) map[string]*writev2.TimeSeries {
	tsMap := make(map[string]*writev2.TimeSeries)
	for i, v := range timeseries {
		tsMap[fmt.Sprintf("%s%d", "timeseries_name", i)] = v
	}
	return tsMap
}
