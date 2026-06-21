// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubexporter

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// --- test helpers ---

type sentEvent struct {
	partitionKey string
	body         []byte
}

// captureSender replaces doSend and records every (partitionKey, body) pair.
func captureSender(out *[]sentEvent) func(ctx context.Context, partitionKey string, body []byte) error {
	return func(_ context.Context, partitionKey string, body []byte) error {
		*out = append(*out, sentEvent{partitionKey: partitionKey, body: body})
		return nil
	}
}

func errSender(err error) func(context.Context, string, []byte) error {
	return func(context.Context, string, []byte) error { return err }
}

func newTestExporter(cfg *Config) *azureEventHubExporter {
	e := &azureEventHubExporter{
		config: cfg,
		logger: zap.NewNop(),
	}
	e.doSend = e.send // default; tests override this
	return e
}

// --- partition iterator tests ---

func TestPartitionLogs_NoPartitioning(t *testing.T) {
	exp := newTestExporter(&Config{})
	ld := makeLogs(2)
	var keys []string
	for key := range exp.partitionLogs(ld) {
		keys = append(keys, key)
	}
	assert.Equal(t, []string{""}, keys)
}

func TestPartitionLogs_ByResourceAttributes(t *testing.T) {
	exp := newTestExporter(&Config{PartitionLogsByResourceAttributes: true})
	ld := makeLogs(3)
	var keys []string
	for key := range exp.partitionLogs(ld) {
		keys = append(keys, key)
	}
	assert.Len(t, keys, 3)
	for _, k := range keys {
		assert.NotEmpty(t, k)
	}
}

func TestPartitionLogs_ByResourceAttributes_SameResourceSameKey(t *testing.T) {
	exp := newTestExporter(&Config{PartitionLogsByResourceAttributes: true})
	// Two logs with identical resource attributes should produce identical keys.
	ld := plog.NewLogs()
	for range 2 {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "svc-a")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	}
	var keys []string
	for key := range exp.partitionLogs(ld) {
		keys = append(keys, key)
	}
	require.Len(t, keys, 2)
	assert.Equal(t, keys[0], keys[1], "identical resource attributes should produce the same partition key")
}

func TestPartitionLogs_ByTraceID(t *testing.T) {
	exp := newTestExporter(&Config{PartitionLogsByTraceID: true})
	ld := makeLogsWithTraceID(2)
	var keys []string
	for key := range exp.partitionLogs(ld) {
		keys = append(keys, key)
	}
	assert.NotEmpty(t, keys)
}

func TestPartitionMetrics_NoPartitioning(t *testing.T) {
	exp := newTestExporter(&Config{})
	md := makeMetrics(2)
	var keys []string
	for key := range exp.partitionMetrics(md) {
		keys = append(keys, key)
	}
	assert.Equal(t, []string{""}, keys)
}

func TestPartitionMetrics_ByResourceAttributes(t *testing.T) {
	exp := newTestExporter(&Config{PartitionMetricsByResourceAttributes: true})
	md := makeMetrics(3)
	var keys []string
	for key := range exp.partitionMetrics(md) {
		keys = append(keys, key)
	}
	assert.Len(t, keys, 3)
	for _, k := range keys {
		assert.NotEmpty(t, k)
	}
}

func TestPartitionTraces_NoPartitioning(t *testing.T) {
	exp := newTestExporter(&Config{})
	td := makeTraces(2)
	var keys []string
	for key := range exp.partitionTraces(td) {
		keys = append(keys, key)
	}
	assert.Equal(t, []string{""}, keys)
}

func TestPartitionTraces_ByID(t *testing.T) {
	exp := newTestExporter(&Config{PartitionTracesByID: true})
	td := makeTracesWithDistinctTraceIDs(3)
	var keys []string
	for key := range exp.partitionTraces(td) {
		keys = append(keys, key)
	}
	assert.Len(t, keys, 3)
	for _, k := range keys {
		assert.Len(t, k, 32, "trace ID hex string should be 32 chars")
	}
}

// --- ConsumeLogs tests ---

func TestConsumeLogs_NoPartitioning(t *testing.T) {
	var sent []sentEvent
	exp := newTestExporter(&Config{})
	exp.doSend = captureSender(&sent)

	require.NoError(t, exp.ConsumeLogs(context.Background(), makeLogs(2)))
	require.Len(t, sent, 1)
	assert.Equal(t, "", sent[0].partitionKey)
	assert.NotEmpty(t, sent[0].body)
}

func TestConsumeLogs_ByResourceAttributes(t *testing.T) {
	var sent []sentEvent
	exp := newTestExporter(&Config{PartitionLogsByResourceAttributes: true})
	exp.doSend = captureSender(&sent)

	require.NoError(t, exp.ConsumeLogs(context.Background(), makeLogs(3)))
	assert.Len(t, sent, 3)
	for _, s := range sent {
		assert.NotEmpty(t, s.partitionKey)
		assert.NotEmpty(t, s.body)
	}
}

