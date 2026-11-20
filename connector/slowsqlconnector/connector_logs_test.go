// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package slowsqlconnector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestConsumeTraces_SlowQuery(t *testing.T) {
	sink := new(consumertest.LogsSink)
	cfg := &Config{
		Threshold: 100 * time.Millisecond,
		DBSystem:  []string{"mysql", "postgresql"},
		Dimensions: []Dimension{
			{Name: "db.port"},
			{Name: "custom.flag"},
		},
	}
	conn := newLogsConnector(zap.NewNop(), cfg)
	conn.logsConsumer = sink
	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl("https://opentelemetry.io/schemas/1.43.0")
	rs.Resource().Attributes().PutStr("service.name", "order-service")
	rs.Resource().Attributes().PutStr("host.name", "worker-1")
	rs.Resource().Attributes().PutStr("k8s.pod.name", "order-pod-123")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test-scope")
	ss.Scope().SetVersion("1.0.0")
	ss.SetSchemaUrl("https://opentelemetry.io/schemas/scope/1.0.0")

	// Span 1: Slow query matching threshold (500ms > 100ms) with "db.system.name"
	span1 := ss.Spans().AppendEmpty()
	span1.SetName("SELECT * FROM orders")
	span1.SetKind(ptrace.SpanKindClient)
	span1.Attributes().PutStr("db.system.name", "mysql")
	span1.Attributes().PutStr("db.statement", "SELECT * FROM orders")
	span1.Attributes().PutInt("db.port", 3306)
	span1.Attributes().PutBool("custom.flag", true)
	span1.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))
	span1.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, int64(500*time.Millisecond))))

	// Span 2: Fast query under threshold (50ms < 100ms) - should NOT generate a log
	span2 := ss.Spans().AppendEmpty()
	span2.SetName("SELECT 1")
	span2.SetKind(ptrace.SpanKindClient)
	span2.Attributes().PutStr("db.system.name", "mysql")
	span2.Attributes().PutStr("db.statement", "SELECT 1")
	span2.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(200, 0)))
	span2.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(200, int64(50*time.Millisecond))))

	// Span 3: Slow query using legacy "db.system" attribute
	span3 := ss.Spans().AppendEmpty()
	span3.SetName("UPDATE users")
	span3.SetKind(ptrace.SpanKindClient)
	span3.Attributes().PutStr("db.system", "postgresql")
	span3.Attributes().PutStr("db.statement", "UPDATE users SET active = true")
	span3.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(300, 0)))
	span3.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(300, int64(200*time.Millisecond))))

	// Span 4: Server span (not client) even if slow - should NOT generate a log
	span4 := ss.Spans().AppendEmpty()
	span4.SetName("SERVER SPAN")
	span4.SetKind(ptrace.SpanKindServer)
	span4.Attributes().PutStr("db.system.name", "mysql")
	span4.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(400, 0)))
	span4.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(400, int64(300*time.Millisecond))))

	err := conn.ConsumeTraces(t.Context(), traces)
	require.NoError(t, err)

	logs := sink.AllLogs()
	require.Len(t, logs, 1)
	require.Equal(t, 2, logs[0].LogRecordCount(), "expected exactly 2 slow SQL logs (span1 and span3)")

	rl := logs[0].ResourceLogs().At(0)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.43.0", rl.SchemaUrl())
	assert.Equal(t, "order-service", rl.Resource().Attributes().AsRaw()["service.name"])
	assert.Equal(t, "worker-1", rl.Resource().Attributes().AsRaw()["host.name"])
	assert.Equal(t, "order-pod-123", rl.Resource().Attributes().AsRaw()["k8s.pod.name"])

	sl := rl.ScopeLogs().At(0)
	assert.Equal(t, "test-scope", sl.Scope().Name())
	assert.Equal(t, "1.0.0", sl.Scope().Version())
	assert.Equal(t, "https://opentelemetry.io/schemas/scope/1.0.0", sl.SchemaUrl())

	records := sl.LogRecords()
	require.Equal(t, 2, records.Len())

	// Record 0 corresponds to span 1
	rec0 := records.At(0)
	assert.Equal(t, "SLOW", rec0.SeverityText())
	assert.Equal(t, plog.SeverityNumberError, rec0.SeverityNumber())
	assert.Equal(t, "SELECT * FROM orders", rec0.Attributes().AsRaw()["db.statement"])
	assert.Equal(t, "3306", rec0.Attributes().AsRaw()["db.port"])
	assert.Equal(t, "true", rec0.Attributes().AsRaw()["custom.flag"])
	assert.Equal(t, "order-service", rec0.Attributes().AsRaw()["service.name"])

	// Record 1 corresponds to span 3
	rec1 := records.At(1)
	assert.Equal(t, "UPDATE users SET active = true", rec1.Attributes().AsRaw()["db.statement"])
	assert.Equal(t, "order-service", rec1.Attributes().AsRaw()["service.name"])
}

