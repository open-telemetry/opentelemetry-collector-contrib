// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"
)

type hist = expotest.Histogram

func TestWidenZero(t *testing.T) {
	cases := []struct {
		name string
		hist hist
		want hist
		min  float64
	}{{
		// -3            -2          -1       0      1      2      3       4
		// (0.125,0.25], (0.25,0.5], (0.5,1], (1,2], (2,4], (4,8], (8,16], (16,32]
		//
		//                      -3 -2 -1 0  1  2  3  4
		hist: hist{PosNeg: bins{ø, ø, ø, ø, ø, ø, ø, ø}.Into(), Zt: 0, Zc: 0},
		want: hist{PosNeg: bins{ø, ø, ø, ø, ø, ø, ø, ø}.Into(), Zt: 0, Zc: 0},
	}, {
		// zt=2 is upper boundary of bucket 0. keep buckets [1:n]
		hist: hist{PosNeg: bins{ø, ø, 1, 2, 3, 4, 5, ø}.Into(), Zt: 0, Zc: 2},
		want: hist{PosNeg: bins{ø, ø, ø, ø, 3, 4, 5, ø}.Into(), Zt: 2, Zc: 2 + 2*(1+2)},
	}, {
		// zt=3 is within bucket 1. keep buckets [2:n]
		// set zt=4 because it must cover full buckets
		hist: hist{PosNeg: bins{ø, ø, 1, 2, 3, 4, 5, ø}.Into(), Zt: 0, Zc: 2},
		min:  3,
		want: hist{PosNeg: bins{ø, ø, ø, ø, ø, 4, 5, ø}.Into(), Zt: 4, Zc: 2 + 2*(1+2+3)},
	}, {
		// zt=2 is higher, but no change expected as no buckets in this range are populated
		hist: hist{PosNeg: bins{ø, ø, ø, ø, ø, ø, 1, 1}.Into(), Zt: 1.0, Zc: 2},
		want: hist{PosNeg: bins{ø, ø, ø, ø, ø, ø, 1, 1}.Into(), Zt: 2.0, Zc: 2},
	}, {
		// zt=16 is upper boundary of bucket 3. keep buckets [4:n]
		hist: hist{PosNeg: bins{ø, ø, 1, 2, 3, 4, 5, 6}.Into(), Zt: 0, Zc: 2},
		want: hist{PosNeg: bins{ø, ø, ø, ø, ø, ø, ø, 6}.Into(), Zt: 16, Zc: 2 + 2*(1+2+3+4+5)},
	}, {
		// zt just below 16 is within bucket 3, but Idx may round it up to bucket 4.
		// keep buckets [4:n] and set zt=16
		hist: hist{PosNeg: bins{ø, ø, 1, 2, 3, 4, 5, 6}.Into(), Zt: 0, Zc: 2},
		min:  math.Nextafter(16, 0),
		want: hist{PosNeg: bins{ø, ø, ø, ø, ø, ø, ø, 6}.Into(), Zt: 16, Zc: 2 + 2*(1+2+3+4+5)},
	}, {
		// zt just above 0.125 is within bucket -3, (0.125,0.25]. keep buckets [-2:n]
		// and set zt=0.25
		hist: hist{PosNeg: bins{1, 2, 3, 4, 5, 6, 7, 8}.Into(), Zt: 0, Zc: 2},
		min:  math.Nextafter(0.125, 1),
		want: hist{PosNeg: bins{ø, 2, 3, 4, 5, 6, 7, 8}.Into(), Zt: 0.25, Zc: 2 + 2*1},
	}}

	for _, cs := range cases {
		name := fmt.Sprintf("%.2f->%.2f", cs.hist.Zt, cs.want.Zt)
		t.Run(name, func(t *testing.T) {
			hist := cs.hist.Into()
			want := cs.want.Into()

			zt := cs.min
			if zt == 0 {
				zt = want.ZeroThreshold()
			}
			expo.WidenZero(hist, zt)
			assert.Equal(t, want, hist)
		})
	}

	t.Run("panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "min must be larger than current threshold (1.500000)", func() {
			hist := hist{Zt: 1.5}.Into()
			expo.WidenZero(hist, 0.5)
		})
	})
}

// Near a boundary, Idx can be one bucket off from Bounds due to floating point
// rounding. WidenZero must use the bucket that Bounds places width in, as the
// new threshold is the upper boundary of that bucket.
func TestWidenZeroAtBoundaries(t *testing.T) {
	for scale := expo.Scale(-2); scale <= 8; scale++ {
		for i := -100; i <= 100; i++ {
			_, upper := scale.Bounds(i)
			cases := []struct {
				width float64
				zero  int // the largest bucket index inside the zero bucket
			}{
				{width: upper, zero: i},                                  // the upper boundary of bucket i
				{width: math.Nextafter(upper, math.Inf(1)), zero: i + 1}, // just above it, in bucket i+1
			}

			for _, cs := range cases {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetScale(int32(scale))
				// one observation in each bucket from i-1 to i+2
				dp.Positive().SetOffset(int32(i - 1))
				dp.Positive().BucketCounts().FromRaw([]uint64{1, 1, 1, 1})

				expo.WidenZero(dp, cs.width)

				_, zt := scale.Bounds(cs.zero)
				msg := fmt.Sprintf("scale=%d width=%v", scale, cs.width)
				assert.Equal(t, zt, dp.ZeroThreshold(), msg)
				assert.Equal(t, uint64(cs.zero-i+2), dp.ZeroCount(), msg)
				assert.Equal(t, int32(cs.zero+1), dp.Positive().Offset(), msg)
			}
		}
	}
}

func TestSlice(t *testing.T) {
	cases := []struct {
		bins bins
		want bins
	}{{
		//        -3 -2 -1  0  1  2  3  4
		bins: bins{ø, ø, ø, ø, ø, ø, ø, ø},
		want: bins{ø, ø, ø, ø, ø, ø, ø, ø},
	}, {
		bins: bins{1, 2, 3, 4, 5, 6, 7, 8},
		want: bins{1, 2, 3, 4, 5, 6, 7, 8},
	}, {
		bins: bins{ø, 2, 3, 4, 5, 6, 7, ø},
		want: bins{ø, ø, 3, 4, 5, ø, ø, ø},
	}}

	for _, cs := range cases {
		from, to := 0, len(cs.want)
		for i := range len(cs.want) {
			if cs.want[i] != ø {
				from += i
				break
			}
		}
		for i := from; i < len(cs.want); i++ {
			if cs.want[i] == ø {
				to = i
				break
			}
		}
		from -= 3
		to -= 3

		t.Run(fmt.Sprintf("[%d:%d]", from, to), func(t *testing.T) {
			bins := cs.bins.Into()
			want := cs.want.Into()

			expo.Abs(bins).Slice(from, to)
			assert.Equal(t, want, bins)
		})
	}

	t.Run("panics", func(t *testing.T) {
		data := expo.Abs(bins{1, 2, 3, 4, 5, 6, 7, 8}.Into())
		assert.PanicsWithValue(t, "bad bounds: must be from<=to (got 8<=4)", func() {
			data.Slice(8, 4)
		})
		assert.PanicsWithValue(t, "-6:12 is out of bounds for -3:5", func() {
			data.Slice(-6, 12)
		})
	})
}
