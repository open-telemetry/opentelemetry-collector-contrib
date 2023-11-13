// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/protocol"

import (
	"math"
	"sort"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gonum.org/v1/gonum/stat"
)

var (
	statsDDefaultPercentiles = []float64{0, 10, 50, 90, 95, 100}

	explicitHistogramMaxSize                              = 30 // covers values from 2^-14..2^14, with -Inf and +Inf boundaries
	explicitHistogramBoundaries, explicitHistogramBuckets = getExplicitHistogramBoundaries()
)

func getExplicitHistogramBoundaries() ([]float64, []uint64) {
	boundaries := []float64{}
	buckets := []uint64{}
	// add buckets for (-Inf...N-1)
	for i := 0; i < (explicitHistogramMaxSize - 1); i++ {
		exponent := i - (explicitHistogramMaxSize / 2) + 1
		boundaries = append(boundaries, math.Pow(2, float64(exponent)*math.Pow(2.0, 0)))
		buckets = append(buckets, 0)
	}
	// add final bucket for (N-1, +Inf) to catch any datapoints that
	// fall outside the static bucketing
	buckets = append(buckets, 0)
	return boundaries, buckets
}

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

	if parsedMetric.timestamp != 0 {
		dp.SetTimestamp(pcommon.Timestamp(parsedMetric.timestamp))
	}

	return ilm
}

func setTimestampsForCounterMetric(ilm pmetric.ScopeMetrics, startTime, timeNow time.Time) {
	dp := ilm.Metrics().At(0).Sum().DataPoints().At(0)
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))

	if dp.Timestamp() == 0 {
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
	}
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

func buildExponentialHistogramMetric(desc statsDMetricDescription, histogram histogramMetric, startTime, timeNow time.Time, ilm pmetric.ScopeMetrics) {
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

func buildHistogramMetric(desc statsDMetricDescription, histogram explicitHistogramMetric, startTime, timeNow time.Time, ilm pmetric.ScopeMetrics) {
	nm := ilm.Metrics().AppendEmpty()
	nm.SetName(desc.name)
	hist := nm.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dp := hist.DataPoints().AppendEmpty()
	dp.ExplicitBounds().FromRaw(explicitHistogramBoundaries)
	dp.BucketCounts().FromRaw(explicitHistogramBuckets)

	sum := 0.0
	min := math.MaxFloat64
	max := -math.MaxFloat64
	for _, dpt := range histogram.points {
		sum += dpt
		min = math.Min(min, dpt)
		max = math.Max(max, dpt)

		// This search will return an index in the range [0, len(boundaries)], where
		// it will return len(boundaries) if value is greater than the last element
		// of boundaries. This aligns with the buckets in that the length of buckets
		// is len(boundaries)+1, with the last bucket representing:
		// (boundaries[len(boundaries)-1], +∞).
		idx := sort.SearchFloat64s(explicitHistogramBoundaries, float64(dpt))

		dp.BucketCounts().SetAt(idx, dp.BucketCounts().At(idx)+1)
	}

	dp.SetCount(uint64(len(histogram.points)))
	dp.SetSum(sum)
	if dp.Count() != 0 {
		dp.SetMin(min)
		dp.SetMax(max)
	}

	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))

	for i := desc.attrs.Iter(); i.Next(); {
		dp.Attributes().PutStr(string(i.Attribute().Key), i.Attribute().Value.AsString())
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
