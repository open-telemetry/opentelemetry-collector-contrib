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
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

type mockHubWrapper struct {
}

func (m mockHubWrapper) GetRuntimeInformation(ctx context.Context) (*eventhub.HubRuntimeInformation, error) {
	return &eventhub.HubRuntimeInformation{
		Path:           "foo",
		CreatedAt:      time.Now(),
		PartitionCount: 1,
		PartitionIDs:   []string{"foo"},
	}, nil
}

func (m mockHubWrapper) Receive(ctx context.Context, partitionID string, handler eventhub.Handler, opts ...eventhub.ReceiveOption) (listerHandleWrapper, error) {
	return &mockListenerHandleWrapper{
		ctx: context.Background(),
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
	logsUnmarshaler  eventLogsUnmarshaler
	nextLogsConsumer consumer.Logs
	obsrecv          *obsreport.Receiver
}

func (m *mockDataConsumer) setNextLogsConsumer(nextLogsConsumer consumer.Logs) {
	m.nextLogsConsumer = nextLogsConsumer
}

func (m *mockDataConsumer) setNextMetricsConsumer(nextMetricsConsumer consumer.Metrics) {
}

func (m *mockDataConsumer) consume(ctx context.Context, event *eventhub.Event) error {

	logsContext := m.obsrecv.StartLogsOp(ctx)

	logs, err := m.logsUnmarshaler.UnmarshalLogs(event)
	if err != nil {
		return err
	}

	err = m.nextLogsConsumer.ConsumeLogs(logsContext, logs)
	m.obsrecv.EndLogsOp(logsContext, metadata.Type, 1, err)

	return err
}

func TestEventhubHandler_Start(t *testing.T) {

	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	ehHandler := &eventhubHandler{
		settings:     receivertest.NewNopCreateSettings(),
		dataConsumer: &mockDataConsumer{},
		config:       config.(*Config),
	}
	ehHandler.hub = &mockHubWrapper{}

	err := ehHandler.run(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	err = ehHandler.close(context.Background())
	assert.NoError(t, err)
}

func TestEventhubHandler_newMessageHandler(t *testing.T) {

	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	sink := new(consumertest.LogsSink)
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             component.NewID(metadata.Type),
		Transport:              "",
		LongLivedCtx:           false,
		ReceiverCreateSettings: receivertest.NewNopCreateSettings(),
	})
	require.NoError(t, err)

	ehHandler := &eventhubHandler{
		settings: receivertest.NewNopCreateSettings(),
		config:   config.(*Config),
		dataConsumer: &mockDataConsumer{
			logsUnmarshaler:  newRawLogsUnmarshaler(zap.NewNop()),
			nextLogsConsumer: sink,
			obsrecv:          obsrecv,
		},
	}
	ehHandler.hub = &mockHubWrapper{}

	err = ehHandler.run(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	now := time.Now()
	err = ehHandler.newMessageHandler(context.Background(), &eventhub.Event{
		Data:         []byte("hello"),
		PartitionKey: nil,
		Properties:   map[string]interface{}{"foo": "bar"},
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
}
