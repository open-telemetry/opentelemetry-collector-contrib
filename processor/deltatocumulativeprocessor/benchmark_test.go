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
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/histo"
)

var out *consumertest.MetricsSink

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
			st := setup(b, nil)
			out = st.sink

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
				err := st.proc.ConsumeMetrics(ctx, req)
				b.StopTimer()
				require.NoError(b, err)
			}

			// verify all dps are processed without error
			b.StopTimer()
			require.Equal(b, b.N*metrics*streams, st.sink.DataPointCount())
		})
	}
}
