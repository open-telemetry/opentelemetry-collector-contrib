// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapointstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/starttimecache"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestTimeseriesMap_Get(t *testing.T) {
	tsm := newTimeseriesMap()
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetEmptyGauge()
	attrs := pcommon.NewMap()
	attrs.PutStr("k1", "v1")

	tsi, found := tsm.Get(metric, attrs)
	assert.NotNil(t, tsi)
	assert.False(t, found)
	assert.True(t, tsm.Mark)
	assert.True(t, tsi.Mark)

	tsi2, found2 := tsm.Get(metric, attrs)
	assert.Equal(t, tsi, tsi2)
	assert.True(t, found2)
	assert.True(t, tsm.Mark)
	assert.True(t, tsi2.Mark)

	metricHistogram := pmetric.NewMetric()
	metricHistogram.SetName("test_histogram")
	histogram := metricHistogram.SetEmptyHistogram()
	histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	tsi3, found3 := tsm.Get(metricHistogram, attrs)
	assert.NotNil(t, tsi3)
	assert.False(t, found3)
	assert.True(t, tsm.Mark)
	assert.True(t, tsi3.Mark)

	metricGaugeHistogram := pmetric.NewMetric()
	metricGaugeHistogram.SetName("test_gauge_histogram")
	gaugeHistogram := metricGaugeHistogram.SetEmptyHistogram()
	gaugeHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	tsi4, found4 := tsm.Get(metricGaugeHistogram, attrs)
	assert.NotNil(t, tsi4)
	assert.False(t, found4)
	assert.True(t, tsm.Mark)
	assert.True(t, tsi4.Mark)

	metricExponentialHistogram := pmetric.NewMetric()
	metricExponentialHistogram.SetName("test_exponential_histogram")
	exponentialHistogram := metricExponentialHistogram.SetEmptyExponentialHistogram()
	exponentialHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	tsi5, found5 := tsm.Get(metricExponentialHistogram, attrs)
	assert.NotNil(t, tsi5)
	assert.False(t, found5)
	assert.True(t, tsm.Mark)
	assert.True(t, tsi5.Mark)

	metricGaugeExponentialHistogram := pmetric.NewMetric()
	metricGaugeExponentialHistogram.SetName("test_gauge_exponential_histogram")
	gaugeExponentialHistogram := metricGaugeExponentialHistogram.SetEmptyExponentialHistogram()
	gaugeExponentialHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	tsi6, found6 := tsm.Get(metricGaugeExponentialHistogram, attrs)
	assert.NotNil(t, tsi6)
	assert.False(t, found6)
	assert.True(t, tsm.Mark)
	assert.True(t, tsi6.Mark)
}

func TestTimeseriesMap_GC(t *testing.T) {
	tsm := newTimeseriesMap()
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	metric.SetEmptyGauge()
	attrs := pcommon.NewMap()
	attrs.PutStr("k1", "v1")

	tsi, _ := tsm.Get(metric, attrs)
	assert.True(t, tsm.Mark)
	assert.True(t, tsi.Mark)

	tsm.GC()
	assert.False(t, tsm.Mark)
	assert.False(t, tsi.Mark)

	tsi2, _ := tsm.Get(metric, attrs)
	assert.True(t, tsi2.Mark)
	assert.True(t, tsm.Mark)

	tsm.GC()
	assert.False(t, tsm.Mark)
	assert.False(t, tsi2.Mark)

	tsm.GC()
	assert.False(t, tsm.Mark)
	assert.Empty(t, tsm.TsiMap)

	// Test GC when tsm.Mark is false
	tsm2 := newTimeseriesMap()
	tsm2.Mark = false
	tsm2.GC()
	assert.False(t, tsm2.Mark)
	assert.Empty(t, tsm2.TsiMap)
}

func TestNewTimeseriesMap(t *testing.T) {
	tsm := newTimeseriesMap()
	assert.NotNil(t, tsm)
	assert.True(t, tsm.Mark)
	assert.Empty(t, tsm.TsiMap)
}

