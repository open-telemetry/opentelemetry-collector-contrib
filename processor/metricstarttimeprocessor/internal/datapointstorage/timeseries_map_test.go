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