func TestConsumeTraces_NoSlowQueriesEmitsNothing(t *testing.T) {
	sink := new(consumertest.LogsSink)
	cfg := &Config{
		Threshold: 500 * time.Millisecond,
		DBSystem:  []string{"mysql"},
	}
	conn := newLogsConnector(zap.NewNop(), cfg)
	conn.logsConsumer = sink
	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "my-service")
	ss := rs.ScopeSpans().AppendEmpty()

	// Fast client span (10ms < 500ms)
	span := ss.Spans().AppendEmpty()
	span.SetKind(ptrace.SpanKindClient)
	span.Attributes().PutStr("db.system.name", "mysql")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 0)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, int64(10*time.Millisecond))))

	err := conn.ConsumeTraces(t.Context(), traces)
	require.NoError(t, err)

	// Verify sink received zero logs (no empty ResourceLogs emitted)
	assert.Empty(t, sink.AllLogs(), "expected no logs emitted when no slow queries exist")
}

func TestConsumeTraces_MultipleScopesInResource(t *testing.T) {
	sink := new(consumertest.LogsSink)
	cfg := &Config{
		Threshold: 100 * time.Millisecond,
		DBSystem:  []string{"mysql"},
	}
	conn := newLogsConnector(zap.NewNop(), cfg)
	conn.logsConsumer = sink
	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "my-service")

	// Scope 1: has a slow query
	ss1 := rs.ScopeSpans().AppendEmpty()
	ss1.Scope().SetName("scope-1")
	span1 := ss1.Spans().AppendEmpty()
	span1.SetKind(ptrace.SpanKindClient)
	span1.Attributes().PutStr("db.system.name", "mysql")
	span1.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 0)))
	span1.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, int64(200*time.Millisecond))))

	// Scope 2: only has fast queries
	ss2 := rs.ScopeSpans().AppendEmpty()
	ss2.Scope().SetName("scope-2")
	span2 := ss2.Spans().AppendEmpty()
	span2.SetKind(ptrace.SpanKindClient)
	span2.Attributes().PutStr("db.system.name", "mysql")
	span2.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 0)))
	span2.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, int64(20*time.Millisecond))))

	// Scope 3: has a slow query
	ss3 := rs.ScopeSpans().AppendEmpty()
	ss3.Scope().SetName("scope-3")
	span3 := ss3.Spans().AppendEmpty()
	span3.SetKind(ptrace.SpanKindClient)
	span3.Attributes().PutStr("db.system.name", "mysql")
	span3.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 0)))
	span3.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, int64(300*time.Millisecond))))

	err := conn.ConsumeTraces(t.Context(), traces)
	require.NoError(t, err)

	logs := sink.AllLogs()
	require.Len(t, logs, 1)

	// Exactly 1 ResourceLogs containing 2 ScopeLogs (scope-1 and scope-3; scope-2 omitted)
	require.Equal(t, 1, logs[0].ResourceLogs().Len())
	rl := logs[0].ResourceLogs().At(0)
	require.Equal(t, 2, rl.ScopeLogs().Len())
	assert.Equal(t, "scope-1", rl.ScopeLogs().At(0).Scope().Name())
	assert.Equal(t, "scope-3", rl.ScopeLogs().At(1).Scope().Name())
}

func TestConsumeTraces_MissingServiceName(t *testing.T) {
	sink := new(consumertest.LogsSink)
	cfg := &Config{
		Threshold: 100 * time.Millisecond,
		DBSystem:  []string{"mysql"},
	}
	conn := newLogsConnector(zap.NewNop(), cfg)
	conn.logsConsumer = sink
	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	// No service.name attribute
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetKind(ptrace.SpanKindClient)
	span.Attributes().PutStr("db.system.name", "mysql")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 0)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, int64(200*time.Millisecond))))

	err := conn.ConsumeTraces(t.Context(), traces)
	require.NoError(t, err)
	assert.Empty(t, sink.AllLogs())
}
