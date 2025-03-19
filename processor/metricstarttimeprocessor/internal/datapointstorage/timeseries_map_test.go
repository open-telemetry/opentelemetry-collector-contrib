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

func TestGetAttributesSignature(t *testing.T) {
	m1 := pcommon.NewMap()
	m1.PutStr("k1", "v1")
	m1.PutStr("k2", "v2")
	m1.PutStr("k3", "")

	m2 := pcommon.NewMap()
	m2.PutStr("k2", "v2")
	m2.PutStr("k1", "v1")
	m2.PutStr("k4", "")

	m3 := pcommon.NewMap()
	m3.PutStr("k1", "v1")
	m3.PutStr("k2", "v2")

	m4 := pcommon.NewMap()
	m4.PutStr("k1", "v1")
	m4.PutStr("k2", "v2")
	m4.PutStr("k3", "v3")

	sig1 := getAttributesSignature(m1)
	sig2 := getAttributesSignature(m2)
	sig3 := getAttributesSignature(m3)
	sig4 := getAttributesSignature(m4)

	assert.Equal(t, sig1, sig2)
	assert.Equal(t, sig1, sig3)
	assert.NotEqual(t, sig1, sig4)
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
				tsi.Histogram = pmetric.NewHistogramDataPoint()
				tsi.Histogram.SetCount(10)
				tsi.Histogram.SetSum(50)
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
				tsi.Histogram = pmetric.NewHistogramDataPoint()
				tsi.Histogram.SetCount(10)
				tsi.Histogram.SetSum(50)
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
				tsi.Histogram = pmetric.NewHistogramDataPoint()
				tsi.Histogram.SetCount(10)
				tsi.Histogram.SetSum(50)
				tsi.Histogram.ExplicitBounds().FromRaw([]float64{1, 2, 3})
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
				tsi.Histogram = pmetric.NewHistogramDataPoint()
				tsi.Histogram.SetCount(10)
				tsi.Histogram.SetSum(50)
				tsi.Histogram.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				tsi.Histogram.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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
				tsi.Histogram = pmetric.NewHistogramDataPoint()
				tsi.Histogram.SetCount(10)
				tsi.Histogram.SetSum(50)
				tsi.Histogram.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				tsi.Histogram.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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
				tsi.Histogram = pmetric.NewHistogramDataPoint()
				tsi.Histogram.SetCount(10)
				tsi.Histogram.SetSum(50)
				tsi.Histogram.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				tsi.Histogram.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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
				tsi.Histogram = pmetric.NewHistogramDataPoint()
				tsi.Histogram.SetCount(10)
				tsi.Histogram.SetSum(50)
				tsi.Histogram.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				tsi.Histogram.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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
				tsi.Histogram = pmetric.NewHistogramDataPoint()
				tsi.Histogram.SetCount(0)
				tsi.Histogram.SetSum(0)
				tsi.Histogram.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				tsi.Histogram.BucketCounts().FromRaw([]uint64{0, 0, 0, 0})
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
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(10)
				tsi.ExponentialHistogram.SetSum(50)
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
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(10)
				tsi.ExponentialHistogram.SetSum(50)
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
			name: "Positive Bucket Counts Decreased",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(10)
				tsi.ExponentialHistogram.SetSum(50)
				tsi.ExponentialHistogram.Positive().BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(10)
				tsi.ExponentialHistogram.SetSum(50)
				tsi.ExponentialHistogram.Negative().BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(10)
				tsi.ExponentialHistogram.SetSum(50)
				tsi.ExponentialHistogram.Positive().BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
				tsi.ExponentialHistogram.Negative().BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(10)
				tsi.ExponentialHistogram.SetSum(50)
				tsi.ExponentialHistogram.Positive().BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(10)
				tsi.ExponentialHistogram.SetSum(50)
				tsi.ExponentialHistogram.Negative().BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
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
			name: "Scale mismatch",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(10)
				tsi.ExponentialHistogram.SetSum(50)
				tsi.ExponentialHistogram.SetScale(2)
				return tsi
			},
			setupEh: func() pmetric.ExponentialHistogramDataPoint {
				eh := pmetric.NewExponentialHistogramDataPoint()
				eh.SetCount(15)
				eh.SetSum(60)
				eh.SetScale(1)
				return eh
			},
			expectedReset: true,
		},
		{
			name: "Reset on zero count",
			setupTsi: func() *TimeseriesInfo {
				tsi := &TimeseriesInfo{}
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(10)
				tsi.ExponentialHistogram.SetSum(50)
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
				tsi.ExponentialHistogram = pmetric.NewExponentialHistogramDataPoint()
				tsi.ExponentialHistogram.SetCount(0)
				tsi.ExponentialHistogram.SetSum(0)
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
				tsi.Summary = pmetric.NewSummaryDataPoint()
				tsi.Summary.SetCount(10)
				tsi.Summary.SetSum(50)
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
				tsi.Summary = pmetric.NewSummaryDataPoint()
				tsi.Summary.SetCount(10)
				tsi.Summary.SetSum(50)
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
				tsi.Summary = pmetric.NewSummaryDataPoint()
				tsi.Summary.SetCount(10)
				tsi.Summary.SetSum(50)
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
				tsi.Summary = pmetric.NewSummaryDataPoint()
				tsi.Summary.SetCount(10)
				tsi.Summary.SetSum(50)
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
				tsi.Number = pmetric.NewNumberDataPoint()
				tsi.Number.SetDoubleValue(10)
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
				tsi.Number = pmetric.NewNumberDataPoint()
				tsi.Number.SetDoubleValue(10)
				return tsi
			},
			setupS: func() pmetric.NumberDataPoint {
				s := pmetric.NewNumberDataPoint()
				s.SetDoubleValue(15)
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

func BenchmarkGetAttributesSignature(b *testing.B) {
	attrs := pcommon.NewMap()
	attrs.PutStr("key1", "some-random-test-value-1")
	attrs.PutStr("key2", "some-random-test-value-2")
	attrs.PutStr("key6", "some-random-test-value-6")
	attrs.PutStr("key3", "some-random-test-value-3")
	attrs.PutStr("key4", "some-random-test-value-4")
	attrs.PutStr("key5", "some-random-test-value-5")
	attrs.PutStr("key7", "some-random-test-value-7")
	attrs.PutStr("key8", "some-random-test-value-8")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		getAttributesSignature(attrs)
	}
}
