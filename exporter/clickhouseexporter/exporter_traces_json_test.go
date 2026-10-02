// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

// TestTracesJSONExporter_StartIsNonFatalOnDescFailure is the unit-level regression
// for #48875: when DESC TABLE fails at start (e.g. transient ClickHouse outage),
// start() must complete successfully but the cached INSERT must NOT yet be rendered.
// The first pushTraceData call must re-probe; on continued failure the push returns
// a retryable error rather than silently appending rows without the keys columns.
func TestTracesJSONExporter_StartIsNonFatalOnDescFailure(t *testing.T) {
	cfg := withDefaultConfig()
	cfg.Endpoint = defaultEndpoint
	cfg.CreateSchema = false
	exp := newTracesJSONExporter(zaptest.NewLogger(t), cfg)

	// Replace the real DESC TABLE probe with a controllable stub. We don't go
	// through start() (which would open a real DB connection) — we only verify
	// the detector + push path.
	descCalls := atomic.Int32{}
	descFails := atomic.Bool{}
	descFails.Store(true)

	stubProbe := func(_ context.Context) error {
		descCalls.Add(1)
		if descFails.Load() {
			return errors.New("dial tcp clickhouse:9440: connect: connection refused")
		}
		// Simulate a healthy CH returning the keys columns. probeSchemaFeatures
		// would normally populate schemaFeatures and render the INSERT — do the
		// same here so the assertion below sees the expected INSERT.
		exp.schemaFeatures.AttributeKeys = true
		exp.renderInsertTracesJSONSQL()
		return nil
	}

	// Simulate start() reaching ensureDetected after the table-create branch.
	// The first probe fails — start() must still return nil (non-fatal).
	err := exp.detector.ensureDetected(t.Context(), exp.logger, stubProbe, nil)
	require.Error(t, err, "first probe must propagate the failure")
	require.ErrorContains(t, err, "schema detection deferred")
	require.Equal(t, int32(1), descCalls.Load())
	require.Equal(t, schemaUnknown, schemaDetectionState(exp.detector.state.Load()))
	require.Empty(t, exp.insertSQL, "INSERT must not be rendered before a successful probe")

	// First batch arrives while ClickHouse is still down: the push helper
	// re-probes, gets the same failure, and returns a retryable error.
	err = exp.detector.ensureDetected(t.Context(), exp.logger, stubProbe, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "schema detection deferred")
	require.Equal(t, int32(2), descCalls.Load())
	require.Empty(t, exp.insertSQL, "INSERT must still not be rendered after a second failed probe")

	// ClickHouse recovers: the next probe succeeds, INSERT is rendered with the
	// keys columns, and subsequent calls reuse the cached INSERT.
	descFails.Store(false)
	require.NoError(t, exp.detector.ensureDetected(t.Context(), exp.logger, stubProbe, nil))
	require.Equal(t, int32(3), descCalls.Load())
	require.Equal(t, schemaDetected, schemaDetectionState(exp.detector.state.Load()))
	require.NotEmpty(t, exp.insertSQL)
	assert.Contains(t, exp.insertSQL, tracesJSONColumnResourceAttributesKeys,
		"INSERT must include ResourceAttributesKeys once detection succeeds")
	assert.Contains(t, exp.insertSQL, tracesJSONColumnSpanAttributesKeys,
		"INSERT must include SpanAttributesKeys once detection succeeds")

	// Further ensureDetected calls are no-ops; the stub is not invoked again.
	require.NoError(t, exp.detector.ensureDetected(t.Context(), exp.logger, stubProbe, nil))
	require.NoError(t, exp.detector.ensureDetected(t.Context(), exp.logger, stubProbe, nil))
	assert.Equal(t, int32(3), descCalls.Load())
}

// TestTracesJSONExporter_RenderInsertOmitsKeysColumnsWhenAbsent verifies the
// non-#48875 path stays correct: when DESC TABLE succeeds and reports that
// the AttributesKeys columns are NOT present, the INSERT must omit them.
func TestTracesJSONExporter_RenderInsertOmitsKeysColumnsWhenAbsent(t *testing.T) {
	cfg := withDefaultConfig()
	cfg.Endpoint = defaultEndpoint
	exp := newTracesJSONExporter(zaptest.NewLogger(t), cfg)
	exp.schemaFeatures.AttributeKeys = false
	exp.renderInsertTracesJSONSQL()

	require.NotEmpty(t, exp.insertSQL)
	assert.NotContains(t, exp.insertSQL, tracesJSONColumnResourceAttributesKeys)
	assert.NotContains(t, exp.insertSQL, tracesJSONColumnSpanAttributesKeys)
}

// makeMinimalTraces returns the smallest possible ptrace.Traces value that can
// exercise pushTraceData without nil-derefs.
func makeMinimalTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3}))
	return td
}

