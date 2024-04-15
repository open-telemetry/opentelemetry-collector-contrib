// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	// eventhub "github.com/Azure/azure-event-hubs-go/v3"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type eventHandler interface {
	run(ctx context.Context, host component.Host) error
	close(ctx context.Context) error
	setDataConsumer(dataConsumer dataConsumer)
}

type hubWrapper interface {
	GetEvenHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error)
	GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (*azeventhubs.PartitionProperties, error)
	NewConsumer(ctx context.Context, options azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error)
	Close(ctx context.Context) error
}

type hubWrapperImpl struct {
	client *azeventhubs.ConsumerClient
	// hub    *azeventhubs.Event
}

func (h *hubWrapperImpl) GetEventHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	return h.client.GetEventHubProperties(ctx, options)
}

func (h *hubWrapperImpl) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	return h.client.GetPartitionProperties(ctx, partitionID, options)
	// return h.client.GetPartitionProperties(ctx, partitionID, options)
}

func (h *hubWrapperImpl) NewConsumer(ctx context.Context, options azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error) {
	// defer consumerClient.Close(context.TODO())
	cc, err := newCheckpointingProcessor()
	partitionClient, err := consumerClient.NewPartitionClient(partitionID, &azeventhubs.PartitionClientOptions{
		StartPosition: azeventhubs.StartPosition{
			Earliest: to.Ptr(true),
		},
	})

	if err != nil {
		panic(err)
	}

	defer partitionClient.Close(context.TODO())

	// Will wait up to 1 minute for 100 events. If the context is cancelled (or expires)
	// you'll get any events that have been collected up to that point.
	receiveCtx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	events, err := partitionClient.ReceiveEvents(receiveCtx, 100, nil)
	cancel()

	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		panic(err)
	}

	for _, event := range events {
		// We're assuming the Body is a byte-encoded string. EventData.Body supports any payload
		// that can be encoded to []byte.
		fmt.Printf("Event received with body '%s'\n", string(event.Body))
	}

	fmt.Printf("Done receiving events\n")
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
	// props, err := azeventhubs.ParseConnectionString(h.config.Connection)
	// if err != nil {
	// 	h.settings.Logger.Debug("Error parsing connection string", zap.Error(err))
	// 	return err
	// }
	processor, err := newCheckpointingProcessor(h.config.Connection, h.config.EventHubName, h.config.StorageConnection, h.config.StorageContainer)
	if err != nil {
		h.settings.Logger.Debug("Error creating processor", zap.Error(err))
		return err
	}

	dispatchPartitionClients(processor)

	// get our processor
	/*
		storageClient, err := adapter.GetStorageClient(ctx, host, h.config.StorageID, h.settings.ID)
		if err != nil {
			h.settings.Logger.Debug("Error connecting to Storage", zap.Error(err))
			return err
		}

		if h.hub == nil { // set manually for testing.
			hub, newHubErr := azeventhubs.NewHubFromConnectionString(
				h.config.Connection,
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
	*/
}

func (h *eventhubHandler) setUpOnePartition(ctx context.Context, partitionID string) error {
	// options := azeventhubs.ConsumerOptions{
	options := azeventhubs.PartitionClientOptions{
		StartPosition: azeventhubs.StartPosition{
			Offset: &h.config.Offset,
		},
	}
	if h.config.Offset != "" {
		options.StartPosition = azeventhubs.EventPosition{
			Offset: &h.config.Offset,
		}
	}

	if h.config.ConsumerGroup != "" {
		options.ConsumerGroup = h.config.ConsumerGroup
	}

	cc, err := h.hub.NewConsumer(ctx, options)
	if err != nil {
		return err
	}

	pc, err := cc.NewPartitionClient(partitionID, nil)
	if err != nil {
		return err
	}

	go func() {
		defer pc.Close(ctx)
		for {
			events, err := pc.ReceiveEvents(ctx, 1, nil)
			if err != nil {
				h.settings.Logger.Error("Error receiving event", zap.Error(err))
				continue
			}

			for _, event := range events {
				err := h.newMessageHandler(ctx, event)
				if err != nil {
					h.settings.Logger.Error("Error handling event", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

func (h *eventhubHandler) newMessageHandler(ctx context.Context, event *azeventhubs.ReceivedEventData) error {
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