func TestTimeseriesInfo_IsResetHistogram(t *testing.T) {
	tests := []struct {
		name          string
		setupTsi      func() *TimeseriesInfo
		setupH        func() pmetric.HistogramDataPoint
		expectedReset bool
	}{
		{
			name: "Count Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Histogram = HistogramInfo{PreviousCount: 10, PreviousSum: 50}
				return tsi
			},
			setupH: func() pmetric.HistogramDataPoint {
				h := pmetric.NewHistogramDataPoint()
				h.SetCount(5)
				h.SetSum(60)
				return h
			},
			expectedReset: true,
		},
		{
			name: "Sum Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Histogram = HistogramInfo{PreviousCount: 10, PreviousSum: 50}
				return tsi
			},
			setupH: func() pmetric.HistogramDataPoint {
				h := pmetric.NewHistogramDataPoint()
				h.SetCount(15)
				h.SetSum(40)
				return h
			},
			expectedReset: true,
		},
		{
			name: "Bounds Mismatch",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Histogram = HistogramInfo{PreviousCount: 10, PreviousSum: 50, ExplicitBounds: []float64{1, 2, 3}}
				return tsi
			},
			setupH: func() pmetric.HistogramDataPoint {
				h := pmetric.NewHistogramDataPoint()
				h.SetCount(15)
				h.SetSum(60)
				h.ExplicitBounds().FromRaw([]float64{1, 2, 4})
				return h
			},
			expectedReset: true,
		},
		{
			name: "Bucket Counts Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Histogram = HistogramInfo{PreviousCount: 10, PreviousSum: 50, ExplicitBounds: []float64{1, 2, 3}, PreviousBucketCounts: []uint64{1, 2, 3, 4}}
				return tsi
			},
			setupH: func() pmetric.HistogramDataPoint {
				h := pmetric.NewHistogramDataPoint()
				h.SetCount(15)
				h.SetSum(60)
				h.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				h.BucketCounts().FromRaw([]uint64{1, 1, 3, 4})
				return h
			},
			expectedReset: true,
		},
		{
			name: "No Reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Histogram = HistogramInfo{PreviousCount: 10, PreviousSum: 50, ExplicitBounds: []float64{1, 2, 3}, PreviousBucketCounts: []uint64{1, 2, 3, 4}}
				return tsi
			},
			setupH: func() pmetric.HistogramDataPoint {
				h := pmetric.NewHistogramDataPoint()
				h.SetCount(15)
				h.SetSum(60)
				h.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				h.BucketCounts().FromRaw([]uint64{2, 3, 4, 5})
				return h
			},
			expectedReset: false,
		},
		{
			name: "Bucket Counts Length Mismatch",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Histogram = HistogramInfo{PreviousCount: 10, PreviousSum: 50, ExplicitBounds: []float64{1, 2, 3}, PreviousBucketCounts: []uint64{1, 2, 3, 4}}
				return tsi
			},
			setupH: func() pmetric.HistogramDataPoint {
				h := pmetric.NewHistogramDataPoint()
				h.SetCount(15)
				h.SetSum(60)
				h.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				h.BucketCounts().FromRaw([]uint64{2, 3, 4})
				return h
			},
			expectedReset: true,
		},
		{
			name: "Zero Bucket Count",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Histogram = HistogramInfo{PreviousCount: 10, PreviousSum: 50, ExplicitBounds: []float64{1, 2, 3}, PreviousBucketCounts: []uint64{1, 2, 3, 4}}
				return tsi
			},
			setupH: func() pmetric.HistogramDataPoint {
				h := pmetric.NewHistogramDataPoint()
				h.SetCount(0)
				h.SetSum(60)
				h.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				h.BucketCounts().FromRaw([]uint64{1, 1, 3, 4})
				return h
			},
			expectedReset: true,
		},
		{
			name: "Zero Bucket Count but no change",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Histogram = HistogramInfo{ExplicitBounds: []float64{1, 2, 3}, PreviousBucketCounts: []uint64{0, 0, 0, 0}}
				return tsi
			},
			setupH: func() pmetric.HistogramDataPoint {
				h := pmetric.NewHistogramDataPoint()
				h.SetCount(0)
				h.SetSum(0)
				h.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				h.BucketCounts().FromRaw([]uint64{0, 0, 0, 0})
				return h
			},
			expectedReset: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsi := tt.setupTsi()
			h := tt.setupH()
			assert.Equal(t, tt.expectedReset, tsi.IsResetHistogram(h))
		})
	}
}