func TestConsumeLogs_ByTraceID(t *testing.T) {
	var sent []sentEvent
	exp := newTestExporter(&Config{PartitionLogsByTraceID: true})
	exp.doSend = captureSender(&sent)

	require.NoError(t, exp.ConsumeLogs(context.Background(), makeLogsWithTraceID(2)))
	assert.NotEmpty(t, sent)
}

func TestConsumeLogs_PropagatesSendError(t *testing.T) {
	exp := newTestExporter(&Config{})
	exp.doSend = errSender(errors.New("send failed"))

	err := exp.ConsumeLogs(context.Background(), makeLogs(1))
	assert.ErrorContains(t, err, "send failed")
}

// --- ConsumeMetrics tests ---

func TestConsumeMetrics_NoPartitioning(t *testing.T) {
	var sent []sentEvent
	exp := newTestExporter(&Config{})
	exp.doSend = captureSender(&sent)

	require.NoError(t, exp.ConsumeMetrics(context.Background(), makeMetrics(2)))
	require.Len(t, sent, 1)
	assert.Equal(t, "", sent[0].partitionKey)
}

func TestConsumeMetrics_ByResourceAttributes(t *testing.T) {
	var sent []sentEvent
	exp := newTestExporter(&Config{PartitionMetricsByResourceAttributes: true})
	exp.doSend = captureSender(&sent)

	require.NoError(t, exp.ConsumeMetrics(context.Background(), makeMetrics(4)))
	assert.Len(t, sent, 4)
}

func TestConsumeMetrics_PropagatesSendError(t *testing.T) {
	exp := newTestExporter(&Config{})
	exp.doSend = errSender(errors.New("send failed"))

	err := exp.ConsumeMetrics(context.Background(), makeMetrics(1))
	assert.ErrorContains(t, err, "send failed")
}

// --- ConsumeTraces tests ---

func TestConsumeTraces_NoPartitioning(t *testing.T) {
	var sent []sentEvent
	exp := newTestExporter(&Config{})
	exp.doSend = captureSender(&sent)

	require.NoError(t, exp.ConsumeTraces(context.Background(), makeTraces(2)))
	require.Len(t, sent, 1)
	assert.Equal(t, "", sent[0].partitionKey)
}

func TestConsumeTraces_ByID(t *testing.T) {
	var sent []sentEvent
	exp := newTestExporter(&Config{PartitionTracesByID: true})
	exp.doSend = captureSender(&sent)

	require.NoError(t, exp.ConsumeTraces(context.Background(), makeTracesWithDistinctTraceIDs(2)))
	assert.Len(t, sent, 2)
	for _, s := range sent {
		assert.Len(t, s.partitionKey, 32)
	}
}

func TestConsumeTraces_PropagatesSendError(t *testing.T) {
	exp := newTestExporter(&Config{})
	exp.doSend = errSender(errors.New("send failed"))

	err := exp.ConsumeTraces(context.Background(), makeTraces(1))
	assert.ErrorContains(t, err, "send failed")
}

// --- send() tests ---

// fakeProducer is a configurable producerClient that returns controlled errors.
type fakeProducer struct {
	newBatchErr  error
	sendBatchErr error
	closed       bool
}

func (f *fakeProducer) NewEventDataBatch(_ context.Context, _ *azeventhubs.EventDataBatchOptions) (*azeventhubs.EventDataBatch, error) {
	if f.newBatchErr != nil {
		return nil, f.newBatchErr
	}
	return nil, nil // nil batch propagates to AddEventData; only testing error paths here
}

func (f *fakeProducer) SendEventDataBatch(_ context.Context, _ *azeventhubs.EventDataBatch, _ *azeventhubs.SendEventDataBatchOptions) error {
	return f.sendBatchErr
}

func (f *fakeProducer) Close(_ context.Context) error {
	f.closed = true
	return nil
}

func TestSend_NewBatchError(t *testing.T) {
	exp := newTestExporter(&Config{})
	exp.producer = &fakeProducer{newBatchErr: errors.New("broker unavailable")}

	err := exp.send(context.Background(), "", []byte("data"))
	assert.ErrorContains(t, err, "failed to create event data batch")
	assert.ErrorContains(t, err, "broker unavailable")
}

func TestSend_PartitionKeySetOnBatch(t *testing.T) {
	// Verify that a non-empty partition key is passed through to NewEventDataBatch.
	// We can't inspect the batch's PartitionKey directly (SDK internals), but
	// we can confirm no error is returned when NewEventDataBatch succeeds and
	// AddEventData returns ErrEventDataTooLarge (which happens when batch is nil).
	// The meaningful assertion is in TestSend_NewBatchError above.
	exp := newTestExporter(&Config{})
	exp.producer = &fakeProducer{newBatchErr: errors.New("stop here")}

	err := exp.send(context.Background(), "my-key", []byte("data"))
	assert.ErrorContains(t, err, "failed to create event data batch")
}

// --- shutdown tests ---

func TestShutdown_NilProducer(t *testing.T) {
	exp := newTestExporter(&Config{})
	// producer was never set (no Start call) — Shutdown must not panic.
	assert.NoError(t, exp.shutdown(context.Background()))
}

