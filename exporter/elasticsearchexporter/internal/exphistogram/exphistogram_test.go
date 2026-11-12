// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exphistogram

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestToTDigest(t *testing.T) {
	for _, tc := range []struct {
		name            string
		scale           int32
		zeroCount       uint64
		positiveOffset  int32
		positiveBuckets []uint64
		negativeOffset  int32
		negativeBuckets []uint64

		expectedCounts []int64
		expectedValues []float64
	}{
		{
			name:           "empty",
			scale:          0,
			expectedCounts: nil,
			expectedValues: nil,
		},
		{
			name:           "empty, scale=1",
			scale:          1,
			expectedCounts: nil,
			expectedValues: nil,
		},
		{
			name:           "empty, scale=-1",
			scale:          -1,
			expectedCounts: nil,
			expectedValues: nil,
		},
		{
			name:           "zeros",
			scale:          0,
			zeroCount:      1,
			expectedCounts: []int64{1},
			expectedValues: []float64{0},
		},
		{
			name:            "scale=0",
			scale:           0,
			zeroCount:       1,
			positiveBuckets: []uint64{1, 1},
			negativeBuckets: []uint64{1, 1},
			expectedCounts:  []int64{1, 1, 1, 1, 1},
			expectedValues:  []float64{-3, -1.5, 0, 1.5, 3},
		},
		{
			name:            "scale=0, no zeros",
			scale:           0,
			zeroCount:       0,
			positiveBuckets: []uint64{1, 1},
			negativeBuckets: []uint64{1, 1},
			expectedCounts:  []int64{1, 1, 1, 1},
			expectedValues:  []float64{-3, -1.5, 1.5, 3},
		},
		{
			name:            "scale=0, offset=1",
			scale:           0,
			zeroCount:       1,
			positiveOffset:  1,
			positiveBuckets: []uint64{1, 1},
			negativeOffset:  1,
			negativeBuckets: []uint64{1, 1},
			expectedCounts:  []int64{1, 1, 1, 1, 1},
			expectedValues:  []float64{-6, -3, 0, 3, 6},
		},
		{
			name:            "scale=0, offset=-1",
			scale:           0,
			zeroCount:       1,
			positiveOffset:  -1,
			positiveBuckets: []uint64{1, 1},
			negativeOffset:  -1,
			negativeBuckets: []uint64{1, 1},
			expectedCounts:  []int64{1, 1, 1, 1, 1},
			expectedValues:  []float64{-1.5, -0.75, 0, 0.75, 1.5},
		},
		{
			name:            "scale=0, different offsets",
			scale:           0,
			zeroCount:       1,
			positiveOffset:  -1,
			positiveBuckets: []uint64{1, 1},
			negativeOffset:  1,
			negativeBuckets: []uint64{1, 1},
			expectedCounts:  []int64{1, 1, 1, 1, 1},
			expectedValues:  []float64{-6, -3, 0, 0.75, 1.5},
		},
		{
			name:            "scale=-1",
			scale:           -1,
			zeroCount:       1,
			positiveBuckets: []uint64{1, 1},
			negativeBuckets: []uint64{1, 1},
			expectedCounts:  []int64{1, 1, 1, 1, 1},
			expectedValues:  []float64{-10, -2.5, 0, 2.5, 10},
		},
		{
			name:            "scale=1",
			scale:           1,
			zeroCount:       1,
			positiveBuckets: []uint64{1, 1},
			negativeBuckets: []uint64{1, 1},
			expectedCounts:  []int64{1, 1, 1, 1, 1},
			expectedValues:  []float64{-1.7071067811865475, -1.2071067811865475, 0, 1.2071067811865475, 1.7071067811865475},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dp := pmetric.NewExponentialHistogramDataPoint()
			dp.SetScale(tc.scale)
			dp.SetZeroCount(tc.zeroCount)
			dp.Positive().SetOffset(tc.positiveOffset)
			dp.Positive().BucketCounts().FromRaw(tc.positiveBuckets)
			dp.Negative().SetOffset(tc.negativeOffset)
			dp.Negative().BucketCounts().FromRaw(tc.negativeBuckets)

			counts, values := ToTDigest(dp)
			assert.Equal(t, tc.expectedCounts, counts)
			assert.Equal(t, tc.expectedValues, values)
		})
	}
}