func TestTimeseriesInfo_IsResetExponentialHistogram(t *testing.T) {
	tests := []struct {
		name          string
		setupTsi      func() *TimeseriesInfo
		setupEh       func() pmetric.ExponentialHistogramDataPoint
		expectedReset bool
	}{
		{
			name: "Count Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(5)
				eh.SetSum(60)
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Sum Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(40)
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Zero Count Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, PreviousZeroCount: 10, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(40)
				eh.SetZeroCount(0)
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Positive Bucket Counts Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				tsi.ExponentialHistogram.PreviousPositive.BucketCounts = []uint64{1, 2, 3, 4}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(60)
				eh.Positive().BucketCounts().FromRaw([]uint64{1, 1, 3, 4})
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Negative Bucket Counts Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				tsi.ExponentialHistogram.PreviousNegative.BucketCounts = []uint64{1, 2, 3, 4}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(60)
				eh.Negative().BucketCounts().FromRaw([]uint64{1, 1, 3, 4})
				return eh
			},
			expectedReset: true,
		},
		{
			name: "No Reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				tsi.ExponentialHistogram.PreviousPositive.BucketCounts = []uint64{1, 2, 3, 4}
				tsi.ExponentialHistogram.PreviousNegative.BucketCounts = []uint64{1, 2, 3, 4}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(60)
				eh.Positive().BucketCounts().FromRaw([]uint64{2, 3, 4, 5})
				eh.Negative().BucketCounts().FromRaw([]uint64{2, 3, 4, 5})
				return eh
			},
			expectedReset: false,
		},
		{
			name: "Positive Bucket Counts Length Mismatch",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				tsi.ExponentialHistogram.PreviousPositive.BucketCounts = []uint64{1, 2, 3, 4}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(60)
				eh.Positive().BucketCounts().FromRaw([]uint64{2, 3, 4})
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Negative Bucket Counts Length Mismatch",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				tsi.ExponentialHistogram.PreviousNegative.BucketCounts = []uint64{1, 2, 3, 4}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(60)
				eh.Negative().BucketCounts().FromRaw([]uint64{2, 3, 4})
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Scale decrease (downscale) is not a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{
					PreviousCount:     10,
					PreviousSum:       50,
					PreviousZeroCount: 2,
					Scale:             2,
					PreviousPositive:  ExponentialHistogramBucketInfo{Offset: 0, BucketCounts: []uint64{1, 2, 3, 2}},
					PreviousNegative:  ExponentialHistogramBucketInfo{Offset: -2, BucketCounts: []uint64{1, 1}},
				}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(60)
				eh.SetZeroCount(2)
				eh.SetScale(1)
				eh.Positive().SetOffset(0)
				eh.Positive().BucketCounts().FromRaw([]uint64{3, 7, 1})
				eh.Negative().SetOffset(-1)
				eh.Negative().BucketCounts().FromRaw([]uint64{2, 2})
				return eh
			},
			expectedReset: false,
		},
		{
			name: "Scale decrease with negative offset spanning boundary is not a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{
					PreviousCount:    6,
					PreviousSum:      20,
					Scale:            1,
					PreviousPositive: ExponentialHistogramBucketInfo{},
					PreviousNegative: ExponentialHistogramBucketInfo{Offset: -3, BucketCounts: []uint64{2, 1, 3}},
				}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(8)
				eh.SetSum(25)
				eh.SetScale(0)
				eh.Negative().SetOffset(-2)
				eh.Negative().BucketCounts().FromRaw([]uint64{2, 5, 1})
				return eh
			},
			expectedReset: false,
		},
		{
			name: "Scale decrease with decreased bucket count is a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{
					PreviousCount:    10,
					PreviousSum:      50,
					Scale:            2,
					PreviousPositive: ExponentialHistogramBucketInfo{Offset: 0, BucketCounts: []uint64{2, 3, 5}},
					PreviousNegative: ExponentialHistogramBucketInfo{},
				}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(12)
				eh.SetSum(60)
				eh.SetScale(1)
				eh.Positive().SetOffset(0)
				eh.Positive().BucketCounts().FromRaw([]uint64{4, 8})
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Scale increase is a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, Scale: 1, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(60)
				eh.SetScale(2)
				return eh
			},
			expectedReset: true,
		},
		{
			name: "ZeroThreshold increase moving buckets into zero count is not a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				// At scale 0, bucket -1 covers (0.5, 1], bucket 0 covers (1, 2], bucket 1 covers (2, 4].
				// Increasing ZeroThreshold from 0 to 2.0 moves buckets -1 and 0 into ZeroCount.
				tsi.ExponentialHistogram = ExponentialHistogramInfo{
					PreviousCount:         10,
					PreviousSum:           50,
					PreviousZeroCount:     1,
					PreviousZeroThreshold: 0,
					Scale:                 0,
					PreviousPositive:      ExponentialHistogramBucketInfo{Offset: -1, BucketCounts: []uint64{2, 3, 4}},
					PreviousNegative:      ExponentialHistogramBucketInfo{},
				}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(12)
				eh.SetSum(55)
				eh.SetZeroCount(6)
				eh.SetZeroThreshold(2.0)
				eh.SetScale(0)
				eh.Positive().SetOffset(1)
				eh.Positive().BucketCounts().FromRaw([]uint64{6})
				return eh
			},
			expectedReset: false,
		},
		{
			name: "ZeroThreshold increase with ZeroCount below trimmed zero count is a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{
					PreviousCount:         10,
					PreviousSum:           50,
					PreviousZeroCount:     1,
					PreviousZeroThreshold: 0,
					Scale:                 0,
					PreviousPositive:      ExponentialHistogramBucketInfo{Offset: -1, BucketCounts: []uint64{2, 3, 4}},
					PreviousNegative:      ExponentialHistogramBucketInfo{},
				}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(12)
				eh.SetSum(55)
				eh.SetZeroCount(5) // 1 + 2 + 3 = 6 expected
				eh.SetZeroThreshold(2.0)
				eh.SetScale(0)
				eh.Positive().SetOffset(1)
				eh.Positive().BucketCounts().FromRaw([]uint64{7})
				return eh
			},
			expectedReset: true,
		},
		{
			name: "ZeroThreshold increase combined with scale decrease is not a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				// At scale 1: bucket -2 covers (0.5, sqrt(2)/2], -1 covers (sqrt(2)/2, 1], 0 covers (1, sqrt(2)],
				// 1 covers (sqrt(2), 2], 2 covers (2, 2*sqrt(2)], 3 covers (2*sqrt(2), 4].
				// Increasing ZeroThreshold to 2.0 trims buckets <= 1 (2 + 3 + 1 + 4 = 10) into ZeroCount,
				// leaving bucket 2 (count 1) -> downscaled to scale 0 bucket [1] with count 1.
				tsi.ExponentialHistogram = ExponentialHistogramInfo{
					PreviousCount:         12,
					PreviousSum:           50,
					PreviousZeroCount:     1,
					PreviousZeroThreshold: 0,
					Scale:                 1,
					PreviousPositive:      ExponentialHistogramBucketInfo{Offset: -2, BucketCounts: []uint64{2, 3, 1, 4, 1}},
					PreviousNegative:      ExponentialHistogramBucketInfo{},
				}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(60)
				eh.SetZeroCount(11)
				eh.SetZeroThreshold(2.0)
				eh.SetScale(0)
				eh.Positive().SetOffset(1)
				eh.Positive().BucketCounts().FromRaw([]uint64{4})
				return eh
			},
			expectedReset: false,
		},
		{
			name: "ZeroThreshold decrease is a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{
					PreviousCount:         10,
					PreviousSum:           50,
					PreviousZeroCount:     5,
					PreviousZeroThreshold: 1.0,
					Scale:                 0,
					PreviousPositive:      ExponentialHistogramBucketInfo{Offset: 1, BucketCounts: []uint64{5}},
					PreviousNegative:      ExponentialHistogramBucketInfo{},
				}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(12)
				eh.SetSum(55)
				eh.SetZeroCount(5)
				eh.SetZeroThreshold(0.5)
				eh.SetScale(0)
				eh.Positive().SetOffset(1)
				eh.Positive().BucketCounts().FromRaw([]uint64{7})
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Offset shift to lower offset at same scale is not a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{
					PreviousCount:    6,
					PreviousSum:      30,
					Scale:            1,
					PreviousPositive: ExponentialHistogramBucketInfo{Offset: 2, BucketCounts: []uint64{2, 4}},
					PreviousNegative: ExponentialHistogramBucketInfo{},
				}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(10)
				eh.SetSum(35)
				eh.SetScale(1)
				eh.Positive().SetOffset(0)
				eh.Positive().BucketCounts().FromRaw([]uint64{1, 1, 2, 5, 1})
				return eh
			},
			expectedReset: false,
		},
		{
			name: "Offset shift with dropped bucket count is a reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{
					PreviousCount:    6,
					PreviousSum:      30,
					Scale:            1,
					PreviousPositive: ExponentialHistogramBucketInfo{Offset: 2, BucketCounts: []uint64{2, 4}},
					PreviousNegative: ExponentialHistogramBucketInfo{},
				}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(10)
				eh.SetSum(35)
				eh.SetScale(1)
				eh.Positive().SetOffset(3) // Bucket 2 had count 2, now missing
				eh.Positive().BucketCounts().FromRaw([]uint64{5, 5})
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Reset on zero count",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousCount: 10, PreviousSum: 50, PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(0)
				eh.SetSum(60)
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Zero values but no reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = ExponentialHistogramInfo{PreviousPositive: ExponentialHistogramBucketInfo{}, PreviousNegative: ExponentialHistogramBucketInfo{}}
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(0)
				eh.SetSum(0)
				return eh
			},
			expectedReset: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsi := tt.setupTsi()
			eh := tt.setupEh()
			assert.Equal(t, tt.expectedReset, tsi.IsResetExponentialHistogram(eh))
		})
	}
}

