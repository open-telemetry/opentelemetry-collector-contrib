// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Batch shape shared by all benchmarks: benchResources x benchScopes scopes,
// each holding benchItems leaf items (records, spans, metrics, profiles).
const (
	benchResources = 10
	benchScopes    = 2
	benchItems     = 500
)

// level selects which layer of the batch carries the partition value.
type level int

const (
	levelResource level = iota
	levelScope
	levelItem
)

func partValue(i, parts int) string { return fmt.Sprintf("p%d", i%parts) }

func putBenchAttrs(m pcommon.Map) {
	for i := range 5 {
		m.PutStr(fmt.Sprintf("attr.%d", i), "some-reasonably-sized-attribute-value")
	}
}

func benchCtx(mdKeys int) context.Context {
	if mdKeys == 0 {
		return context.Background()
	}
	md := make(map[string][]string, mdKeys)
	for i := range mdKeys {
		md[fmt.Sprintf("x-existing-%d", i)] = []string{"value"}
	}
	return client.NewContext(context.Background(), client.Info{Metadata: client.NewMetadata(md)})
}

func newBenchConfig(expr string) *Config {
	return &Config{Keys: map[string]string{"part": expr}}
}

func genBenchLogs(lvl level, parts, items int) plog.Logs {
	ld := plog.NewLogs()
	si, li := 0, 0
	for r := range benchResources {
		rl := ld.ResourceLogs().AppendEmpty()
		putBenchAttrs(rl.Resource().Attributes())
		if lvl == levelResource {
			rl.Resource().Attributes().PutStr("part", partValue(r, parts))
		}
		for range benchScopes {
			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName("scope")
			if lvl == levelScope {
				sl.Scope().SetName(partValue(si, parts))
			}
			si++
			sl.LogRecords().EnsureCapacity(items)
			for range items {
				lr := sl.LogRecords().AppendEmpty()
				lr.Body().SetStr("a log line body of moderate length for benchmarking")
				lr.SetSeverityText("INFO")
				putBenchAttrs(lr.Attributes())
				if lvl == levelItem {
					lr.Attributes().PutStr("part", partValue(li, parts))
				}
				li++
			}
		}
	}
	return ld
}

func genBenchTraces(parts int) ptrace.Traces {
	td := ptrace.NewTraces()
	si := 0
	for range benchResources {
		rs := td.ResourceSpans().AppendEmpty()
		putBenchAttrs(rs.Resource().Attributes())
		for range benchScopes {
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scope")
			ss.Spans().EnsureCapacity(benchItems)
			for range benchItems {
				span := ss.Spans().AppendEmpty()
				span.SetName("span-name")
				span.SetTraceID(pcommon.TraceID{1, 2, 3})
				span.SetSpanID(pcommon.SpanID{4, 5, 6})
				putBenchAttrs(span.Attributes())
				span.Attributes().PutStr("part", partValue(si, parts))
				si++
			}
		}
	}
	return td
}

// genBenchMetrics builds benchMetricsPerScope metrics per scope, each with
// benchDatapoints datapoints, rotating gauge/sum/histogram. If dpLevel is
// true the partition value is set per datapoint, otherwise per metric
// (via the description).
const (
	benchMetricsPerScope = 50
	benchDatapoints      = 10
)

func genBenchMetrics(dpLevel bool, parts int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	mi, di := 0, 0
	for range benchResources {
		rm := md.ResourceMetrics().AppendEmpty()
		putBenchAttrs(rm.Resource().Attributes())
		for range benchScopes {
			sm := rm.ScopeMetrics().AppendEmpty()
			sm.Scope().SetName("scope")
			for k := range benchMetricsPerScope {
				m := sm.Metrics().AppendEmpty()
				m.SetName(fmt.Sprintf("metric.%d", k))
				m.SetUnit("1")
				m.SetDescription("d")
				if !dpLevel {
					m.SetDescription(partValue(mi, parts))
				}
				mi++
				switch k % 3 {
				case 0:
					m.SetEmptyGauge()
				case 1:
					m.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				case 2:
					m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				}
				for range benchDatapoints {
					var attrs pcommon.Map
					switch m.Type() {
					case pmetric.MetricTypeGauge:
						dp := m.Gauge().DataPoints().AppendEmpty()
						dp.SetDoubleValue(1)
						attrs = dp.Attributes()
					case pmetric.MetricTypeSum:
						dp := m.Sum().DataPoints().AppendEmpty()
						dp.SetIntValue(1)
						attrs = dp.Attributes()
					case pmetric.MetricTypeHistogram:
						dp := m.Histogram().DataPoints().AppendEmpty()
						dp.SetCount(10)
						dp.ExplicitBounds().FromRaw([]float64{1, 5, 10})
						dp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
						attrs = dp.Attributes()
					}
					putBenchAttrs(attrs)
					if dpLevel {
						attrs.PutStr("part", partValue(di, parts))
					}
					di++
				}
			}
		}
	}
	return md
}

