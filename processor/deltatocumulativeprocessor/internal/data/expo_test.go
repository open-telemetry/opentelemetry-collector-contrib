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
		dp:   expdp{Scale: 0, PosNeg: generateBins(10, 80, 1), Count: 160},
		in:   expdp{Scale: 0, PosNeg: generateBins(80+10, 60, 2), Count: 120},
		want: expdp{Scale: 0, PosNeg: generateBins(10, 80, 1, 60, 2), Count: 280},
	}, {
		name: "scale/downscale_once_exceeds_limit",
		dp:   expdp{Scale: 0, PosNeg: generateBins(0, 100, 1), Count: 200},
		in:   expdp{Scale: 0, PosNeg: generateBins(80, 100, 2), Count: 200},
		want: expdp{Scale: -1, PosNeg: downscaled(generateBins(0, 80, 1, 20, 3, 80, 2), 1), Count: 400},
	}, {
		name: "scale/downscale_multiple_times_until_within_limit",
		dp:   expdp{Scale: 0, PosNeg: generateBins(0, 200, 1), Count: 400},
		in:   expdp{Scale: 0, PosNeg: generateBins(180, 200, 2), Count: 400},
		want: expdp{Scale: -2, PosNeg: downscaled(generateBins(0, 180, 1, 20, 3, 180, 2), 2), Count: 800},
	}, {
		name: "scale/ignore_leading_trailing_zeros_in_bucket_count",
		dp:   expdp{Scale: 0, PosNeg: generateBins(0, 10, 0, 150, 1, 10, 0), Count: 170},
		in:   expdp{Scale: 0, PosNeg: generateBins(0, 20, 0, 120, 2, 20, 0), Count: 140},
		want: expdp{Scale: 0, PosNeg: generateBins(10, 10, 1, 120, 3, 20, 1), Count: 310},
	}, {
		name: "scale/downscale_with_leading_trailing_zeros",
		dp:   expdp{Scale: 0, PosNeg: generateBins(0, 5, 0, 180, 1, 5, 0), Count: 190},
		in:   expdp{Scale: 0, PosNeg: generateBins(0, 10, 0, 180, 2, 10, 0), Count: 200},
		want: expdp{Scale: -1, PosNeg: downscaled(generateBins(5, 5, 1, 175, 3, 5, 2), 1), Count: 390},
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

// generateBins creates a bins slice by concatenating multiple slices with specified lengths and values.
func generateBins(offset int, pairs ...int) expo.Buckets {
	buckets := pmetric.NewExponentialHistogramDataPointBuckets()
	buckets.SetOffset(int32(offset))
	expectedLength := 0
	for i := 0; i < len(pairs); i += 2 {
		expectedLength += pairs[i]
	}
	counts := make([]uint64, 0, expectedLength)
	for i := 0; i < len(pairs); i += 2 {
		length, value := pairs[i], pairs[i+1]
		for j := 0; j < length; j++ {
			counts = append(counts, uint64(value))
		}
	}
	buckets.BucketCounts().FromRaw(counts)
	return buckets
}

func downscaled(in expo.Buckets, delta int) expo.Buckets {
	out := pmetric.NewExponentialHistogramDataPointBuckets()
	inOffset := in.Offset()
	outOffset := inOffset >> delta
	out.SetOffset(outOffset)
	counts := make([]uint64, 0)
	for i := 0; i < in.BucketCounts().Len(); i++ {
		j := int(((inOffset + int32(i)) >> delta) - outOffset)
		for len(counts) <= j {
			counts = append(counts, 0)
		}
		counts[j] += in.BucketCounts().At(i)
	}
	out.BucketCounts().FromRaw(counts)
	return out
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
