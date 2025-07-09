// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
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

func metricsFromResourceMetrics(metrics ...pmetric.ResourceMetrics) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for _, metric := range metrics {
		mr := md.ResourceMetrics().AppendEmpty()
		metric.CopyTo(mr)
	}
	return md
}

func resourceMetrics(job, instance string, metrics ...pmetric.Metric) pmetric.ResourceMetrics {
	mr := pmetric.NewResourceMetrics()
	mr.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), job)
	mr.Resource().Attributes().PutStr(string(semconv.ServiceInstanceIDKey), instance)
	ms := mr.ScopeMetrics().AppendEmpty().Metrics()

	for _, metric := range metrics {
		destMetric := ms.AppendEmpty()
		metric.CopyTo(destMetric)
	}
	return mr
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
	metric.Metadata().PutStr("prometheus.type", "histogram")
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

func exponentialHistogramMetric(name string, points ...pmetric.ExponentialHistogramDataPoint) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.Metadata().PutStr("prometheus.type", "histogram")
	histogram := metric.SetEmptyExponentialHistogram()
	histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	destPointL := histogram.DataPoints()
	// By default the AggregationTemporality is Cumulative until it'll be changed by the caller.
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
	}

	return metric
}

func exponentialHistogramPointRaw(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.ExponentialHistogramDataPoint {
	hdp := pmetric.NewExponentialHistogramDataPoint()
	hdp.SetStartTimestamp(startTimestamp)
	hdp.SetTimestamp(timestamp)

	attrs := hdp.Attributes()
	for _, kv := range attributes {
		attrs.PutStr(kv.Key, kv.Value)
	}

	return hdp
}

func exponentialHistogramPoint(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp, scale int32, zeroCount uint64, negativeOffset int32, negativeBuckets []uint64, positiveOffset int32, positiveBuckets []uint64) pmetric.ExponentialHistogramDataPoint {
	hdp := exponentialHistogramPointRaw(attributes, startTimestamp, timestamp)
	hdp.SetScale(scale)
	hdp.SetZeroCount(zeroCount)
	hdp.Negative().SetOffset(negativeOffset)
	hdp.Negative().BucketCounts().FromRaw(negativeBuckets)
	hdp.Positive().SetOffset(positiveOffset)
	hdp.Positive().BucketCounts().FromRaw(positiveBuckets)

	count := uint64(0)
	sum := float64(0)
	for i, bCount := range positiveBuckets {
		count += bCount
		sum += float64(bCount) * float64(i)
	}
	for i, bCount := range negativeBuckets {
		count += bCount
		sum -= float64(bCount) * float64(i)
	}
	hdp.SetCount(count)
	hdp.SetSum(sum)
	return hdp
}

func exponentialHistogramPointNoValue(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp) pmetric.ExponentialHistogramDataPoint {
	hdp := exponentialHistogramPointRaw(attributes, startTimestamp, timestamp)
	hdp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))

	return hdp
}

// exponentialHistogramPointSimplified let's you define an exponential
// histogram with just a few parameters.
// Scale and ZeroCount are set to the provided values.
// Positive and negative buckets are generated using the offset and bucketCount
// parameters by adding buckets from offset in both positive and negative
// directions. Bucket counts start from 1 and increase by 1 for each bucket.
// Sum and Count will be proportional to the bucket count.
func exponentialHistogramPointSimplified(attributes []*kv, startTimestamp, timestamp pcommon.Timestamp, scale int32, zeroCount uint64, offset int32, bucketCount int) pmetric.ExponentialHistogramDataPoint {
	hdp := exponentialHistogramPointRaw(attributes, startTimestamp, timestamp)
	hdp.SetScale(scale)
	hdp.SetZeroCount(zeroCount)

	positive := hdp.Positive()
	positive.SetOffset(offset)
	positive.BucketCounts().EnsureCapacity(bucketCount)
	negative := hdp.Negative()
	negative.SetOffset(offset)
	negative.BucketCounts().EnsureCapacity(bucketCount)

	var sum float64
	var count uint64
	for i := 0; i < bucketCount; i++ {
		positive.BucketCounts().Append(uint64(i + 1))
		negative.BucketCounts().Append(uint64(i + 1))
		count += uint64(i+1) + uint64(i+1)
		sum += float64(i+1)*10 + float64(i+1)*10.0
	}
	hdp.SetCount(count)
	hdp.SetSum(sum)

	return hdp
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
	metric.Metadata().PutStr("prometheus.type", "gauge")
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
	metric.Metadata().PutStr("prometheus.type", "counter")
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
	metric.Metadata().PutStr("prometheus.type", "summary")
	destPointL := metric.SetEmptySummary().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
	}

	return metric
}