func TestToRaw(t *testing.T) {
	for _, tc := range []struct {
		name            string
		scale           int32
		zeroCount       uint64
		positiveOffset  int32
		positiveBuckets []uint64
		negativeOffset  int32
		negativeBuckets []uint64

		expectedCounts []int64
		expectedValues []float64
	}{
		{
			name:           "empty",
			scale:          0,
			expectedCounts: nil,
			expectedValues: nil,
		},
		{
			name:           "zeros only",
			scale:          0,
			zeroCount:      1,
			expectedCounts: []int64{1},
			expectedValues: []float64{0},
		},
		{
			name:            "scale=0, uses upper boundaries",
			scale:           0,
			zeroCount:       1,
			positiveBuckets: []uint64{1, 1},
			negativeBuckets: []uint64{1, 1},
			expectedCounts:  []int64{1, 1, 1, 1, 1},
			// Negative upper bounds: -LowerBoundary(0+i, 0) for buckets iterated in reverse
			// neg bucket 1: ub = -LowerBoundary(1, 0) = -2
			// neg bucket 0: ub = -LowerBoundary(0, 0) = -1
			// Positive upper bounds: LowerBoundary(0+i+1, 0)
			// pos bucket 0: ub = LowerBoundary(1, 0) = 2
			// pos bucket 1: ub = LowerBoundary(2, 0) = 4
			expectedValues: []float64{-2, -1, 0, 2, 4},
		},
		{
			name:            "scale=0, offset=1",
			scale:           0,
			zeroCount:       1,
			positiveOffset:  1,
			positiveBuckets: []uint64{1, 1},
			negativeOffset:  1,
			negativeBuckets: []uint64{1, 1},
			expectedCounts:  []int64{1, 1, 1, 1, 1},
			// neg bucket 1 (idx=1): ub = -LowerBoundary(1+1, 0) = -4
			// neg bucket 0 (idx=0): ub = -LowerBoundary(1+0, 0) = -2
			// pos bucket 0: ub = LowerBoundary(1+0+1, 0) = 4
			// pos bucket 1: ub = LowerBoundary(1+1+1, 0) = 8
			expectedValues: []float64{-4, -2, 0, 4, 8},
		},
		{
			name:            "scale=0, zero count buckets skipped",
			scale:           0,
			positiveBuckets: []uint64{0, 1},
			expectedCounts:  []int64{1},
			expectedValues:  []float64{4},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dp := pmetric.NewExponentialHistogramDataPoint()
			dp.SetScale(tc.scale)
			dp.SetZeroCount(tc.zeroCount)
			dp.Positive().SetOffset(tc.positiveOffset)
			dp.Positive().BucketCounts().FromRaw(tc.positiveBuckets)
			dp.Negative().SetOffset(tc.negativeOffset)
			dp.Negative().BucketCounts().FromRaw(tc.negativeBuckets)

			counts, values := ToRaw(dp)
			assert.Equal(t, tc.expectedCounts, counts)
			assert.Equal(t, tc.expectedValues, values)
		})
	}
}

func TestToNativeExponentialHistogram(t *testing.T) {
	for _, tc := range []struct {
		name            string
		scale           int32
		zeroCount       uint64
		zeroThreshold   float64
		positiveOffset  int32
		positiveBuckets []uint64
		negativeOffset  int32
		negativeBuckets []uint64
		sum, min, max   *float64

		expected map[string]any
	}{
		{
			name:     "empty",
			scale:    0,
			expected: map[string]any{"scale": 0},
		},
		{
			name:            "positive only with zero-count buckets dropped",
			scale:           2,
			positiveOffset:  3,
			positiveBuckets: []uint64{1, 0, 4},
			expected: map[string]any{
				"scale":    2,
				"positive": map[string]any{"indices": []any{3, 5}, "counts": []any{1, 4}},
			},
		},
		{
			name:            "negative only",
			negativeOffset:  0,
			negativeBuckets: []uint64{3, 4},
			expected: map[string]any{
				"scale":    0,
				"negative": map[string]any{"indices": []any{0, 1}, "counts": []any{3, 4}},
			},
		},
		{
			name:            "both positive and negative",
			positiveOffset:  3,
			positiveBuckets: []uint64{1, 4},
			negativeOffset:  0,
			negativeBuckets: []uint64{3, 4},
			expected: map[string]any{
				"scale":    0,
				"negative": map[string]any{"indices": []any{0, 1}, "counts": []any{3, 4}},
				"positive": map[string]any{"indices": []any{3, 4}, "counts": []any{1, 4}},
			},
		},
		{
			name:          "zero count with threshold",
			zeroCount:     5,
			zeroThreshold: 0.001,
			expected: map[string]any{
				"scale": 0,
				"zero":  map[string]any{"count": 5, "threshold": 0.001},
			},
		},
		{
			name:      "zero count without threshold",
			zeroCount: 5,
			expected: map[string]any{
				"scale": 0,
				"zero":  map[string]any{"count": 5},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dp := pmetric.NewExponentialHistogramDataPoint()
			dp.SetScale(tc.scale)
			dp.SetZeroCount(tc.zeroCount)
			if tc.zeroThreshold != 0 {
				dp.SetZeroThreshold(tc.zeroThreshold)
			}
			dp.Positive().SetOffset(tc.positiveOffset)
			dp.Positive().BucketCounts().FromRaw(tc.positiveBuckets)
			dp.Negative().SetOffset(tc.negativeOffset)
			dp.Negative().BucketCounts().FromRaw(tc.negativeBuckets)

			if tc.sum != nil {
				dp.SetSum(*tc.sum)
			}
			if tc.min != nil {
				dp.SetMin(*tc.min)
			}
			if tc.max != nil {
				dp.SetMax(*tc.max)
			}

			actual := ToNativeExponentialHistogram(dp)

			expected := pcommon.NewValueMap()
			require.NoError(t, expected.FromRaw(tc.expected))
			assert.True(t, expected.Equal(actual), "expected %v, got %v", tc.expected, actual)
		})
	}
}