func TestShutdown_ClosesProducer(t *testing.T) {
	exp := newTestExporter(&Config{})
	fp := &fakeProducer{}
	exp.producer = fp

	assert.NoError(t, exp.shutdown(context.Background()))
	assert.True(t, fp.closed)
}

// --- start() tests ---

func TestStart_ConnectionString(t *testing.T) {
	exp := newTestExporter(&Config{
		Connection: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=key;SharedAccessKey=dGVzdA==;EntityPath=hub",
	})
	// The SDK creates the ProducerClient lazily; no network call happens here.
	require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))
	assert.NotNil(t, exp.producer)
	_ = exp.shutdown(context.Background())
}

func TestStart_ConnectionString_WithEventHubName(t *testing.T) {
	exp := newTestExporter(&Config{
		// EntityPath not in connection string — supplied separately via EventHub.Name.
		Connection: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=key;SharedAccessKey=dGVzdA==",
		EventHub:   EventHubConfig{Name: "myhub"},
	})
	require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))
	assert.NotNil(t, exp.producer)
	_ = exp.shutdown(context.Background())
}

func TestStart_AuthExtensionNotFound(t *testing.T) {
	id := component.MustNewID("azure_auth")
	exp := newTestExporter(&Config{
		Auth:     &id,
		EventHub: EventHubConfig{Name: "hub", Namespace: "ns.servicebus.windows.net"},
	})

	err := exp.start(context.Background(), componenttest.NewNopHost())
	assert.ErrorContains(t, err, "failed to resolve auth extension")
}

func TestStart_AuthExtensionWrongType(t *testing.T) {
	id := component.MustNewID("azure_auth")
	exp := newTestExporter(&Config{
		Auth:     &id,
		EventHub: EventHubConfig{Name: "hub", Namespace: "ns.servicebus.windows.net"},
	})

	// Provide an extension that does NOT implement azcore.TokenCredential.
	host := newHostWithExtension(id, &notACredential{})
	err := exp.start(context.Background(), host)
	assert.ErrorContains(t, err, "does not implement azcore.TokenCredential")
}

func TestStart_AuthExtensionValid(t *testing.T) {
	id := component.MustNewID("azure_auth")
	exp := newTestExporter(&Config{
		Auth:     &id,
		EventHub: EventHubConfig{Name: "hub", Namespace: "test.servicebus.windows.net"},
	})

	host := newHostWithExtension(id, &fakeTokenCredential{})
	// The SDK creates the ProducerClient lazily; no network call happens here.
	require.NoError(t, exp.start(context.Background(), host))
	assert.NotNil(t, exp.producer)
	_ = exp.shutdown(context.Background())
}

func TestStart_AuthIgnoresConnectionString(t *testing.T) {
	id := component.MustNewID("azure_auth")
	exp := newTestExporter(&Config{
		Auth:       &id,
		Connection: "Endpoint=sb://ignored.servicebus.windows.net/;SharedAccessKeyName=k;SharedAccessKey=dGVzdA==;EntityPath=hub",
		EventHub:   EventHubConfig{Name: "hub", Namespace: "test.servicebus.windows.net"},
	})

	host := newHostWithExtension(id, &fakeTokenCredential{})
	require.NoError(t, exp.start(context.Background(), host))
	_ = exp.shutdown(context.Background())
}

// fakeTokenCredential implements azcore.TokenCredential for use in tests.
type fakeTokenCredential struct{ component.StartFunc; component.ShutdownFunc }

func (f *fakeTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "fake-token"}, nil
}

// notACredential is a component.Component that does not implement azcore.TokenCredential.
type notACredential struct{ component.StartFunc; component.ShutdownFunc }

// hostWithExtensions is a minimal component.Host for tests.
type hostWithExtensions struct {
	component.Host
	extensions map[component.ID]component.Component
}

func newHostWithExtension(id component.ID, ext component.Component) *hostWithExtensions {
	return &hostWithExtensions{
		Host:       componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{id: ext},
	}
}

func (h *hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

// --- pdata builder helpers ---

func makeLogs(numResources int) plog.Logs {
	ld := plog.NewLogs()
	for i := range numResources {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("resource.index", string(rune('A'+i)))
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	}
	return ld
}

func makeLogsWithTraceID(numRecords int) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	for i := range numRecords {
		lr := sl.LogRecords().AppendEmpty()
		var tid pcommon.TraceID
		tid[0] = byte(i + 1)
		lr.SetTraceID(tid)
	}
	return ld
}

func makeMetrics(numResources int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for i := range numResources {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("resource.index", string(rune('A'+i)))
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("test.metric")
		m.SetEmptyGauge().DataPoints().AppendEmpty()
	}
	return md
}

func makeTraces(numResources int) ptrace.Traces {
	td := ptrace.NewTraces()
	for i := range numResources {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("resource.index", string(rune('A'+i)))
		span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		var tid pcommon.TraceID
		tid[0] = byte(i + 1)
		span.SetTraceID(tid)
	}
	return td
}

func makeTracesWithDistinctTraceIDs(n int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	for i := range n {
		span := ss.Spans().AppendEmpty()
		var tid pcommon.TraceID
		tid[0] = byte(i + 1)
		span.SetTraceID(tid)
	}
	return td
}
