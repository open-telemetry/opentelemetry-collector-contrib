// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func buildLogsProcessor(t *testing.T, cfg *Config, next consumer.Logs) processor.Logs {
	t.Helper()
	p, err := createLogsProcessor(t.Context(), nopSettings(), cfg, next)
	require.NoError(t, err)
	return p
}

func buildTracesProcessor(t *testing.T, cfg *Config, next consumer.Traces) processor.Traces {
	t.Helper()
	p, err := createTracesProcessor(t.Context(), nopSettings(), cfg, next)
	require.NoError(t, err)
	return p
}

func TestConsumeLogs_SinglePartition_OneCall(t *testing.T) {
	sink := &consumertest.LogsSink{}
	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, sink)

	logs := plog.NewLogs()
	for range 3 {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("tenant.id", "t1")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log")
	}

	require.NoError(t, proc.ConsumeLogs(t.Context(), logs))
	assert.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 3, sink.AllLogs()[0].ResourceLogs().Len())
}

func TestConsumeLogs_MultiplePartitions_NCalls(t *testing.T) {
	sink := &consumertest.LogsSink{}
	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, sink)

	logs := plog.NewLogs()
	for _, tenant := range []string{"t1", "t2", "t1"} {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("tenant.id", tenant)
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log")
	}

	require.NoError(t, proc.ConsumeLogs(t.Context(), logs))
	require.Len(t, sink.AllLogs(), 2)

	counts := make(map[int]bool)
	for _, l := range sink.AllLogs() {
		counts[l.ResourceLogs().Len()] = true
	}
	assert.True(t, counts[1])
	assert.True(t, counts[2])
}

func TestConsumeLogs_AddsMetadata(t *testing.T) {
	var mu sync.Mutex
	var gotCtxs []context.Context
	next := &capturingLogsConsumer{fn: func(ctx context.Context, _ plog.Logs) error {
		mu.Lock()
		gotCtxs = append(gotCtxs, ctx)
		mu.Unlock()
		return nil
	}}

	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, next)

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("tenant.id", "acme")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	require.NoError(t, proc.ConsumeLogs(t.Context(), logs))
	require.Len(t, gotCtxs, 1)
	md := client.FromContext(gotCtxs[0]).Metadata
	assert.Equal(t, []string{"acme"}, md.Get("tenant_id"))
}

func TestConsumeLogs_PreservesExistingMetadata(t *testing.T) {
	var gotMD client.Metadata
	next := &capturingLogsConsumer{fn: func(ctx context.Context, _ plog.Logs) error {
		gotMD = client.FromContext(ctx).Metadata
		return nil
	}}

	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, next)

	incomingCtx := client.NewContext(t.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{
			"x-existing-header": {"some-value"},
		}),
	})

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("tenant.id", "acme")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	require.NoError(t, proc.ConsumeLogs(incomingCtx, logs))
	assert.Equal(t, []string{"some-value"}, gotMD.Get("x-existing-header"), "existing metadata preserved")
	assert.Equal(t, []string{"acme"}, gotMD.Get("tenant_id"), "partition key added")
}

