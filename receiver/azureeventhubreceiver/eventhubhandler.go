// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/checkpoints"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
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
	GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error)
	NewConsumer(ctx context.Context, options *azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error)
	NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (*azeventhubs.PartitionClient, error)
	Close(ctx context.Context) error
}

type consumerClientWrapperImpl struct {
	consumerClient *azeventhubs.ConsumerClient
}

func newConsumerClientWrapperImplementation(cfg *Config) (*consumerClientWrapperImpl, error) {
	consumerClient, err := azeventhubs.NewConsumerClientFromConnectionString(cfg.Connection, cfg.EventHubName, cfg.ConsumerGroup, nil)
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

func (c *consumerClientWrapperImpl) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	return c.consumerClient.GetPartitionProperties(ctx, partitionID, options)
}

func (c *consumerClientWrapperImpl) NewConsumer(ctx context.Context, options *azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error) {
	return c.consumerClient, nil
}

func (c *consumerClientWrapperImpl) NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (*azeventhubs.PartitionClient, error) {
	return c.consumerClient.NewPartitionClient(partitionID, options)
}

func (c *consumerClientWrapperImpl) Close(ctx context.Context) error {
	return c.consumerClient.Close(ctx)
}

// type processorHandler struct {
// 	processor    *azeventhubs.Processor
// 	dataConsumer dataConsumer
// 	config       *Config
// 	settings     receiver.CreateSettings
// 	cancel       context.CancelFunc
// }

type eventhubHandler struct {
	processor      *azeventhubs.Processor
	consumerClient consumerClientWrapper
	dataConsumer   dataConsumer
	config         *Config
	settings       receiver.CreateSettings
	cancel         context.CancelFunc
	useProcessor   bool
}

var _ eventHandler = (*eventhubHandler)(nil)

// newEventhubHandler creates a handler for Azure Event Hub. This version is enhanced to handle mock configurations for testing.
func newEventhubHandler(config *Config, settings receiver.CreateSettings) *eventhubHandler {
	// Check if the configuration is meant for testing. This can be done by checking a specific field or a pattern in the connection string.
	if strings.Contains(config.Connection, "fake.servicebus.windows.net") {
		return nil
		// Return a mock handler if the connection string is empty or obviously fake.
		// return newMockEventhubHandler()
		// return newMockEventhubHandler(config, settings)
	}

	return &eventhubHandler{
		config:       config,
		settings:     settings,
		useProcessor: false,
	}
}

func (h *eventhubHandler) init(ctx context.Context) error {
	_, h.cancel = context.WithCancel(ctx)
	consumerClient, err := newConsumerClientWrapperImplementation(h.config)
	if err != nil {
		return err
	}
	h.consumerClient = consumerClient
	return nil
}

func (h *eventhubHandler) run(ctx context.Context, host component.Host) error {
	if h.useProcessor {
		return h.runWithProcessor(ctx)
	}
	return h.runWithConsumerClient(ctx, host)
}

func (h *eventhubHandler) runWithProcessor(ctx context.Context) error {
	checkpointStore, err := createCheckpointStore(h.config.StorageConnection, h.config.StorageContainer)
	if err != nil {
		return err
	}

	processor, err := azeventhubs.NewProcessor(h.consumerClient.(*consumerClientWrapperImpl).consumerClient, checkpointStore, nil)
	if err != nil {
		return err
	}

	go h.dispatchPartitionClients(processor)
	processorCtx, processorCancel := context.WithCancel(ctx)
	defer processorCancel()

	return processor.Run(processorCtx)
}

func (h *eventhubHandler) dispatchPartitionClients(processor *azeventhubs.Processor) {
	var wg sync.WaitGroup
	for {
		partitionClient := processor.NextPartitionClient(context.TODO())

		if partitionClient == nil {
			break
		}

		wg.Add(1)
		go func(pc *azeventhubs.ProcessorPartitionClient) {
			defer wg.Done()
			if err := h.processEventsForPartition(pc); err != nil {
				h.settings.Logger.Error("Error processing partition", zap.Error(err))
			}
		}(partitionClient)
	}
	wg.Wait()
}

