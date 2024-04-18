// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Buckets pmetric.ExponentialHistogramDataPointBuckets

func (b Buckets) At(i int) uint64 {
	if idx, ok := b.idx(i); ok {
		return b.data().At(idx)
	}
	return 0
}

func (b Buckets) SetAt(i int, v uint64) {
	if idx, ok := b.idx(i); ok {
		b.data().SetAt(idx, v)
	}
}

func (b Buckets) Len() int {
	return b.data().Len() + b.Offset()
}

func (b Buckets) EnsureLen(n int) {
	sz := n - b.Offset()
	b.data().EnsureCapacity(sz)
	b.data().Append(make([]uint64, sz-b.data().Len())...)
}

func (b Buckets) Truncate(first int) {
	offset := first - b.Offset()
	if b.Offset() >= offset {
		return
	}

	data := b.data().AsRaw()[offset-b.Offset():]
	b.data().FromRaw(data)
	b.as().SetOffset(int32(offset))
}

// Expand the buckets by n slots:
//   - n < 0: prepend at front, lowering offset
//   - n > 0: append to back
func (b Buckets) Expand(n int) {
	switch {
	case n < 0:
		n = -n
		us := pcommon.NewUInt64Slice()
		us.Append(make([]uint64, n+b.data().Len())...)
		for i := 0; i < b.data().Len(); i++ {
			us.SetAt(i+n, b.data().At(i))
		}
		us.MoveTo(b.data())
		b.as().SetOffset(int32(b.Offset() - n))
	case n > 0:
		b.data().Append(make([]uint64, n)...)
	}
}

func (b Buckets) Offset() int {
	return int(b.as().Offset())
}

func (b Buckets) idx(i int) (int, bool) {
	idx := i - b.Offset()
	return idx, idx >= 0 && idx < b.data().Len()
}

func (b Buckets) data() pcommon.UInt64Slice {
	return b.as().BucketCounts()
}

func (b Buckets) as() pmetric.ExponentialHistogramDataPointBuckets {
	return pmetric.ExponentialHistogramDataPointBuckets(b)
}

type ord interface {
	int | int32 | float64
}

// HiLo returns the greater of a and b by comparing the result of applying fn to
// each
func HiLo[T any, N ord](a, b T, fn func(T) N) (hi, lo T) {
	an, bn := fn(a), fn(b)
	if an > bn {
		return a, b
	}
	return b, a
}
