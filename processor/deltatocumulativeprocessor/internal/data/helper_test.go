package data

import (
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type buckets []uint64

func (b buckets) into() Buckets {
	start := 0
	for i := 0; i < len(b); i++ {
		if b[i] != ø {
			start = i
			break
		}
	}

	end := len(b)
	for i := start; i < len(b); i++ {
		if b[i] == ø {
			end = i
			break
		}
	}

	data := pcommon.NewUInt64Slice()
	data.FromRaw([]uint64(b[start:end]))
	return Buckets{
		data:   data,
		offset: start,
	}
}

func TestBucketsHelper(t *testing.T) {
	uints := func(data ...uint64) pcommon.UInt64Slice {
		us := pcommon.NewUInt64Slice()
		us.FromRaw(data)
		return us
	}

	cases := []struct {
		bkts buckets
		want Buckets
	}{{
		bkts: buckets{},
		want: Buckets{data: uints(), offset: 0},
	}, {
		bkts: buckets{1, 2, 3, 4},
		want: Buckets{data: uints(1, 2, 3, 4), offset: 0},
	}, {
		bkts: buckets{ø, ø, 3, 4},
		want: Buckets{data: uints(3, 4), offset: 2},
	}, {
		bkts: buckets{ø, ø, 3, 4, ø, ø},
		want: Buckets{data: uints(3, 4), offset: 2},
	}, {
		bkts: buckets{1, 2, ø, ø},
		want: Buckets{data: uints(1, 2), offset: 0},
	}}

	for _, c := range cases {
		got := c.bkts.into()
		require.Equal(t, c.want, got)
	}
}

func zeros(size int) buckets {
	return make(buckets, size)
}

// observe some points with scale 0
func (dps buckets) observe0(pts ...float64) buckets {
	if len(pts) == 0 {
		return dps
	}

	idx := func(v float64) int {
		return int(math.Ceil(math.Log2(v)))
	}

	sort.Float64s(pts)
	min := idx(pts[0])
	max := idx(pts[len(pts)-1])
	for i := min; i <= max; i++ {
		if dps[i] == ø {
			dps[i] = 0
		}
	}

	for _, pt := range pts {
		dps[idx(pt)] += 1
	}
	return dps
}

func TestObserve0(t *testing.T) {
	cases := []struct {
		pts  []float64
		want buckets
	}{{
		pts:  []float64{1.5, 5.3, 11.6},
		want: buckets{0, 1, 0, 1, 1},
	}, {
		pts:  []float64{0.6, 3.3, 7.9},
		want: buckets{1, 0, 1, 1, 0},
	}}

	for _, c := range cases {
		got := zeros(len(c.want)).observe0(c.pts...)
		require.Equal(t, c.want, got)
	}
}
