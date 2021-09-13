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

package metricstestutil

import "go.opentelemetry.io/collector/model/pdata"

type KV struct {
	Key, Value string
}

func DistPointPdata(ts pdata.Timestamp, bounds []float64, counts []uint64) *pdata.HistogramDataPoint {
	hdp := pdata.NewHistogramDataPoint()
	hdp.SetExplicitBounds(bounds)
	hdp.SetBucketCounts(counts)
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

func GaugeDistMetricPdata(name string, kvp []*KV, startTs pdata.Timestamp, points ...*pdata.HistogramDataPoint) *pdata.Metric {
	hMetric := CumulativeDistMetricPdata(name, kvp, startTs, points...)
	hMetric.Histogram().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	return hMetric
}

func CumulativeDistMetricPdata(name string, kvp []*KV, startTs pdata.Timestamp, points ...*pdata.HistogramDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	histogram := metric.Histogram()
	histogram.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)

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

func DoublePointPdata(ts pdata.Timestamp, value float64) *pdata.NumberDataPoint {
	ndp := pdata.NewNumberDataPoint()
	ndp.SetTimestamp(ts)
	ndp.SetDoubleVal(value)

	return &ndp
}

func GaugeMetricPdata(name string, kvp []*KV, startTs pdata.Timestamp, points ...*pdata.NumberDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeGauge)

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

func SummaryPointPdata(ts pdata.Timestamp, count uint64, sum float64, quantiles, values []float64) *pdata.SummaryDataPoint {
	sdp := pdata.NewSummaryDataPoint()
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

func SummaryMetricPdata(name string, kvp []*KV, startTs pdata.Timestamp, points ...*pdata.SummaryDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSummary)

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

func SumMetricPdata(name string, kvp []*KV, startTs pdata.Timestamp, points ...*pdata.NumberDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSum)

	destPointL := metric.Sum().DataPoints()
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
