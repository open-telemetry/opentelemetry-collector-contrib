// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type kv struct {
	Key, Value string
}

func metrics(metrics ...pmetric.Metric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	for _, metric := range metrics {
		destMetric := ms.AppendEmpty()
		metric.CopyTo(destMetric)
	}

	return md
}

func histogramPointRaw(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.HistogramDataPoint {
	hdp := pmetric.NewHistogramDataPoint()
	hdp.SetStartTimestamp(startTimestamp)
	hdp.SetTimestamp(timestamp)

	attrs := hdp.Attributes()
	for _, kv := range attributes {
		attrs.PutStr(kv.Key, kv.Value)
	}

	return hdp
}

func histogramPoint(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp, bounds []float64, counts []uint64) pmetric.HistogramDataPoint {
	hdp := histogramPointRaw(attributes, startTimestamp, timestamp)
	hdp.ExplicitBounds().FromRaw(bounds)
	hdp.BucketCounts().FromRaw(counts)

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
	hdp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))

	return hdp
}

func histogramMetric(name string, points ...pmetric.HistogramDataPoint) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	histogram := metric.SetEmptyHistogram()
	histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

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
		ndp.Attributes().PutStr(kv.Key, kv.Value)
	}

	return ndp
}

func doublePoint(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp, value float64) pmetric.NumberDataPoint {
	ndp := doublePointRaw(attributes, startTimestamp, timestamp)
	ndp.SetDoubleValue(value)
	return ndp
}

func doublePointNoValue(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.NumberDataPoint {
	ndp := doublePointRaw(attributes, startTimestamp, timestamp)
	ndp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	return ndp
}

func gaugeMetric(name string, points ...pmetric.NumberDataPoint) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	destPointL := metric.SetEmptyGauge().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
	}

	return metric
}

func sumMetric(name string, points ...pmetric.NumberDataPoint) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	sum := metric.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
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
		sdp.Attributes().PutStr(kv.Key, kv.Value)
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
	sdp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))

	return sdp
}

func summaryMetric(name string, points ...pmetric.SummaryDataPoint) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	destPointL := metric.SetEmptySummary().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
	}

	return metric
}
