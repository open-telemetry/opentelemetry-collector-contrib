// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testhelper // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/testhelper"

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

func TimestampFromMs(timeAtMs int64) pcommon.Timestamp {
	return pcommon.Timestamp(timeAtMs * 1e6)
}

type KV struct {
	Key, Value string
}

func Metrics(metrics ...pmetric.Metric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	for _, metric := range metrics {
		destMetric := ms.AppendEmpty()
		metric.CopyTo(destMetric)
	}

	return md
}

func MetricsFromResourceMetrics(metrics ...pmetric.ResourceMetrics) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for _, metric := range metrics {
		mr := md.ResourceMetrics().AppendEmpty()
		metric.CopyTo(mr)
	}
	return md
}

func ResourceMetrics(job, instance string, metrics ...pmetric.Metric) pmetric.ResourceMetrics {
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

func HistogramPointRaw(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp) pmetric.HistogramDataPoint {
	hdp := pmetric.NewHistogramDataPoint()
	hdp.SetStartTimestamp(startTimestamp)
	hdp.SetTimestamp(timestamp)

	attrs := hdp.Attributes()
	for _, kv := range attributes {
		attrs.PutStr(kv.Key, kv.Value)
	}

	return hdp
}

func HistogramPoint(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp, bounds []float64, counts []uint64) pmetric.HistogramDataPoint {
	hdp := HistogramPointRaw(attributes, startTimestamp, timestamp)
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

func HistogramPointNoValue(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp) pmetric.HistogramDataPoint {
	hdp := HistogramPointRaw(attributes, startTimestamp, timestamp)
	hdp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))

	return hdp
}

func HistogramMetric(name string, points ...pmetric.HistogramDataPoint) pmetric.Metric {
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

func ExponentialHistogramMetric(name string, points ...pmetric.ExponentialHistogramDataPoint) pmetric.Metric {
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

func ExponentialHistogramPointRaw(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp) pmetric.ExponentialHistogramDataPoint {
	hdp := pmetric.NewExponentialHistogramDataPoint()
	hdp.SetStartTimestamp(startTimestamp)
	hdp.SetTimestamp(timestamp)

	attrs := hdp.Attributes()
	for _, kv := range attributes {
		attrs.PutStr(kv.Key, kv.Value)
	}

	return hdp
}

func ExponentialHistogramPoint(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp, scale int32, zeroCount uint64, negativeOffset int32, negativeBuckets []uint64, positiveOffset int32, positiveBuckets []uint64) pmetric.ExponentialHistogramDataPoint {
	hdp := ExponentialHistogramPointRaw(attributes, startTimestamp, timestamp)
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

func ExponentialHistogramPointNoValue(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp) pmetric.ExponentialHistogramDataPoint {
	hdp := ExponentialHistogramPointRaw(attributes, startTimestamp, timestamp)
	hdp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))

	return hdp
}

func DoublePointRaw(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp) pmetric.NumberDataPoint {
	ndp := pmetric.NewNumberDataPoint()
	ndp.SetStartTimestamp(startTimestamp)
	ndp.SetTimestamp(timestamp)

	for _, kv := range attributes {
		ndp.Attributes().PutStr(kv.Key, kv.Value)
	}

	return ndp
}

func DoublePoint(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp, value float64) pmetric.NumberDataPoint {
	ndp := DoublePointRaw(attributes, startTimestamp, timestamp)
	ndp.SetDoubleValue(value)
	return ndp
}

func DoublePointNoValue(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp) pmetric.NumberDataPoint {
	ndp := DoublePointRaw(attributes, startTimestamp, timestamp)
	ndp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	return ndp
}

func GaugeMetric(name string, points ...pmetric.NumberDataPoint) pmetric.Metric {
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

func SumMetric(name string, points ...pmetric.NumberDataPoint) pmetric.Metric {
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

func SummaryPointRaw(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp) pmetric.SummaryDataPoint {
	sdp := pmetric.NewSummaryDataPoint()
	sdp.SetStartTimestamp(startTimestamp)
	sdp.SetTimestamp(timestamp)

	for _, kv := range attributes {
		sdp.Attributes().PutStr(kv.Key, kv.Value)
	}

	return sdp
}

func SummaryPoint(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp, count uint64, sum float64, quantiles, values []float64) pmetric.SummaryDataPoint {
	sdp := SummaryPointRaw(attributes, startTimestamp, timestamp)
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

func SummaryPointNoValue(attributes []*KV, startTimestamp, timestamp pcommon.Timestamp) pmetric.SummaryDataPoint {
	sdp := SummaryPointRaw(attributes, startTimestamp, timestamp)
	sdp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))

	return sdp
}

func SummaryMetric(name string, points ...pmetric.SummaryDataPoint) pmetric.Metric {
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

type Adjuster interface {
	AdjustMetrics(context.Context, pmetric.Metrics) (pmetric.Metrics, error)
}

type MetricsAdjusterTest struct {
	Description string
	Metrics     pmetric.Metrics
	Adjusted    pmetric.Metrics
}

func RunScript(t *testing.T, ma Adjuster, tests []*MetricsAdjusterTest, additionalResourceAttrs ...string) {
	for _, test := range tests {
		t.Run(test.Description, func(t *testing.T) {
			adjusted := pmetric.NewMetrics()
			test.Metrics.CopyTo(adjusted)
			// Add the instance/job to the input metrics if they aren't already present.
			for i := 0; i < adjusted.ResourceMetrics().Len(); i++ {
				rm := adjusted.ResourceMetrics().At(i)
				for i, attr := range additionalResourceAttrs {
					rm.Resource().Attributes().PutStr(fmt.Sprintf("%d", i), attr)
				}
			}
			var err error
			adjusted, err = ma.AdjustMetrics(context.Background(), adjusted)
			assert.NoError(t, err)

			// Add the instance/job to the expected metrics as well if they aren't already present.
			for i := 0; i < test.Adjusted.ResourceMetrics().Len(); i++ {
				rm := test.Adjusted.ResourceMetrics().At(i)
				for i, attr := range additionalResourceAttrs {
					rm.Resource().Attributes().PutStr(fmt.Sprintf("%d", i), attr)
				}
			}
			assert.Equal(t, test.Adjusted, adjusted)
		})
	}
}
