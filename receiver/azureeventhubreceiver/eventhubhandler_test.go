// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"testing"
	"time"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

type mockHubWrapper struct{}

func (mockHubWrapper) GetRuntimeInformation(context.Context) (*hubRuntimeInfo, error) {
	return &hubRuntimeInfo{
		Path:           "foo",
		CreatedAt:      time.Now(),
		PartitionCount: 1,
		PartitionIDs:   []string{"foo"},
	}, nil
}

func (mockHubWrapper) Receive(ctx context.Context, _ string, _ hubHandler, _ bool) (listenerHandleWrapper, error) {
	return &mockListenerHandleWrapper{
		ctx: ctx,
	}, nil
}

func (mockHubWrapper) Close(context.Context) error {
	return nil
}

type mockListenerHandleWrapper struct {
	ctx context.Context
}

func (m *mockListenerHandleWrapper) Done() <-chan struct{} {
	return m.ctx.Done()
}

func (mockListenerHandleWrapper) Err() error {
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

func (*mockDataConsumer) setNextMetricsConsumer(consumer.Metrics) {}

func (m *mockDataConsumer) consume(ctx context.Context, event *azureEvent) error {
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
		settings:     receivertest.NewNopSettings(metadata.Type),
		dataConsumer: &mockDataConsumer{},
		config:       config.(*Config),
	}
	ehHandler.hub = &mockHubWrapper{}

	assert.NoError(t, ehHandler.run(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, ehHandler.close(t.Context()))
}

func TestShouldInitializeStorageClient(t *testing.T) {
	testCases := []struct {
		name          string
		storageClient storage.Client
		storageID     *component.ID
		expected      bool
	}{
		{
			name:          "no storage client and no storage ID - should not initialize",
			storageClient: nil,
			storageID:     nil,
			expected:      false,
		},
		{
			name:          "no storage client but has storage ID - should initialize",
			storageClient: nil,
			storageID:     &component.ID{},
			expected:      true,
		},
		{
			name:          "has storage client and storage ID - should not initialize",
			storageClient: &mockStorageClient{},
			storageID:     &component.ID{},
			expected:      false,
		},
		{
			name:          "has storage client but no storage ID - should not initialize",
			storageClient: &mockStorageClient{},
			storageID:     nil,
			expected:      false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			result := shouldInitializeStorageClient(test.storageClient, test.storageID)
			assert.Equal(t, test.expected, result)
		})
	}
}

type mockStorageClient struct {
	storage map[string][]byte
}

func (m *mockStorageClient) Get(_ context.Context, key string) ([]byte, error) {
	if len(m.storage[key]) > 0 {
		return m.storage[key], nil
	}
	return nil, nil
}

func (m *mockStorageClient) Set(_ context.Context, key string, val []byte) error {
	m.storage[key] = val
	return nil
}

func (m *mockStorageClient) Delete(_ context.Context, key string) error {
	m.storage[key] = []byte{}
	return nil
}

func (*mockStorageClient) Batch(_ context.Context, _ ...*storage.Operation) error {
	return nil
}

func (*mockStorageClient) Close(_ context.Context) error {
	return nil
}

func TestEventhubHandler_newAzeventhubsMessageHandler(t *testing.T) {
	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	sink := new(consumertest.LogsSink)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID(metadata.Type),
		Transport:              "",
		LongLivedCtx:           false,
		ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
	})
	require.NoError(t, err)

	ehHandler := &eventhubHandler{
		settings: receivertest.NewNopSettings(metadata.Type),
		config:   config.(*Config),
		dataConsumer: &mockDataConsumer{
			logsUnmarshaler:  newRawLogsUnmarshaler(zap.NewNop()),
			nextLogsConsumer: sink,
			obsrecv:          obsrecv,
		},
	}
	ehHandler.hub = &mockHubWrapper{}

	assert.NoError(t, ehHandler.run(t.Context(), componenttest.NewNopHost()))

	now := time.Now()
	err = ehHandler.newMessageHandler(t.Context(), &azureEvent{
		AzEventData: &azeventhubs.ReceivedEventData{
			EventData: azeventhubs.EventData{
				MessageID:  to.Ptr("11234"),
				Body:       []byte("hello"),
				Properties: map[string]any{"foo": "bar"},
			},
			EnqueuedTime: &now,
			Offset:       "",
			PartitionKey: nil,
		},
	})

	assert.NoError(t, err)
	assert.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 1, sink.AllLogs()[0].LogRecordCount())
	assert.Equal(t, []byte("hello"), sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Bytes().AsRaw())

	read, ok := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", read.AsString())
	assert.NoError(t, ehHandler.close(t.Context()))
}

func TestEventhubHandler_newLegacyMessageHandler(t *testing.T) {
	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	sink := new(consumertest.LogsSink)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID(metadata.Type),
		Transport:              "",
		LongLivedCtx:           false,
		ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
	})
	require.NoError(t, err)

	ehHandler := &eventhubHandler{
		settings: receivertest.NewNopSettings(metadata.Type),
		config:   config.(*Config),
		dataConsumer: &mockDataConsumer{
			logsUnmarshaler:  newRawLogsUnmarshaler(zap.NewNop()),
			nextLogsConsumer: sink,
			obsrecv:          obsrecv,
		},
	}
	ehHandler.hub = &mockHubWrapper{}

	assert.NoError(t, ehHandler.run(t.Context(), componenttest.NewNopHost()))

	now := time.Now()
	err = ehHandler.newMessageHandler(t.Context(), &azureEvent{
		EventHubEvent: &eventhub.Event{
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
		},
	})

	assert.NoError(t, err)
	assert.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 1, sink.AllLogs()[0].LogRecordCount())
	assert.Equal(t, []byte("hello"), sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Bytes().AsRaw())

	read, ok := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", read.AsString())
	assert.NoError(t, ehHandler.close(t.Context()))
}

func TestEventhubHandler_closeWithStorageClient(t *testing.T) {
	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	ehHandler := &eventhubHandler{
		settings:     receivertest.NewNopSettings(metadata.Type),
		dataConsumer: &mockDataConsumer{},
		config:       config.(*Config),
	}
	ehHandler.hub = &mockHubWrapper{}
	mockClient := newMockClient()
	ehHandler.storageClient = mockClient

	assert.NoError(t, ehHandler.run(t.Context(), componenttest.NewNopHost()))
	require.NotNil(t, ehHandler.storageClient)
	require.NotNil(t, mockClient.cache)
	assert.NoError(t, ehHandler.close(t.Context()))
	require.Nil(t, ehHandler.storageClient)
	require.Nil(t, mockClient.cache)
}
