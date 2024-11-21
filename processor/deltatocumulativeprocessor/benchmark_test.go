// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor

import (
	"context"
	"math/rand/v2"
	"strconv"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/sdktest"
)

var out consumertest.MetricsSink

func BenchmarkProcessor(gb *testing.B) {
	const (
		metrics = 5
		streams = 10
	)

	type Case struct {
		name string
		fill func(m pmetric.Metric)
		next func(m pmetric.Metric)
	}

	run := func(b *testing.B, proc consumer.Metrics, cs Case) {
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
	}

	now := time.Now()
	start := pcommon.NewTimestampFromTime(now)
	ts := pcommon.NewTimestampFromTime(now.Add(time.Minute))

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
		next: next(pmetric.Metric.Sum),
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
		next: next(pmetric.Metric.Histogram),
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
		next: next(pmetric.Metric.ExponentialHistogram),
	}}

	tel := func(n int) sdktest.Spec {
		total := int64(n * metrics * streams)
		tracked := int64(metrics * streams)
		return sdktest.Expect(map[string]sdktest.Metric{
			"otelcol_deltatocumulative.datapoints.linear": {
				Type:      sdktest.TypeSum,
				Numbers:   []sdktest.Number{{Int: &total}},
				Monotonic: true,
			},
			"otelcol_deltatocumulative.streams.tracked.linear": {
				Type:    sdktest.TypeSum,
				Numbers: []sdktest.Number{{Int: &tracked}},
			},
		})
	}

	for _, cs := range cases {
		gb.Run(cs.name, func(b *testing.B) {
			st := setup(b, nil)
			run(b, st.proc, cs)

			// verify all dps are processed without error
			b.StopTimer()
			if err := sdktest.Test(tel(b.N), st.tel.reader); err != nil {
				b.Fatal(err)
			}
		})
	}
}

func next[
	T interface{ DataPoints() Ps },
	Ps interface {
		At(int) P
		Len() int
	},
	P interface {
		Timestamp() pcommon.Timestamp
		SetStartTimestamp(pcommon.Timestamp)
		SetTimestamp(pcommon.Timestamp)
	},
](sel func(pmetric.Metric) T) func(m pmetric.Metric) {
	return func(m pmetric.Metric) {
		dps := sel(m).DataPoints()
		for i := range dps.Len() {
			dp := dps.At(i)
			dp.SetStartTimestamp(dp.Timestamp())
			dp.SetTimestamp(pcommon.NewTimestampFromTime(
				dp.Timestamp().AsTime().Add(time.Minute),
			))
		}
	}
}
