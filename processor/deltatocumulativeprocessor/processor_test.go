// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor_test

import (
	"context"
	"math"
	"math/rand"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	self "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest/compare"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
)

func setup(t *testing.T, cfg *self.Config) (processor.Metrics, *consumertest.MetricsSink) {
	t.Helper()

	next := &consumertest.MetricsSink{}
	if cfg == nil {
		cfg = &self.Config{MaxStale: 0, MaxStreams: math.MaxInt}
	}

	proc, err := self.NewFactory().CreateMetricsProcessor(
		context.Background(),
		processortest.NewNopSettings(),
		cfg,
		next,
	)
	require.NoError(t, err)

	return proc, next
}

// TestAccumulation verifies stream identification works correctly by writing
// 100 random dps spread across 10 different streams.
// Processor output is compared against a manual aggregation on a per-stream basis.
//
// Uses Sum datatype for testing, as we are not testing actual aggregation (see
// internal/data for tests), but proper stream separation
func TestAccumulation(t *testing.T) {
	proc, sink := setup(t, nil)

	sum := random.Sum()

	// create 10 distinct streams
	const N = 10
	sbs := make([]SumBuilder, N)
	for i := range sbs {
		_, base := sum.Stream()
		sbs[i] = SumBuilder{Metric: sum, base: base}
	}

	// init manual aggregation state
	want := make(map[identity.Stream]data.Number)
	for _, s := range sbs {
		id := s.id(pmetric.AggregationTemporalityCumulative)
		want[id] = s.point(0, 0, 0)
	}

	for i := 0; i < 100; i++ {
		s := sbs[rand.Intn(N)]

		v := int64(rand.Intn(255))
		ts := pcommon.Timestamp(i)

		// write to processor
		in := s.delta(s.point(0, ts, v))
		rms := s.resourceMetrics(in)
		err := proc.ConsumeMetrics(context.Background(), rms)
		require.NoError(t, err)

		// aggregate manually
		wantv := want[s.id(pmetric.AggregationTemporalityCumulative)]
		wantv.SetIntValue(wantv.IntValue() + v)
		wantv.SetTimestamp(ts)
	}

	// get the final processor output for each stream
	got := make(map[identity.Stream]data.Number)
	for _, md := range sink.AllMetrics() {
		metrics.All(md)(func(m metrics.Metric) bool {
			sum := metrics.Sum(m)
			streams.Datapoints(sum)(func(id identity.Stream, dp data.Number) bool {
				got[id] = dp
				return true
			})
			return true
		})
	}

	sort := cmpopts.SortMaps(func(a, b identity.Stream) bool {
		return a.Hash().Sum64() < b.Hash().Sum64()
	})
	if diff := compare.Diff(want, got, sort); diff != "" {
		t.Fatal(diff)
	}
}

