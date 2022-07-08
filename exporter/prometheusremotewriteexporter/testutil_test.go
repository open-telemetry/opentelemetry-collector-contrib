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

package prometheusremotewriteexporter

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/prometheus/prompb"
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

	intVal1   int64 = 1
	intVal2   int64 = 2
	floatVal1       = 1.0
	floatVal2       = 2.0
	floatVal3       = 3.0

	lbs1    = getAttributes(label11, value11, label12, value12)
	lbs2    = getAttributes(label21, value21, label22, value22)
	bounds  = pcommon.NewImmutableFloat64Slice([]float64{0.1, 0.5, 0.99})
	buckets = pcommon.NewImmutableUInt64Slice([]uint64{1, 2, 3})

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
	validMetrics1 = map[string]pmetric.Metric{
		validIntGauge:    getIntGaugeMetric(validIntGauge, lbs1, intVal1, time1),
		validDoubleGauge: getDoubleGaugeMetric(validDoubleGauge, lbs1, floatVal1, time1),
		validIntSum:      getIntSumMetric(validIntSum, lbs1, intVal1, time1),
		suffixedCounter:  getIntSumMetric(suffixedCounter, lbs1, intVal1, time1),
		validSum:         getSumMetric(validSum, lbs1, floatVal1, time1),
		validHistogram:   getHistogramMetric(validHistogram, lbs1, time1, floatVal1, uint64(intVal1), bounds, buckets),
		validSummary:     getSummaryMetric(validSummary, lbs1, time1, floatVal1, uint64(intVal1), quantiles),
	}
	validMetrics2 = map[string]pmetric.Metric{
		validIntGauge:      getIntGaugeMetric(validIntGauge, lbs2, intVal2, time2),
		validDoubleGauge:   getDoubleGaugeMetric(validDoubleGauge, lbs2, floatVal2, time2),
		validIntSum:        getIntSumMetric(validIntSum, lbs2, intVal2, time2),
		validSum:           getSumMetric(validSum, lbs2, floatVal2, time2),
		validHistogram:     getHistogramMetric(validHistogram, lbs2, time2, floatVal2, uint64(intVal2), bounds, buckets),
		validSummary:       getSummaryMetric(validSummary, lbs2, time2, floatVal2, uint64(intVal2), quantiles),
		validIntGaugeDirty: getIntGaugeMetric(validIntGaugeDirty, lbs1, intVal1, time1),
		unmatchedBoundBucketHist: getHistogramMetric(unmatchedBoundBucketHist, pcommon.NewMap(), 0, 0, 0,
			pcommon.NewImmutableFloat64Slice([]float64{0.1, 0.2, 0.3}),
			pcommon.NewImmutableUInt64Slice([]uint64{1, 2})),
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
		staleNaNHistogram:   getHistogramMetric(staleNaNHistogram, lbs1, time1, floatVal2, uint64(intVal2), bounds, buckets),
		staleNaNEmptyHistogram: getHistogramMetric(staleNaNEmptyHistogram, lbs1, time1, floatVal2, uint64(intVal2),
			pcommon.NewImmutableFloat64Slice([]float64{}),
			pcommon.NewImmutableUInt64Slice([]uint64{})),
		staleNaNSummary: getSummaryMetric(staleNaNSummary, lbs2, time2, floatVal2, uint64(intVal2), quantiles),
	}
)

// OTLP metrics
// attributes must come in pairs
func getAttributes(labels ...string) pcommon.Map {
	attributeMap := pcommon.NewMap()
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
	metric.SetDataType(pmetric.MetricDataTypeGauge)
	return metric
}

func getIntGaugeMetric(name string, attributes pcommon.Map, value int64, ts uint64) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeGauge)
	dp := metric.Gauge().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetIntVal(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pcommon.Timestamp(0))
	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getDoubleGaugeMetric(name string, attributes pcommon.Map, value float64, ts uint64) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeGauge)
	dp := metric.Gauge().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetDoubleVal(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pcommon.Timestamp(0))
	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getEmptySumMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeSum)
	return metric
}

func getIntSumMetric(name string, attributes pcommon.Map, value int64, ts uint64) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeSum)
	metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	dp := metric.Sum().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetIntVal(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pcommon.Timestamp(0))
	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getEmptyCumulativeSumMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeSum)
	metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	return metric
}

func getSumMetric(name string, attributes pcommon.Map, value float64, ts uint64) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeSum)
	metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	dp := metric.Sum().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetDoubleVal(value)
	attributes.CopyTo(dp.Attributes())

	dp.SetStartTimestamp(pcommon.Timestamp(0))
	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getEmptyHistogramMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeHistogram)
	return metric
}

func getEmptyCumulativeHistogramMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeHistogram)
	metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	return metric
}

func getHistogramMetric(name string, attributes pcommon.Map, ts uint64, sum float64, count uint64, bounds pcommon.ImmutableFloat64Slice,
	buckets pcommon.ImmutableUInt64Slice) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeHistogram)
	metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	dp := metric.Histogram().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetCount(count)
	dp.SetSum(sum)
	dp.SetBucketCounts(buckets)
	dp.SetExplicitBounds(bounds)
	attributes.CopyTo(dp.Attributes())

	dp.SetTimestamp(pcommon.Timestamp(ts))
	return metric
}

func getEmptySummaryMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeSummary)
	return metric
}

func getSummaryMetric(name string, attributes pcommon.Map, ts uint64, sum float64, count uint64, quantiles pmetric.ValueAtQuantileSlice) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeSummary)
	dp := metric.Summary().DataPoints().AppendEmpty()
	if strings.HasPrefix(name, "staleNaN") {
		dp.SetFlags(1)
	}
	dp.SetCount(count)
	dp.SetSum(sum)
	attributes.Range(func(k string, v pcommon.Value) bool {
		dp.Attributes().Upsert(k, v)
		return true
	})

	dp.SetTimestamp(pcommon.Timestamp(ts))

	quantiles.CopyTo(dp.QuantileValues())
	quantiles.At(0).Quantile()

	return metric
}

func getQuantiles(bounds []float64, values []float64) pmetric.ValueAtQuantileSlice {
	quantiles := pmetric.NewValueAtQuantileSlice()
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