// cloneable is implemented by the top-level pdata containers.
type cloneable[T any] interface{ CopyTo(T) }

// runBench calls consume once per iteration on a fresh copy of in, made
// outside the timer, because partitioning may move items out of its input.
func runBench[T cloneable[T]](b *testing.B, in T, newT func() T, consume func(T) error) {
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		c := newT()
		in.CopyTo(c)
		b.StartTimer()
		if err := consume(c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogs(b *testing.B) {
	cases := []struct {
		name   string
		lvl    level
		expr   string
		parts  []int
		mdKeys int
	}{
		{"resource", levelResource, `resource.attributes["part"]`, []int{1, 10}, 0},
		{"resource_md10", levelResource, `resource.attributes["part"]`, []int{10}, 10},
		{"scope", levelScope, `scope.name`, []int{1, 10}, 0},
		{"log", levelItem, `log.attributes["part"]`, []int{1, 10, 1000}, 0},
	}
	for _, tc := range cases {
		for _, parts := range tc.parts {
			b.Run(fmt.Sprintf("%s/%dpartitions", tc.name, parts), func(b *testing.B) {
				p, err := createLogsProcessor(b.Context(), nopSettings(), newBenchConfig(tc.expr), consumertest.NewNop())
				require.NoError(b, err)
				ld := genBenchLogs(tc.lvl, parts, benchItems)
				ctx := benchCtx(tc.mdKeys)
				runBench(b, ld, plog.NewLogs, func(in plog.Logs) error { return p.ConsumeLogs(ctx, in) })
			})
		}
	}
	b.Run("otelcol", func(b *testing.B) {
		p, err := createLogsProcessor(b.Context(), nopSettings(),
			newBenchConfig(`otelcol.client.metadata["x-existing-0"][0]`), consumertest.NewNop())
		require.NoError(b, err)
		ld := genBenchLogs(levelItem, 1, benchItems)
		ctx := benchCtx(1)
		runBench(b, ld, plog.NewLogs, func(in plog.Logs) error { return p.ConsumeLogs(ctx, in) })
	})
}

func BenchmarkTraces(b *testing.B) {
	for _, parts := range []int{1, 10, 1000} {
		b.Run(fmt.Sprintf("span/%dpartitions", parts), func(b *testing.B) {
			p, err := createTracesProcessor(b.Context(), nopSettings(),
				newBenchConfig(`span.attributes["part"]`), consumertest.NewNop())
			require.NoError(b, err)
			td := genBenchTraces(parts)
			runBench(b, td, ptrace.NewTraces, func(in ptrace.Traces) error { return p.ConsumeTraces(b.Context(), in) })
		})
	}
}

func BenchmarkMetrics(b *testing.B) {
	cases := []struct {
		name    string
		dpLevel bool
		expr    string
		parts   []int
	}{
		{"metric", false, `metric.description`, []int{1, 10, 100}},
		{"datapoint", true, `datapoint.attributes["part"]`, []int{1, 10, 1000}},
	}
	for _, tc := range cases {
		for _, parts := range tc.parts {
			b.Run(fmt.Sprintf("%s/%dpartitions", tc.name, parts), func(b *testing.B) {
				p, err := createMetricsProcessor(b.Context(), nopSettings(), newBenchConfig(tc.expr), consumertest.NewNop())
				require.NoError(b, err)
				md := genBenchMetrics(tc.dpLevel, parts)
				runBench(b, md, pmetric.NewMetrics, func(in pmetric.Metrics) error { return p.ConsumeMetrics(b.Context(), in) })
			})
		}
	}
}