func TestExplicitToNativeExponentialHistogram(t *testing.T) {
	for _, tc := range []struct {
		name           string
		explicitBounds []float64
		bucketCounts   []uint64
		temporality    pmetric.AggregationTemporality
		min            float64
		max            float64
		sum            float64

		expected map[string]any
	}{
		{
			name:           "empty",
			explicitBounds: nil,
			bucketCounts:   nil,
			expected:       map[string]any{"scale": MaxScale},
		},
		{
			name:           "converted (delta)",
			explicitBounds: []float64{1, 2, 3},
			bucketCounts:   []uint64{1, 2, 3, 4},
			min:            0.5, max: 5, sum: 22,
			temporality: pmetric.AggregationTemporalityDelta,
			expected: map[string]any{
				"scale": MaxScale,
				"min":   0.5, "max": 5.0, "sum": 22.0,
				"positive": map[string]any{
					"indices": []any{int64(-274877906945), int64(160793267838), int64(363368827853), int64(435671174782), int64(638246734797)},
					"counts":  []any{1, 2, 3, 3, 1},
				},
			},
		},
		{
			name:           "zeros dropped (delta)",
			explicitBounds: []float64{1, 2, 3},
			bucketCounts:   []uint64{0, 2, 0, 4},
			min:            1.2, max: 6, sum: 28,
			temporality: pmetric.AggregationTemporalityDelta,
			expected: map[string]any{
				"scale": MaxScale,
				"min":   1.2, "max": 6.0, "sum": 28.0,
				"positive": map[string]any{
					"indices": []any{int64(72302346929), int64(160793267838), int64(435671174782), int64(710549081726)},
					"counts":  []any{1, 1, 3, 1},
				},
			},
		},
		{
			name:           "negative and positive (delta)",
			explicitBounds: []float64{-4, -2, 0, 2},
			bucketCounts:   []uint64{1, 2, 3, 4, 5},
			min:            -6, max: 5, sum: 5,
			temporality: pmetric.AggregationTemporalityDelta,
			expected: map[string]any{
				"scale": MaxScale,
				"min":   -6.0, "max": 5.0, "sum": 5.0,
				"negative": map[string]any{
					"indices": []any{int64(-1), int64(435671174782), int64(710549081726)},
					"counts":  []any{3, 2, 1},
				},
				"positive": map[string]any{
					"indices": []any{int64(-1), int64(274877906943), int64(638246734797)},
					"counts":  []any{4, 4, 1},
				},
			},
		},
		{
			name:           "cumulative (no injection)",
			explicitBounds: []float64{1, 2, 3},
			bucketCounts:   []uint64{1, 2, 3, 4},
			min:            0.5, max: 5, sum: 22,
			temporality: pmetric.AggregationTemporalityCumulative,
			expected: map[string]any{
				"scale": MaxScale,
				"min":   0.5, "max": 5.0, "sum": 22.0,
				"positive": map[string]any{
					"indices": []any{int64(-274877906945), int64(160793267838), int64(363368827853), int64(435671174782)},
					"counts":  []any{1, 2, 3, 4},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dp := pmetric.NewHistogramDataPoint()
			dp.ExplicitBounds().FromRaw(tc.explicitBounds)
			dp.BucketCounts().FromRaw(tc.bucketCounts)
			dp.SetMin(tc.min)
			dp.SetMax(tc.max)
			dp.SetSum(tc.sum)

			actual := ExplicitToNativeExponentialHistogram(dp, tc.temporality)
			expected := pcommon.NewValueMap()
			require.NoError(t, expected.FromRaw(tc.expected))
			assert.True(t, expected.Equal(actual), "expected %v, got %v", tc.expected, actual.AsRaw())
		})
	}
}
