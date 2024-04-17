package data

import (
	"math"
	"testing"

	"github.com/matryer/is"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// represents none/absent/unset in several tests
const ø = math.MaxUint64

func TestAdd(t *testing.T) {
	type tcase struct {
		name   string
		dp, in expdp
		want   expdp
	}

	cases := []tcase{{
		name: "noop",
		dp:   expdp{ts: 0, pos: bins{0, 0, 0, 0, 0, 0, 0, 0, 0}, neg: bins{0, 0, 0, 0, 0, 0, 0, 0, 0}, count: 0},
		in:   expdp{ts: 1, pos: bins{0, 0, 0, 0, 0, 0, 0, 0, 0}, neg: bins{0, 0, 0, 0, 0, 0, 0, 0, 0}, count: 0},
		want: expdp{ts: 1, pos: bins{0, 0, 0, 0, 0, 0, 0, 0, 0}, neg: bins{0, 0, 0, 0, 0, 0, 0, 0, 0}, count: 0},
	}, {
		name: "simple",
		dp:   expdp{ts: 0, pos: bins{0, 0, 0, 0, 0, 0, 0, 0, 0}, neg: bins{0, 0, 0, 0, 0, 0, 0, 0, 0}, count: 0},
		in:   expdp{ts: 1, pos: bins{1, 2, 3, 4, 5, 6, 7, 8, 9}, neg: bins{1, 2, 3, 4, 5, 6, 7, 8, 9}, count: 2 * 45},
		want: expdp{ts: 1, pos: bins{1, 2, 3, 4, 5, 6, 7, 8, 9}, neg: bins{1, 2, 3, 4, 5, 6, 7, 8, 9}, count: 2 * (0 + 45)},
	}, {
		name: "lower+shorter",
		dp:   expdp{ts: 0, pos: bins{ø, ø, ø, ø, ø, 1, 1, 1, 1}, neg: bins{ø, ø, ø, ø, ø, 1, 1, 1, 1}, count: 2 * 4},
		in:   expdp{ts: 1, pos: bins{ø, ø, 1, 1, 1, 1, 1, ø, ø}, neg: bins{ø, ø, 1, 1, 1, 1, 1, ø, ø}, count: 2 * 5},
		want: expdp{ts: 1, pos: bins{ø, ø, 1, 1, 1, 2, 2, 1, 1}, neg: bins{ø, ø, 1, 1, 1, 2, 2, 1, 1}, count: 2 * (4 + 5)},
	}, {
		name: "longer",
		dp:   expdp{ts: 0, pos: bins{1, 1, 1, 1, 1, 1, ø, ø, ø}, neg: bins{1, 1, 1, 1, 1, 1, ø, ø, ø}, count: 2 * 6},
		in:   expdp{ts: 1, pos: bins{1, 1, 1, 1, 1, 1, 1, 1, 1}, neg: bins{1, 1, 1, 1, 1, 1, 1, 1, 1}, count: 2 * 9},
		want: expdp{ts: 1, pos: bins{2, 2, 2, 2, 2, 2, 1, 1, 1}, neg: bins{2, 2, 2, 2, 2, 2, 1, 1, 1}, count: 2 * (6 + 9)},
	}, {
		name: "optional/missing-dp",
		dp:   expdp{ts: 0, pos: zeros(5).observe0(0.6, 2.4) /*                                                 */, count: 2},
		in:   expdp{ts: 1, pos: zeros(5).observe0(1.5, 3.2, 6.3), min: some(1.5), max: some(6.3), sum: some(11.0), count: 3},
		want: expdp{ts: 1, pos: zeros(5).observe0(0.6, 2.4, 1.5, 3.2, 6.3) /*                                  */, count: 5},
	}, {
		name: "optional/missing-in",
		dp:   expdp{ts: 0, pos: zeros(5).observe0(1.5, 3.2, 6.3), min: some(1.5), max: some(6.3), sum: some(11.0), count: 3},
		in:   expdp{ts: 1, pos: zeros(5).observe0(0.6, 2.4) /*                                                 */, count: 2},
		want: expdp{ts: 1, pos: zeros(5).observe0(0.6, 2.4, 1.5, 3.2, 6.3) /*                                  */, count: 5},
	}, {
		name: "min-max-sum",
		dp:   expdp{ts: 0, pos: zeros(5).observe0(1.5, 5.3, 11.6) /*          */, min: some(1.5), max: some(11.6), sum: some(18.4), count: 3},
		in:   expdp{ts: 1, pos: zeros(5).observe0(0.6, 3.3, 7.9) /*           */, min: some(0.6), max: some(07.9), sum: some(11.8), count: 3},
		want: expdp{ts: 1, pos: zeros(5).observe0(1.5, 5.3, 11.6, 0.6, 3.3, 7.9), min: some(0.6), max: some(11.6), sum: some(30.2), count: 6},
	}, {
		name: "zero/count",
		dp:   expdp{ts: 0, pos: bins{1, 2}, zc: u64(3), count: 5},
		in:   expdp{ts: 1, pos: bins{1, 0}, zc: u64(2), count: 3},
		want: expdp{ts: 1, pos: bins{2, 2}, zc: u64(5), count: 8},
	}, {
		// (1, 2], (2, 4], (4, 8], (8, 16], (16, 32]
		name: "zero/diff-simple",
		dp:   expdp{ts: 0, pos: bins{1, 1, 1}, zt: some(0.0), zc: u64(2)},
		in:   expdp{ts: 1, pos: bins{ø, 1, 1}, zt: some(2.0), zc: u64(2)},
		want: expdp{ts: 1, pos: bins{ø, 2, 2}, zt: some(2.0), zc: u64(5)},
	}, {
		name: "negative-offset",
		dp:   expdp{ts: 0, posb: some(buckets([]uint64{ /*   */ 1, 2}, -2))},
		in:   expdp{ts: 1, posb: some(buckets([]uint64{1, 2, 3 /* */}, -5))},
		want: expdp{ts: 1, posb: some(buckets([]uint64{1, 2, 3, 1, 2}, -5))},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dp, in := c.dp.into(), c.in.into()
			want := c.want.into()

			got := dp.Add(in)

			is := is.NewRelaxed(t)
			is.Equal(from(expo.Buckets(want.Positive())).String(), from(expo.Buckets(got.Positive())).String())
			if err := pmetrictest.CompareExponentialHistogramDataPoint(want.ExponentialHistogramDataPoint, got.ExponentialHistogramDataPoint); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBucketsIter(t *testing.T) {
	type tcase struct {
		name string
		data expo.Buckets
		want []uint64
	}

	cases := []tcase{{
		name: "full",
		data: bins{1, 2, 3, 4, 5, 6, 7, 8, 9}.into(),
		want: bins{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}, {
		name: "3-6",
		data: bins{ø, ø, ø, 4, 5, 6, ø, ø, ø, ø}.into(),
		want: bins{0, 0, 0, 4, 5, 6},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := make([]uint64, c.data.Len())
			for i := 0; i < c.data.Len(); i++ {
				got[i] = c.data.At(i)
			}
			require.ElementsMatch(t, c.want, got)
			require.Equal(t, len(c.want), len(got))
		})
	}
}

func TestBucketsSet(t *testing.T) {
	var (
		aggr = bins{0, 0, 0, 0, 0, 0, 0, 0, 0}.into()
		bkt0 = bins{ø, 1, 1, 1, 1, ø, ø, ø, ø}.into()
		bkt1 = bins{ø, ø, ø, 1, 1, 1, 1, ø, ø}.into()
		want = bins{0, 1, 1, 2, 2, 1, 1, 0, 0}
	)

	for i := 0; i < aggr.Len(); i++ {
		aggr.SetAt(i, bkt0.At(i)+bkt1.At(i))
	}

	got := make(bins, aggr.Len())
	for i := 0; i < aggr.Len(); i++ {
		got[i] = aggr.At(i)
	}
	require.Equal(t, want, got)
}

func TestBucketsEnsureLen(t *testing.T) {
	var (
		data = bins{ø, ø, 3, 4}
		want = bins{0, 0, 3, 4, 0, 0, 0}
	)
	bkt := data.into()
	bkt.EnsureLen(len(want))

	got := make(bins, bkt.Len())
	for i := 0; i < bkt.Len(); i++ {
		got[i] = bkt.At(i)
	}
	require.Equal(t, want, got)
}

type expdp struct {
	ts int

	pos, neg   bins
	posb, negb *expo.Buckets

	scale int32
	count uint64
	sum   *float64

	min, max *float64

	zc *uint64
	zt *float64
}

func (e expdp) into() ExpHistogram {
	dp := pmetric.NewExponentialHistogramDataPoint()
	dp.SetTimestamp(pcommon.Timestamp(e.ts))

	posb := e.pos.into()
	if e.posb != nil {
		posb = *e.posb
	}
	pmetric.ExponentialHistogramDataPointBuckets(posb).CopyTo(dp.Positive())

	negb := e.neg.into()
	if e.negb != nil {
		negb = *e.negb
	}
	pmetric.ExponentialHistogramDataPointBuckets(negb).CopyTo(dp.Negative())

	dp.SetScale(e.scale)
	dp.SetCount(e.count)

	setnn(e.sum, dp.SetSum)
	setnn(e.min, dp.SetMin)
	setnn(e.max, dp.SetMax)
	setnn(e.zc, dp.SetZeroCount)
	setnn(e.zt, dp.SetZeroThreshold)

	return ExpHistogram{ExponentialHistogramDataPoint: dp}
}

func setnn[T any](ptr *T, set func(T)) {
	if ptr != nil {
		set(*ptr)
	}
}

func some[T any](v T) *T {
	return &v
}

func u64(v uint64) *uint64 {
	return &v
}
