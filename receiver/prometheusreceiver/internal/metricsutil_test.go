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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type kv struct {
	Key, Value string
}

func metricSlice(metrics ...pmetric.Metric) pmetric.MetricSlice {
	ms := pmetric.NewMetricSlice()
	for _, metric := range metrics {
		destMetric := ms.AppendEmpty()
		metric.CopyTo(destMetric)
	}

	return ms
}

func histogramPointRaw(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.HistogramDataPoint {
	hdp := pmetric.NewHistogramDataPoint()
	hdp.SetStartTimestamp(startTimestamp)
	hdp.SetTimestamp(timestamp)

	attrs := hdp.Attributes()
	for _, kv := range attributes {
		attrs.InsertString(kv.Key, kv.Value)
	}

	return hdp
}

func histogramPoint(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp, bounds []float64, counts []uint64) pmetric.HistogramDataPoint {
	hdp := histogramPointRaw(attributes, startTimestamp, timestamp)
	hdp.SetExplicitBounds(pcommon.NewImmutableFloat64Slice(bounds))
	hdp.SetBucketCounts(pcommon.NewImmutableUInt64Slice(counts))

	var sum float64
	var count uint64
	for i, bcount := range counts {
		count += bcount
		if i > 0 {
			sum += float64(bcount) * bounds[i-1]
		}
	}
	hdp.SetCount(count)
	hdp.SetSum(sum)

	return hdp
}

func histogramPointNoValue(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.HistogramDataPoint {
	hdp := histogramPointRaw(attributes, startTimestamp, timestamp)
	hdp.Flags().SetNoRecordedValue(true)

	return hdp
}

func histogramMetric(name string, points ...pmetric.HistogramDataPoint) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeHistogram)
	histogram := metric.Histogram()
	histogram.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)

	destPointL := histogram.DataPoints()
	// By default the AggregationTemporality is Cumulative until it'll be changed by the caller.
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
	}

	return metric
}

func doublePointRaw(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.NumberDataPoint {
	ndp := pmetric.NewNumberDataPoint()
	ndp.SetStartTimestamp(startTimestamp)
	ndp.SetTimestamp(timestamp)

	for _, kv := range attributes {
		ndp.Attributes().InsertString(kv.Key, kv.Value)
	}

	return ndp
}

func doublePoint(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp, value float64) pmetric.NumberDataPoint {
	ndp := doublePointRaw(attributes, startTimestamp, timestamp)
	ndp.SetDoubleVal(value)
	return ndp
}

func doublePointNoValue(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.NumberDataPoint {
	ndp := doublePointRaw(attributes, startTimestamp, timestamp)
	ndp.Flags().SetNoRecordedValue(true)
	return ndp
}

func gaugeMetric(name string, points ...pmetric.NumberDataPoint) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeGauge)

	destPointL := metric.Gauge().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
	}

	return metric
}

func sumMetric(name string, points ...pmetric.NumberDataPoint) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeSum)
	sum := metric.Sum()
	sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	sum.SetIsMonotonic(true)

	destPointL := sum.DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
	}

	return metric
}

func summaryPointRaw(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.SummaryDataPoint {
	sdp := pmetric.NewSummaryDataPoint()
	sdp.SetStartTimestamp(startTimestamp)
	sdp.SetTimestamp(timestamp)

	for _, kv := range attributes {
		sdp.Attributes().InsertString(kv.Key, kv.Value)
	}

	return sdp
}

func summaryPoint(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp, count uint64, sum float64, quantiles, values []float64) pmetric.SummaryDataPoint {
	sdp := summaryPointRaw(attributes, startTimestamp, timestamp)
	sdp.SetCount(count)
	sdp.SetSum(sum)

	qvL := sdp.QuantileValues()
	for i := 0; i < len(quantiles); i++ {
		qvi := qvL.AppendEmpty()
		qvi.SetQuantile(quantiles[i])
		qvi.SetValue(values[i])
	}

	return sdp
}

func summaryPointNoValue(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.SummaryDataPoint {
	sdp := summaryPointRaw(attributes, startTimestamp, timestamp)
	sdp.Flags().SetNoRecordedValue(true)

	return sdp
}

func summaryMetric(name string, points ...pmetric.SummaryDataPoint) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeSummary)

	destPointL := metric.Summary().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
	}

	return metric
}
