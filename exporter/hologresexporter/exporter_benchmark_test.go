// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// benchMockDB is a lightweight mock of pgxDB for benchmarking.
// It consumes rows without recording them to minimize overhead.
type benchMockDB struct{}

func (m *benchMockDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (m *benchMockDB) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, rowSrc pgx.CopyFromSource) (int64, error) {
	var count int64
	for rowSrc.Next() {
		_, _ = rowSrc.Values()
		count++
	}
	return count, rowSrc.Err()
}

func (m *benchMockDB) Ping(_ context.Context) error { return nil }
func (m *benchMockDB) Close()                       {}

// --- Test data generators ---

func generateTraces(spanCount int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "bench-service")
	rs.Resource().Attributes().PutStr("host.name", "bench-host")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("bench-scope")
	ss.Scope().SetVersion("1.0.0")

	now := time.Now()
	for i := 0; i < spanCount; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetName(fmt.Sprintf("operation-%d", i))
		span.SetTraceID(pcommon.TraceID([16]byte{byte(i), byte(i >> 8), 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}))
		span.SetSpanID(pcommon.SpanID([8]byte{byte(i), byte(i >> 8), 1, 2, 3, 4, 5, 6}))
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-time.Second)))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(now))
		span.SetKind(ptrace.SpanKindServer)
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.Attributes().PutStr("http.method", "GET")
		span.Attributes().PutStr("http.url", fmt.Sprintf("http://example.com/api/v1/resource/%d", i))
		span.Attributes().PutInt("http.status_code", 200)

		// Add event
		evt := span.Events().AppendEmpty()
		evt.SetName("log")
		evt.SetTimestamp(pcommon.NewTimestampFromTime(now))
		evt.Attributes().PutStr("message", "benchmark event")

		// Add link
		link := span.Links().AppendEmpty()
		link.SetTraceID(pcommon.TraceID([16]byte{15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, byte(i)}))
		link.SetSpanID(pcommon.SpanID([8]byte{8, 7, 6, 5, 4, 3, 2, byte(i)}))
	}
	return td
}

func generateLogs(logCount int) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "bench-service")
	rl.Resource().Attributes().PutStr("host.name", "bench-host")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("bench-scope")
	sl.Scope().SetVersion("1.0.0")

	now := time.Now()
	for i := 0; i < logCount; i++ {
		lr := sl.LogRecords().AppendEmpty()
		lr.SetTimestamp(pcommon.NewTimestampFromTime(now))
		lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
		lr.SetSeverityNumber(plog.SeverityNumberInfo)
		lr.SetSeverityText("INFO")
		lr.Body().SetStr(fmt.Sprintf("Benchmark log message %d with some content to simulate real logs", i))
		lr.Attributes().PutStr("log.source", "benchmark")
		lr.Attributes().PutInt("log.index", int64(i))
		lr.SetTraceID(pcommon.TraceID([16]byte{byte(i), 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))
		lr.SetSpanID(pcommon.SpanID([8]byte{byte(i), 1, 2, 3, 4, 5, 6, 7}))
	}
	return ld
}

func generateMetrics(dataPointCount int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "bench-service")
	rm.Resource().Attributes().PutStr("host.name", "bench-host")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("bench-scope")
	sm.Scope().SetVersion("1.0.0")

	m := sm.Metrics().AppendEmpty()
	m.SetName("bench.gauge.metric")
	gauge := m.SetEmptyGauge()

	now := time.Now()
	for i := 0; i < dataPointCount; i++ {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-time.Minute)))
		dp.SetDoubleValue(float64(i) * 1.5)
		dp.Attributes().PutStr("host", fmt.Sprintf("host-%d", i%10))
		dp.Attributes().PutStr("region", "us-west-2")
	}
	return md
}

// --- Mock Benchmarks ---

func BenchmarkTracesPush(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("spans_%d", size), func(b *testing.B) {
			cfg := &Config{
				DSN:             "mock",
				TracesTableName: "bench_traces",
			}
			exp := &tracesExporter{
				logger: zap.NewNop(),
				cfg:    cfg,
				db:     &benchMockDB{},
			}
			td := generateTraces(size)
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := exp.pushTraceData(ctx, td); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			elapsed := b.Elapsed()
			b.ReportMetric(float64(size), "items/op")
			if elapsed > 0 {
				b.ReportMetric(float64(int64(size)*int64(b.N))/elapsed.Seconds(), "items/s")
			}
		})
	}
}

func BenchmarkLogsPush(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("logs_%d", size), func(b *testing.B) {
			cfg := &Config{
				DSN:           "mock",
				LogsTableName: "bench_logs",
			}
			exp := &logsExporter{
				logger: zap.NewNop(),
				cfg:    cfg,
				db:     &benchMockDB{},
			}
			ld := generateLogs(size)
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := exp.pushLogData(ctx, ld); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			elapsed := b.Elapsed()
			b.ReportMetric(float64(size), "items/op")
			if elapsed > 0 {
				b.ReportMetric(float64(int64(size)*int64(b.N))/elapsed.Seconds(), "items/s")
			}
		})
	}
}

func BenchmarkMetricsPush(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("datapoints_%d", size), func(b *testing.B) {
			cfg := &Config{
				DSN:              "mock",
				MetricsTableName: "bench_metrics",
			}
			exp := &metricsExporter{
				logger: zap.NewNop(),
				cfg:    cfg,
				db:     &benchMockDB{},
			}
			md := generateMetrics(size)
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := exp.pushMetricData(ctx, md); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			elapsed := b.Elapsed()
			b.ReportMetric(float64(size), "items/op")
			if elapsed > 0 {
				b.ReportMetric(float64(int64(size)*int64(b.N))/elapsed.Seconds(), "items/s")
			}
		})
	}
}
