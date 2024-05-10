// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
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

type mockconsumerClientWrapper struct {
}

func (m mockconsumerClientWrapper) GetEventHubProperties(_ context.Context, _ *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	return azeventhubs.EventHubProperties{
		Name:         "mynameis",
		PartitionIDs: []string{"foo", "bar"},
	}, nil
}

func (m mockconsumerClientWrapper) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	return azeventhubs.PartitionProperties{}, nil
}

func (m mockconsumerClientWrapper) NextConsumer(ctx context.Context, options azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error) {
	return &azeventhubs.ConsumerClient{}, nil
}

func (m mockconsumerClientWrapper) NewConsumer(ctx context.Context, options *azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error) {
	return &azeventhubs.ConsumerClient{}, nil
}

func (m mockconsumerClientWrapper) NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (*azeventhubs.PartitionClient, error) {
	return &azeventhubs.PartitionClient{}, nil
}

func (m mockconsumerClientWrapper) Close(_ context.Context) error {
	return nil
}

// Function to create mock implementation
func newMockConsumerClientWrapperImplementation(cfg *Config) (consumerClientWrapper, error) {
	var ccw consumerClientWrapper = &mockconsumerClientWrapper{}
	return ccw, nil
}

type mockDataConsumer struct {
	logsUnmarshaler  eventLogsUnmarshaler
	nextLogsConsumer consumer.Logs
	obsrecv          *receiverhelper.ObsReport
}

func (m *mockDataConsumer) setNextLogsConsumer(nextLogsConsumer consumer.Logs) {
	m.nextLogsConsumer = nextLogsConsumer
}

func (m *mockDataConsumer) setNextMetricsConsumer(_ consumer.Metrics) {}

func (m *mockDataConsumer) consume(ctx context.Context, event *azeventhubs.ReceivedEventData) error {
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
	config.(*Config).Connection = "DefaultEndpointsProtocol=https;AccountName=<accountName>;AccountKey=<accountKey>;EndpointSuffix=core.windows.net"

	ehHandler := &eventhubHandler{
		settings:       receivertest.NewNopCreateSettings(),
		dataConsumer:   &mockDataConsumer{},
		config:         config.(*Config),
		consumerClient: &mockconsumerClientWrapper{},
		useProcessor: true,
	}

	ehHandler.consumerClient, _ = newMockConsumerClientWrapperImplementation(config.(*Config))

	err := ehHandler.run(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	err = ehHandler.close(context.Background())
	assert.NoError(t, err)
}

func TestEventhubHandler_newMessageHandler(t *testing.T) {
	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	sink := new(consumertest.LogsSink)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID(metadata.Type),
		Transport:              "",
		LongLivedCtx:           false,
		ReceiverCreateSettings: receivertest.NewNopCreateSettings(),
	})
	require.NoError(t, err)

	mockConsumer := &mockDataConsumer{
		logsUnmarshaler:  newRawLogsUnmarshaler(zap.NewNop()),
		nextLogsConsumer: sink,
		obsrecv:          obsrecv,
	}

	ehHandler := &eventhubHandler{
		settings: receivertest.NewNopCreateSettings(),
		config:   config.(*Config),
		dataConsumer: mockConsumer,
		consumerClient: mockconsumerClientWrapper{},
		// useProcessor:   true,
	}

	ehHandler.consumerClient, _ = newMockConsumerClientWrapperImplementation(config.(*Config))
	// The expected connection string should contain key value pairs separated by semicolons.
	// For example 'DefaultEndpointsProtocol=https;AccountName=<accountName>;AccountKey=<accountKey>;EndpointSuffix=core.windows.net'
	err = ehHandler.run(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	now := time.Now()
	testEvent := &azeventhubs.ReceivedEventData{
		EventData: azeventhubs.EventData{
			Body:        []byte("hello"),
			Properties:  map[string]interface{}{"foo": "bar"},
		},
		EnqueuedTime: &now,
		SystemProperties: map[string]interface{}{
			"the_time": now,
		},
	}

	err = ehHandler.newMessageHandler(context.Background(), testEvent)
	assert.NoError(t, err)

	assert.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 1, sink.AllLogs()[0].LogRecordCount())
	assert.Equal(t, []byte("hello"), sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Bytes().AsRaw())

	read, ok := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", read.AsString())
}


