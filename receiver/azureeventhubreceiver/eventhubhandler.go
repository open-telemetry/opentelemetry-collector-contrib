// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"

	// eventhub "github.com/Azure/azure-event-hubs-go/v3"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

type eventHandler interface {
	run(ctx context.Context, host component.Host) error
	close(ctx context.Context) error
	setDataConsumer(dataConsumer dataConsumer)
}

type hubWrapper interface {
	GetEventHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (*azeventhubs.EventHubProperties, error)
	GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (*azeventhubs.PartitionProperties, error)
	NewConsumer(ctx context.Context, options azeventhubs.ConsumerOptions) (*azeventhubs.Consumer, error)
	Close(ctx context.Context) error
}

type hubWrapperImpl struct {
	client *azeventhubs.Client
	hub    *azeventhubs.Hub
}

func (h *hubWrapperImpl) GetEventHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (*azeventhubs.EventHubProperties, error) {
	return h.client.GetEventHubProperties(ctx, options)
}

func (h *hubWrapperImpl) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (*azeventhubs.PartitionProperties, error) {
	return h.client.GetPartitionProperties(ctx, partitionID, options)
}

func (h *hubWrapperImpl) NewConsumer(ctx context.Context, options azeventhubs.ConsumerOptions) (*azeventhubs.Consumer, error) {
	return h.client.NewConsumer(ctx, options)
}

func (h *hubWrapperImpl) Close(ctx context.Context) error {
	return h.client.Close(ctx)
}

type listerHandleWrapper interface {
	Done() <-chan struct{}
	Err() error
}

type eventhubHandler struct {
	hub          hubWrapper
	dataConsumer dataConsumer
	config       *Config
	settings     receiver.CreateSettings
}

// Implement eventHandler Interface
var _ eventHandler = (*eventhubHandler)(nil)

func (h *eventhubHandler) run(ctx context.Context, host component.Host) error {
	storageClient, err := adapter.GetStorageClient(ctx, host, h.config.StorageID, h.settings.ID)
	if err != nil {
		h.settings.Logger.Debug("Error connecting to Storage", zap.Error(err))
		return err
	}

	storageClient, err := adapter.GetStorageClient(ctx, host, h.config.StorageID, h.settings.ID)
	if err != nil {
		h.settings.Logger.Debug("Error connecting to Storage", zap.Error(err))
		return err
	}

	if h.hub == nil { // set manually for testing.
		hub, newHubErr := azeventhubs.NewHubFromConnectionString(
			h.config.Connection,
			azeventhubs.HubWithOffsetPersistence(
				&storageCheckpointPersister{
					storageClient: storageClient,
				},
			),
		)
		if newHubErr != nil {
			h.settings.Logger.Debug("Error connecting to Event Hub", zap.Error(newHubErr))
			return newHubErr
		}
		h.hub = &hubWrapperImpl{
			hub: hub,
		}
	}

	if h.config.Partition == "" {
		properties, err := h.hub.GetEventHubProperties(ctx, nil)
		if err != nil {
			h.settings.Logger.Debug("Error getting Event Hub properties", zap.Error(err))
			return err
		}

		for _, partitionID := range properties.PartitionIDs {
			err = h.setUpOnePartition(ctx, partitionID)
			if err != nil {
				h.settings.Logger.Debug("Error setting up partition", zap.Error(err))
				return err
			}
		}
	} else {
		err = h.setUpOnePartition(ctx, h.config.Partition)
		if err != nil {
			h.settings.Logger.Debug("Error setting up partition", zap.Error(err))
			return err
		}
	}

	return nil
}

func (h *eventhubHandler) setUpOnePartition(ctx context.Context, partitionID string) error {
	options := azeventhubs.ConsumerOptions{
		PartitionID: partitionID,
	}
	if h.config.Offset != "" {
		options.StartPosition = azeventhubs.EventPosition{
			Offset: &h.config.Offset,
		}
	}

	if h.config.ConsumerGroup != "" {
		options.ConsumerGroup = h.config.ConsumerGroup
	}

	consumer, err := h.hub.NewConsumer(ctx, options)
	if err != nil {
		return err
	}

	go func() {
		defer consumer.Close(ctx)
		for {
			event, err := consumer.Receive(ctx)
			if err != nil {
				h.settings.Logger.Error("Error receiving event", zap.Error(err))
				continue
			}

			err = h.newMessageHandler(ctx, event)
			if err != nil {
				h.settings.Logger.Error("Error handling event", zap.Error(err))
			}
		}
	}()

	return nil
}

func (h *eventhubHandler) newMessageHandler(ctx context.Context, event *azeventhubs.Event) error {
	err := h.dataConsumer.consume(ctx, event)
	if err != nil {
		h.settings.Logger.Error("error decoding message", zap.Error(err))
		return err
	}

	return nil
}

func (h *eventhubHandler) close(ctx context.Context) error {

	if h.hub != nil {
		err := h.hub.Close(ctx)
		if err != nil {
			return err
		}
		h.hub = nil
	}
	return nil
}

func (h *eventhubHandler) setDataConsumer(dataConsumer dataConsumer) {

	h.dataConsumer = dataConsumer
}

func newEventhubHandler(config *Config, settings receiver.CreateSettings) *eventhubHandler {

	return &eventhubHandler{
		config:   config,
		settings: settings,
	}
}
