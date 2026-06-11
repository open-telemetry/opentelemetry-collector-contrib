// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"math"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// newTestExplicitHistogram builds a single cumulative explicit-bucket histogram
// metric with bounds [1,2,3] and per-bucket counts [1,2,3,4] (cumulative
// 1,3,6,10), Count=10, Sum=42.5.
func newTestExplicitHistogram(ts pcommon.Timestamp) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("test_hist")
	metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	pt := metric.Histogram().DataPoints().AppendEmpty()
	pt.SetTimestamp(ts)
	pt.ExplicitBounds().FromRaw([]float64{1, 2, 3})
	pt.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
	pt.SetCount(10)
	pt.SetSum(42.5)
	return metric
}

func runHistogram(t *testing.T, metric pmetric.Metric, settings Settings) *prometheusConverter {
	t.Helper()
	converter := newPrometheusConverter(settings)
	err := converter.addHistogramDataPoints(
		metric.Histogram().DataPoints(),
		pcommon.NewResource(),
		pcommon.NewInstrumentationScope(),
		settings,
		metric.Name(),
	)
	require.NoError(t, err)
	return converter
}

// findSeriesTS returns the time series whose __name__ label equals name, or nil.
func findSeriesTS(c *prometheusConverter, name string) *prompb.TimeSeries {
	for _, ts := range c.unique {
		for _, l := range ts.Labels {
			if l.Name == model.MetricNameLabel && l.Value == name {
				return ts
			}
		}
	}
	return nil
}

func TestExplicitToNHCBHistogram(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	pt := newTestExplicitHistogram(ts).Histogram().DataPoints().At(0)

	h, err := explicitToNHCBHistogram(pt)
	require.NoError(t, err)

	assert.Equal(t, histogram.CustomBucketsSchema, h.Schema, "must be NHCB schema -53")
	assert.Equal(t, []float64{1, 2, 3}, h.CustomValues, "explicit bounds carried as custom values")
	assert.Equal(t, uint64(10), h.GetCountInt(), "count preserved")
	assert.InDelta(t, 42.5, h.Sum, 1e-9, "sum preserved")
	assert.Equal(t, convertTimeStamp(ts), h.Timestamp)
}

// TestExplicitToNHCBHistogram_BucketCountsRoundTrip decodes the produced wire
// histogram back and asserts every cumulative bucket count and upper bound
// matches the original OTLP histogram. This is the core correctness property:
// the bucket payload — not just count/sum — must survive conversion.
func TestExplicitToNHCBHistogram_BucketCountsRoundTrip(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	pt := newTestExplicitHistogram(ts).Histogram().DataPoints().At(0)

	h, err := explicitToNHCBHistogram(pt)
	require.NoError(t, err)

	ih := h.ToIntHistogram()
	require.NotNil(t, ih, "OTLP integer bucket counts must yield an integer histogram")

	type bucket struct {
		upper float64
		cum   uint64
	}
	var got []bucket
	for it := ih.CumulativeBucketIterator(); it.Next(); {
		b := it.At()
		got = append(got, bucket{b.Upper, b.Count})
	}
	// bounds [1,2,3,+Inf], cumulative counts [1,3,6,10].
	want := []bucket{{1, 1}, {2, 3}, {3, 6}, {math.Inf(1), 10}}
	assert.Equal(t, want, got)
}

func TestExplicitToNHCBHistogram_NoSum(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	metric := pmetric.NewMetric()
	metric.SetName("test_hist")
	metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	pt := metric.Histogram().DataPoints().AppendEmpty()
	pt.SetTimestamp(ts)
	pt.ExplicitBounds().FromRaw([]float64{1, 2, 3})
	pt.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
	pt.SetCount(10)
	// deliberately no SetSum
	require.False(t, pt.HasSum())

	h, err := explicitToNHCBHistogram(pt)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), h.GetCountInt())
	assert.Equal(t, 0.0, h.Sum, "missing sum defaults to 0")
	assert.Equal(t, []float64{1, 2, 3}, h.CustomValues)
}

func TestExplicitToNHCBHistogram_NoBounds(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	metric := pmetric.NewMetric()
	metric.SetName("test_hist")
	metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	pt := metric.Histogram().DataPoints().AppendEmpty()
	pt.SetTimestamp(ts)
	// No explicit bounds: a single (+Inf) bucket carrying the whole count.
	pt.BucketCounts().FromRaw([]uint64{5})
	pt.SetCount(5)
	pt.SetSum(12.5)

	h, err := explicitToNHCBHistogram(pt)
	require.NoError(t, err)
	assert.Equal(t, histogram.CustomBucketsSchema, h.Schema)
	assert.Equal(t, uint64(5), h.GetCountInt())
	assert.Empty(t, h.CustomValues, "single +Inf bucket carries no finite bounds")
}

