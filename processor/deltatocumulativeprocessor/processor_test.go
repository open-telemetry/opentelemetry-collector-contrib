// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	self "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest/compare"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
)

func TestProcessor(t *testing.T) {
	fis, err := os.ReadDir("testdata")
	require.NoError(t, err)

	for _, fi := range fis {
		if !fi.IsDir() {
			continue
		}

		ar := reader()

		type Stage struct {
			In  pmetric.Metrics `testar:"in,pmetric"`
			Out pmetric.Metrics `testar:"out,pmetric"`
		}

		dir := fi.Name()
		t.Run(dir, func(t *testing.T) {
			file := func(f string) string {
				return filepath.Join("testdata", dir, f)
			}

			ctx := context.Background()
			cfg := config(t, file("config.yaml"))
			proc, sink := setup(t, cfg)

			stages, _ := filepath.Glob(file("*.test"))
			for _, file := range stages {
				var stage Stage
				err := ar.ReadFile(file, &stage)
				require.NoError(t, err)

				sink.Reset()
				err = proc.ConsumeMetrics(ctx, stage.In)
				require.NoError(t, err)

				out := []pmetric.Metrics{stage.Out}
				if diff := compare.Diff(out, sink.AllMetrics()); diff != "" {
					t.Fatal(diff)
				}
			}
		})

	}
}

func config(t *testing.T, file string) *self.Config {
	cfg := self.NewFactory().CreateDefaultConfig().(*self.Config)
	cm, err := confmaptest.LoadConf(file)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg
	}
	require.NoError(t, err)

	err = cm.Unmarshal(cfg)
	require.NoError(t, err)
	return cfg
}

func reader() testar.Reader {
	return testar.Reader{
		Formats: testar.Formats{
			"pmetric": func(data []byte, into any) error {
				var tmp any
				if err := yaml.Unmarshal(data, &tmp); err != nil {
					return err
				}
				data, err := json.Marshal(tmp)
				if err != nil {
					return err
				}
				md, err := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics(data)
				if err != nil {
					return err
				}
				*(into.(*pmetric.Metrics)) = md
				return nil
			},
		},
	}
}

func setup(t *testing.T, cfg *self.Config) (processor.Metrics, *consumertest.MetricsSink) {
	t.Helper()

	next := &consumertest.MetricsSink{}
	if cfg == nil {
		cfg = &self.Config{MaxStale: 0, MaxStreams: math.MaxInt}
	}

	proc, err := self.NewFactory().CreateMetrics(
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
