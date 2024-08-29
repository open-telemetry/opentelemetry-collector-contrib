// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exphistogram

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
