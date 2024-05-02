// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"

import (
	"fmt"
)

// WidenZero widens the zero-bucket to span at least [-width,width], possibly wider
// if min falls in the middle of a bucket.
//
// Both buckets counts MUST be of same scale.
func WidenZero(dp DataPoint, width float64) {
	switch {
	case width == dp.ZeroThreshold():
		return
	case width < dp.ZeroThreshold():
		panic(fmt.Sprintf("min must be larger than current threshold (%f)", dp.ZeroThreshold()))
	}

	scale := Scale(dp.Scale())
	lo := scale.Idx(width)

	widen := func(bs Buckets) {
		abs := Abs(bs)
		for i := abs.Lower(); i <= lo; i++ {
			dp.SetZeroCount(dp.ZeroCount() + abs.Abs(i))
		}
		up := abs.Upper()
		abs.Slice(min(lo+1, up), up)
	}

	widen(dp.Positive())
	widen(dp.Negative())

	_, max := scale.Bounds(lo)
	dp.SetZeroThreshold(max)
}

// Slice drops data outside the range from <= i < to from the bucket counts. It behaves the same as Go's [a:b]
//
// Limitations:
//   - due to a limitation of the pcommon package, slicing cannot happen in-place and allocates
//   - in consequence, data outside the range is garbage collected
func (a Absolute) Slice(from, to int) {
	lo, up := a.Lower(), a.Upper()
	switch {
	case from > to:
		panic(fmt.Sprintf("bad bounds: must be from<=to (got %d<=%d)", from, to))
	case from < lo || to > up:
		panic(fmt.Sprintf("%d:%d is out of bounds for %d:%d", from, to, lo, up))
	}

	first := from - lo
	last := to - lo

	a.BucketCounts().FromRaw(a.BucketCounts().AsRaw()[first:last])
	a.SetOffset(int32(from))
}