func TestExponentialHistogramBucketInfo_Downscale(t *testing.T) {
	tests := []struct {
		name           string
		initial        ExponentialHistogramBucketInfo
		bitsLost       uint32
		expectedOffset int32
		expectedCounts []uint64
	}{
		{
			name:           "empty buckets",
			initial:        ExponentialHistogramBucketInfo{Offset: 5, BucketCounts: nil},
			bitsLost:       2,
			expectedOffset: 5,
			expectedCounts: nil,
		},
		{
			name:           "zero bits lost",
			initial:        ExponentialHistogramBucketInfo{Offset: 1, BucketCounts: []uint64{2, 3}},
			bitsLost:       0,
			expectedOffset: 1,
			expectedCounts: []uint64{2, 3},
		},
		{
			name:           "positive offset downscale by 1",
			initial:        ExponentialHistogramBucketInfo{Offset: 1, BucketCounts: []uint64{1, 2, 3, 4}},
			bitsLost:       1,
			expectedOffset: 0,
			expectedCounts: []uint64{1, 5, 4},
		},
		{
			name:           "negative offset downscale by 1",
			initial:        ExponentialHistogramBucketInfo{Offset: -3, BucketCounts: []uint64{2, 1, 3}},
			bitsLost:       1,
			expectedOffset: -2,
			expectedCounts: []uint64{2, 4},
		},
		{
			name:           "downscale by 2 bits",
			initial:        ExponentialHistogramBucketInfo{Offset: 2, BucketCounts: []uint64{1, 2, 3, 4, 5}},
			bitsLost:       2,
			expectedOffset: 0,
			expectedCounts: []uint64{3, 12},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.initial.Downscale(tt.bitsLost)
			assert.Equal(t, tt.expectedOffset, got.Offset)
			assert.Equal(t, tt.expectedCounts, got.BucketCounts)
		})
	}
}

