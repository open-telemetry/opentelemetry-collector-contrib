// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"

import (
	"sort"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gonum.org/v1/gonum/stat"
)

var (
	statsDDefaultPercentiles = []float64{0, 10, 50, 90, 95, 100}
)

func buildCounterMetric(parsedMetric statsDMetric, isMonotonicCounter bool) pmetric.ScopeMetrics {
	ilm := pmetric.NewScopeMetrics()
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(parsedMetric.description.name)
	if parsedMetric.unit != "" {
		nm.SetUnit(parsedMetric.unit)
	}

	nm.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	nm.Sum().SetIsMonotonic(isMonotonicCounter)

	dp := nm.Sum().DataPoints().AppendEmpty()
	dp.SetIntValue(parsedMetric.counterValue())
	for i := parsedMetric.description.attrs.Iter(); i.Next(); {
		dp.Attributes().PutStr(string(i.Attribute().Key), i.Attribute().Value.AsString())
	}

	return ilm
}

func setTimestampsForCounterMetric(ilm pmetric.ScopeMetrics, startTime, timeNow time.Time) {
	dp := ilm.Metrics().At(0).Sum().DataPoints().At(0)
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
}

func buildGaugeMetric(parsedMetric statsDMetric, timeNow time.Time) pmetric.ScopeMetrics {
	ilm := pmetric.NewScopeMetrics()
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(parsedMetric.description.name)
	if parsedMetric.unit != "" {
		nm.SetUnit(parsedMetric.unit)
	}
	dp := nm.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(parsedMetric.gaugeValue())
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
	for i := parsedMetric.description.attrs.Iter(); i.Next(); {
		dp.Attributes().PutStr(string(i.Attribute().Key), i.Attribute().Value.AsString())
	}

	return ilm
}

func buildSummaryMetric(desc statsDMetricDescription, summary summaryMetric, startTime, timeNow time.Time, percentiles []float64, ilm pmetric.ScopeMetrics) {
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(desc.name)
	dp := nm.SetEmptySummary().DataPoints().AppendEmpty()

	count := float64(0)
	sum := float64(0)
	for i := range summary.points {
		c := summary.weights[i]
		count += c
		sum += summary.points[i] * c
	}

	// Note: count is rounded here, see note in counterValue().
	dp.SetCount(uint64(count))
	dp.SetSum(sum)

	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
	for i := desc.attrs.Iter(); i.Next(); {
		dp.Attributes().PutStr(string(i.Attribute().Key), i.Attribute().Value.AsString())
	}

	sort.Sort(dualSorter{summary.points, summary.weights})

	for _, pct := range percentiles {
		eachQuantile := dp.QuantileValues().AppendEmpty()
		eachQuantile.SetQuantile(pct / 100)
		eachQuantile.SetValue(stat.Quantile(pct/100, stat.Empirical, summary.points, summary.weights))
	}
}

func buildHistogramMetric(desc statsDMetricDescription, histogram histogramMetric, startTime, timeNow time.Time, ilm pmetric.ScopeMetrics) {
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(desc.name)
	expo := nm.SetEmptyExponentialHistogram()
	expo.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dp := expo.DataPoints().AppendEmpty()
	agg := histogram.agg

	dp.SetCount(agg.Count())
	dp.SetSum(agg.Sum())
	if agg.Count() != 0 {
		dp.SetMin(agg.Min())
		dp.SetMax(agg.Max())
	}

	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))

	for i := desc.attrs.Iter(); i.Next(); {
		dp.Attributes().PutStr(string(i.Attribute().Key), i.Attribute().Value.AsString())
	}

	dp.SetZeroCount(agg.ZeroCount())
	dp.SetScale(agg.Scale())

	for _, half := range []struct {
		inFunc  func() *structure.Buckets
		outFunc func() pmetric.ExponentialHistogramDataPointBuckets
	}{
		{agg.Positive, dp.Positive},
		{agg.Negative, dp.Negative},
	} {
		in := half.inFunc()
		out := half.outFunc()
		out.SetOffset(in.Offset())

		out.BucketCounts().EnsureCapacity(int(in.Len()))

		for i := uint32(0); i < in.Len(); i++ {
			out.BucketCounts().Append(in.At(i))
		}
	}
}

func (s statsDMetric) counterValue() int64 {
	x := s.asFloat
	// Note statds counters are always represented as integers.
	// There is no statsd specification that says what should or
	// shouldn't be done here.  Rounding may occur for sample
	// rates that are not integer reciprocals.  Recommendation:
	// use integer reciprocal sampling rates.
	if 0 < s.sampleRate && s.sampleRate < 1 {
		x /= s.sampleRate
	}
	return int64(x)
}

func (s statsDMetric) gaugeValue() float64 {
	// sampleRate does not have effect for gauge points.
	return s.asFloat
}

func (s statsDMetric) sampleValue() sampleValue {
	count := 1.0
	if 0 < s.sampleRate && s.sampleRate < 1 {
		count /= s.sampleRate
	}
	return sampleValue{
		value: s.asFloat,
		count: count,
	}
}

type dualSorter struct {
	values, weights []float64
}

func (d dualSorter) Len() int {
	return len(d.values)
}

func (d dualSorter) Swap(i, j int) {
	d.values[i], d.values[j] = d.values[j], d.values[i]
	d.weights[i], d.weights[j] = d.weights[j], d.weights[i]
}

func (d dualSorter) Less(i, j int) bool {
	return d.values[i] < d.values[j]
}
