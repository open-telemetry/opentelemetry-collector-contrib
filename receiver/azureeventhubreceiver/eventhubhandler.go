// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

type hubWrapper interface {
	GetRuntimeInformation(ctx context.Context) (*hubRuntimeInfo, error)
	Receive(ctx context.Context, partitionID string, handler hubHandler, applyOffset bool) (listenerHandleWrapper, error)
	Close(ctx context.Context) error
}

type listenerHandleWrapper interface {
	Done() <-chan struct{}
	Err() error
}

type hubHandler func(ctx context.Context, event *azureEvent) error

type hubRuntimeInfo struct {
	Path           string
	CreatedAt      time.Time
	PartitionCount int
	PartitionIDs   []string
}

var errNoConfig = errors.New("Configuration error, hub not accessible")

type eventhubHandler struct {
	hub           hubWrapper
	dataConsumer  dataConsumer
	config        *Config
	settings      receiver.Settings
	cancel        context.CancelFunc
	storageClient storage.Client
}

func shouldInitializeStorageClient(storageClient storage.Client, storageID *component.ID) bool {
	return storageClient == nil && storageID != nil
}

func (h *eventhubHandler) run(ctx context.Context, host component.Host) error {
	ctx, h.cancel = context.WithCancel(ctx)

	if shouldInitializeStorageClient(h.storageClient, h.config.StorageID) { // set manually for testing.
		storageClient, err := adapter.GetStorageClient(ctx, host, h.config.StorageID, h.settings.ID)
		if err != nil {
			h.settings.Logger.Debug("Error connecting to Storage", zap.Error(err))
			return err
		}
		h.storageClient = storageClient
	}

	if h.hub == nil { // set manually for testing.
		if azEventHubFeatureGate.IsEnabled() {
			newHub, err := newAzeventhubWrapper(h)
			if err != nil {
				return err
			}
			h.hub = newHub
		} else {
			newHub, err := newLegacyHubWrapper(h)
			if err != nil {
				return err
			}
			h.hub = newHub
		}
	}

	if h.config.Partition != "" {
		err := h.setUpOnePartition(ctx, h.config.Partition, true)
		if err != nil {
			h.settings.Logger.Debug("Error setting up partition", zap.Error(err))
		}
		return err
	}

	// listen to each partition of the Event Hub
	runtimeInfo, err := h.hub.GetRuntimeInformation(ctx)
	if err != nil {
		h.settings.Logger.Debug("Error getting Runtime Information", zap.Error(err))
		return err
	}

	var errs []error
	for _, partitionID := range runtimeInfo.PartitionIDs {
		err = h.setUpOnePartition(ctx, partitionID, false)
		if err != nil {
			h.settings.Logger.Debug("Error setting up partition", zap.Error(err), zap.String("partition", partitionID))
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h *eventhubHandler) setUpOnePartition(ctx context.Context, partitionID string, applyOffset bool) error {
	handle, err := h.hub.Receive(ctx, partitionID, h.newMessageHandler, applyOffset)
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

func (h *eventhubHandler) newMessageHandler(ctx context.Context, event *azureEvent) error {
	err := h.dataConsumer.consume(ctx, event)
	if err != nil {
		h.settings.Logger.Error("error decoding message", zap.Error(err))
		return err
	}

	return nil
}

func (h *eventhubHandler) close(ctx context.Context) error {
	var errs error
	if h.storageClient != nil {
		if err := h.storageClient.Close(ctx); err != nil {
			errs = errors.Join(errs, err)
		}
		h.storageClient = nil
	}

	if h.hub != nil {
		err := h.hub.Close(ctx)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		h.hub = nil
	}
	if h.cancel != nil {
		h.cancel()
	}

	return errs
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