func TestExponentialHistogramBucketInfo_TrimZeros(t *testing.T) {
	tests := []struct {
		name            string
		initial         ExponentialHistogramBucketInfo
		thresholdBucket int32
		expectedTrimmed uint64
		expectedOffset  int32
		expectedCounts  []uint64
	}{
		{
			name:            "threshold before all buckets",
			initial:         ExponentialHistogramBucketInfo{Offset: 2, BucketCounts: []uint64{3, 4}},
			thresholdBucket: 1,
			expectedTrimmed: 0,
			expectedOffset:  2,
			expectedCounts:  []uint64{3, 4},
		},
		{
			name:            "threshold trims prefix",
			initial:         ExponentialHistogramBucketInfo{Offset: -1, BucketCounts: []uint64{2, 3, 4, 5}},
			thresholdBucket: 0,
			expectedTrimmed: 5,
			expectedOffset:  1,
			expectedCounts:  []uint64{4, 5},
		},
		{
			name:            "threshold trims all buckets",
			initial:         ExponentialHistogramBucketInfo{Offset: -1, BucketCounts: []uint64{2, 3}},
			thresholdBucket: 5,
			expectedTrimmed: 5,
			expectedOffset:  0,
			expectedCounts:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.initial
			trimmed := b.TrimZeros(tt.thresholdBucket)
			assert.Equal(t, tt.expectedTrimmed, trimmed)
			assert.Equal(t, tt.expectedOffset, b.Offset)
			assert.Equal(t, tt.expectedCounts, b.BucketCounts)
		})
	}
}

