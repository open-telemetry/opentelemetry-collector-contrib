package delta_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

var result any = nil

func BenchmarkAccumulator(b *testing.B) {
	acc := delta.Numbers()
	sum := random.Sum()

	bench := func(b *testing.B, nstreams int) {
		nsamples := b.N / nstreams

		ids := make([]streams.Ident, nstreams)
		dps := make([]data.Number, nstreams)
		for i := 0; i < nstreams; i++ {
			ids[i], dps[i] = sum.Stream()
		}

		b.ResetTimer()

		var wg sync.WaitGroup
		for i := 0; i < nstreams; i++ {
			wg.Add(1)
			go func(id streams.Ident, num data.Number) {
				for n := 0; n < nsamples; n++ {
					num.SetTimestamp(num.Timestamp() + 1)
					val, err := acc.Aggregate(id, num)
					if err != nil {
						panic(err)
					}
					result = val
				}
				wg.Done()
			}(ids[i], dps[i])
		}

		wg.Wait()
	}

	nstreams := []int{1, 2, 10, 100, 1000}
	for _, n := range nstreams {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			bench(b, n)
		})
	}
}

// verify the distinction between streams and the accumulated value
func TestAddition(t *testing.T) {
	acc := delta.Numbers()
	sum := random.Sum()

	type Idx int
	type Stream struct {
		idx Idx
		id  streams.Ident
		dp  data.Number
	}

	streams := make([]Stream, 10)
	for i := range streams {
		id, dp := sum.Stream()
		streams[i] = Stream{
			idx: Idx(i),
			id:  id,
			dp:  dp,
		}
	}

	want := make(map[Idx]int64)
	for i := 0; i < 100; i++ {
		stream := streams[rand.Intn(10)]
		dp := stream.dp.Clone()
		dp.SetTimestamp(dp.Timestamp() + pcommon.Timestamp(i))

		val := int64(rand.Intn(255))
		dp.SetIntValue(val)
		want[stream.idx] += val

		got, err := acc.Aggregate(stream.id, dp)
		require.NoError(t, err)

		require.Equal(t, want[stream.idx], got.IntValue())
	}
}

// verify that start + last times are updated
func TestTimes(t *testing.T) {
	acc := delta.Numbers()
	id, data := random.Sum().Stream()

	start := pcommon.Timestamp(1234)
	last := start
	sum := int64(0)
	for i := 0; i < 10; i++ {
		last += 1
		dp := data.Clone()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(last)

		v := int64(rand.Intn(255))
		sum += v
		dp.SetIntValue(v)

		res, err := acc.Aggregate(id, dp)
		require.NoError(t, err)

		// spec: Upon receiving the first Delta point for a given counter we set up the following:
		// A new counter which stores the cumulative sum, set to the initial counter.
		// A start time that aligns with the start time of the first point.
		// A “last seen” time that aligns with the time of the first point.
		require.Equal(t, start, res.StartTimestamp())
		require.Equal(t, last, res.Timestamp())
		require.Equal(t, sum, res.IntValue())
	}
}

func TestErrs(t *testing.T) {
	type Point struct {
		Start int
		Time  int
		Value int
	}
	type Case struct {
		Good Point
		Bad  Point
		Err  error
	}

	cases := []Case{
		{
			Good: Point{Start: 1234, Time: 1337, Value: 42},
			Bad:  Point{Start: 1000, Time: 2000, Value: 24},
			Err:  delta.ErrOlderStart{Start: time(1234), Sample: time(1000)},
		},
		{
			Good: Point{Start: 1234, Time: 1337, Value: 42},
			Bad:  Point{Start: 1234, Time: 1336, Value: 24},
			Err:  delta.ErrOutOfOrder{Last: time(1337), Sample: time(1336)},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%T", c.Err), func(t *testing.T) {
			acc := delta.Numbers()
			id, data := random.Sum().Stream()

			good := data.Clone()
			good.SetStartTimestamp(pcommon.Timestamp(c.Good.Start))
			good.SetTimestamp(pcommon.Timestamp(c.Good.Time))
			good.SetIntValue(int64(c.Good.Value))

			r1, err := acc.Aggregate(id, good)
			require.NoError(t, err)

			require.Equal(t, good.StartTimestamp(), r1.StartTimestamp())
			require.Equal(t, good.Timestamp(), r1.Timestamp())
			require.Equal(t, good.IntValue(), r1.IntValue())

			bad := data.Clone()
			bad.SetStartTimestamp(pcommon.Timestamp(c.Bad.Start))
			bad.SetTimestamp(pcommon.Timestamp(c.Bad.Time))
			bad.SetIntValue(int64(c.Bad.Value))

			r2, err := acc.Aggregate(id, bad)
			require.ErrorIs(t, err, c.Err)

			// sample must be dropped => no change
			require.Equal(t, r1.StartTimestamp(), r2.StartTimestamp())
			require.Equal(t, r1.Timestamp(), r2.Timestamp())
			require.Equal(t, r1.IntValue(), r2.IntValue())
		})
	}

}

func randv() int64 {
	return int64(rand.Intn(255))
}

func time(ts int) pcommon.Timestamp {
	return pcommon.Timestamp(ts)
}
