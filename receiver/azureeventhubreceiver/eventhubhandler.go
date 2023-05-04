// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
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

	offsetOption := eventhub.ReceiveWithLatestOffset()
	if applyOffset && h.config.Offset != "" {
		offsetOption = eventhub.ReceiveWithStartingOffset(h.config.Offset)
	}

	handle, err := h.hub.Receive(ctx, partitionID, h.newMessageHandler, offsetOption)
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