func TestConsumeLogs_NilValueOmitsMetadataKey(t *testing.T) {
	var gotMD client.Metadata
	next := &capturingLogsConsumer{fn: func(ctx context.Context, _ plog.Logs) error {
		gotMD = client.FromContext(ctx).Metadata
		return nil
	}}

	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["missing"]`,
	}}, next)

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("other", "value")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	require.NoError(t, proc.ConsumeLogs(t.Context(), logs))
	assert.Empty(t, gotMD.Get("tenant_id"), "nil OTTL result must not add key to metadata")
}

func TestConsumeLogs_EmptyInput_NoCalls(t *testing.T) {
	sink := &consumertest.LogsSink{}
	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, sink)

	require.NoError(t, proc.ConsumeLogs(t.Context(), plog.NewLogs()))
	assert.Empty(t, sink.AllLogs())
}

func TestConsumeLogs_DownstreamErrorPropagates(t *testing.T) {
	wantErr := errors.New("downstream error")
	next := &capturingLogsConsumer{fn: func(_ context.Context, _ plog.Logs) error {
		return wantErr
	}}

	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, next)

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("tenant.id", "t1")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err := proc.ConsumeLogs(t.Context(), logs)
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestConsumeLogs_ConcurrentDispatch(t *testing.T) {
	var (
		mu          sync.Mutex
		inFlight    int
		maxInFlight int
		gate        = make(chan struct{})
	)

	next := &capturingLogsConsumer{fn: func(_ context.Context, _ plog.Logs) error {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()

		<-gate

		mu.Lock()
		inFlight--
		mu.Unlock()
		return nil
	}}

	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, next)

	logs := plog.NewLogs()
	for _, tenant := range []string{"t1", "t2"} {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("tenant.id", tenant)
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	}

	done := make(chan error, 1)
	go func() {
		done <- proc.ConsumeLogs(t.Context(), logs)
	}()

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return inFlight == 2
	}, testTimeout, testPoll)

	close(gate)
	require.NoError(t, <-done)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 2, maxInFlight, "both partitions must be dispatched concurrently")
}

func TestConsumeLogs_DownstreamErrorDoesNotCancelSiblings(t *testing.T) {
	errT1 := errors.New("t1 failed")
	errT3 := errors.New("t3 failed")

	var (
		mu        sync.Mutex
		delivered []string
		ctxs      []context.Context
	)
	next := &capturingLogsConsumer{fn: func(ctx context.Context, _ plog.Logs) error {
		tenant := client.FromContext(ctx).Metadata.Get("tenant_id")[0]
		mu.Lock()
		delivered = append(delivered, tenant)
		ctxs = append(ctxs, ctx)
		mu.Unlock()
		switch tenant {
		case "t1":
			return errT1
		case "t3":
			return errT3
		}
		return nil
	}}

	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, next)

	logs := plog.NewLogs()
	for _, tenant := range []string{"t1", "t2", "t3"} {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("tenant.id", tenant)
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	}

	err := proc.ConsumeLogs(t.Context(), logs)
	require.Error(t, err)
	assert.ErrorIs(t, err, errT1)
	assert.ErrorIs(t, err, errT3)
	assert.ElementsMatch(t, []string{"t1", "t2", "t3"}, delivered)
	for _, ctx := range ctxs {
		assert.NoError(t, ctx.Err(), "downstream context must not be canceled after ConsumeLogs returns")
	}
}

func TestConsumeLogs_ContextNotCanceledAfterSuccess(t *testing.T) {
	var ctxs []context.Context
	var mu sync.Mutex
	next := &capturingLogsConsumer{fn: func(ctx context.Context, _ plog.Logs) error {
		mu.Lock()
		ctxs = append(ctxs, ctx)
		mu.Unlock()
		return nil
	}}
	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, next)

	logs := plog.NewLogs()
	for _, tenant := range []string{"t1", "t2"} {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("tenant.id", tenant)
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	}
	require.NoError(t, proc.ConsumeLogs(t.Context(), logs))
	require.Len(t, ctxs, 2)
	for _, ctx := range ctxs {
		assert.NoError(t, ctx.Err())
	}
}

func TestWithPartitionMetadata_NilValueRemovesExistingKey(t *testing.T) {
	ctx := client.NewContext(t.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{
			"tenant_id": {"inbound-value"},
			"x-other":   {"preserved"},
		}),
	})

	info := client.FromContext(ctx)
	keyNames := []string{"tenant_id"}
	out := client.FromContext(withPartitionMetadata(ctx, info, baseMetadata(info.Metadata, keyNames), keyNames, []keyValue{{isNil: true}})).Metadata
	assert.Empty(t, out.Get("tenant_id"), "nil partition value should remove colliding inbound metadata")
	assert.Equal(t, []string{"preserved"}, out.Get("x-other"))
}

const (
	testTimeout = 5e9 // 5 seconds
	testPoll    = 1e6 // 1 ms
)

type capturingLogsConsumer struct {
	fn func(ctx context.Context, ld plog.Logs) error
}

func (*capturingLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *capturingLogsConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return c.fn(ctx, ld)
}

func TestConsumeLogs_NonStringKeyIsPermanentError(t *testing.T) {
	proc := buildLogsProcessor(t, &Config{Keys: map[string]string{
		"count": `resource.attributes["count"]`,
	}}, consumertest.NewNop())

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutInt("count", 1)
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err := proc.ConsumeLogs(t.Context(), logs)
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
}

func TestConsumeLogs_RecordsProcessorTelemetry(t *testing.T) {
	tel := componenttest.NewTelemetry()
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()
	set := nopSettings()
	set.TelemetrySettings = tel.NewTelemetrySettings()

	sink := &consumertest.LogsSink{}
	proc, err := createLogsProcessor(t.Context(), set, validConfig(), sink)
	require.NoError(t, err)

	logs := plog.NewLogs()
	for _, tenant := range []string{"t1", "t2", "t1"} {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("tenant.id", tenant)
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	}
	require.NoError(t, proc.ConsumeLogs(t.Context(), logs))
	assert.Len(t, sink.AllLogs(), 2)

	for _, name := range []string{"otelcol_processor_incoming_items", "otelcol_processor_outgoing_items"} {
		m, err := tel.GetMetric(name)
		require.NoError(t, err, name)
		sum, ok := m.Data.(metricdata.Sum[int64])
		require.True(t, ok, name)
		require.Len(t, sum.DataPoints, 1, name)
		assert.Equal(t, int64(3), sum.DataPoints[0].Value, name)
	}
}

func TestConsumeTraces_Basic(t *testing.T) {
	var (
		calls int
		gotMD client.Metadata
	)
	next := &capturingTracesConsumer{fn: func(ctx context.Context, _ ptrace.Traces) error {
		calls++
		gotMD = client.FromContext(ctx).Metadata
		return nil
	}}

	proc := buildTracesProcessor(t, &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}, next)

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("tenant.id", "acme")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Spans().AppendEmpty().SetName("op")

	require.NoError(t, proc.ConsumeTraces(t.Context(), traces))
	assert.Equal(t, 1, calls)
	assert.Equal(t, []string{"acme"}, gotMD.Get("tenant_id"))
}

func TestConsumeTraces_OTelColContext_ClientMetadata(t *testing.T) {
	var (
		calls int
		gotMD client.Metadata
	)
	next := &capturingTracesConsumer{fn: func(ctx context.Context, _ ptrace.Traces) error {
		calls++
		gotMD = client.FromContext(ctx).Metadata
		return nil
	}}

	proc := buildTracesProcessor(t, &Config{Keys: map[string]string{
		"trace_topic": `otelcol.client.metadata["x-tenant-id"][0]`,
	}}, next)

	ctx := client.NewContext(t.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{
			"x-tenant-id": {"acme"},
		}),
	})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("op")

	require.NoError(t, proc.ConsumeTraces(ctx, traces))
	assert.Equal(t, 1, calls)
	assert.Equal(t, []string{"acme"}, gotMD.Get("x-tenant-id"))
	assert.Equal(t, []string{"acme"}, gotMD.Get("trace_topic"))
}

func TestConsumeTraces_NonStringKeyIsPermanentError(t *testing.T) {
	proc := buildTracesProcessor(t, &Config{Keys: map[string]string{
		"count": `resource.attributes["count"]`,
	}}, consumertest.NewNop())

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutInt("count", 1)
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	err := proc.ConsumeTraces(t.Context(), traces)
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
}

func TestConsumeTraces_RecordsProcessorTelemetry(t *testing.T) {
	tel := componenttest.NewTelemetry()
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()
	set := nopSettings()
	set.TelemetrySettings = tel.NewTelemetrySettings()

	sink := &consumertest.TracesSink{}
	proc, err := createTracesProcessor(t.Context(), set, validConfig(), sink)
	require.NoError(t, err)

	traces := ptrace.NewTraces()
	for _, tenant := range []string{"t1", "t2", "t1"} {
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("tenant.id", tenant)
		rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	}
	require.NoError(t, proc.ConsumeTraces(t.Context(), traces))
	assert.Len(t, sink.AllTraces(), 2)

	for _, name := range []string{"otelcol_processor_incoming_items", "otelcol_processor_outgoing_items"} {
		m, err := tel.GetMetric(name)
		require.NoError(t, err, name)
		sum, ok := m.Data.(metricdata.Sum[int64])
		require.True(t, ok, name)
		require.Len(t, sum.DataPoints, 1, name)
		assert.Equal(t, int64(3), sum.DataPoints[0].Value, name)
	}
}

type capturingTracesConsumer struct {
	fn func(ctx context.Context, td ptrace.Traces) error
}

func (*capturingTracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *capturingTracesConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return c.fn(ctx, td)
}
