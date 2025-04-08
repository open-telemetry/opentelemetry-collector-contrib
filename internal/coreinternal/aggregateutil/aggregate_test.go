// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregateutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Test_CopyMetricDetails(t *testing.T) {
	gaugeFunc := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetDescription("desc")
		m.SetName("name")
		m.SetUnit("unit")
		m.SetEmptyGauge()
		return m
	}

	sumFunc := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetDescription("desc")
		m.SetName("name")
		m.SetUnit("unit")
		s := m.SetEmptySum()
		s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		s.SetIsMonotonic(true)
		return m
	}

	summaryFunc := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetDescription("desc")
		m.SetName("name")
		m.SetUnit("unit")
		m.SetEmptySummary()
		return m
	}

	histogramFunc := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetDescription("desc")
		m.SetName("name")
		m.SetUnit("unit")
		m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		return m
	}

	expHistogramFunc := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetDescription("desc")
		m.SetName("name")
		m.SetUnit("unit")
		m.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		return m
	}
	tests := []struct {
		name string
		from func() pmetric.Metric
		to   func() pmetric.Metric
	}{
		{
			name: "gauge",
			from: gaugeFunc,
			to:   gaugeFunc,
		},
		{
			name: "summary",
			from: summaryFunc,
			to:   summaryFunc,
		},
		{
			name: "sum",
			from: sumFunc,
			to:   sumFunc,
		},
		{
			name: "histogram",
			from: histogramFunc,
			to:   histogramFunc,
		},
		{
			name: " exp histogram",
			from: expHistogramFunc,
			to:   expHistogramFunc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pmetric.NewMetric()
			from := tt.from()
			to := tt.to()
			CopyMetricDetails(from, result)
			require.Equal(t, to, result)
		})
	}
}

func Test_FilterAttributes(t *testing.T) {
	tests := []struct {
		name string
		attr []string
		want func() pmetric.Metric
	}{
		{
			name: "nil",
			attr: nil,
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySum()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.Attributes().PutStr("attr2", "val2")
				return m
			},
		},
		{
			name: "empty",
			attr: []string{},
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySum()
				s.DataPoints().AppendEmpty()
				return m
			},
		},
		{
			name: "valid",
			attr: []string{"attr1"},
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySum()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				return m
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			s := m.SetEmptySum()
			d := s.DataPoints().AppendEmpty()
			d.Attributes().PutStr("attr1", "val1")
			d.Attributes().PutStr("attr2", "val2")

			FilterAttrs(m, tt.attr)
			require.Equal(t, tt.want(), m)
		})
	}
}

func Test_RangeDataPointAttributes(t *testing.T) {
	fun := func(attrs pcommon.Map) bool {
		attrs.RemoveIf(func(k string, _ pcommon.Value) bool {
			return isNotPresent(k, []string{"attr1"})
		})
		return true
	}

	tests := []struct {
		name string
		in   func() pmetric.Metric
		want func() pmetric.Metric
	}{
		{
			name: "sum",
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySum()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.Attributes().PutStr("attr2", "val2")
				return m
			},
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySum()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				return m
			},
		},
		{
			name: "gauge",
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyGauge()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.Attributes().PutStr("attr2", "val2")
				return m
			},
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyGauge()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				return m
			},
		},
		{
			name: "summary",
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySummary()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.Attributes().PutStr("attr2", "val2")
				return m
			},
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySummary()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				return m
			},
		},
		{
			name: "histogram",
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyHistogram()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.Attributes().PutStr("attr2", "val2")
				return m
			},
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyHistogram()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				return m
			},
		},
		{
			name: "exp histogram",
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyExponentialHistogram()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.Attributes().PutStr("attr2", "val2")
				return m
			},
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyExponentialHistogram()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				return m
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.in()
			RangeDataPointAttributes(m, fun)
			require.Equal(t, tt.want(), m)
		})
	}
}

