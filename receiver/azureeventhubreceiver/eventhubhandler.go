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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

const (
	batchCount = 100
)

type hubWrapper interface {
	GetRuntimeInformation(ctx context.Context) (*eventhub.HubRuntimeInformation, error)
	Receive(ctx context.Context, partitionID string, handler eventhub.Handler, opts ...eventhub.ReceiveOption) (listerHandleWrapper, error)
	Close(ctx context.Context) error
}

type consumerClientWrapperImpl struct {
	consumerClient *azeventhubs.ConsumerClient
}

func newConsumerClientWrapperImplementation(cfg *Config) (*consumerClientWrapperImpl, error) {
	splits := strings.Split(cfg.Connection, "/")
	eventhubName := splits[len(splits)-1]
	// if that didn't work it's ok as the SDK will try to parse it to create the client

	consumerClient, err := azeventhubs.NewConsumerClientFromConnectionString(cfg.Connection, eventhubName, cfg.ConsumerGroup, nil)
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

func (c *consumerClientWrapperImpl) NewConsumer(_ context.Context, _ *azeventhubs.ConsumerClientOptions) (*azeventhubs.ConsumerClient, error) {
	return c.consumerClient, nil
}

func (c *consumerClientWrapperImpl) NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (*azeventhubs.PartitionClient, error) {
	return c.consumerClient.NewPartitionClient(partitionID, options)
}

func (c *consumerClientWrapperImpl) Close(ctx context.Context) error {
	return c.consumerClient.Close(ctx)
}

type eventhubHandler struct {
	hub          hubWrapper
	dataConsumer dataConsumer
	config       *Config
	settings     receiver.Settings
	cancel       context.CancelFunc
}

func (h *eventhubHandler) run(ctx context.Context, host component.Host) error {
	ctx, h.cancel = context.WithCancel(ctx)

	processor, err := azeventhubs.NewProcessor(consumerClientImpl.consumerClient, checkpointStore, nil)
	if err != nil {
		h.settings.Logger.Debug("Error creating Processor", zap.Error(err))
		return err
	}

	processorCtx, processorCancel := context.WithCancel(ctx)
	go h.dispatchPartitionClients(processor)
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
		events, err := partitionClient.ReceiveEvents(receiveCtx, 100, nil)
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

func (h *eventhubHandler) runWithConsumerClient(ctx context.Context, _ component.Host) error {
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
	defer cc.Close(ctx)

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
	for {
		rcvCtx, rcvCtxCancel := context.WithTimeout(context.TODO(), time.Second*10)
		events, err := pc.ReceiveEvents(rcvCtx, batchCount, nil)
		rcvCtxCancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			h.settings.Logger.Error("Error receiving events", zap.Error(err))
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
	if h.cancel != nil {
		h.cancel()
	}

	return nil
}

func (h *eventhubHandler) setDataConsumer(dataConsumer dataConsumer) {
	h.dataConsumer = dataConsumer
}

func newEventhubHandler(config *Config, settings receiver.Settings) *eventhubHandler {

	return &eventhubHandler{
		config:   config,
		settings: settings,
	}
}
