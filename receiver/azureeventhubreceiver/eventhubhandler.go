// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

type hubWrapper interface {
	GetRuntimeInformation(ctx context.Context) (*hubRuntimeInfo, error)
	Receive(ctx context.Context, partitionID string, handler hubHandler, applyOffset bool, logger *zap.Logger) (listenerHandleWrapper, error)
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

const (
	minRetryInterval = time.Second
	maxRetryInterval = time.Minute
)

type eventhubHandler struct {
	hub            hubWrapper
	dataConsumer   dataConsumer
	config         *Config
	settings       receiver.Settings
	cancel         context.CancelFunc
	storageClient  storage.Client
	consumerClient *azeventhubs.ConsumerClient // non-nil when in distributed mode
	wg             sync.WaitGroup              // tracks goroutines spawned by runDistributed
}

func shouldInitializeStorageClient(storageClient storage.Client, storageID *component.ID) bool {
	return storageClient == nil && storageID != nil
}

func (h *eventhubHandler) run(ctx context.Context, host component.Host) error {
	ctx, h.cancel = context.WithCancel(ctx)
	if h.config.BlobCheckpointStore != nil {
		return h.runDistributed(ctx, host)
	}
	return h.runSingle(ctx, host)
}

func (h *eventhubHandler) runSingle(ctx context.Context, host component.Host) error {
	if shouldInitializeStorageClient(h.storageClient, h.config.StorageID) { // set manually for testing.
		storageClient, err := adapter.GetStorageClient(ctx, host, h.config.StorageID, h.settings.ID)
		if err != nil {
			h.settings.Logger.Debug("Error connecting to Storage", zap.Error(err))
			return err
		}
		h.storageClient = storageClient
	}

	if h.hub == nil { // set manually for testing.
		newHub, err := newAzeventhubWrapper(h, host)
		if err != nil {
			return err
		}
		h.hub = newHub
	}

	hub := h.hub
	h.wg.Go(func() {
		h.setUpPartitionsWithRetry(ctx, host, hub)
	})
	return nil
}

func (h *eventhubHandler) setUpPartitionsWithRetry(ctx context.Context, host component.Host, hub hubWrapper) {
	delay := minRetryInterval
	started := map[string]bool{}
	for {
		err := h.setUpPartitions(ctx, hub, started)
		if err == nil {
			componentstatus.ReportStatus(host, componentstatus.NewEvent(componentstatus.StatusOK))
			return
		}
		if ctx.Err() != nil {
			return
		}
		h.settings.Logger.Error("Error setting up Event Hub consumption, retrying", zap.Error(err), zap.Duration("retry_in", delay))

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, maxRetryInterval)
		componentstatus.ReportStatus(host, componentstatus.NewRecoverableErrorEvent(err))
	}
}

func (h *eventhubHandler) setUpPartitions(ctx context.Context, hub hubWrapper, started map[string]bool) error {
	if h.config.Partition != "" {
		return h.setUpOnePartition(ctx, hub, h.config.Partition, true)
	}

	// listen to each partition of the Event Hub
	runtimeInfo, err := hub.GetRuntimeInformation(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, partitionID := range runtimeInfo.PartitionIDs {
		if started[partitionID] {
			continue
		}
		if err := h.setUpOnePartition(ctx, hub, partitionID, false); err != nil {
			errs = append(errs, fmt.Errorf("partition %s: %w", partitionID, err))
			continue
		}
		started[partitionID] = true
	}
	return errors.Join(errs...)
}

func (h *eventhubHandler) runDistributed(ctx context.Context, host component.Host) error {
	h.settings.Logger.Info("Starting distributed Event Hub consumption with blob checkpoint store")

	processor, consumerClient, err := createProcessor(h.config, host, h.settings.Logger)
	if err != nil {
		return err
	}
	h.consumerClient = consumerClient

	// Dispatch partition clients as the Processor assigns them.
	h.wg.Go(func() {
		for {
			partitionClient := processor.NextPartitionClient(ctx)
			if partitionClient == nil {
				// Processor has stopped
				return
			}
			h.wg.Go(func() {
				processPartitionEvents(ctx, partitionClient, h.newMessageHandler, h.config, h.settings.Logger)
			})
		}
	})

	// Run the processor's load balancer in a background goroutine.
	// It returns when the context is cancelled.
	h.wg.Go(func() {
		if err := processor.Run(ctx); err != nil {
			h.settings.Logger.Error("Processor exited with error", zap.Error(err))
		}
	})

	return nil
}

func (h *eventhubHandler) setUpOnePartition(ctx context.Context, hub hubWrapper, partitionID string, applyOffset bool) error {
	handle, err := hub.Receive(ctx, partitionID, h.newMessageHandler, applyOffset, h.settings.Logger)
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
	if h.cancel != nil {
		h.cancel()
	}
	h.wg.Wait()

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

	if h.consumerClient != nil {
		if err := h.consumerClient.Close(ctx); err != nil {
			errs = errors.Join(errs, err)
		}
		h.consumerClient = nil
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
