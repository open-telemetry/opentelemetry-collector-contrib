// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type MockConsumerClientWrapper struct {
	mock.Mock
}

func (m *MockConsumerClientWrapper) GetEventHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(azeventhubs.EventHubProperties), args.Error(1)
}

func (m *MockConsumerClientWrapper) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	args := m.Called(ctx, partitionID, options)
	return args.Get(0).(azeventhubs.PartitionProperties), args.Error(1)
}

func (m *MockConsumerClientWrapper) NewConsumer(ctx context.Context, options *azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(*azeventhubs.ConsumerClient), args.Error(1)
}

func (m *MockConsumerClientWrapper) NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (*azeventhubs.PartitionClient, error) {
	args := m.Called(partitionID, options)
	return args.Get(0).(*azeventhubs.PartitionClient), args.Error(1)
}

func (m *MockConsumerClientWrapper) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(1)
}

func TestEventHubHandler_Start(t *testing.T) {
	logger := zap.NewNop()
	settings := receiver.CreateSettings{
		TelemetrySettings: component.TelemetrySettings{Logger: logger},
	}
	config := &Config{Connection: "Endpoint=sb://namespace.servicebus.windows.net/;EntityPath=hubName", ConsumerGroup: "$Default"}

	mockConsumerClient := new(MockConsumerClientWrapper)

	handler := newEventhubHandler(config, settings)
	handler.consumerClient = mockConsumerClient

	host := componenttest.NewNopHost()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.run(ctx, host)
	assert.NoError(t, err)

	mockConsumerClient.AssertExpectations(t)
}

func TestEventHubHandler_HandleEvent(t *testing.T) {
	logger := zap.NewNop()
	settings := receiver.CreateSettings{
		TelemetrySettings: component.TelemetrySettings{Logger: logger},
	}
	config := &Config{Connection: "Endpoint=sb://namespace.servicebus.windows.net/;EntityPath=hubName", ConsumerGroup: "$Default"}

	mockConsumerClient := new(MockConsumerClientWrapper)
	event := &azeventhubs.ReceivedEventData{
		EventData: azeventhubs.EventData{
			Body: []byte(`{"message":"test"}`),
		},
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

func (m *MockDataConsumer) consume(ctx context.Context, event *azeventhubs.ReceivedEventData) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockDataConsumer) setNextLogsConsumer(c consumer.Logs) {
	_ = m.Called(c)
}

	read, ok := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", read.AsString())
	assert.NoError(t, ehHandler.close(context.Background()))
}
