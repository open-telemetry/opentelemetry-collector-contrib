package data

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestAdd(t *testing.T) {
	type tcase struct {
		name   string
		dp, in exphist
		want   exphist
	}

	cases := []tcase{{
		name: "noop",
		dp:   exphist{ts: 0, bkt: buckets([]uint64{0, 0, 0, 0, 0, 0, 0, 0, 0})},
		in:   exphist{ts: 1, bkt: buckets([]uint64{0, 0, 0, 0, 0, 0, 0, 0, 0})},
		want: exphist{ts: 1, bkt: buckets([]uint64{0, 0, 0, 0, 0, 0, 0, 0, 0})},
	}, {
		name: "simple",
		dp:   exphist{ts: 0, bkt: buckets([]uint64{0, 0, 0, 0, 0, 0, 0, 0, 0})},
		in:   exphist{ts: 1, bkt: buckets([]uint64{1, 2, 3, 4, 5, 6, 7, 8, 9})},
		want: exphist{ts: 1, bkt: buckets([]uint64{1, 2, 3, 4, 5, 6, 7, 8, 9})},
	}, {
		name: "lower+shorter",
		dp:   exphist{ts: 0, bkt: buckets([]uint64{0, 0, 0, 0, 0, 1, 1, 1, 1}, 5)},
		in:   exphist{ts: 1, bkt: buckets([]uint64{0, 0, 1, 1, 1, 1, 1}, 2)},
		want: exphist{ts: 1, bkt: buckets([]uint64{0, 0, 1, 1, 1, 2, 2, 1, 1}, 2)},
	}, {
		name: "longer",
		dp:   exphist{ts: 0, bkt: buckets([]uint64{1, 1, 1, 1, 1, 1})},
		in:   exphist{ts: 1, bkt: buckets([]uint64{1, 1, 1, 1, 1, 1, 1, 1, 1})},
		want: exphist{ts: 1, bkt: buckets([]uint64{2, 2, 2, 2, 2, 2, 1, 1, 1})},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dp, in := c.dp.Into(), c.in.Into()
			want := c.want.Into()

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

	var data = []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	cases := []tcase{{
		name: "full",
		data: buckets(data, 0, len(data)),
		want: []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}, {
		name: "3-6",
		data: buckets(data, 3, 6),
		want: []uint64{0, 0, 0, 4, 5, 6},
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
		ones = []uint64{1, 1, 1, 1, 1, 1, 1, 1, 1}
		zero = []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0}
		want = []uint64{0, 1, 1, 2, 2, 1, 1, 0, 0}
	)

	var (
		aggr = buckets(zero, 0, len(zero))
		bkt0 = buckets(ones, 1, 5)
		bkt1 = buckets(ones, 3, 7)
	)

	for i := 0; i < aggr.Len(); i++ {
		aggr.SetAt(i, bkt0.At(i)+bkt1.At(i))
	}

	got := make([]uint64, aggr.Len())
	for i := 0; i < aggr.Len(); i++ {
		got[i] = aggr.At(i)
	}
	require.Equal(t, want, got)
}

func TestBucketsEnsureLen(t *testing.T) {
	var (
		data = []uint64{1, 2, 3, 4}
		want = []uint64{0, 0, 3, 4, 0, 0, 0}
	)
	bkt := buckets(data, 2, len(data))
	bkt.EnsureLen(len(want))

	got := make([]uint64, bkt.Len())
	for i := 0; i < bkt.Len(); i++ {
		got[i] = bkt.At(i)
	}
	require.Equal(t, want, got)
}

func buckets(counts []uint64, bounds ...int) Buckets {
	from, to := 0, len(counts)
	if len(bounds) > 0 {
		from = bounds[0]
	}
	if len(bounds) > 1 {
		to = bounds[1]
	}

	data := pcommon.NewUInt64Slice()
	data.FromRaw(counts[from:to])
	return Buckets{data: data, offset: from}
}

type exphist struct {
	ts  int
	bkt Buckets
}

func (e exphist) Into() ExpHistogram {
	dp := pmetric.NewExponentialHistogramDataPoint()
	dp.SetTimestamp(pcommon.Timestamp(e.ts))

	e.bkt.data.CopyTo(dp.Positive().BucketCounts())
	dp.Positive().SetOffset(int32(e.bkt.offset))
	e.bkt.data.CopyTo(dp.Negative().BucketCounts())
	dp.Negative().SetOffset(int32(e.bkt.offset))

	var (
		count uint64
		sum   float64
	)
	pos := dp.Positive().BucketCounts()
	for i := 0; i < pos.Len(); i++ {
		at := pos.At(i)
		if at != 0 {
			count++
			sum += float64(pos.At(i))
		}
	}
	dp.SetCount(count)
	dp.SetSum(sum)

	return ExpHistogram{ExponentialHistogramDataPoint: dp}
}
