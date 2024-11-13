// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package delta_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
)

var result any

func aggr[P point[P]]() streams.Aggregator[P] {
	return streams.IntoAggregator(delta.New[P]())
}

func BenchmarkAccumulator(b *testing.B) {
	acc := aggr[data.Number]()
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
	acc := aggr[data.Number]()
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
	t.Run("sum", testTimes(random.Sum()))
	t.Run("histogram", testTimes(random.Histogram()))
	t.Run("exponential", testTimes(random.Exponential()))
}

func testTimes[P point[P]](metric random.Metric[P]) func(t *testing.T) {
	return func(t *testing.T) {
		acc := aggr[P]()
		id, base := metric.Stream()
		point := func(start, last pcommon.Timestamp) P {
			dp := base.Clone()
			dp.SetStartTimestamp(start)
			dp.SetTimestamp(last)
			return dp
		}

		// first sample: its the first ever, so take it as-is
		{
			dp := point(1000, 1000)
			res, err := acc.Aggregate(id, dp)

			require.NoError(t, err)
			require.Equal(t, time(1000), res.StartTimestamp())
			require.Equal(t, time(1000), res.Timestamp())
		}

		// second sample: its subsequent, so keep original startTime, but update lastSeen
		{
			dp := point(1000, 1100)
			res, err := acc.Aggregate(id, dp)

			require.NoError(t, err)
			require.Equal(t, time(1000), res.StartTimestamp())
			require.Equal(t, time(1100), res.Timestamp())
		}

		// third sample: its subsequent, but has a more recent startTime, which is
		// PERMITTED by the spec.
		// still keep original startTime, but update lastSeen.
		{
			dp := point(1100, 1200)
			res, err := acc.Aggregate(id, dp)

			require.NoError(t, err)
			require.Equal(t, time(1000), res.StartTimestamp())
			require.Equal(t, time(1200), res.Timestamp())
		}
	}
}

type point[Self any] interface {
	random.Point[Self]

	SetTimestamp(pcommon.Timestamp)
	SetStartTimestamp(pcommon.Timestamp)
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
			acc := aggr[data.Number]()
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

func time(ts int) pcommon.Timestamp {
	return pcommon.Timestamp(ts)
}
