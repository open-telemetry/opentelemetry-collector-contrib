// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"math"
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"
)

// represents none/absent/unset in several tests
const ø = math.MaxUint64

func TestExpoAdd(t *testing.T) {
	type expdp = expotest.Histogram
	type bins = expotest.Bins
	obs0 := expotest.Observe0

	prevMaxBuckets := maxBuckets
	maxBuckets = 8
	defer func() { maxBuckets = prevMaxBuckets }()

	cases := []struct {
		name   string
		dp, in expdp
		want   expdp
		flip   bool
	}{{
		name: "noop",
		dp:   expdp{PosNeg: bins{0, 0, 0, 0, 0, 0, 0, 0}.Into(), Count: 0},
		in:   expdp{PosNeg: bins{0, 0, 0, 0, 0, 0, 0, 0}.Into(), Count: 0},
		want: expdp{PosNeg: rawbs(nil, 5), Count: 0},
	}, {
		name: "simple",
		dp:   expdp{PosNeg: bins{0, 0, 0, 0, 0, 0, 0, 0}.Into(), Count: 0},
		in:   expdp{PosNeg: bins{1, 2, 3, 4, 5, 6, 7, 8}.Into(), Count: 2 * (1 + 2 + 3 + 4 + 5 + 6 + 7 + 8)},
		want: expdp{PosNeg: bins{1, 2, 3, 4, 5, 6, 7, 8}.Into(), Count: 2 * (0 + (1 + 2 + 3 + 4 + 5 + 6 + 7 + 8))},
	}, {
		name: "lower+shorter",
		dp:   expdp{PosNeg: bins{ø, ø, ø, ø, ø, 1, 1, 1}.Into(), Count: 2 * 3},
		in:   expdp{PosNeg: bins{ø, ø, 1, 1, 1, 1, 1, ø}.Into(), Count: 2 * 5},
		want: expdp{PosNeg: bins{ø, ø, 1, 1, 1, 2, 2, 1}.Into(), Count: 2 * (3 + 5)},
	}, {
		name: "longer",
		dp:   expdp{PosNeg: bins{1, 1, 1, 1, 1, ø, ø, ø}.Into(), Count: 2 * 5},
		in:   expdp{PosNeg: bins{1, 1, 1, 1, 1, 1, 1, 1}.Into(), Count: 2 * 8},
		want: expdp{PosNeg: bins{2, 2, 2, 2, 2, 1, 1, 1}.Into(), Count: 2 * (5 + 8)},
	}, {
		name: "optional/missing", flip: true,
		dp:   expdp{PosNeg: obs0(0.6, 2.4) /*                                                 */, Count: 2},
		in:   expdp{PosNeg: obs0(1.5, 3.2, 6.3), Min: some(1.5), Max: some(6.3), Sum: some(11.0), Count: 3},
		want: expdp{PosNeg: obs0(0.6, 2.4, 1.5, 3.2, 6.3) /*                                  */, Count: 5},
	}, {
		name: "optional/min-max-sum",
		dp:   expdp{PosNeg: obs0(1.5, 5.3, 11.6) /*          */, Min: some(1.5), Max: some(11.6), Sum: some(18.4), Count: 3},
		in:   expdp{PosNeg: obs0(0.6, 3.3, 7.9) /*           */, Min: some(0.6), Max: some(07.9), Sum: some(11.8), Count: 3},
		want: expdp{PosNeg: obs0(1.5, 5.3, 11.6, 0.6, 3.3, 7.9), Min: some(0.6), Max: some(11.6), Sum: some(30.2), Count: 6},
	}, {
		name: "zero/count",
		dp:   expdp{PosNeg: bins{0, 1, 2}.Into(), Zt: 0, Zc: 3, Count: 5},
		in:   expdp{PosNeg: bins{0, 1, 0}.Into(), Zt: 0, Zc: 2, Count: 3},
		want: expdp{PosNeg: bins{ø, 2, 2, ø, ø, ø, ø, ø}.Into(), Zt: 0, Zc: 5, Count: 8},
	}, {
		name: "zero/diff",
		dp:   expdp{PosNeg: bins{ø, ø, 0, 1, 1, 1}.Into(), Zt: 0.0, Zc: 2},
		in:   expdp{PosNeg: bins{ø, ø, ø, ø, 1, 1}.Into(), Zt: 2.0, Zc: 2},
		want: expdp{PosNeg: bins{ø, ø, ø, ø, 2, 2, ø, ø}.Into(), Zt: 2.0, Zc: 4 + 2*1},
	}, {
		name: "zero/subzero",
		dp:   expdp{PosNeg: bins{ø, 1, 1, 1, 1, 1}.Into(), Zt: 0.2, Zc: 2},
		in:   expdp{PosNeg: bins{ø, ø, 1, 1, 1, 1}.Into(), Zt: 0.3, Zc: 2},
		want: expdp{PosNeg: bins{ø, ø, 2, 2, 2, 2, ø, ø}.Into(), Zt: 0.5, Zc: 4 + 2*1},
	}, {
		name: "negative-offset",
		dp:   expdp{PosNeg: rawbs([]uint64{ /*   */ 1, 2}, -2)},
		in:   expdp{PosNeg: rawbs([]uint64{1, 2, 3 /* */}, -5)},
		want: expdp{PosNeg: rawbs([]uint64{1, 2, 3, 1, 2}, -5)},
	}, {
		name: "scale/diff",
		dp:   expdp{PosNeg: expotest.Observe(expo.Scale(1), 1, 2, 3, 4), Scale: 1},
		in:   expdp{PosNeg: expotest.Observe(expo.Scale(0), 4, 3, 2, 1), Scale: 0},
		want: expdp{Scale: 0, PosNeg: func() expo.Buckets {
			bs := pmetric.NewExponentialHistogramDataPointBuckets()
			expotest.ObserveInto(bs, expo.Scale(0), 1, 2, 3, 4)
			expotest.ObserveInto(bs, expo.Scale(0), 4, 3, 2, 1)
			return bs
		}()},
	}, {
		name: "scale/no_downscale_within_limit",
		dp: expdp{
			Scale:  0,
			PosNeg: bins{1, 1, 1, 1, 1, 1, 1, 1}.Into(),
			Count:  8,
		},
		in: expdp{
			Scale:  0,
			PosNeg: bins{2, 2, 2, 2, 2, 2, 2, 2}.Into(),
			Count:  16,
		},
		want: expdp{
			Scale:  0,
			PosNeg: bins{3, 3, 3, 3, 3, 3, 3, 3}.Into(),
			Count:  24,
		},
	}, {
		name: "scale/downscale_once_exceeds_limit",
		dp: expdp{
			Scale:  0,
			PosNeg: rawbs([]uint64{1, 1, 1, 1, 1, 1, 1, 1}, 0),
			Count:  8,
		},
		in: expdp{
			Scale:  0,
			PosNeg: rawbs([]uint64{2, 2, 2, 2, 2, 2, 2, 2}, 6),
			Count:  16,
		},
		want: expdp{
			Scale:  -1,
			PosNeg: rawbs([]uint64{2, 2, 2, 6, 4, 4, 4}, 0),
			Count:  24,
		},
	}, {
		name: "scale/downscale_multiple_times_until_within_limit",
		dp: expdp{
			Scale:  0,
			PosNeg: rawbs([]uint64{1, 1, 1, 1, 1, 1, 1, 1}, -6),
			Count:  8,
		},
		in: expdp{
			Scale:  0,
			PosNeg: rawbs([]uint64{2, 2, 2, 2, 2, 2, 2, 2}, 6),
			Count:  16,
		},
		want: expdp{
			Scale:  -2,
			PosNeg: rawbs([]uint64{2, 4, 2, 4, 8, 4}, -2),
			Count:  24,
		},
	}, {
		name: "scale/ignore_leading_trailing_zeros_in_bucket_count",
		dp: expdp{
			Scale:  0,
			PosNeg: rawbs([]uint64{0, 0, 1, 5, 5, 1, 0, 0}, -2),
			Count:  12,
		},
		in: expdp{
			Scale:  0,
			PosNeg: rawbs([]uint64{0, 2, 2, 3, 3, 2, 2, 0}, 0),
			Count:  14,
		},
		want: expdp{
			Scale:  0,
			PosNeg: rawbs([]uint64{1, 7, 7, 4, 3, 2, 2}, 0),
			Count:  26,
		},
	}, {
		name: "scale/downscale_with_leading_trailing_zeros",
		dp: expdp{
			Scale:  0,
			PosNeg: rawbs([]uint64{0, 0, 1, 10, 10, 1, 0, 0}, -4),
			Count:  22,
		},
		in: expdp{
			Scale:  0,
			PosNeg: rawbs([]uint64{0, 0, 2, 10, 10, 2, 0, 0}, 4),
			Count:  24,
		},
		want: expdp{
			Scale:  -1,
			PosNeg: rawbs([]uint64{11, 11, 0, 0, 12, 12}, -1),
			Count:  46,
		},
	}}

	for _, cs := range cases {
		run := func(dp, in expdp) func(t *testing.T) {
			return func(t *testing.T) {
				is := datatest.New(t)

				var (
					dp   = ExpHistogram{dp.Into()}
					in   = ExpHistogram{in.Into()}
					want = ExpHistogram{cs.want.Into()}
				)

				dp.SetTimestamp(0)
				in.SetTimestamp(1)
				want.SetTimestamp(1)

				got := dp.Add(in)
				is.Equal(want.DataPoint, got.DataPoint)
			}
		}

		if cs.flip {
			t.Run(cs.name+"-dp", run(cs.dp, cs.in))
			t.Run(cs.name+"-in", run(cs.in, cs.dp))
			continue
		}
		t.Run(cs.name, run(cs.dp, cs.in))
	}
}

func rawbs(data []uint64, offset int32) expo.Buckets {
	bs := pmetric.NewExponentialHistogramDataPointBuckets()
	bs.BucketCounts().FromRaw(data)
	bs.SetOffset(offset)
	return bs
}

func some[T any](v T) *T {
	return &v
}
