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

func distPoint(ts pcommon.Timestamp, bounds []float64, counts []uint64) *pmetric.HistogramDataPoint {
	hdp := pmetric.NewHistogramDataPoint()
	hdp.SetExplicitBounds(pcommon.NewImmutableFloat64Slice(bounds))
	hdp.SetBucketCounts(pcommon.NewImmutableUInt64Slice(counts))
	hdp.SetTimestamp(ts)
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

	return &hdp
}

func cumulativeDistMetric(name string, kvp []*kv, startTs pcommon.Timestamp, points ...*pmetric.HistogramDataPoint) *pmetric.Metric {
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
		point.SetStartTimestamp(startTs)
		attrs := destPoint.Attributes()
		for _, kv := range kvp {
			attrs.InsertString(kv.Key, kv.Value)
		}
	}
	return &metric
}

func doublePoint(ts pcommon.Timestamp, value float64) *pmetric.NumberDataPoint {
	ndp := pmetric.NewNumberDataPoint()
	ndp.SetTimestamp(ts)
	ndp.SetDoubleVal(value)

	return &ndp
}

func gaugeMetric(name string, kvp []*kv, startTs pcommon.Timestamp, points ...*pmetric.NumberDataPoint) *pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeGauge)

	destPointL := metric.Gauge().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		point.SetStartTimestamp(startTs)
		attrs := destPoint.Attributes()
		for _, kv := range kvp {
			attrs.InsertString(kv.Key, kv.Value)
		}
	}
	return &metric
}

func summaryPoint(ts pcommon.Timestamp, count uint64, sum float64, quantiles, values []float64) *pmetric.SummaryDataPoint {
	sdp := pmetric.NewSummaryDataPoint()
	sdp.SetTimestamp(ts)
	sdp.SetCount(count)
	sdp.SetSum(sum)
	qvL := sdp.QuantileValues()
	for i := 0; i < len(quantiles); i++ {
		qvi := qvL.AppendEmpty()
		qvi.SetQuantile(quantiles[i])
		qvi.SetValue(values[i])
	}
	return &sdp
}

func summaryMetric(name string, kvp []*kv, startTs pcommon.Timestamp, points ...*pmetric.SummaryDataPoint) *pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pmetric.MetricDataTypeSummary)

	destPointL := metric.Summary().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		point.SetStartTimestamp(startTs)
		attrs := destPoint.Attributes()
		for _, kv := range kvp {
			attrs.InsertString(kv.Key, kv.Value)
		}
	}
	return &metric
}

func sumMetric(name string, kvp []*kv, startTs pcommon.Timestamp, points ...*pmetric.NumberDataPoint) *pmetric.Metric {
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
		point.SetStartTimestamp(startTs)
		attrs := destPoint.Attributes()
		for _, kv := range kvp {
			attrs.InsertString(kv.Key, kv.Value)
		}
	}
	return &metric
}
