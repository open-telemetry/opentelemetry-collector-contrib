// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type eventHandler interface {
	run(ctx context.Context, host component.Host) error
	close(ctx context.Context) error
	setDataConsumer(dataConsumer dataConsumer)
}

type consumerClientWrapper interface {
	GetEventHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error)
	GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (*azeventhubs.PartitionProperties, error)
	NextConsumer(ctx context.Context, options azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error)
	NewConsumer(ctx context.Context, options azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error)
	NewPartitionClient(ctx context.Context, partitionID string, options *azeventhubs.PartitionClientOptions) (*azeventhubs.PartitionClient, error)
	Close(ctx context.Context) error
}

type consumerClientWrapperImpl struct {
	consumerClient *azeventhubs.ConsumerClient
}

func newConsumerClientWrapperImplementation(cfg *Config) (*consumerClientWrapperImpl, error) {
	// TODO: expand call to NewConsumerClientFromConnectionString to include additional arguments (Connection, EventHub, ConsumerGroup, Options)
	// consumerClient, err := azeventhubs.NewConsumerClientFromConnectionString(cfg.Connection, cfg.ConsumerGroup)
	consumerClient, err := azeventhubs.NewConsumerClientFromConnectionString(cfg.Connection, cfg.ConsumerGroup)
	if err != nil {
		return nil, err
	}

	return &consumerClientWrapperImpl{
		consumerClient: consumerClient,
	}, nil
}

func (c *consumerClientWrapperImpl) GetEventHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	return c.consumerClient.GetEventHubProperties(ctx, options)
}

func (h *consumerClientWrapperImpl) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	return h.consumerClient.GetPartitionProperties(ctx, partitionID, options)
}

func (h *consumerClientWrapperImpl) Receive(ctx context.Context, options *azeventhubs.ReceiveEventsOptions) ([]*azeventhubs.ReceivedEventData, error) {
	return h.consumerClient.NewPartitionClient()
	// return h.consumerClient.Receive(ctx, options)
}

// func (h *consumerClientWrapperImpl) NewConsumer(ctx context.Context, options azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error) {
// 	return h.consumerClient.NewPartitionClient(options.PartitionID, nil)
// 	// NextConsumer(ctx, options)
// }

// func (h *consumerClientWrapperImpl) NewConsumer(ctx context.Context, options azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error) {
// 	return h.consumerClient, nil
// }

// func (h *consumerClientWrapperImpl) NewPartitionClient(ctx context.Context, partitionID string, options *azeventhubs.PartitionClientOptions) (*azeventhubs.PartitionClient, error) {
// 	return h.consumerClient.NewPartitionClient(partitionID, options)
// }

func (h *consumerClientWrapperImpl) Close(ctx context.Context) error {
	return h.consumerClient.Close(ctx)
}

type listerHandleWrapper interface {
	Done() <-chan struct{}
	Err() error
}

// eventhubHandler implements eventHandler interface
type eventhubHandler struct {
	consumerClient consumerClientWrapper
	dataConsumer   dataConsumer
	config         *Config
	settings       receiver.CreateSettings
}

// Implement eventHandler Interface
var _ eventHandler = (*eventhubHandler)(nil)

func (h *eventhubHandler) init(ctx context.Context) error {
	ctx, h.cancel = context.WithCancel(ctx)
	storageClient, err := adapter.GetStorageClient(ctx, host, h.config.StorageID, h.settings.ID)
	if err != nil {
		h.settings.Logger.Debug("Error connecting to Storage", zap.Error(err))
		return err
	}
	consumerClient, err := newConsumerClientWrapperImplementation(h.config)
	if err != nil {
		return err
	}
	h.consumerClient = consumerClient
}

// if h.hub == nil { // set manually for testing.
// 	hub, newHubErr := eventhub.NewHubFromConnectionString(h.config.Connection, eventhub.HubWithOffsetPersistence(&storageCheckpointPersister{storageClient: storageClient}))
// 	if newHubErr != nil {
// 		h.settings.Logger.Debug("Error connecting to Event Hub", zap.Error(newHubErr))
// 		return newHubErr
// 	}
// 	h.hub = &hubWrapperImpl{
// 		hub: hub,
// 	}
// }

// consumerClient, err := newConsumerClientWrapperImplementation(h.config)
// if err != nil {
// 	return err

func (h *eventhubHandler) run(ctx context.Context, host component.Host) error {
	// when consumerClient is initialized (for testing), skip initialization
	if h.consumerClient == nil {
		if err := h.init(ctx); err != nil {
			return err
		}
	}
	if h.config.Partition == "" {
		properties, err := h.consumerClient.GetEventHubProperties(ctx, nil)
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
		err := h.setUpOnePartition(ctx, h.config.Partition)
		if err != nil {
			h.settings.Logger.Debug("Error setting up partition", zap.Error(err))
			return err
		}
	}

	return nil
}

func (h *eventhubHandler) setUpPartitions(ctx context.Context) error {
	// if partition is specified, only set up that partition
	if h.config.Partition != "" {
		return h.setupPartition(ctx, h.config.Partition)
	}
	// otherwise, get all partitions and set each up
	properties, err := h.consumerClient.GetEventHubProperties(ctx, nil)
	if err != nil {
		return err
	}
	for _, partitionID := range properties.PartitionIDs {
		err = h.setupPartition(ctx, partitionID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *eventhubHandler) setupPartition(ctx context.Context, partitionID string) error {
	cc, err := h.consumerClient.NewConsumer(ctx, azeventhubs.ConsumerClientOptions{})
	if err != nil {
		return err
	}
	defer cc.Close(ctx)

	pcOpts := &azeventhubs.PartitionClientOptions{
		// StartPosition: defaults to latest
		// OwnerLevel: defaults to off
		// Prefetch: defaults to 300
	}
	pc, err := cc.NewPartitionClient(partitionID, pcOpts)
	if err != nil {
		return err
	}
	defer pc.Close(ctx)

	go func() {
		var wait = 1
		for {
			rcvCtx, err := context.WithTimeout(ctx, time.Second*h.config.BatchTimeout)
			events, err := pc.ReceiveEvents(ctx, h.config.BatchCount, nil)
			if err != nil {
				h.settings.Logger.Error("Error receiving event", zap.Error(err))
				// retry with backoff
				time.Sleep(1)
				wait *= 2
				continue
			}

			for _, event := range events {
				if err := h.newMessageHandler(ctx, event); err != nil {
					h.settings.Logger.Error("Error handling event", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

func (h *eventhubHandler) newMessageHandler(ctx context.Context, event *azeventhubs.ReceivedEventData) error {
	// existing code for newMessageHandler
	err := h.dataConsumer.consume(ctx, event)
	if err != nil {
		h.settings.Logger.Error("error decoding message", zap.Error(err))
		return err
	}

	return nil
}

func (h *eventhubHandler) close(ctx context.Context) error {
	if h.consumerClient != nil {
		err := h.consumerClient.Close(ctx)
		if err != nil {
			return err
		}
		h.consumerClient = nil
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
