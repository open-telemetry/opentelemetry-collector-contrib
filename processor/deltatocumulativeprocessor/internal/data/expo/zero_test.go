// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest"
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

			is := datatest.New(t)
			is.Equal(want, hist)
		})
	}

	t.Run("panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "min must be larger than current threshold (1.500000)", func() {
			hist := hist{Zt: 1.5}.Into()
			expo.WidenZero(hist, 0.5)
		})
	})
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
		for i := 0; i < len(cs.want); i++ {
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

			is := datatest.New(t)
			is.Equal(want, bins)
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
