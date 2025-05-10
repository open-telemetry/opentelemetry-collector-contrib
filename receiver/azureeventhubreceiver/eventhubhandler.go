// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

type hubWrapper interface {
	GetRuntimeInformation(ctx context.Context) (*eventhub.HubRuntimeInformation, error)
	Receive(ctx context.Context, partitionID string, handler eventhub.Handler, opts ...eventhub.ReceiveOption) (listerHandleWrapper, error)
	Close(ctx context.Context) error
}

type hubWrapperImpl struct {
	hub *eventhub.Hub
}

func (h *hubWrapperImpl) GetRuntimeInformation(ctx context.Context) (*eventhub.HubRuntimeInformation, error) {
	return h.hub.GetRuntimeInformation(ctx)
}

func (h *hubWrapperImpl) Receive(ctx context.Context, partitionID string, handler eventhub.Handler, opts ...eventhub.ReceiveOption) (listerHandleWrapper, error) {
	l, err := h.hub.Receive(ctx, partitionID, handler, opts...)
	return l, err
}

func (h *hubWrapperImpl) Close(ctx context.Context) error {
	return h.hub.Close(ctx)
}

type listerHandleWrapper interface {
	Done() <-chan struct{}
	Err() error
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

	storageClient, err := adapter.GetStorageClient(ctx, host, h.config.StorageID, h.settings.ID)
	if err != nil {
		h.settings.Logger.Debug("Error connecting to Storage", zap.Error(err))
		return err
	}

	if h.hub == nil { // set manually for testing.
		hub, newHubErr := eventhub.NewHubFromConnectionString(h.config.Connection, eventhub.HubWithOffsetPersistence(&storageCheckpointPersister{storageClient: storageClient}))
		if newHubErr != nil {
			h.settings.Logger.Debug("Error connecting to Event Hub", zap.Error(newHubErr))
			return newHubErr
		}
		h.hub = &hubWrapperImpl{
			hub: hub,
		}
	}

	if h.config.Partition == "" {
		// listen to each partition of the Event Hub
		var runtimeInfo *eventhub.HubRuntimeInformation
		runtimeInfo, err = h.hub.GetRuntimeInformation(ctx)
		if err != nil {
			h.settings.Logger.Debug("Error getting Runtime Information", zap.Error(err))
			return err
		}

		for _, partitionID := range runtimeInfo.PartitionIDs {
			err = h.setUpOnePartition(ctx, partitionID, false)
			if err != nil {
				h.settings.Logger.Debug("Error setting up partition", zap.Error(err))
				return err
			}
		}
	} else {
		err = h.setUpOnePartition(ctx, h.config.Partition, true)
		if err != nil {
			h.settings.Logger.Debug("Error setting up partition", zap.Error(err))
			return err
		}
	}

	if h.hub != nil {
		return nil
	}

	hub, err := eventhub.NewHubFromConnectionString(h.config.Connection)
	if err != nil {
		return err
	}

	h.hub = &hubWrapperImpl{
		hub: hub,
	}

	runtimeInfo, err := hub.GetRuntimeInformation(ctx)
	if err != nil {
		return err
	}

	for _, partitionID := range runtimeInfo.PartitionIDs {
		_, err := hub.Receive(ctx, partitionID, h.newMessageHandler, eventhub.ReceiveWithLatestOffset())
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *eventhubHandler) setUpOnePartition(ctx context.Context, partitionID string, applyOffset bool) error {
	receiverOptions := []eventhub.ReceiveOption{}
	if applyOffset && h.config.Offset != "" {
		receiverOptions = append(receiverOptions, eventhub.ReceiveWithStartingOffset(h.config.Offset))
	} else {
		receiverOptions = append(receiverOptions, eventhub.ReceiveWithLatestOffset())
	}

	if h.config.ConsumerGroup != "" {
		receiverOptions = append(receiverOptions, eventhub.ReceiveWithConsumerGroup(h.config.ConsumerGroup))
	}

	handle, err := h.hub.Receive(ctx, partitionID, h.newMessageHandler, receiverOptions...)
	if err != nil {
		return err
	}
	go func() {
		<-handle.Done()
		err := handle.Err()
		if err != nil {
			h.settings.Logger.Error("Error reported by event hub", zap.Error(err))
		}
	}()

	return nil
}

func (h *eventhubHandler) newMessageHandler(ctx context.Context, event *eventhub.Event) error {
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
