// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"math"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func runHistogramV2(t *testing.T, metric pmetric.Metric, settings Settings) *prometheusConverterV2 {
	t.Helper()
	converter := newPrometheusConverterV2(settings)
	m := metadata{
		Type: otelMetricTypeToPromMetricTypeV2(metric),
		Help: metric.Description(),
	}
	require.NoError(t, converter.addHistogramDataPoints(
		metric.Histogram().DataPoints(),
		pcommon.NewResource(),
		pcommon.NewInstrumentationScope(),
		settings,
		metric.Name(),
		m,
	))
	return converter
}

// v2SeriesByName resolves each time series' labels through the symbol table and
// returns the one whose __name__ equals name, or nil.
func v2SeriesByName(t *testing.T, c *prometheusConverterV2, name string) *writev2.TimeSeries {
	t.Helper()
	symbols := c.symbolTable.Symbols()
	all := c.timeSeries()
	var b labels.ScratchBuilder
	for i := range all {
		lbls, err := all[i].ToLabels(&b, symbols)
		require.NoError(t, err)
		if lbls.Get(model.MetricNameLabel) == name {
			return &all[i]
		}
	}
	return nil
}

func TestExplicitToNHCBHistogramV2(t *testing.T) {
	pt := newTestExplicitHistogram().Histogram().DataPoints().At(0)

	h, err := explicitToNHCBHistogramV2(pt)
	require.NoError(t, err)

	assert.Equal(t, histogram.CustomBucketsSchema, h.Schema, "must be NHCB schema -53")
	assert.Equal(t, []float64{1, 2, 3}, h.CustomValues, "explicit bounds carried as custom values")
	assert.Equal(t, uint64(10), h.GetCountInt(), "count preserved")
	assert.InDelta(t, 42.5, h.Sum, 1e-9, "sum preserved")
	assert.Equal(t, convertTimeStamp(testHistTimestamp), h.Timestamp)
}

// TestExplicitToNHCBHistogramV2_BucketCountsRoundTrip decodes the produced wire
// histogram back and asserts every cumulative bucket count and upper bound
// matches the original OTLP histogram.
func TestExplicitToNHCBHistogramV2_BucketCountsRoundTrip(t *testing.T) {
	pt := newTestExplicitHistogram().Histogram().DataPoints().At(0)

	h, err := explicitToNHCBHistogramV2(pt)
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
	want := []bucket{{1, 1}, {2, 3}, {3, 6}, {math.Inf(1), 10}}
	assert.Equal(t, want, got)
}

func TestExplicitToNHCBHistogramV2_StaleMarker(t *testing.T) {
	metric := newTestExplicitHistogram()
	pt := metric.Histogram().DataPoints().At(0)
	pt.SetFlags(pt.Flags().WithNoRecordedValue(true))

	h, err := explicitToNHCBHistogramV2(pt)
	require.NoError(t, err)
	assert.Equal(t, histogram.CustomBucketsSchema, h.Schema)
	assert.Equal(t, value.StaleNaN, h.GetCountInt(), "stale marker count")
	assert.True(t, math.IsNaN(h.Sum), "stale marker sum")
}

func TestAddHistogramDataPointsV2_NHCBOnly(t *testing.T) {
	metric := newTestExplicitHistogram()
	c := runHistogramV2(t, metric, Settings{ConvertHistogramsToNHCB: true})

	require.Len(t, c.unique, 1, "NHCB-only should emit a single native series")
	for _, series := range c.unique {
		require.Len(t, series.Histograms, 1)
		assert.Empty(t, series.Samples, "no classic samples in NHCB-only mode")
		assert.Equal(t, histogram.CustomBucketsSchema, series.Histograms[0].Schema)
	}
	require.NotNil(t, v2SeriesByName(t, c, "test_hist"), "native series uses the base metric name")
}

func TestAddHistogramDataPointsV2_KeepClassic(t *testing.T) {
	metric := newTestExplicitHistogram()
	c := runHistogramV2(t, metric, Settings{ConvertHistogramsToNHCB: true, KeepClassicHistograms: true})

	var nativeSeries, classicSamples int
	for _, series := range c.unique {
		nativeSeries += len(series.Histograms)
		classicSamples += len(series.Samples)
	}
	assert.Equal(t, 1, nativeSeries, "one NHCB datapoint emitted alongside classic")
	assert.Positive(t, classicSamples, "classic _bucket/_sum/_count still emitted")

	require.NotNil(t, v2SeriesByName(t, c, "test_hist"), "native series present under base name")
	require.NotNil(t, v2SeriesByName(t, c, "test_hist_bucket"), "classic _bucket series present")
	require.NotNil(t, v2SeriesByName(t, c, "test_hist_count"), "classic _count series present")
}

func TestAddHistogramDataPointsV2_NHCBExemplars(t *testing.T) {
	metric := newTestExplicitHistogram()
	ex := metric.Histogram().DataPoints().At(0).Exemplars().AppendEmpty()
	ex.SetTimestamp(testHistTimestamp)
	ex.SetDoubleValue(7)

	c := runHistogramV2(t, metric, Settings{ConvertHistogramsToNHCB: true})

	nativeTS := v2SeriesByName(t, c, "test_hist")
	require.NotNil(t, nativeTS)
	require.Len(t, nativeTS.Exemplars, 1, "exemplar carried onto the NHCB series")
	assert.Equal(t, 7.0, nativeTS.Exemplars[0].Value)
}

func TestAddHistogramDataPointsV2_ClassicDefault(t *testing.T) {
	metric := newTestExplicitHistogram()
	c := runHistogramV2(t, metric, Settings{})

	for _, series := range c.unique {
		assert.Empty(t, series.Histograms, "no native histograms when conversion is off")
	}
	assert.Nil(t, v2SeriesByName(t, c, "test_hist"),
		"classic mode emits only _bucket/_sum/_count, never the bare name")
}

func TestAddHistogramDataPointsV2_ConversionErrorKeepsClassic(t *testing.T) {
	metric := newTestExplicitHistogram()
	metric.Histogram().DataPoints().At(0).ExplicitBounds().FromRaw([]float64{1, math.NaN(), 3})

	settings := Settings{ConvertHistogramsToNHCB: true, KeepClassicHistograms: true}
	converter := newPrometheusConverterV2(settings)
	m := metadata{Type: otelMetricTypeToPromMetricTypeV2(metric), Help: metric.Description()}
	err := converter.addHistogramDataPoints(
		metric.Histogram().DataPoints(),
		pcommon.NewResource(),
		pcommon.NewInstrumentationScope(),
		settings,
		metric.Name(),
		m,
	)
	require.Error(t, err, "conversion failure must surface")

	if nativeTS := v2SeriesByName(t, converter, "test_hist"); nativeTS != nil {
		assert.Empty(t, nativeTS.Histograms, "no native histogram appended on conversion error")
	}
	require.NotNil(t, v2SeriesByName(t, converter, "test_hist_count"), "classic series still emitted on NHCB error")
}
