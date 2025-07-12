// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

func Test_DropBucket(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		input    func() pmetric.Metric
		want     func() pmetric.Metric
		wantErr  bool
		regexErr bool
	}{
		{
			name:    "drop single bucket from histogram",
			pattern: "^10$",
			input: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 5, 10, 20})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4, 5})
				dp.SetCount(15)
				return metric
			},
			want: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 5, 20})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 5})
				dp.SetCount(11)
				return metric
			},
			wantErr: false,
		},
		{
			name:    "drop multiple buckets from histogram",
			pattern: "^(5|20)$",
			input: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 5, 10, 20})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4, 5})
				dp.SetCount(15)
				return metric
			},
			want: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 10})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 4})
				dp.SetCount(7)
				return metric
			},
			wantErr: false,
		},
		{
			name:    "drop bucket with decimal pattern",
			pattern: "^1\\.5$",
			input: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 1.5, 2, 3})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4, 5})
				dp.SetCount(15)
				return metric
			},
			want: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 4, 5})
				dp.SetCount(12)
				return metric
			},
			wantErr: false,
		},
		{
			name:    "no matching buckets",
			pattern: "^100$",
			input: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 5, 10, 20})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4, 5})
				dp.SetCount(15)
				return metric
			},
			want: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 5, 10, 20})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4, 5})
				dp.SetCount(15)
				return metric
			},
			wantErr: false,
		},
		{
			name:    "empty histogram",
			pattern: "^10$",
			input: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				metric.Histogram().DataPoints().AppendEmpty()
				return metric
			},
			want: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				metric.Histogram().DataPoints().AppendEmpty()
				return metric
			},
			wantErr: false,
		},
		{
			name:    "drop bucket using wildcard pattern",
			pattern: "^1.*",
			input: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 10, 15, 20})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4, 5})
				dp.SetCount(15)
				return metric
			},
			want: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{20})
				dp.BucketCounts().FromRaw([]uint64{1, 5})
				dp.SetCount(6)
				return metric
			},
			wantErr: false,
		},
		{
			name:    "drop first bucket",
			pattern: "^1$",
			input: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 5, 10})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
				dp.SetCount(10)
				return metric
			},
			want: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{5, 10})
				dp.BucketCounts().FromRaw([]uint64{1, 3, 4})
				dp.SetCount(8)
				return metric
			},
			wantErr: false,
		},
		{
			name:    "drop last bucket",
			pattern: "^10$",
			input: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 5, 10})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
				dp.SetCount(10)
				return metric
			},
			want: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.ExplicitBounds().FromRaw([]float64{1, 5})
				dp.BucketCounts().FromRaw([]uint64{1, 2, 3})
				dp.SetCount(6)
				return metric
			},
			wantErr: false,
		},
		{
			name:    "invalid regex pattern",
			pattern: "[",
			input: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				return metric
			},
			want: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_histogram")
				metric.SetEmptyHistogram()
				return metric
			},
			wantErr:  true,
			regexErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := ottlmetric.NewTransformContext(
				tt.input(),
				pmetric.NewMetricSlice(),
				pcommon.NewInstrumentationScope(),
				pcommon.NewResource(),
				pmetric.NewScopeMetrics(),
				pmetric.NewResourceMetrics(),
			)

			compiledPattern, err := regexp.Compile(tt.pattern)
			if tt.regexErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			exprFunc := dropHistogramBucketsFunc(compiledPattern)
			_, err = exprFunc(context.Background(), target)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want(), target.GetMetric())
			}
		})
	}
}

func Test_CreateDropHistogramBucketsFunction(t *testing.T) {
	tests := []struct {
		name    string
		args    ottl.Arguments
		wantErr bool
	}{
		{
			name: "valid arguments",
			args: &DropBucketArguments{
				Pattern: "^10$",
			},
			wantErr: false,
		},
		{
			name:    "invalid arguments type",
			args:    "invalid_type",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := createDropHistogramBucketsFunction(ottl.FunctionContext{}, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_DropBucketMultipleDataPoints(t *testing.T) {
	pattern := "^10$"

	metric := pmetric.NewMetric()
	metric.SetName("test_histogram")
	metric.SetEmptyHistogram()

	dp1 := metric.Histogram().DataPoints().AppendEmpty()
	dp1.ExplicitBounds().FromRaw([]float64{1, 5, 10, 20})
	dp1.BucketCounts().FromRaw([]uint64{1, 2, 3, 4, 5})

	dp2 := metric.Histogram().DataPoints().AppendEmpty()
	dp2.ExplicitBounds().FromRaw([]float64{2, 10, 15})
	dp2.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})

	target := ottlmetric.NewTransformContext(
		metric,
		pmetric.NewMetricSlice(),
		pcommon.NewInstrumentationScope(),
		pcommon.NewResource(),
		pmetric.NewScopeMetrics(),
		pmetric.NewResourceMetrics(),
	)

	compiledPattern, err := regexp.Compile(pattern)
	require.NoError(t, err)
	exprFunc := dropHistogramBucketsFunc(compiledPattern)
	_, err = exprFunc(context.Background(), target)
	assert.NoError(t, err)

	resultDP1 := target.GetMetric().Histogram().DataPoints().At(0)
	expectedBounds1 := []float64{1, 5, 20}
	expectedCounts1 := []uint64{1, 2, 3, 5}

	assert.Equal(t, len(expectedBounds1), resultDP1.ExplicitBounds().Len())
	assert.Equal(t, len(expectedCounts1), resultDP1.BucketCounts().Len())

	for i := 0; i < len(expectedBounds1); i++ {
		assert.Equal(t, expectedBounds1[i], resultDP1.ExplicitBounds().At(i))
	}
	for i := 0; i < len(expectedCounts1); i++ {
		assert.Equal(t, expectedCounts1[i], resultDP1.BucketCounts().At(i))
	}

	resultDP2 := target.GetMetric().Histogram().DataPoints().At(1)
	expectedBounds2 := []float64{2, 15}
	expectedCounts2 := []uint64{1, 2, 4}

	assert.Equal(t, len(expectedBounds2), resultDP2.ExplicitBounds().Len())
	assert.Equal(t, len(expectedCounts2), resultDP2.BucketCounts().Len())

	for i := 0; i < len(expectedBounds2); i++ {
		assert.Equal(t, expectedBounds2[i], resultDP2.ExplicitBounds().At(i))
	}
	for i := 0; i < len(expectedCounts2); i++ {
		assert.Equal(t, expectedCounts2[i], resultDP2.BucketCounts().At(i))
	}
}
