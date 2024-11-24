// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"testing"
	"time"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

type mockHubWrapper struct{}

func (m mockHubWrapper) GetRuntimeInformation(_ context.Context) (*eventhub.HubRuntimeInformation, error) {
	return &eventhub.HubRuntimeInformation{
		Path:           "foo",
		CreatedAt:      time.Now(),
		PartitionCount: 1,
		PartitionIDs:   []string{"foo"},
	}, nil
}

func (m mockHubWrapper) Receive(ctx context.Context, _ string, _ eventhub.Handler, _ ...eventhub.ReceiveOption) (listerHandleWrapper, error) {
	return &mockListenerHandleWrapper{
		ctx: ctx,
	}, nil
}

func (m mockHubWrapper) Close(_ context.Context) error {
	return nil
}

type mockListenerHandleWrapper struct {
	ctx context.Context
}

func (m *mockListenerHandleWrapper) Done() <-chan struct{} {
	return m.ctx.Done()
}

func (m mockListenerHandleWrapper) Err() error {
	return nil
}

type mockDataConsumer struct {
	logsUnmarshaler    eventLogsUnmarshaler
	nextLogsConsumer   consumer.Logs
	nextTracesConsumer consumer.Traces
	obsrecv            *receiverhelper.ObsReport
}

func (m *mockDataConsumer) setNextLogsConsumer(nextLogsConsumer consumer.Logs) {
	m.nextLogsConsumer = nextLogsConsumer
}

func (m *mockDataConsumer) setNextTracesConsumer(nextTracesConsumer consumer.Traces) {
	m.nextTracesConsumer = nextTracesConsumer
}

func (m *mockDataConsumer) setNextMetricsConsumer(_ consumer.Metrics) {}

func (m *mockDataConsumer) consume(ctx context.Context, event *eventhub.Event) error {
	logsContext := m.obsrecv.StartLogsOp(ctx)

	logs, err := m.logsUnmarshaler.UnmarshalLogs(event)
	if err != nil {
		return err
	}

	err = m.nextLogsConsumer.ConsumeLogs(logsContext, logs)
	m.obsrecv.EndLogsOp(logsContext, metadata.Type.String(), 1, err)

	return err
}

func TestEventhubHandler_Start(t *testing.T) {
	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	ehHandler := &eventhubHandler{
		settings:     receivertest.NewNopSettings(),
		dataConsumer: &mockDataConsumer{},
		config:       config.(*Config),
	}
	ehHandler.hub = &mockHubWrapper{}

	assert.NoError(t, ehHandler.run(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, ehHandler.close(context.Background()))
}

func TestEventhubHandler_newMessageHandler(t *testing.T) {
	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	sink := new(consumertest.LogsSink)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID(metadata.Type),
		Transport:              "",
		LongLivedCtx:           false,
		ReceiverCreateSettings: receivertest.NewNopSettings(),
	})
	require.NoError(t, err)

	ehHandler := &eventhubHandler{
		settings: receivertest.NewNopSettings(),
		config:   config.(*Config),
		dataConsumer: &mockDataConsumer{
			logsUnmarshaler:  newRawLogsUnmarshaler(zap.NewNop()),
			nextLogsConsumer: sink,
			obsrecv:          obsrecv,
		},
	}
	ehHandler.hub = &mockHubWrapper{}

	assert.NoError(t, ehHandler.run(context.Background(), componenttest.NewNopHost()))

	now := time.Now()
	err = ehHandler.newMessageHandler(context.Background(), &eventhub.Event{
		Data:         []byte("hello"),
		PartitionKey: nil,
		Properties:   map[string]any{"foo": "bar"},
		ID:           "11234",
		SystemProperties: &eventhub.SystemProperties{
			SequenceNumber: nil,
			EnqueuedTime:   &now,
			Offset:         nil,
			PartitionID:    nil,
			PartitionKey:   nil,
			Annotations:    nil,
		},
	})

	assert.NoError(t, err)
	assert.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 1, sink.AllLogs()[0].LogRecordCount())
	assert.Equal(t, []byte("hello"), sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Bytes().AsRaw())

	read, ok := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", read.AsString())
	assert.NoError(t, ehHandler.close(context.Background()))
}
