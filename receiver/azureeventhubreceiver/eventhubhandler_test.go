// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

type mockProcessor struct{}

func (m *mockProcessor) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Millisecond):
		return nil
	}
}

func (m *mockProcessor) NextPartitionClient(ctx context.Context) *azeventhubs.ProcessorPartitionClient {
	return &azeventhubs.ProcessorPartitionClient{}
}

type mockCheckpointStore struct{}

func (m *mockCheckpointStore) SetCheckpoint(ctx context.Context, checkpoint azeventhubs.Checkpoint, options *azeventhubs.SetCheckpointOptions) error {
	return nil
}

func (m *mockCheckpointStore) GetCheckpoint(ctx context.Context, partitionID string) (azeventhubs.Checkpoint, error) {
	return azeventhubs.Checkpoint{}, nil
}

func (m *mockCheckpointStore) GetCheckpoints(ctx context.Context) ([]azeventhubs.Checkpoint, error) {
	return []azeventhubs.Checkpoint{}, nil
}

func newMockProcessor(*eventhubHandler) (*mockProcessor, error) {
	return &mockProcessor{}, nil
}

type mockconsumerClientWrapper struct {
}

func (m mockconsumerClientWrapper) GetEventHubProperties(_ context.Context, _ *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	return azeventhubs.EventHubProperties{
		Name:         "mynameis",
		PartitionIDs: []string{"foo", "bar"},
	}, nil
}

func (m mockconsumerClientWrapper) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	return azeventhubs.PartitionProperties{
		PartitionID:        "abc123",
		LastEnqueuedOffset: 1111,
	}, nil
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

// newMockEventhubHandler creates a mock handler for Azure Event Hub for use in unit tests.
func newMockEventhubHandler(config *Config, settings receiver.CreateSettings) *eventhubHandler {
	// Mock implementation: No real operations are performed.
	consumerClient, err := newMockConsumerClientWrapperImplementation(config)
	if err != nil {
		panic(err)
	}

	eh := &eventhubHandler{
		processor:      &azeventhubs.Processor{},
		consumerClient: consumerClient,
		dataConsumer:   &mockDataConsumer{},
		config:         config,
		settings:       settings,
		useProcessor:   false,
	}
	return eh
}
