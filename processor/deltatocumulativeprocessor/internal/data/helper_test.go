package data

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type bins []uint64

func (bins bins) into() expo.Buckets {
	off, counts := bins.split()
	return buckets(counts, offset(len(off)))
}

func (bins bins) split() (offset, counts []uint64) {
	start := 0
	for i := 0; i < len(bins); i++ {
		if bins[i] != ø {
			start = i
			break
		}
	}

	end := len(bins)
	for i := start; i < len(bins); i++ {
		if bins[i] == ø {
			end = i
			break
		}
	}

	return bins[0:start], bins[start:end]
}

func (bins bins) String() string {
	var b strings.Builder
	b.WriteString("[")
	for i, v := range bins {
		if i != 0 {
			b.WriteString(", ")
		}
		if v == ø {
			b.WriteString("_")
			continue
		}
		b.WriteString(strconv.FormatUint(v, 10))
	}
	b.WriteString("]")
	return b.String()
}

func from(buckets expo.Buckets) bins {
	counts := pmetric.ExponentialHistogramDataPointBuckets(buckets).BucketCounts()
	off := buckets.Offset()
	if off < 0 {
		off = -off
	}
	bs := make(bins, counts.Len()+off)
	for i := 0; i < off; i++ {
		bs[i] = ø
	}
	for i := 0; i < counts.Len(); i++ {
		bs[i+off] = counts.At(i)
	}
	return bs
}

func TestBucketsHelper(t *testing.T) {
	cases := []struct {
		bins bins
		want expo.Buckets
	}{{
		bins: bins{},
		want: buckets(nil, offset(0)),
	}, {
		bins: bins{1, 2, 3, 4},
		want: buckets([]uint64{1, 2, 3, 4}, offset(0)),
	}, {
		bins: bins{ø, ø, 3, 4},
		want: buckets([]uint64{3, 4}, offset(2)),
	}, {
		bins: bins{ø, ø, 3, 4, ø, ø},
		want: buckets([]uint64{3, 4}, offset(2)),
	}, {
		bins: bins{1, 2, ø, ø},
		want: buckets([]uint64{1, 2}, offset(0)),
	}}

	for _, c := range cases {
		got := c.bins.into()
		require.Equal(t, c.want, got)
	}
}

type offset int

func buckets(data []uint64, offset offset) expo.Buckets {
	if data == nil {
		data = make([]uint64, 0)
	}
	bs := pmetric.NewExponentialHistogramDataPointBuckets()
	bs.BucketCounts().FromRaw(data)
	bs.SetOffset(int32(offset))
	return expo.Buckets(bs)
}

func zeros(size int) bins {
	return make(bins, size)
}

// observe some points with scale 0
func (dps bins) observe0(pts ...float64) bins {
	if len(pts) == 0 {
		return dps
	}

	// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#scale-zero-extract-the-exponent
	idx := func(v float64) int {
		frac, exp := math.Frexp(v)
		if frac == 0.5 {
			exp--
		}
		return exp
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
		want bins
	}{{
		pts:  []float64{1.5, 5.3, 11.6},
		want: bins{0, 1, 0, 1, 1},
	}, {
		pts:  []float64{0.6, 3.3, 7.9},
		want: bins{1, 0, 1, 1, 0},
	}}

	for _, c := range cases {
		got := zeros(len(c.want)).observe0(c.pts...)
		require.Equal(t, c.want, got)
	}
}