func Test_GroupDataPoints(t *testing.T) {
	mapAttr := pcommon.NewMap()
	mapAttr.PutStr("attr1", "val1")
	hash := dataPointHashKey(mapAttr, pcommon.NewTimestampFromTime(time.Time{}))

	hashHistogram := dataPointHashKey(mapAttr, pcommon.NewTimestampFromTime(time.Time{}), false, false, 0)

	hashExpHistogram := dataPointHashKey(mapAttr, pcommon.NewTimestampFromTime(time.Time{}), 0, false, false, 0, 0, 0)

	tests := []struct {
		name     string
		in       func() pmetric.Metric
		aggGroup AggGroups
		want     AggGroups
	}{
		{
			name: "sum",
			aggGroup: AggGroups{
				sum: map[string]pmetric.NumberDataPointSlice{
					hash: testDataNumber(),
				},
			},
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySum()
				s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				d := s.DataPoints().AppendEmpty()
				d.SetTimestamp(pcommon.NewTimestampFromTime(time.Time{}))
				d.Attributes().PutStr("attr1", "val1")
				d.SetIntValue(5)
				return m
			},
			want: AggGroups{
				sum: map[string]pmetric.NumberDataPointSlice{
					hash: testDataNumberDouble(),
				},
			},
		},
		{
			name: "gauge",
			aggGroup: AggGroups{
				gauge: map[string]pmetric.NumberDataPointSlice{
					hash: testDataNumber(),
				},
			},
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyGauge()
				d := s.DataPoints().AppendEmpty()
				d.SetTimestamp(pcommon.NewTimestampFromTime(time.Time{}))
				d.Attributes().PutStr("attr1", "val1")
				d.SetIntValue(5)
				return m
			},
			want: AggGroups{
				gauge: map[string]pmetric.NumberDataPointSlice{
					hash: testDataNumberDouble(),
				},
			},
		},
		{
			name: "histogram",
			aggGroup: AggGroups{
				histogram: map[string]pmetric.HistogramDataPointSlice{
					hashHistogram: testDataHistogram(),
				},
			},
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyHistogram()
				s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				d := s.DataPoints().AppendEmpty()
				d.SetTimestamp(pcommon.NewTimestampFromTime(time.Time{}))
				d.Attributes().PutStr("attr1", "val1")
				d.SetCount(1)
				return m
			},
			want: AggGroups{
				histogram: map[string]pmetric.HistogramDataPointSlice{
					hashHistogram: testDataHistogramDouble(),
				},
			},
		},
		{
			name: "exp histogram",
			aggGroup: AggGroups{
				expHistogram: map[string]pmetric.ExponentialHistogramDataPointSlice{
					hashExpHistogram: testDataExpHistogram(),
				},
			},
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyExponentialHistogram()
				s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				d := s.DataPoints().AppendEmpty()
				d.SetTimestamp(pcommon.NewTimestampFromTime(time.Time{}))
				d.Attributes().PutStr("attr1", "val1")
				d.SetCount(9)
				d.SetZeroCount(2)
				d.Positive().BucketCounts().Append(0, 1, 2, 3)
				d.Negative().BucketCounts().Append(0, 1)
				return m
			},
			want: AggGroups{
				expHistogram: map[string]pmetric.ExponentialHistogramDataPointSlice{
					hashExpHistogram: testDataExpHistogramDouble(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.aggGroup
			GroupDataPoints(tt.in(), &a)
			require.Equal(t, tt.want, a)
		})
	}
}

func Test_MergeDataPoints(t *testing.T) {
	mapAttr := pcommon.NewMap()
	mapAttr.PutStr("attr1", "val1")

	hash := dataPointHashKey(mapAttr, pcommon.NewTimestampFromTime(time.Time{}))

	hashHistogram := dataPointHashKey(mapAttr, pcommon.NewTimestampFromTime(time.Time{}), false, false, 0)

	hashExpHistogram := dataPointHashKey(mapAttr, pcommon.NewTimestampFromTime(time.Time{}), 0, false, false, 0, 0, 0)

	tests := []struct {
		name     string
		typ      AggregationType
		aggGroup AggGroups
		want     func() pmetric.Metric
		in       func() pmetric.Metric
	}{
		{
			name: "sum",
			aggGroup: AggGroups{
				sum: map[string]pmetric.NumberDataPointSlice{
					hash: testDataNumberDouble(),
				},
			},
			typ: Sum,
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySum()
				s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.SetIntValue(6)
				return m
			},
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySum()
				s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				return m
			},
		},
		{
			name: "gauge",
			aggGroup: AggGroups{
				gauge: map[string]pmetric.NumberDataPointSlice{
					hash: testDataNumberDouble(),
				},
			},
			typ: Sum,
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyGauge()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.SetIntValue(6)
				return m
			},
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptyGauge()
				return m
			},
		},
		{
			name: "histogram",
			aggGroup: AggGroups{
				histogram: map[string]pmetric.HistogramDataPointSlice{
					hashHistogram: testDataHistogramDouble(),
				},
			},
			typ: Sum,
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyHistogram()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.SetCount(3)
				d.SetSum(0)
				return m
			},
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptyHistogram()
				return m
			},
		},
		{
			name: "exp histogram",
			aggGroup: AggGroups{
				expHistogram: map[string]pmetric.ExponentialHistogramDataPointSlice{
					hashExpHistogram: testDataExpHistogramDouble(),
				},
			},
			typ: Sum,
			want: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyExponentialHistogram()
				d := s.DataPoints().AppendEmpty()
				d.Attributes().PutStr("attr1", "val1")
				d.SetCount(16)
				d.SetSum(0)
				d.SetZeroCount(3)
				d.Positive().BucketCounts().Append(0, 2, 4, 3)
				d.Negative().BucketCounts().Append(0, 2, 2)
				return m
			},
			in: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetEmptyExponentialHistogram()
				return m
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.in()
			MergeDataPoints(m, tt.typ, tt.aggGroup)
			require.Equal(t, tt.want(), m)
		})
	}
}

