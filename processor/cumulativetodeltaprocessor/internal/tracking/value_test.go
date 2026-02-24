// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracking

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExponentialBuckets_Diff(t *testing.T) {
	tests := []struct {
		name    string
		current ExponentialBuckets
		old     ExponentialBuckets
		want    ExponentialBuckets
	}{
		{
			name: "sparse buckets without previous",
			current: ExponentialBuckets{
				Offset:       0,
				BucketCounts: []uint64{1, 0, 2, 0, 3},
			},
			old: ExponentialBuckets{
				Offset:       0,
				BucketCounts: nil,
			},
			want: ExponentialBuckets{
				Offset:       0,
				BucketCounts: []uint64{1, 0, 2, 0, 3},
			},
		},
		{
			name: "leading zeros and non-zero offset",
			current: ExponentialBuckets{
				Offset:       5,
				BucketCounts: []uint64{0, 4, 0, 2},
			},
			old: ExponentialBuckets{
				Offset:       5,
				BucketCounts: nil,
			},
			want: ExponentialBuckets{
				Offset:       6,
				BucketCounts: []uint64{4, 0, 2},
			},
		},
		{
			name: "subtract previous counts on overlapping buckets",
			current: ExponentialBuckets{
				Offset:       2,
				BucketCounts: []uint64{5, 0, 9, 1},
			},
			old: ExponentialBuckets{
				Offset:       1,
				BucketCounts: []uint64{3, 0, 6, 1, 1},
			},
			want: ExponentialBuckets{
				Offset:       2,
				BucketCounts: []uint64{5, 0, 8},
			},
		},
		{
			name: "monotonicity fallback keeps later valid bucket",
			current: ExponentialBuckets{
				Offset:       0,
				BucketCounts: []uint64{1, 2, 1, 5},
			},
			old: ExponentialBuckets{
				Offset:       0,
				BucketCounts: []uint64{1, 3, 2, 3},
			},
			want: ExponentialBuckets{
				Offset:       3,
				BucketCounts: []uint64{2},
			},
		},
		{
			name: "empty when all buckets are non-increasing",
			current: ExponentialBuckets{
				Offset:       10,
				BucketCounts: []uint64{0, 1},
			},
			old: ExponentialBuckets{
				Offset:       10,
				BucketCounts: []uint64{0, 2},
			},
			want: ExponentialBuckets{},
		},
		{
			name: "negative offsets",
			current: ExponentialBuckets{
				Offset:       -3,
				BucketCounts: []uint64{2, 0, 4},
			},
			old: ExponentialBuckets{
				Offset:       -4,
				BucketCounts: []uint64{1, 1, 3, 1},
			},
			want: ExponentialBuckets{
				Offset:       -3,
				BucketCounts: []uint64{1, 0, 3},
			},
		},
		{
			name: "first non-zero bucket sets output offset",
			current: ExponentialBuckets{
				Offset:       100,
				BucketCounts: []uint64{0, 0, 9},
			},
			old: ExponentialBuckets{
				Offset:       0,
				BucketCounts: nil,
			},
			want: ExponentialBuckets{
				Offset:       102,
				BucketCounts: []uint64{9},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.current.Diff(&tc.old)
			assert.Equal(t, tc.want, got)
		})
	}
}
