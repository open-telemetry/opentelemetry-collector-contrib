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

	mockDataConsumer := new(MockDataConsumer)
	handler := newEventhubHandler(config, settings)
	handler.consumerClient = mockConsumerClient
	handler.dataConsumer = mockDataConsumer

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// expect
	mockDataConsumer.On("consume", mock.Anything, mock.Anything).Return(nil)

	// act
	err := handler.newMessageHandler(ctx, event)
	assert.NoError(t, err)
}

type MockDataConsumer struct {
	mock.Mock
}

func (m *MockDataConsumer) consume(ctx context.Context, event *azeventhubs.ReceivedEventData) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockDataConsumer) setNextLogsConsumer(c consumer.Logs) {
	_ = m.Called(c)
}

func (m *MockDataConsumer) setNextMetricsConsumer(c consumer.Metrics) {
	_ = m.Called(c)
}