/*
func (m *mockconsumerClientWrapper) GetEventHubProperties(_ context.Context, _ *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	return azeventhubs.EventHubProperties{
		Name:         "mynameis",
		PartitionIDs: []string{"foo", "bar"},
	}, nil
}

func (m *mockconsumerClientWrapper) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	return azeventhubs.PartitionProperties{}, nil
}

func (m *mockconsumerClientWrapper) NewConsumer(ctx context.Context, options *azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error) {
	return &azeventhubs.ConsumerClient{}, nil
}

func (m *mockconsumerClientWrapper) NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (*azeventhubs.PartitionClient, error) {
	return &azeventhubs.PartitionClient{}, nil
}

func (m *mockconsumerClientWrapper) Close(_ context.Context) error {
	return nil
}
*/
/*
func TestEventhubHandler_newMessageHandler(t *testing.T) {
	config := createDefaultConfig()
	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

	// Mock the sink to collect logs
	sink := new(consumertest.LogsSink)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID(metadata.Type),
		Transport:              "",
		LongLivedCtx:           false,
		ReceiverCreateSettings: receivertest.NewNopCreateSettings(),
	})
	require.NoError(t, err)

	// Create a mock data consumer
	mockConsumer := &mockDataConsumer{
		logsUnmarshaler:  newRawLogsUnmarshaler(zap.NewNop()),
		nextLogsConsumer: sink,
		obsrecv:          obsrecv,
	}

	ehHandler := &eventhubHandler{
		settings: receivertest.NewNopCreateSettings(),
		config:   config.(*Config),
		dataConsumer: mockConsumer,
		consumerClient: mockconsumerClientWrapper{},
		useProcessor: true,
	}

	ehHandler.consumerClient, _ = newMockConsumerClientWrapperImplementation(config.(*Config))

	err = ehHandler.run(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	// Create a sample event data for testing
	now := time.Now()
	testEvent := &azeventhubs.ReceivedEventData{
		EventData: azeventhubs.EventData{
			Body:        []byte("hello"),
			Properties:  map[string]interface{}{"foo": "bar"},
		},
		EnqueuedTime: &now,
		SystemProperties: map[string]interface{}{
			"the_time": now,
		},
	}

	err = ehHandler.newMessageHandler(context.Background(), testEvent)
	assert.NoError(t, err)

	// Validate the processed logs
	assert.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 1, sink.AllLogs()[0].LogRecordCount())
	assert.Equal(t, []byte("hello"), sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Bytes().AsRaw())

	// Validate additional attributes
	read, ok := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", read.AsString())
}

// func TestEventhubHandler_newMessageHandler(t *testing.T) {
// 	config := createDefaultConfig()
// 	config.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

// 	sink := new(consumertest.LogsSink)
// 	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
// 		ReceiverID:             component.NewID(metadata.Type),
// 		Transport:              "",
// 		LongLivedCtx:           false,
// 		ReceiverCreateSettings: receivertest.NewNopCreateSettings(),
// 	})
// 	require.NoError(t, err)

// 	ehHandler := &eventhubHandler{
// 		settings: receivertest.NewNopCreateSettings(),
// 		config:   config.(*Config),
// 		dataConsumer: &mockDataConsumer{
// 			logsUnmarshaler:  newRawLogsUnmarshaler(zap.NewNop()),
// 			nextLogsConsumer: sink,
// 			obsrecv:          obsrecv,
// 		},
// 		consumerClient: mockconsumerClientWrapper{},
// 		useProcessor: true,
// 	}

// 	err = ehHandler.run(context.Background(), componenttest.NewNopHost())
// 	assert.NoError(t, err)

// 	now := time.Now()
// 	err = ehHandler.newMessageHandler(context.Background(), &azeventhubs.ReceivedEventData{
// 		EventData: azeventhubs.EventData{
// 			Properties: map[string]interface{}{
// 				"foo": "bar",
// 			},
// 		},
// 		EnqueuedTime: &now,
// 		SystemProperties: map[string]any{
// 			"the_time": now,
// 		},
// 	})

// 	assert.NoError(t, err)
// 	assert.Len(t, sink.AllLogs(), 1)
// 	assert.Equal(t, 1, sink.AllLogs()[0].LogRecordCount())
// 	assert.Equal(t, []byte("hello"), sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Bytes().AsRaw())

// 	read, ok := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("foo")
// 	assert.True(t, ok)
// 	assert.Equal(t, "bar", read.AsString())
// }
*/
