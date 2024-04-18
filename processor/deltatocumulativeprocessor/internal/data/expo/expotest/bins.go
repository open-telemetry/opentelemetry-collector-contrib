package expotest

import (
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	Empty = math.MaxUint64
	ø     = Empty
)

// index:  0  1  2 3 4 5 6 7
// bucket: -3 -2 -1 0 1 2 3 4
// bounds: (0.125,0.25], (0.25,0.5], (0.5,1], (1,2], (2,4], (4,8], (8,16], (16,32]
type Bins [8]uint64

func Buckets(bins Bins) pmetric.ExponentialHistogramDataPointBuckets {
	start := 0
	for i := 0; i < len(bins); i++ {
		if bins[i] != ø {
			start = i
			break
		}
	}

	end := len(bins)
	for i := start; i < len(bins); i++ {
		if bins[i] == ø {
			end = i
			break
		}
	}

	counts := bins[start:end]

	buckets := pmetric.NewExponentialHistogramDataPointBuckets()
	buckets.SetOffset(int32(start - 3))
	buckets.BucketCounts().FromRaw(counts)
	return buckets
}
