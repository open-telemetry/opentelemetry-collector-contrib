package expotest

import (
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	Empty = math.MaxUint64
	ø     = Empty
)

// idx:  0  1  2 3 4 5 6 7
// bkt: -3 -2 -1 0 1 2 3 4
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
