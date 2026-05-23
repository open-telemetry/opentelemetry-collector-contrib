// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// noopBatch implements driver.Batch but discards all data.
// This isolates the data conversion and allocation cost from network I/O.
type noopBatch struct{}

func (noopBatch) Abort() error                    { return nil }
func (noopBatch) Append(_ ...any) error           { return nil }
func (noopBatch) AppendStruct(_ any) error        { return nil }
func (noopBatch) Column(_ int) driver.BatchColumn { return nil }
func (noopBatch) Flush() error                    { return nil }
func (noopBatch) Send() error                     { return nil }
func (noopBatch) IsSent() bool                    { return false }
func (noopBatch) Rows() int                       { return 0 }
func (noopBatch) Columns() []column.Interface     { return nil }
func (noopBatch) Close() error                    { return nil }

// noopConn implements driver.Conn, returning noopBatch for PrepareBatch.
type noopConn struct{}

func (noopConn) Contributors() []string                                           { return nil }
func (noopConn) ServerVersion() (*driver.ServerVersion, error)                    { return nil, nil }
func (noopConn) Select(_ context.Context, _ any, _ string, _ ...any) error        { return nil }
func (noopConn) Query(_ context.Context, _ string, _ ...any) (driver.Rows, error) { return nil, nil }
func (noopConn) QueryRow(_ context.Context, _ string, _ ...any) driver.Row        { return nil }
func (noopConn) PrepareBatch(_ context.Context, _ string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	return noopBatch{}, nil
}
func (noopConn) Exec(_ context.Context, _ string, _ ...any) error { return nil }
func (noopConn) AsyncInsert(_ context.Context, _ string, _ bool, _ ...any) error {
	return nil
}
func (noopConn) Ping(_ context.Context) error { return nil }
func (noopConn) Stats() driver.Stats          { return driver.Stats{} }
func (noopConn) Close() error                 { return nil }

// realisticLogs generates a plog.Logs payload that mimics real-world telemetry:
// multiple resource groups, scopes, and records with varied attribute counts.
func realisticLogs(totalRecords int) plog.Logs {
	logs := plog.NewLogs()
	now := time.Now()

	// Distribute records across 5 services × 2 scopes
	services := 5
	scopes := 2
	recordsPerScope := totalRecords / (services * scopes)

	for s := range services {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.4.0")
		res := rl.Resource().Attributes()
		res.PutStr("service.name", fmt.Sprintf("service-%d", s))
		res.PutStr("service.version", "1.2.3")
		res.PutStr("deployment.environment", "production")
		res.PutStr("host.name", fmt.Sprintf("host-%d.example.com", s))
		res.PutStr("cloud.region", "us-east-1")
		res.PutStr("cloud.provider", "aws")

		for sc := range scopes {
			sl := rl.ScopeLogs().AppendEmpty()
			sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.7.0")
			sl.Scope().SetName(fmt.Sprintf("io.opentelemetry.scope-%d", sc))
			sl.Scope().SetVersion("1.0.0")
			sl.Scope().Attributes().PutStr("lib", "clickhouse")

			for r := range recordsPerScope {
				rec := sl.LogRecords().AppendEmpty()
				rec.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Duration(r) * time.Millisecond)))
				rec.SetObservedTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Duration(r) * time.Millisecond)))
				rec.SetSeverityNumber(plog.SeverityNumberInfo)
				rec.SetSeverityText("INFO")
				rec.SetEventName("http.request")
				rec.Body().SetStr("GET /api/users/123 HTTP/1.1")
				rec.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, byte(r % 256)})
				rec.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, byte(r % 256)})

				attrs := rec.Attributes()
				attrs.PutStr("http.method", "GET")
				attrs.PutStr("http.url", "/api/users/123")
				attrs.PutStr("http.status_code", "200")
				attrs.PutStr("http.user_agent", "Mozilla/5.0")
				attrs.PutStr("net.peer.ip", "10.0.0.1")
				attrs.PutStr("service.namespace", "default")
			}
		}
	}
	return logs
}