// TestTracesJSONExporter_PushDeferredOnSchemaUnknown verifies the precise
// regression: pushTraceData returns a retryable error (the literal "schema
// detection deferred" wrapping the original DESC failure) when the schema is
// still unknown, so the sending queue defers the batch instead of dropping it.
func TestTracesJSONExporter_PushDeferredOnSchemaUnknown(t *testing.T) {
	cfg := withDefaultConfig()
	cfg.Endpoint = defaultEndpoint
	exp := newTracesJSONExporter(zaptest.NewLogger(t), cfg)
	exp.db = &stubFailingConn{}

	// Detector starts in schemaUnknown. pushTraceData drives ensureDetected →
	// probeSchemaFeatures → GetTableColumns(stubFailingConn) → error path.
	err := exp.pushTraceData(t.Context(), makeMinimalTraces())
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema detection deferred",
		"push must return a retryable error wrapping the DESC failure")
	require.Equal(t, schemaUnknown, schemaDetectionState(exp.detector.state.Load()))
	require.Empty(t, exp.insertSQL)
}

// stubFailingConn is a driver.Conn that returns an error from Query (used by
// internal.GetTableColumns for DESC TABLE) so we can simulate a transient
// ClickHouse outage without standing up a real test container.
type stubFailingConn struct{}

var _ driver.Conn = (*stubFailingConn)(nil)

func (*stubFailingConn) Contributors() []string                        { return nil }
func (*stubFailingConn) ServerVersion() (*driver.ServerVersion, error) { return nil, nil }
func (*stubFailingConn) Select(context.Context, any, string, ...any) error {
	return errors.New("dial tcp clickhouse:9440: connect: connection refused")
}

func (*stubFailingConn) Query(context.Context, string, ...any) (driver.Rows, error) {
	return nil, errors.New("dial tcp clickhouse:9440: connect: connection refused")
}
func (*stubFailingConn) QueryRow(context.Context, string, ...any) driver.Row { return nil }
func (*stubFailingConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, errors.New("dial tcp clickhouse:9440: connect: connection refused")
}
func (*stubFailingConn) Exec(context.Context, string, ...any) error { return nil }
func (*stubFailingConn) AsyncInsert(context.Context, string, bool, ...any) error {
	return nil
}
func (*stubFailingConn) Ping(context.Context) error { return nil }
func (*stubFailingConn) Stats() driver.Stats        { return driver.Stats{} }
func (*stubFailingConn) Close() error               { return nil }

// stubAccessDeniedConn is a driver.Conn that returns a permanent ACCESS_DENIED
// exception from Query, simulating a write-only ClickHouse user that has
// INSERT privilege but not SELECT/DESC.
type stubAccessDeniedConn struct {
	stubFailingConn
}

func (*stubAccessDeniedConn) Query(context.Context, string, ...any) (driver.Rows, error) {
	return nil, &proto.Exception{
		Code:    chCodeAccessDenied,
		Name:    "ACCESS_DENIED",
		Message: "Not enough privileges. To execute this query, it's necessary to have the grant SELECT(name) ON system.columns",
	}
}

func (*stubAccessDeniedConn) PrepareBatch(_ context.Context, _ string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	return noopBatch{}, nil
}

// TestTracesJSONExporter_PermanentFailureFallsBack verifies that a write-only
// ClickHouse user (INSERT granted, SELECT/DESC denied) does not lose data.
// When DESC TABLE returns ACCESS_DENIED the exporter must fall back to a
// degraded INSERT (without the optional keys columns) and deliver rows.
func TestTracesJSONExporter_PermanentFailureFallsBack(t *testing.T) {
	cfg := withDefaultConfig()
	cfg.Endpoint = defaultEndpoint
	exp := newTracesJSONExporter(zaptest.NewLogger(t), cfg)
	exp.db = &stubAccessDeniedConn{}

	// pushTraceData drives ensureDetected → probeSchemaFeatures → ACCESS_DENIED
	// → fallback renders degraded INSERT → push proceeds.
	err := exp.pushTraceData(t.Context(), makeMinimalTraces())
	require.NoError(t, err, "push must succeed after falling back to degraded INSERT")
	require.Equal(t, schemaDetected, schemaDetectionState(exp.detector.state.Load()))
	require.NotEmpty(t, exp.insertSQL, "INSERT must be rendered even after permanent DESC failure")
	assert.NotContains(t, exp.insertSQL, tracesJSONColumnResourceAttributesKeys,
		"degraded INSERT must NOT include keys columns")
	assert.NotContains(t, exp.insertSQL, tracesJSONColumnSpanAttributesKeys,
		"degraded INSERT must NOT include keys columns")

	// Subsequent pushes must succeed without re-probing.
	require.NoError(t, exp.pushTraceData(t.Context(), makeMinimalTraces()))
}
