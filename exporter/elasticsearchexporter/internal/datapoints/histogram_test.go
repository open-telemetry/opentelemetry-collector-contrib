// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func TestHistogramValue(t *testing.T) {
	for _, tc := range []struct {
		name        string
		histogramDP pmetric.HistogramDataPoint
		mapping     HistogramMapping
		expected    func(t *testing.T) pcommon.Value
	}{
		{
			name: "empty",
			histogramDP: func() pmetric.HistogramDataPoint {
				now := time.Now()
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				return dp
			}(),
			expected: func(t *testing.T) pcommon.Value {
				t.Helper()
				v := pcommon.NewValueMap()
				require.NoError(t, v.FromRaw(map[string]any{
					"counts": []any{},
					"values": []any{},
				}))
				return v
			},
		},
		{
			name: "required_fields_only",
			histogramDP: func() pmetric.HistogramDataPoint {
				now := time.Now()
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				dp.SetCount(100)
				dp.SetSum(1000)
				return dp
			}(),
			mapping: HistogramMappingRaw,
			expected: func(t *testing.T) pcommon.Value {
				t.Helper()
				v := pcommon.NewValueMap()
				require.NoError(t, v.FromRaw(map[string]any{
					"counts": []any{100},
					"values": []any{10.0},
				}))
				return v
			},
		},
		{
			name: "exponential histogram",
			histogramDP: func() pmetric.HistogramDataPoint {
				now := time.Now()
				dp := pmetric.NewHistogramDataPoint()
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-1 * time.Hour)))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
				dp.SetCount(100)
				dp.SetSum(1000)
				return dp
			}(),
			mapping: HistogramMappingExponential,
			expected: func(t *testing.T) pcommon.Value {
				t.Helper()
				v := pcommon.NewValueMap()
				require.NoError(t, v.FromRaw(map[string]any{
					"scale": 38,
					"positive": map[string]any{
						"indices": []any{int64(913124641741)},
						"counts":  []any{100},
					},
					"sum": 1000.0,
				}))
				return v
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			m.SetName("test")
			tc.histogramDP.MoveTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())

			esHist := NewHistogram(m, m.Histogram().DataPoints().At(0), tc.mapping)
			actual, err := esHist.Value()
			require.NoError(t, err)
			assert.True(t, tc.expected(t).Equal(actual), "expected: %v, got: %v", tc.expected(t).AsRaw(), actual.AsRaw())
		})
	}
}

func TestHistogramValueInvalidBucketCounts(t *testing.T) {
	for _, mapping := range []HistogramMapping{
		HistogramMappingExponential,
		HistogramMappingTDigest,
		HistogramMappingRaw,
	} {
		t.Run(mapping.String(), func(t *testing.T) {
			dp := pmetric.NewHistogramDataPoint()
			dp.ExplicitBounds().FromRaw([]float64{1, 2})
			dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})

			m := pmetric.NewMetric()
			m.SetName("test")
			dp.MoveTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())

			h := NewHistogram(m, m.Histogram().DataPoints().At(0), mapping)

			_, err := h.Value()
			require.Error(t, err)
		})
	}
}

func TestHistogramValueDefault(t *testing.T) {
	for _, tc := range []struct {
		name           string
		explicitBounds []float64
		bucketCounts   []uint64
		wantValues     []any
		wantCounts     []any
	}{
		{
			name:           "midpoint of middle buckets",
			explicitBounds: []float64{1.0, 2.0, 3.0},
			bucketCounts:   []uint64{1, 2, 3, 4},
			wantValues:     []any{0.5, 1.5, 2.5, 3.0},
			wantCounts:     []any{1, 2, 3, 4},
		},
		{
			name:           "first bucket halved",
			explicitBounds: []float64{10.0},
			bucketCounts:   []uint64{5, 3},
			wantValues:     []any{5.0, 10.0},
			wantCounts:     []any{5, 3},
		},
		{
			name:           "zero count buckets skipped",
			explicitBounds: []float64{1.0, 2.0, 3.0},
			bucketCounts:   []uint64{0, 2, 0, 4},
			wantValues:     []any{1.5, 3.0},
			wantCounts:     []any{2, 4},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dp := pmetric.NewHistogramDataPoint()
			dp.ExplicitBounds().FromRaw(tc.explicitBounds)
			dp.BucketCounts().FromRaw(tc.bucketCounts)

			m := pmetric.NewMetric()
			m.SetName("test")
			dp.MoveTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())

			esHist := NewHistogram(m, m.Histogram().DataPoints().At(0), HistogramMappingRaw)
			actual, err := esHist.Value()
			require.NoError(t, err)

			expected := pcommon.NewValueMap()
			require.NoError(t, expected.FromRaw(map[string]any{
				"counts": tc.wantCounts,
				"values": tc.wantValues,
			}))
			assert.True(t, expected.Equal(actual))
		})
	}
}

