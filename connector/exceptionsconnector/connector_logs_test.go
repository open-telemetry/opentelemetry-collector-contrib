// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exceptionsconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestConnectorLogConsumeTraces(t *testing.T) {
	traces := []ptrace.Traces{buildSampleTrace()}
	lsink := new(consumertest.LogsSink)

	p := newTestLogsConnector(lsink, zaptest.NewLogger(t))

	ctx := metadata.NewIncomingContext(t.Context(), nil)
	err := p.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
	require.NoError(t, err)

	expectedLogs, err := golden.ReadLogs("testdata/logs.yml")
	require.NoError(t, err)

	for _, traces := range traces {
		err = p.ConsumeTraces(ctx, traces)
		assert.NoError(t, err)

		logs := lsink.AllLogs()
		assert.Len(t, logs, 1)
		err = plogtest.CompareLogs(expectedLogs, logs[len(logs)-1])
		assert.NoError(t, err)
	}
}

func TestConnectorLogConsumeTracesPropagatesSpanMetadata(t *testing.T) {
	lsink := new(consumertest.LogsSink)
	c := newTestLogsConnector(lsink, zaptest.NewLogger(t))

	traces := ptrace.NewTraces()
	rspans := traces.ResourceSpans().AppendEmpty()
	rspans.SetSchemaUrl("https://opentelemetry.io/schemas/1.31.0")
	rspans.Resource().Attributes().PutStr(serviceNameKey, "service-a")
	ils := rspans.ScopeSpans().AppendEmpty()
	ils.SetSchemaUrl("https://opentelemetry.io/schemas/1.31.0/scope")

	span := ils.Spans().AppendEmpty()
	span.SetName("op")
	// 0x101: sampled bit (0x01) plus a span-only bit (0x100, e.g. "context is remote") that has
	// no meaning on LogRecordFlags and must not leak into it.
	span.SetFlags(0x101)
	exc := span.Events().AppendEmpty()
	exc.SetName(eventNameExc)
	exc.Attributes().PutStr(exceptionTypeKey, "boom")

	require.NoError(t, c.ConsumeTraces(t.Context(), traces))

	out := lsink.AllLogs()
	require.Len(t, out, 1)
	outRL := out[0].ResourceLogs().At(0)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.31.0", outRL.SchemaUrl())
	assert.Equal(t, "https://opentelemetry.io/schemas/1.31.0/scope", outRL.ScopeLogs().At(0).SchemaUrl())

	lr := outRL.ScopeLogs().At(0).LogRecords().At(0)
	assert.True(t, lr.Flags().IsSampled())
	assert.Equal(t, plog.LogRecordFlags(0x1), lr.Flags(), "only the low trace-flags byte must be copied")
}

func TestConnectorLogConsumeTracesRequiredAttrsIndependentOfDimensions(t *testing.T) {
	lsink := new(consumertest.LogsSink)
	// Empty dimensions: exception.type/message must still appear (unlike newTestLogsConnector's default config).
	c := newLogsConnector(zaptest.NewLogger(t), &Config{})
	c.logsConsumer = lsink

	require.NoError(t, c.ConsumeTraces(t.Context(), buildSampleTrace()))

	logs := lsink.AllLogs()
	require.Len(t, logs, 1)
	rl := logs[0].ResourceLogs().At(0)
	lr := rl.ScopeLogs().At(0).LogRecords().At(0)

	for _, key := range []string{exceptionTypeKey, exceptionMessageKey, exceptionStacktraceKey} {
		v, ok := lr.Attributes().Get(key)
		require.Truef(t, ok, "%s must be present regardless of the dimensions config", key)
		assert.NotEmpty(t, v.Str())
	}
}

func TestConnectorLogsConsumeLogsFiltersNonExceptions(t *testing.T) {
	lsink := new(consumertest.LogsSink)
	c := newTestLogsConnector(lsink, zaptest.NewLogger(t))

	logs := plog.NewLogs()

	// ResourceLogs #0: exception + plain record - only the exception survives, resource attrs kept.
	rl0 := logs.ResourceLogs().AppendEmpty()
	rl0.SetSchemaUrl("https://opentelemetry.io/schemas/1.31.0")
	rl0.Resource().Attributes().PutStr(serviceNameKey, "service-a")
	sl0 := rl0.ScopeLogs().AppendEmpty()
	sl0.SetSchemaUrl("https://opentelemetry.io/schemas/1.31.0/scope")
	sl0.Scope().SetName("scope-a")

	exc := sl0.LogRecords().AppendEmpty()
	exc.SetEventName(eventNameExc)
	exc.Attributes().PutStr(exceptionTypeKey, "java.lang.NullPointerException")

	other := sl0.LogRecords().AppendEmpty()
	other.SetEventName("request")
	other.Body().SetStr("just a regular log line")

	// ResourceLogs #1: no exceptions - must be dropped entirely, not forwarded as an empty group.
	rl1 := logs.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr(serviceNameKey, "service-b")
	sl1 := rl1.ScopeLogs().AppendEmpty()
	sl1.LogRecords().AppendEmpty().Body().SetStr("nothing to see here")

	require.NoError(t, c.ConsumeLogs(t.Context(), logs))

	out := lsink.AllLogs()
	require.Len(t, out, 1)
	outLogs := out[0]

	require.Equal(t, 1, outLogs.ResourceLogs().Len(), "resource groups with no exceptions must be dropped")
	outRL := outLogs.ResourceLogs().At(0)
	v, ok := outRL.Resource().Attributes().Get(serviceNameKey)
	require.True(t, ok)
	assert.Equal(t, "service-a", v.Str())
	assert.Equal(t, "https://opentelemetry.io/schemas/1.31.0", outRL.SchemaUrl())

	require.Equal(t, 1, outRL.ScopeLogs().Len())
	outSL := outRL.ScopeLogs().At(0)
	assert.Equal(t, "scope-a", outSL.Scope().Name())
	assert.Equal(t, "https://opentelemetry.io/schemas/1.31.0/scope", outSL.SchemaUrl())

	outRecords := outSL.LogRecords()
	require.Equal(t, 1, outRecords.Len(), "only the exception record should be forwarded")
	assert.Equal(t, eventNameExc, outRecords.At(0).EventName())
	tv, ok := outRecords.At(0).Attributes().Get(exceptionTypeKey)
	require.True(t, ok)
	assert.Equal(t, "java.lang.NullPointerException", tv.Str())
}

func newTestLogsConnector(lcon consumer.Logs, logger *zap.Logger) *logsConnector {
	cfg := &Config{
		Dimensions: []Dimension{
			{Name: exceptionTypeKey},
			{Name: exceptionMessageKey},
		},
	}

	c := newLogsConnector(logger, cfg)
	c.logsConsumer = lcon
	return c
}