// TestTimestamp verifies timestamp handling, most notably:
// - Timestamp() keeps getting advanced
// - StartTimestamp() stays the same
func TestTimestamps(t *testing.T) {
	proc, sink := setup(t, nil)

	sb := stream()
	point := func(start, last pcommon.Timestamp) data.Number {
		return sb.point(start, last, 0)
	}

	cases := []struct {
		in   data.Number
		out  data.Number
		drop bool
	}{{
		// first: take as-is
		in:  point(1000, 1100),
		out: point(1000, 1100),
	}, {
		// subsequent: take, but keep start-ts
		in:  point(1100, 1200),
		out: point(1000, 1200),
	}, {
		// gap: take
		in:  point(1300, 1400),
		out: point(1000, 1400),
	}, {
		// out of order
		in:   point(1200, 1300),
		drop: true,
	}, {
		// older start
		in:   point(500, 550),
		drop: true,
	}}

	for i, cs := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			sink.Reset()

			in := sb.resourceMetrics(sb.delta(cs.in))
			want := make([]pmetric.Metrics, 0)
			if !cs.drop {
				want = []pmetric.Metrics{sb.resourceMetrics(sb.cumul(cs.out))}
			}

			err := proc.ConsumeMetrics(context.Background(), in)
			require.NoError(t, err)

			out := sink.AllMetrics()
			if diff := compare.Diff(want, out); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestStreamLimit(t *testing.T) {
	proc, sink := setup(t, &self.Config{MaxStale: 5 * time.Minute, MaxStreams: 10})

	good := make([]SumBuilder, 10)
	for i := range good {
		good[i] = stream()
	}
	bad := stream()
	_ = bad

	diff := func(want, got []pmetric.Metrics) {
		t.Helper()
		if diff := compare.Diff(want, got); diff != "" {
			t.Fatal(diff)
		}
	}

	writeGood := func(ts pcommon.Timestamp) {
		for i, sb := range good {
			in := sb.resourceMetrics(sb.delta(sb.point(0, ts+pcommon.Timestamp(i), 0)))
			want := sb.resourceMetrics(sb.cumul(sb.point(0, ts+pcommon.Timestamp(i), 0)))

			err := proc.ConsumeMetrics(context.Background(), in)
			require.NoError(t, err)

			diff([]pmetric.Metrics{want}, sink.AllMetrics())
			sink.Reset()
		}
	}

	// write up to limit must work
	writeGood(0)

	// extra stream must be dropped, nothing written
	in := bad.resourceMetrics(bad.delta(bad.point(0, 0, 0)))
	err := proc.ConsumeMetrics(context.Background(), in)
	require.NoError(t, err)
	diff([]pmetric.Metrics{}, sink.AllMetrics())
	sink.Reset()

	// writing existing streams must still work
	writeGood(100)
}

type copyable interface {
	CopyTo(pmetric.Metric)
}

func (s SumBuilder) resourceMetrics(metrics ...copyable) pmetric.Metrics {
	md := pmetric.NewMetrics()

	rm := md.ResourceMetrics().AppendEmpty()
	s.Resource().CopyTo(rm.Resource())

	sm := rm.ScopeMetrics().AppendEmpty()
	s.Scope().CopyTo(sm.Scope())

	for _, m := range metrics {
		m.CopyTo(sm.Metrics().AppendEmpty())
	}
	return md
}

type SumBuilder struct {
	random.Metric[data.Number]
	base data.Number
}

func (s SumBuilder) with(dps ...data.Number) pmetric.Metric {
	m := pmetric.NewMetric()
	s.Metric.CopyTo(m)

	for _, dp := range dps {
		dp.NumberDataPoint.CopyTo(m.Sum().DataPoints().AppendEmpty())
	}

	return m
}

func (s SumBuilder) delta(dps ...data.Number) pmetric.Metric {
	m := s.with(dps...)
	m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	return m
}

func (s SumBuilder) cumul(dps ...data.Number) pmetric.Metric {
	m := s.with(dps...)
	m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	return m
}

func (s SumBuilder) id(temp pmetric.AggregationTemporality) identity.Stream {
	m := s.with(s.base)
	m.Sum().SetAggregationTemporality(temp)

	mid := identity.OfMetric(s.Ident().Scope(), m)
	return identity.OfStream(mid, s.base)
}

func (s SumBuilder) point(start, ts pcommon.Timestamp, value int64) data.Number {
	dp := s.base.Clone()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(value)
	return dp
}

func stream() SumBuilder {
	sum := random.Sum()
	_, base := sum.Stream()
	return SumBuilder{Metric: sum, base: base}
}

func TestIgnore(t *testing.T) {
	proc, sink := setup(t, nil)

	dir := "./testdata/notemporality-ignored"
	open := func(file string) pmetric.Metrics {
		t.Helper()
		md, err := golden.ReadMetrics(filepath.Join(dir, file))
		require.NoError(t, err)
		return md
	}

	in := open("in.yaml")
	out := open("out.yaml")

	ctx := context.Background()

	err := proc.ConsumeMetrics(ctx, in)
	require.NoError(t, err)

	if diff := compare.Diff([]pmetric.Metrics{out}, sink.AllMetrics()); diff != "" {
		t.Fatal(diff)
	}
}
