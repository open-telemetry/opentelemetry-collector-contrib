// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package histograms

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestCheckValidity(t *testing.T) {
	tests := []struct {
		name  string
		dp    pmetric.HistogramDataPoint
		valid bool
	}{
		{
			name: "Boundaries Not Ascending",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(5000)
				dp.SetMin(10.0)
				dp.SetMax(200.0)
				dp.ExplicitBounds().FromRaw([]float64{25, 50, 40, 100, 150}) // 40 < 50
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
		{
			name: "Counts Length Mismatch",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(5000)
				dp.SetMin(10.0)
				dp.SetMax(200.0)
				dp.ExplicitBounds().FromRaw([]float64{25, 50, 75, 100})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2}) // Should be 5 counts for 4 boundaries
				return dp
			}(),
			valid: false,
		},
		{
			name: "min greater than max",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(5000)
				dp.SetMin(200.0)
				dp.SetMax(10.0)
				dp.ExplicitBounds().FromRaw([]float64{25, 50, 70, 100, 150})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
		{
			name: "Inf min",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(5000)
				dp.SetMin(math.Inf(-1))
				dp.SetMax(10.0)
				dp.ExplicitBounds().FromRaw([]float64{25, 50, 70, 100, 150})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
		{
			name: "NaN min",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(5000)
				dp.SetMin(math.NaN())
				dp.SetMax(10.0)
				dp.ExplicitBounds().FromRaw([]float64{25, 50, 70, 100, 150})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
		{
			name: "Inf max",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(5000)
				dp.SetMin(10.0)
				dp.SetMax(math.Inf(1))
				dp.ExplicitBounds().FromRaw([]float64{25, 50, 70, 100, 150})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
		{
			name: "NaN max",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(5000)
				dp.SetMin(10.0)
				dp.SetMax(math.NaN())
				dp.ExplicitBounds().FromRaw([]float64{25, 50, 70, 100, 150})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
		{
			name: "NaN Sum",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(math.NaN())
				dp.SetMin(10.0)
				dp.SetMax(200.0)
				dp.ExplicitBounds().FromRaw([]float64{25, 50, 75, 100, 150})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
		{
			name: "Inf Sum",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(math.Inf(1))
				dp.SetMin(10.0)
				dp.SetMax(200.0)
				dp.ExplicitBounds().FromRaw([]float64{25, 50, 75, 100, 150})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
		{
			name: "NaN Boundary",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(5000)
				dp.SetMin(10.0)
				dp.SetMax(200.0)
				dp.ExplicitBounds().FromRaw([]float64{25, math.NaN(), 75, 100, 150})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
		{
			name: "Inf Boundary",
			dp: func() pmetric.HistogramDataPoint {
				dp := pmetric.NewHistogramDataPoint()
				dp.SetCount(100)
				dp.SetSum(5000)
				dp.SetMin(10.0)
				dp.SetMax(200.0)
				dp.ExplicitBounds().FromRaw([]float64{25, 50, math.Inf(1), 100, 150})
				dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})
				return dp
			}(),
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, CheckValidity(tt.dp))
		})
	}
}

func BenchmarkCheckValidity(b *testing.B) {
	dp := pmetric.NewHistogramDataPoint()
	dp.SetCount(100)
	dp.SetSum(5000)
	dp.SetMin(10.0)
	dp.SetMax(200.0)
	dp.ExplicitBounds().FromRaw([]float64{25, 50, 75, 100, 150})
	dp.BucketCounts().FromRaw([]uint64{20, 30, 25, 15, 8, 2})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assert.NoError(b, CheckValidity(dp))
	}
}