func TestTimeseriesInfo_IsResetSummary(t *testing.T) {
	tests := []struct {
		name          string
		setupTsi      func() *TimeseriesInfo
		setupS        func() pmetric.SummaryDataPoint
		expectedReset bool
	}{
		{
			name: "Count Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Summary = SummaryInfo{PreviousCount: 10, PreviousSum: 50}
				return tsi
			},
			setupS: func() pmetric.SummaryDataPoint {
				s := pmetric.NewSummaryDataPoint()
				s.SetCount(5)
				s.SetSum(60)
				return s
			},
			expectedReset: true,
		},
		{
			name: "Sum Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Summary = SummaryInfo{PreviousCount: 10, PreviousSum: 50}
				return tsi
			},
			setupS: func() pmetric.SummaryDataPoint {
				s := pmetric.NewSummaryDataPoint()
				s.SetCount(15)
				s.SetSum(40)
				return s
			},
			expectedReset: true,
		},
		{
			name: "No Reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Summary = SummaryInfo{PreviousCount: 10, PreviousSum: 50}
				return tsi
			},
			setupS: func() pmetric.SummaryDataPoint {
				s := pmetric.NewSummaryDataPoint()
				s.SetCount(15)
				s.SetSum(60)
				return s
			},
			expectedReset: false,
		},
		{
			name: "Reset on zero count",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Summary = SummaryInfo{PreviousCount: 10, PreviousSum: 50}
				return tsi
			},
			setupS: func() pmetric.SummaryDataPoint {
				s := pmetric.NewSummaryDataPoint()
				s.SetCount(0)
				s.SetSum(60)
				return s
			},
			expectedReset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsi := tt.setupTsi()
			s := tt.setupS()
			assert.Equal(t, tt.expectedReset, tsi.IsResetSummary(s))
		})
	}
}

func TestTimeseriesInfo_IsResetSum(t *testing.T) {
	tests := []struct {
		name          string
		setupTsi      func() *TimeseriesInfo
		setupS        func() pmetric.NumberDataPoint
		expectedReset bool
	}{
		{
			name: "Double Value decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Number = NumberInfo{PreviousDoubleValue: 10}
				return tsi
			},
			setupS: func() pmetric.NumberDataPoint {
				s := pmetric.NewNumberDataPoint()
				s.SetDoubleValue(5)
				return s
			},
			expectedReset: true,
		},
		{
			name: "No Reset",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Number = NumberInfo{PreviousDoubleValue: 10}
				return tsi
			},
			setupS: func() pmetric.NumberDataPoint {
				s := pmetric.NewNumberDataPoint()
				s.SetDoubleValue(15)
				return s
			},
			expectedReset: false,
		},
		{
			name: "Double Value decreased - INT",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Number = NumberInfo{PreviousIntValue: 10}
				return tsi
			},
			setupS: func() pmetric.NumberDataPoint {
				s := pmetric.NewNumberDataPoint()
				s.SetIntValue(5)
				return s
			},
			expectedReset: true,
		},
		{
			name: "No Reset - INT",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.Number = NumberInfo{PreviousIntValue: 10}
				return tsi
			},
			setupS: func() pmetric.NumberDataPoint {
				s := pmetric.NewNumberDataPoint()
				s.SetIntValue(15)
				return s
			},
			expectedReset: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsi := tt.setupTsi()
			s := tt.setupS()
			assert.Equal(t, tt.expectedReset, tsi.IsResetSum(s))
		})
	}
}
