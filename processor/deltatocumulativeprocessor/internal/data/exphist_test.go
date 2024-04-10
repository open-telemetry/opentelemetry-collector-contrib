package data

import (
	"math"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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
		dp:   expdp{ts: 0, pos: buckets{0, 0, 0, 0, 0, 0, 0, 0, 0}, neg: buckets{0, 0, 0, 0, 0, 0, 0, 0, 0}, count: 0},
		in:   expdp{ts: 1, pos: buckets{0, 0, 0, 0, 0, 0, 0, 0, 0}, neg: buckets{0, 0, 0, 0, 0, 0, 0, 0, 0}, count: 0},
		want: expdp{ts: 1, pos: buckets{0, 0, 0, 0, 0, 0, 0, 0, 0}, neg: buckets{0, 0, 0, 0, 0, 0, 0, 0, 0}, count: 0},
	}, {
		name: "simple",
		dp:   expdp{ts: 0, pos: buckets{0, 0, 0, 0, 0, 0, 0, 0, 0}, neg: buckets{0, 0, 0, 0, 0, 0, 0, 0, 0}, count: 0},
		in:   expdp{ts: 1, pos: buckets{1, 2, 3, 4, 5, 6, 7, 8, 9}, neg: buckets{1, 2, 3, 4, 5, 6, 7, 8, 9}, count: 2 * 45},
		want: expdp{ts: 1, pos: buckets{1, 2, 3, 4, 5, 6, 7, 8, 9}, neg: buckets{1, 2, 3, 4, 5, 6, 7, 8, 9}, count: 2 * (0 + 45)},
	}, {
		name: "lower+shorter",
		dp:   expdp{ts: 0, pos: buckets{ø, ø, ø, ø, ø, 1, 1, 1, 1}, neg: buckets{ø, ø, ø, ø, ø, 1, 1, 1, 1}, count: 2 * 4},
		in:   expdp{ts: 1, pos: buckets{ø, ø, 1, 1, 1, 1, 1, ø, ø}, neg: buckets{ø, ø, 1, 1, 1, 1, 1, ø, ø}, count: 2 * 5},
		want: expdp{ts: 1, pos: buckets{ø, ø, 1, 1, 1, 2, 2, 1, 1}, neg: buckets{ø, ø, 1, 1, 1, 2, 2, 1, 1}, count: 2 * (4 + 5)},
	}, {
		name: "longer",
		dp:   expdp{ts: 0, pos: buckets{1, 1, 1, 1, 1, 1, ø, ø, ø}, neg: buckets{1, 1, 1, 1, 1, 1, ø, ø, ø}, count: 2 * 6},
		in:   expdp{ts: 1, pos: buckets{1, 1, 1, 1, 1, 1, 1, 1, 1}, neg: buckets{1, 1, 1, 1, 1, 1, 1, 1, 1}, count: 2 * 9},
		want: expdp{ts: 1, pos: buckets{2, 2, 2, 2, 2, 2, 1, 1, 1}, neg: buckets{2, 2, 2, 2, 2, 2, 1, 1, 1}, count: 2 * (6 + 9)},
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
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dp, in := c.dp.into(), c.in.into()
			want := c.want.into()

			got := dp.Add(in)
			if err := pmetrictest.CompareExponentialHistogramDataPoint(want.ExponentialHistogramDataPoint, got.ExponentialHistogramDataPoint); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBucketsIter(t *testing.T) {
	type tcase struct {
		name string
		data Buckets
		want []uint64
	}

	cases := []tcase{{
		name: "full",
		data: buckets{1, 2, 3, 4, 5, 6, 7, 8, 9}.into(),
		want: buckets{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}, {
		name: "3-6",
		data: buckets{ø, ø, ø, 4, 5, 6, ø, ø, ø, ø}.into(),
		want: buckets{0, 0, 0, 4, 5, 6},
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
		aggr = buckets{0, 0, 0, 0, 0, 0, 0, 0, 0}.into()
		bkt0 = buckets{ø, 1, 1, 1, 1, ø, ø, ø, ø}.into()
		bkt1 = buckets{ø, ø, ø, 1, 1, 1, 1, ø, ø}.into()
		want = buckets{0, 1, 1, 2, 2, 1, 1, 0, 0}
	)

	for i := 0; i < aggr.Len(); i++ {
		aggr.SetAt(i, bkt0.At(i)+bkt1.At(i))
	}

	got := make(buckets, aggr.Len())
	for i := 0; i < aggr.Len(); i++ {
		got[i] = aggr.At(i)
	}
	require.Equal(t, want, got)
}

func TestBucketsEnsureLen(t *testing.T) {
	var (
		data = buckets{ø, ø, 3, 4}
		want = buckets{0, 0, 3, 4, 0, 0, 0}
	)
	bkt := data.into()
	bkt.EnsureLen(len(want))

	got := make(buckets, bkt.Len())
	for i := 0; i < bkt.Len(); i++ {
		got[i] = bkt.At(i)
	}
	require.Equal(t, want, got)
}

type expdp struct {
	ts  int
	pos buckets
	neg buckets

	scale int32
	count uint64
	sum   *float64

	min, max *float64
}

func (e expdp) into() ExpHistogram {
	dp := pmetric.NewExponentialHistogramDataPoint()
	dp.SetTimestamp(pcommon.Timestamp(e.ts))

	pos := e.pos.into()
	pos.data.CopyTo(dp.Positive().BucketCounts())
	dp.Positive().SetOffset(int32(pos.offset))

	neg := e.neg.into()
	neg.data.CopyTo(dp.Negative().BucketCounts())
	dp.Negative().SetOffset(int32(neg.offset))

	dp.SetScale(e.scale)
	dp.SetCount(e.count)
	if e.sum != nil {
		dp.SetSum(*e.sum)
	}
	if e.min != nil {
		dp.SetMin(*e.min)
	}
	if e.max != nil {
		dp.SetMax(*e.max)
	}

	return ExpHistogram{ExponentialHistogramDataPoint: dp}
}

func some[T any](v T) *T {
	return &v
}