func TestHistogramValueRaw(t *testing.T) {
	for _, tc := range []struct {
		name           string
		explicitBounds []float64
		bucketCounts   []uint64
		sum            float64
		count          uint64
		wantValues     []any
		wantCounts     []any
	}{
		{
			name:           "values passed through without transformation",
			explicitBounds: []float64{10.0, 20.0, 30.0},
			bucketCounts:   []uint64{1, 3, 5, 0},
			wantValues:     []any{10.0, 20.0, 30.0},
			wantCounts:     []any{1, 3, 5},
		},
		{
			name:           "single bucket",
			explicitBounds: []float64{5.0},
			bucketCounts:   []uint64{3, 0},
			wantValues:     []any{5.0},
			wantCounts:     []any{3},
		},
		{
			name:           "zero count buckets skipped",
			explicitBounds: []float64{10.0, 20.0, 30.0},
			bucketCounts:   []uint64{0, 3, 0, 0},
			wantValues:     []any{20.0},
			wantCounts:     []any{3},
		},
		{
			name:           "overflow bucket merged into last real bucket",
			explicitBounds: []float64{10.0, 20.0},
			bucketCounts:   []uint64{1, 2, 3},
			wantValues:     []any{10.0, 20.0},
			wantCounts:     []any{1, 5},
		},
		{
			name:           "overflow bucket with zero count",
			explicitBounds: []float64{10.0, 20.0},
			bucketCounts:   []uint64{1, 2, 0},
			wantValues:     []any{10.0, 20.0},
			wantCounts:     []any{1, 2},
		},
		{
			name:           "no explicit bounds with count and sum",
			explicitBounds: []float64{},
			bucketCounts:   []uint64{},
			sum:            100.0,
			count:          10,
			wantValues:     []any{10.0},
			wantCounts:     []any{10},
		},
		{
			name:           "no explicit bounds with zero count",
			explicitBounds: []float64{},
			bucketCounts:   []uint64{},
			sum:            0,
			count:          0,
			wantValues:     []any{},
			wantCounts:     []any{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dp := pmetric.NewHistogramDataPoint()
			dp.ExplicitBounds().FromRaw(tc.explicitBounds)
			dp.BucketCounts().FromRaw(tc.bucketCounts)
			dp.SetSum(tc.sum)
			dp.SetCount(tc.count)
			setMappingHint(dp.Attributes(), elasticsearch.HintHistogramRaw)

			m := pmetric.NewMetric()
			m.SetName("test")
			dp.MoveTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())

			esHist := NewHistogram(m, m.Histogram().DataPoints().At(0), HistogramMappingRaw)
			actual, err := esHist.Value()
			require.NoError(t, err)

			expected := pcommon.NewValueMap()
			require.NoError(t, expected.FromRaw(map[string]any{
				"counts": tc.wantCounts,
				"values": tc.wantValues,
			}))
			assert.True(t, expected.Equal(actual))
		})
	}
}

func TestHistogramValueAggregateMetricDoubleTakesPrecedence(t *testing.T) {
	dp := pmetric.NewHistogramDataPoint()
	dp.ExplicitBounds().FromRaw([]float64{10.0, 20.0, 30.0})
	dp.BucketCounts().FromRaw([]uint64{1, 3, 5, 0})
	dp.SetSum(200.0)
	dp.SetCount(9)
	hints := dp.Attributes().PutEmptySlice(elasticsearch.MappingHintsAttrKey)
	hints.AppendEmpty().SetStr(string(elasticsearch.HintHistogramRaw))
	hints.AppendEmpty().SetStr(string(elasticsearch.HintAggregateMetricDouble))

	m := pmetric.NewMetric()
	m.SetName("test")
	dp.MoveTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())

	esHist := NewHistogram(m, m.Histogram().DataPoints().At(0), HistogramMappingRaw)
	actual, err := esHist.Value()
	require.NoError(t, err)

	expected := pcommon.NewValueMap()
	require.NoError(t, expected.FromRaw(map[string]any{
		"sum":         200.0,
		"value_count": 9,
	}))
	assert.True(t, expected.Equal(actual))
}

func setMappingHint(attrs pcommon.Map, hint elasticsearch.MappingHint) {
	s := attrs.PutEmptySlice(elasticsearch.MappingHintsAttrKey)
	s.AppendEmpty().SetStr(string(hint))
}

func TestHistogramDynamicTemplate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dp       pmetric.HistogramDataPoint
		hm       HistogramMapping
		mode     DynamicTemplateMode
		expected string
	}{
		{
			name: "ecs_mode returns summary_metrics when hint is aggregate double",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				setMappingHint(dp.Attributes(), elasticsearch.HintAggregateMetricDouble)
				return dp
			}(),
			hm:       HistogramMappingAggregateMetricDouble,
			mode:     DynamicTemplateModeECS,
			expected: "summary_metrics",
		},
		{
			name: "ecs_mode returns histogram_metrics when no hint is supplied",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				return dp
			}(),
			hm:       HistogramMappingTDigest,
			mode:     DynamicTemplateModeECS,
			expected: "histogram_metrics",
		},
		{
			name: "otel_mode returns `exponential_histogram` when default histogram mapping is exponential_histogram",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				return dp
			}(),
			hm:       HistogramMappingExponential,
			mode:     DynamicTemplateModeOTel,
			expected: "exponential_histogram",
		},
		{
			name: "otel_mode returns `histogram` when no hint is supplied",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				return dp
			}(),
			hm:       HistogramMappingTDigest,
			mode:     DynamicTemplateModeOTel,
			expected: "histogram",
		},
		{
			name: "otel_mode returns `summary` when mapping is aggregate double",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				return dp
			}(),
			hm:       HistogramMappingAggregateMetricDouble,
			mode:     DynamicTemplateModeOTel,
			expected: "summary",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("%s\n", tc.name)
			m := pmetric.NewMetric()
			m.SetName("test")
			tc.dp.MoveTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())

			esHist := NewHistogram(m, m.Histogram().DataPoints().At(0), tc.hm)

			_, err := esHist.Value()
			require.NoError(t, err)

			actual := esHist.DynamicTemplate(m, tc.mode)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
