// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func TestExponentialHistogramValue(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dp       pmetric.ExponentialHistogramDataPoint
		hm       HistogramMapping
		expected map[string]any
	}{
		{
			name: "Native exponential_histogram",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetScale(0)
				dp.Positive().SetOffset(3)
				dp.Positive().BucketCounts().FromRaw([]uint64{1, 4, 6, 15})
				dp.SetSum(14513)
				dp.SetMin(8)
				dp.SetMax(1024)

				return dp
			}(),
			hm: HistogramMappingExponential,
			expected: map[string]any{
				"scale":    0,
				"sum":      14513.0,
				"min":      8.0,
				"max":      1024.0,
				"positive": map[string]any{"indices": []any{3, 4, 5, 6}, "counts": []any{1, 4, 6, 15}},
			},
		},
		{
			name: "default returns t-digest histogram",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetScale(0)
				dp.Positive().SetOffset(3)
				dp.Positive().BucketCounts().FromRaw([]uint64{1, 4, 6, 15})
				dp.Negative().BucketCounts().FromRaw([]uint64{3, 4, 8, 20})

				return dp
			}(),
			hm: HistogramMappingTDigest,
			expected: map[string]any{
				"counts": []any{20, 8, 4, 3, 1, 4, 6, 15},
				"values": []any{-12.0, -6.0, -3.0, -1.5, 12.0, 24.0, 48.0, 96.0},
			},
		},
		{
			name: "raw hint returns raw histogram",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetScale(0)
				dp.Positive().SetOffset(3)
				dp.Positive().BucketCounts().FromRaw([]uint64{1, 4, 6, 15})
				dp.Negative().BucketCounts().FromRaw([]uint64{3, 4, 8, 20})
				setMappingHint(dp.Attributes(), elasticsearch.HintHistogramRaw)

				return dp
			}(),
			hm: HistogramMappingRaw,
			expected: map[string]any{
				"counts": []any{20, 8, 4, 3, 1, 4, 6, 15},
				"values": []any{-8.0, -4.0, -2.0, -1.0, 16.0, 32.0, 64.0, 128.0},
			},
		},
		{
			name: "Aggregate metric double returns sum",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				setMappingHint(dp.Attributes(), elasticsearch.HintAggregateMetricDouble)

				return dp
			}(),
			hm: HistogramMappingAggregateMetricDouble,
			expected: map[string]any{
				"sum":         123.0,
				"value_count": 3,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			m.SetName("test")
			tc.dp.MoveTo(m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty())

			esHist := NewExponentialHistogram(m, m.ExponentialHistogram().DataPoints().At(0), tc.hm)
			actual, err := esHist.Value()
			require.NoError(t, err)

			expected := pcommon.NewValueMap()
			require.NoError(t, expected.FromRaw(tc.expected))
			assert.True(t, expected.Equal(actual))
		})
	}
}

func TestExponentialHistogramDynamicTemplate(t *testing.T) {
	dynamicTemplateTests := []struct {
		name     string
		dp       pmetric.ExponentialHistogramDataPoint
		hm       HistogramMapping
		mode     DynamicTemplateMode
		expected string
	}{
		{
			name: "ecs_mode returns summary_metrics when hint has aggregate double",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				setMappingHint(dp.Attributes(), elasticsearch.HintAggregateMetricDouble)
				return dp
			}(),
			hm:       HistogramMappingTDigest,
			mode:     DynamicTemplateModeECS,
			expected: "summary_metrics",
		},
		{
			name: "ecs_mode returns histogram_metrics when no hint is supplied",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				return dp
			}(),
			hm:       HistogramMappingTDigest,
			mode:     DynamicTemplateModeECS,
			expected: "histogram_metrics",
		},
		{
			name: "ecs_mode ignores exponential hint",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				return dp
			}(),
			hm:       HistogramMappingTDigest,
			mode:     DynamicTemplateModeECS,
			expected: "histogram_metrics",
		},
		{
			name: "otel_mode returns `summary` when hint is aggregate metric double",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				setMappingHint(dp.Attributes(), elasticsearch.HintAggregateMetricDouble)
				return dp
			}(),
			hm:       HistogramMappingTDigest,
			mode:     DynamicTemplateModeOTel,
			expected: "summary",
		},
		{
			name: "otel_mode returns `exponential_histogram` when default histogram mapping is exponential_histogram",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
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
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				return dp
			}(),
			hm:       HistogramMappingTDigest,
			mode:     DynamicTemplateModeOTel,
			expected: "histogram",
		},
		{
			name: "otel_mode returns `histogram` when histogram:raw hint is supplied",
			dp: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetSum(123)
				dp.SetCount(3)
				setMappingHint(dp.Attributes(), elasticsearch.HintHistogramRaw)
				return dp
			}(),
			hm:       HistogramMappingTDigest,
			mode:     DynamicTemplateModeOTel,
			expected: "histogram",
		},
	}
	for _, tc := range dynamicTemplateTests {
		t.Run(tc.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			m.SetName("test")
			tc.dp.MoveTo(m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty())

			esHist := NewExponentialHistogram(m, m.ExponentialHistogram().DataPoints().At(0), tc.hm)
			_, err := esHist.Value()
			require.NoError(t, err)

			val := esHist.DynamicTemplate(m, tc.mode)
			assert.Equal(t, tc.expected, val)
		})
	}
}