func TestExplicitToNHCBHistogram_StaleMarker(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	metric := newTestExplicitHistogram(ts)
	pt := metric.Histogram().DataPoints().At(0)
	pt.SetFlags(pt.Flags().WithNoRecordedValue(true))

	h, err := explicitToNHCBHistogram(pt)
	require.NoError(t, err)
	assert.Equal(t, histogram.CustomBucketsSchema, h.Schema)
	assert.Equal(t, value.StaleNaN, h.GetCountInt(), "stale marker count")
	assert.True(t, math.IsNaN(h.Sum), "stale marker sum")
}

func TestAddHistogramDataPoints_NHCBOnly(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	metric := newTestExplicitHistogram(ts)
	c := runHistogram(t, metric, Settings{ConvertHistogramsToNHCB: true})

	// Exactly one series, named test_hist, carrying a native histogram.
	require.Len(t, c.unique, 1, "NHCB-only should emit a single native series")
	for _, series := range c.unique {
		require.Len(t, series.Histograms, 1)
		assert.Empty(t, series.Samples, "no classic samples in NHCB-only mode")
		assert.Equal(t, histogram.CustomBucketsSchema, series.Histograms[0].Schema)
	}
	require.NotNil(t, findSeriesTS(c, "test_hist"), "native series uses the base metric name")
}

func TestAddHistogramDataPoints_KeepClassic(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	metric := newTestExplicitHistogram(ts)
	c := runHistogram(t, metric, Settings{ConvertHistogramsToNHCB: true, KeepClassicHistograms: true})

	var nativeSeries, classicSamples int
	for _, series := range c.unique {
		nativeSeries += len(series.Histograms)
		classicSamples += len(series.Samples)
	}
	assert.Equal(t, 1, nativeSeries, "one NHCB datapoint emitted alongside classic")
	// classic emits _sum, _count, and one _bucket series per bound (incl. +Inf).
	assert.Positive(t, classicSamples, "classic _bucket/_sum/_count still emitted")

	// The native series (bare name) and a classic _bucket series must coexist.
	require.NotNil(t, findSeriesTS(c, "test_hist"), "native series present under base name")
	require.NotNil(t, findSeriesTS(c, "test_hist_bucket"), "classic _bucket series present")
	require.NotNil(t, findSeriesTS(c, "test_hist_count"), "classic _count series present")
}

func TestAddHistogramDataPoints_NHCBExemplars(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	metric := newTestExplicitHistogram(ts)
	ex := metric.Histogram().DataPoints().At(0).Exemplars().AppendEmpty()
	ex.SetTimestamp(ts)
	ex.SetDoubleValue(7)

	c := runHistogram(t, metric, Settings{ConvertHistogramsToNHCB: true})

	nativeTS := findSeriesTS(c, "test_hist")
	require.NotNil(t, nativeTS)
	require.Len(t, nativeTS.Exemplars, 1, "exemplar carried onto the NHCB series")
	assert.Equal(t, 7.0, nativeTS.Exemplars[0].Value)
}

func TestAddHistogramDataPoints_ClassicDefault(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	metric := newTestExplicitHistogram(ts)
	c := runHistogram(t, metric, Settings{})

	for _, series := range c.unique {
		assert.Empty(t, series.Histograms, "no native histograms when conversion is off")
	}
	// Base name must NOT exist as its own series in classic mode.
	assert.Nil(t, findSeriesTS(c, "test_hist"),
		"classic mode emits only _bucket/_sum/_count, never the bare name")
}

// TestAddHistogramDataPoints_ConversionErrorKeepsClassic exercises the error
// branch: a NaN bound makes the NHCB conversion fail. The error must surface,
// no empty native series may be created, and (with keep_classic) the classic
// series must still be emitted.
func TestAddHistogramDataPoints_ConversionErrorKeepsClassic(t *testing.T) {
	ts := pcommon.Timestamp(1_700_000_000_000_000_000)
	metric := newTestExplicitHistogram(ts)
	metric.Histogram().DataPoints().At(0).ExplicitBounds().FromRaw([]float64{1, math.NaN(), 3})

	converter := newPrometheusConverter(Settings{ConvertHistogramsToNHCB: true, KeepClassicHistograms: true})
	err := converter.addHistogramDataPoints(
		metric.Histogram().DataPoints(),
		pcommon.NewResource(),
		pcommon.NewInstrumentationScope(),
		Settings{ConvertHistogramsToNHCB: true, KeepClassicHistograms: true},
		metric.Name(),
	)
	require.Error(t, err, "conversion failure must surface")

	// No native histogram and no empty base-name series.
	if ts := findSeriesTS(converter, "test_hist"); ts != nil {
		assert.Empty(t, ts.Histograms, "no native histogram appended on conversion error")
	}
	// Classic series still emitted.
	require.NotNil(t, findSeriesTS(converter, "test_hist_count"), "classic series still emitted on NHCB error")
}
