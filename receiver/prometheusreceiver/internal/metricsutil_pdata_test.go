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

import "go.opentelemetry.io/collector/model/pdata"

type kv struct {
	Key, Value string
}

func distPointPdata(ts pdata.Timestamp, bounds []float64, counts []uint64) *pdata.HistogramDataPoint {
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

func cumulativeDistMetricPdata(name string, kvp []*kv, startTs pdata.Timestamp, points ...*pdata.HistogramDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	histogram := metric.Histogram()
	histogram.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)

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

func doublePointPdata(ts pdata.Timestamp, value float64) *pdata.NumberDataPoint {
	ndp := pdata.NewNumberDataPoint()
	ndp.SetTimestamp(ts)
	ndp.SetDoubleVal(value)

	return &ndp
}

func gaugeMetricPdata(name string, kvp []*kv, startTs pdata.Timestamp, points ...*pdata.NumberDataPoint) *pdata.Metric {
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

func summaryPointPdata(ts pdata.Timestamp, count uint64, sum float64, quantiles, values []float64) *pdata.SummaryDataPoint {
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

func summaryMetricPdata(name string, kvp []*kv, startTs pdata.Timestamp, points ...*pdata.SummaryDataPoint) *pdata.Metric {
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

func sumMetricPdata(name string, kvp []*kv, startTs pdata.Timestamp, points ...*pdata.NumberDataPoint) *pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSum)
	sum := metric.Sum()
	sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
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