func (h *eventhubHandler) processEventsForPartition(partitionClient *azeventhubs.ProcessorPartitionClient) error {
	defer partitionClient.Close(context.TODO())

	for {
		receiveCtx, cancelReceive := context.WithTimeout(context.TODO(), time.Minute)
		events, err := partitionClient.ReceiveEvents(receiveCtx, h.config.BatchCount, nil)
		cancelReceive()

		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			var eventHubError *azeventhubs.Error
			if errors.As(err, &eventHubError) && eventHubError.Code == azeventhubs.ErrorCodeOwnershipLost {
				return nil
			}
			return err
		}

		if len(events) == 0 {
			continue
		}

		for _, event := range events {
			if err := h.newMessageHandler(context.TODO(), event); err != nil {
				h.settings.Logger.Error("Error handling event", zap.Error(err))
			}
		}

		if err := partitionClient.UpdateCheckpoint(context.TODO(), events[len(events)-1], nil); err != nil {
			h.settings.Logger.Error("Error updating checkpoint", zap.Error(err))
		}
	}
}

func (h *eventhubHandler) runWithConsumerClient(ctx context.Context, host component.Host) error {
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
			err = h.setupPartition(ctx, partitionID)
			if err != nil {
				h.settings.Logger.Debug("Error setting up partition", zap.Error(err))
				return err
			}
		}
	} else {
		err := h.setupPartition(ctx, h.config.Partition)
		if err != nil {
			h.settings.Logger.Debug("Error setting up partition", zap.Error(err))
			return err
		}
	}
	return nil
}

func (h *eventhubHandler) setupPartition(ctx context.Context, partitionID string) error {
	cc, err := h.consumerClient.NewConsumer(ctx, nil)
	if err != nil {
		return err
	}
	if cc == nil {
		return errors.New("failed to initialize consumer client")
	}
	defer func() {
		if cc != nil {
			cc.Close(ctx)
		}
	}()

	pcOpts := &azeventhubs.PartitionClientOptions{
		StartPosition: azeventhubs.StartPosition{
			Earliest: to.Ptr(true),
		},
	}

	pc, err := cc.NewPartitionClient(partitionID, pcOpts)
	if err != nil {
		return err
	}
	if pc == nil {
		return errors.New("failed to initialize partition client")
	}
	defer func() {
		if pc != nil {
			pc.Close(ctx)
		}
	}()

	go h.receivePartitionEvents(ctx, pc)

	return nil
}

func (h *eventhubHandler) receivePartitionEvents(ctx context.Context, pc *azeventhubs.PartitionClient) {
	var wait = 1
	for {
		rcvCtx, _ := context.WithTimeout(context.TODO(), time.Second*10)
		events, err := pc.ReceiveEvents(rcvCtx, h.config.BatchCount, nil)
		if err != nil {
			h.settings.Logger.Error("Error receiving event", zap.Error(err))
			time.Sleep(time.Duration(wait) * time.Second)
			wait *= 2
			continue
		}

		for _, event := range events {
			if err := h.newMessageHandler(ctx, event); err != nil {
				h.settings.Logger.Error("Error handling event", zap.Error(err))
			}
		}
	}
}

func (h *eventhubHandler) newMessageHandler(ctx context.Context, event *azeventhubs.ReceivedEventData) error {
	err := h.dataConsumer.consume(ctx, event)
	if err != nil {
		h.settings.Logger.Error("Error decoding message", zap.Error(err))
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

func createCheckpointStore(storageConnectionString, containerName string) (azeventhubs.CheckpointStore, error) {
	azBlobContainerClient, err := container.NewClientFromConnectionString(storageConnectionString, containerName, nil)
	if err != nil {
		return nil, err
	}
	return checkpoints.NewBlobStore(azBlobContainerClient, nil)
}