func testDataNumber() pmetric.NumberDataPointSlice {
	data := pmetric.NewNumberDataPointSlice()
	d := data.AppendEmpty()
	d.Attributes().PutStr("attr1", "val1")
	d.SetIntValue(1)
	return data
}

func testDataNumberDouble() pmetric.NumberDataPointSlice {
	dataWant := pmetric.NewNumberDataPointSlice()
	dWant := dataWant.AppendEmpty()
	dWant.Attributes().PutStr("attr1", "val1")
	dWant.SetIntValue(1)
	dWant2 := dataWant.AppendEmpty()
	dWant2.SetTimestamp(pcommon.NewTimestampFromTime(time.Time{}))
	dWant2.Attributes().PutStr("attr1", "val1")
	dWant2.SetIntValue(5)
	return dataWant
}

func testDataHistogram() pmetric.HistogramDataPointSlice {
	data := pmetric.NewHistogramDataPointSlice()
	d := data.AppendEmpty()
	d.Attributes().PutStr("attr1", "val1")
	d.SetCount(2)
	return data
}

func testDataHistogramDouble() pmetric.HistogramDataPointSlice {
	dataWant := pmetric.NewHistogramDataPointSlice()
	dWant := dataWant.AppendEmpty()
	dWant.Attributes().PutStr("attr1", "val1")
	dWant.SetCount(2)
	dWant2 := dataWant.AppendEmpty()
	dWant2.SetTimestamp(pcommon.NewTimestampFromTime(time.Time{}))
	dWant2.Attributes().PutStr("attr1", "val1")
	dWant2.SetCount(1)
	return dataWant
}

func testDataExpHistogram() pmetric.ExponentialHistogramDataPointSlice {
	data := pmetric.NewExponentialHistogramDataPointSlice()
	d := data.AppendEmpty()
	d.Attributes().PutStr("attr1", "val1")
	d.SetCount(7)
	d.SetZeroCount(1)
	d.Positive().BucketCounts().Append(0, 1, 2)
	d.Negative().BucketCounts().Append(0, 1, 2)
	return data
}

func testDataExpHistogramDouble() pmetric.ExponentialHistogramDataPointSlice {
	dataWant := pmetric.NewExponentialHistogramDataPointSlice()

	dWant := dataWant.AppendEmpty()
	dWant.Attributes().PutStr("attr1", "val1")
	dWant.SetCount(7)
	dWant.SetZeroCount(1)
	dWant.Positive().BucketCounts().Append(0, 1, 2)
	dWant.Negative().BucketCounts().Append(0, 1, 2)

	dWant2 := dataWant.AppendEmpty()
	dWant2.SetTimestamp(pcommon.NewTimestampFromTime(time.Time{}))
	dWant2.Attributes().PutStr("attr1", "val1")
	dWant2.SetCount(9)
	dWant2.SetZeroCount(2)
	// Use a larger number of buckets than above to check that we expand the
	// destination array as needed while merging.
	dWant2.Positive().BucketCounts().Append(0, 1, 2, 3)
	// Use a smaller number of buckets than above to check that we merge values
	// into the correct, existing buckets.
	dWant2.Negative().BucketCounts().Append(0, 1)

	return dataWant
}
