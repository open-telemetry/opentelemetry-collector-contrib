// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor

import (
	"context"
	"fmt"
	"math/rand/v2"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/histo"
)

func BenchmarkProcessor(gb *testing.B) {
	const (
		metrics = 5
		streams = 10
	)

	now := time.Now()
	start := pcommon.NewTimestampFromTime(now)
	ts := pcommon.NewTimestampFromTime(now.Add(time.Minute))

	type Case struct {
		name string
		fill func(m pmetric.Metric)
		next func(m pmetric.Metric)
	}
	cases := []Case{{
		name: "sums",
		fill: func(m pmetric.Metric) {
			sum := m.SetEmptySum()
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			for i := range streams {
				dp := sum.DataPoints().AppendEmpty()
				dp.SetIntValue(int64(rand.IntN(10)))
				dp.Attributes().PutStr("idx", strconv.Itoa(i))
				dp.SetStartTimestamp(start)
				dp.SetTimestamp(ts)
			}
		},
		next: func(m pmetric.Metric) {
			dps := m.Sum().DataPoints()
			for i := range dps.Len() {
				dp := dps.At(i)
				dp.SetStartTimestamp(dp.Timestamp())
				dp.SetTimestamp(pcommon.NewTimestampFromTime(
					dp.Timestamp().AsTime().Add(time.Minute),
				))
			}
		},
	}, {
		name: "histogram",
		fill: func(m pmetric.Metric) {
			hist := m.SetEmptyHistogram()
			hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			for i := range streams {
				dp := hist.DataPoints().AppendEmpty()
				histo.DefaultBounds.Observe(
					float64(rand.IntN(1000)),
					float64(rand.IntN(1000)),
					float64(rand.IntN(1000)),
					float64(rand.IntN(1000)),
				).CopyTo(dp)

				dp.SetStartTimestamp(start)
				dp.SetTimestamp(ts)
				dp.Attributes().PutStr("idx", strconv.Itoa(i))
			}
		},
		next: func(m pmetric.Metric) {
			dps := m.Histogram().DataPoints()
			for i := range dps.Len() {
				dp := dps.At(i)
				dp.SetStartTimestamp(dp.Timestamp())
				dp.SetTimestamp(pcommon.NewTimestampFromTime(
					dp.Timestamp().AsTime().Add(time.Minute),
				))
			}
		},
	}, {
		name: "exponential",
		fill: func(m pmetric.Metric) {
			ex := m.SetEmptyExponentialHistogram()
			ex.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			for i := range streams {
				dp := ex.DataPoints().AppendEmpty()
				o := expotest.Observe(expo.Scale(2),
					float64(rand.IntN(31)+1),
					float64(rand.IntN(31)+1),
					float64(rand.IntN(31)+1),
					float64(rand.IntN(31)+1),
				)
				o.CopyTo(dp.Positive())
				o.CopyTo(dp.Negative())

				dp.SetStartTimestamp(start)
				dp.SetTimestamp(ts)
				dp.Attributes().PutStr("idx", strconv.Itoa(i))
			}
		},
		next: func(m pmetric.Metric) {
			dps := m.ExponentialHistogram().DataPoints()
			for i := range dps.Len() {
				dp := dps.At(i)
				dp.SetStartTimestamp(dp.Timestamp())
				dp.SetTimestamp(pcommon.NewTimestampFromTime(
					dp.Timestamp().AsTime().Add(time.Minute),
				))
			}
		},
	}}

	for _, cs := range cases {
		gb.Run(cs.name, func(b *testing.B) {
			sink := new(consumertest.MetricsSink)
			proc, _ := setup(b, nil, sink)

			md := pmetric.NewMetrics()
			ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			for i := range metrics {
				m := ms.AppendEmpty()
				m.SetName(strconv.Itoa(i))
				cs.fill(m)
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()

			ctx := context.Background()
			for range b.N {
				for i := range ms.Len() {
					cs.next(ms.At(i))
				}
				req := pmetric.NewMetrics()
				md.CopyTo(req)

				b.StartTimer()
				err := proc.ConsumeMetrics(ctx, req)
				b.StopTimer()
				require.NoError(b, err)
			}

			// verify all dps are processed without error
			b.StopTimer()
			require.Equal(b, b.N*metrics*streams, sink.DataPointCount())
		})
	}
}

func Benchmark(b *testing.B) {
	const numMetrics = 64
	const numStreams = 3
	numRoutines := runtime.GOMAXPROCS(0)
	if numRoutines > numMetrics {
		b.Fatal("increase K")
	}

	var metricID atomic.Int64
	metricID.Store(-1)

	var init sync.WaitGroup
	init.Add(numRoutines)
	wait := make(chan struct{})

	start := time.Now()

	sink := new(CountingSink)
	proc, _ := setup(b, nil, sink)

	b.ReportAllocs()
	go func() {
		init.Wait()
		b.ResetTimer()
		close(wait)
	}()

	batchSize := numMetrics / numRoutines
	b.RunParallel(func(pb *testing.PB) {
		md := pmetric.NewMetrics()
		ms := make([]pmetric.Metric, batchSize)
		for i := range ms {
			m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			mid := metricID.Add(1)
			m.SetName(fmt.Sprintf("metric-%d", mid))
			sum := m.SetEmptySum()
			for s := range numStreams {
				dp := sum.DataPoints().AppendEmpty()
				dp.Attributes().PutInt("s", int64(s))
				dp.SetIntValue(rand.Int64N(100))
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
			}
			ms[i] = m
		}
		ctx := context.Background()
		init.Done()

		<-wait
		for n := 0; pb.Next(); n++ {
			for _, m := range ms {
				m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				dps := m.Sum().DataPoints()
				for i := range dps.Len() {
					dp := dps.At(i)
					dp.SetTimestamp(dp.Timestamp() + pcommon.Timestamp(time.Minute.Nanoseconds()))
				}
			}
			if err := proc.ConsumeMetrics(ctx, md); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.StopTimer()

	dps := batchSize * numStreams * b.N
	require.Equal(b, int64(dps), sink.Load())
}

type CountingSink struct {
	atomic.Int64
}

func (cs *CountingSink) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	cs.Add(int64(md.DataPointCount()))
	return nil
}

func (cs *CountingSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}