// realisticTraces generates a ptrace.Traces payload with varied spans,
// events, and links per span — matching real-world distributed tracing data.
func realisticTraces(totalSpans int) ptrace.Traces {
	traces := ptrace.NewTraces()
	now := time.Now()

	// Distribute spans across 5 services × 2 scopes
	services := 5
	scopes := 2
	spansPerScope := totalSpans / (services * scopes)

	for s := range services {
		rs := traces.ResourceSpans().AppendEmpty()
		rs.SetSchemaUrl("https://opentelemetry.io/schemas/1.4.0")
		res := rs.Resource().Attributes()
		res.PutStr("service.name", fmt.Sprintf("service-%d", s))
		res.PutStr("service.version", "1.2.3")
		res.PutStr("deployment.environment", "production")
		res.PutStr("host.name", fmt.Sprintf("host-%d.example.com", s))
		res.PutStr("cloud.region", "us-east-1")
		res.PutStr("cloud.provider", "aws")

		for sc := range scopes {
			ss := rs.ScopeSpans().AppendEmpty()
			ss.SetSchemaUrl("https://opentelemetry.io/schemas/1.7.0")
			ss.Scope().SetName(fmt.Sprintf("io.opentelemetry.scope-%d", sc))
			ss.Scope().SetVersion("1.0.0")
			ss.Scope().Attributes().PutStr("lib", "clickhouse")

			for sp := range spansPerScope {
				span := ss.Spans().AppendEmpty()
				span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, byte(sp % 256)})
				span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, byte(sp % 256)})
				span.SetParentSpanID([8]byte{1, 2, 3, 4, 5, 6, 8, byte(sp % 256)})
				span.TraceState().FromRaw("sampled=true")
				span.SetName(fmt.Sprintf("GET /api/resource/%d", sp%10))
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Duration(sp) * time.Millisecond)))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Duration(sp)*time.Millisecond + 50*time.Millisecond)))
				span.Status().SetCode(ptrace.StatusCodeOk)
				span.Status().SetMessage("")

				attrs := span.Attributes()
				attrs.PutStr("http.method", "GET")
				attrs.PutStr("http.url", "/api/resource/123")
				attrs.PutStr("http.status_code", "200")
				attrs.PutStr("net.peer.ip", "10.0.0.1")
				attrs.PutStr("db.system", "postgresql")
				attrs.PutStr("db.statement", "SELECT * FROM users WHERE id = $1")

				// ~60% of spans have 1-2 events, ~20% have 3+, rest have 0.
				numEvents := 0
				switch {
				case sp%5 < 3:
					numEvents = 1 + sp%2
				case sp%5 == 3:
					numEvents = 3
				}
				for e := range numEvents {
					event := span.Events().AppendEmpty()
					event.SetName(fmt.Sprintf("event-%d", e))
					event.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Duration(sp)*time.Millisecond + time.Duration(e)*time.Microsecond)))
					event.Attributes().PutStr("level", "info")
					event.Attributes().PutStr("message", "processing request")
				}

				// ~30% of spans have 1 link
				if sp%3 == 0 {
					link := span.Links().AppendEmpty()
					link.SetTraceID([16]byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5, byte(sp % 256)})
					link.SetSpanID([8]byte{9, 8, 7, 6, 5, 4, 3, byte(sp % 256)})
					link.TraceState().FromRaw("upstream=true")
					link.Attributes().PutStr("link.type", "parent")
				}
			}
		}
	}
	return traces
}

// BenchmarkPushLogsData benchmarks the full log batch conversion path:
// AttributesToMap (with Range), columnValues reuse, and batch.Append.
// Uses 1000 records across 5 services to simulate a realistic batch.
func BenchmarkPushLogsData(b *testing.B) {
	logs := realisticLogs(1000)
	exporter := &logsExporter{
		db:     noopConn{},
		logger: zap.NewNop(),
		cfg:    withDefaultConfig(),
	}
	exporter.schemaFeatures.EventName = true
	exporter.insertSQL = "INSERT INTO test"
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := exporter.pushLogsData(b.Context(), logs); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPushTraceData benchmarks the full trace batch conversion path:
// AttributesToMap (with Range), convertEvents, convertLinks, and batch.Append.
// Uses 1000 spans with varied events/links to simulate a realistic batch.
func BenchmarkPushTraceData(b *testing.B) {
	traces := realisticTraces(1000)
	exporter := &tracesExporter{
		db:        noopConn{},
		logger:    zap.NewNop(),
		cfg:       withDefaultConfig(),
		insertSQL: "INSERT INTO test",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := exporter.pushTraceData(b.Context(), traces); err != nil {
			b.Fatal(err)
		}
	}
}
