// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2/checkpoints"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

type checkpointSeqNumber struct {
	// Offset only used for backwards compatibility
	Offset         string `json:"offset"`
	SequenceNumber int64  `json:"sequenceNumber"`
}

// UnmarshalJSON is a custom unmarshaller to allow for backward compatibility
// with the sequence number field
func (c *checkpointSeqNumber) UnmarshalJSON(data []byte) error {
	// Primary struct shape
	type Alias checkpointSeqNumber
	var tmp struct {
		Alias
		SeqNumber *int64 `json:"seqNumber"` // fallback
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*c = checkpointSeqNumber(tmp.Alias)
	if tmp.SeqNumber != nil {
		c.SequenceNumber = *tmp.SeqNumber
	}
	return nil
}

type azPartitionClient interface {
	Close(ctx context.Context) error
	ReceiveEvents(ctx context.Context, maxBatchSize int, options *azeventhubs.ReceiveEventsOptions) ([]*azeventhubs.ReceivedEventData, error)
}

func getPollConfig(config *Config) (int, int) {
	maxPollEvents, pollRate := 100, 5
	if config != nil {
		if config.MaxPollEvents != 0 {
			maxPollEvents = config.MaxPollEvents
		}
		if config.PollRate != 0 {
			pollRate = config.PollRate
		}
	}
	return maxPollEvents, pollRate
}

func getConsumerGroup(config *Config) string {
	if config.ConsumerGroup == "" {
		return "$Default"
	}
	return config.ConsumerGroup
}

// createConsumerClient creates a new Azure Event Hub consumer client.
// If auth is configured, it uses the auth extension to create the client.
// Otherwise, it uses the connection string.
func createConsumerClient(
	config *Config,
	host component.Host,
	consumerGroup string,
	logger *zap.Logger,
) (*azeventhubs.ConsumerClient, error) {
	if config.Auth != nil {
		if config.Connection != "" {
			logger.Warn("both 'auth' and 'connection' are specified, 'connection' will be ignored.")
		}
		ext, ok := host.GetExtensions()[*config.Auth]
		if !ok {
			return nil, fmt.Errorf("failed to resolve auth extension %q", *config.Auth)
		}
		credential, ok := ext.(azcore.TokenCredential)
		if !ok {
			return nil, fmt.Errorf("extension %q does not implement azcore.TokenCredential", *config.Auth)
		}

		return azeventhubs.NewConsumerClient(
			config.EventHub.Namespace,
			config.EventHub.Name,
			consumerGroup,
			credential,
			&azeventhubs.ConsumerClientOptions{},
		)
	}
	return azeventhubs.NewConsumerClientFromConnectionString(
		config.Connection,
		"",
		consumerGroup,
		&azeventhubs.ConsumerClientOptions{},
	)
}

func newAzeventhubWrapper(h *eventhubHandler, host component.Host) (*hubWrapperAzeventhubImpl, error) {
	consumerGroup := getConsumerGroup(h.config)

	hub, newHubErr := createConsumerClient(h.config, host, consumerGroup, h.settings.Logger)
	if newHubErr != nil {
		h.settings.Logger.Debug("Error connecting to Event Hub", zap.Error(newHubErr))
		return nil, newHubErr
	}

	return &hubWrapperAzeventhubImpl{
		hub:     azEventHubWrapper{hub},
		config:  h.config,
		storage: getStorageCheckpointPersister(h.storageClient),
	}, nil
}

func getStorageCheckpointPersister(storageClient storage.Client) *storageCheckpointPersister[checkpointSeqNumber] {
	if storageClient == nil {
		return nil
	}
	return &storageCheckpointPersister[checkpointSeqNumber]{
		storageClient: storageClient,
		defaultValue: checkpointSeqNumber{
			SequenceNumber: -1,
		},
	}
}

type azEventHubWrapper struct {
	*azeventhubs.ConsumerClient
}

func (w azEventHubWrapper) GetEventHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error) {
	return w.ConsumerClient.GetEventHubProperties(ctx, options)
}

func (w azEventHubWrapper) GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error) {
	return w.ConsumerClient.GetPartitionProperties(ctx, partitionID, options)
}

func (w azEventHubWrapper) NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (azPartitionClient, error) {
	return w.ConsumerClient.NewPartitionClient(partitionID, options)
}

func (w azEventHubWrapper) Close(ctx context.Context) error {
	return w.ConsumerClient.Close(ctx)
}

type azEventHub interface {
	GetEventHubProperties(ctx context.Context, options *azeventhubs.GetEventHubPropertiesOptions) (azeventhubs.EventHubProperties, error)
	GetPartitionProperties(ctx context.Context, partitionID string, options *azeventhubs.GetPartitionPropertiesOptions) (azeventhubs.PartitionProperties, error)
	NewPartitionClient(partitionID string, options *azeventhubs.PartitionClientOptions) (azPartitionClient, error)
	Close(ctx context.Context) error
}

type hubWrapperAzeventhubImpl struct {
	hub     azEventHub
	config  *Config
	storage *storageCheckpointPersister[checkpointSeqNumber]
}

func (h *hubWrapperAzeventhubImpl) GetRuntimeInformation(ctx context.Context) (*hubRuntimeInfo, error) {
	if h.hub != nil {
		info, err := h.hub.GetEventHubProperties(ctx, nil)
		if err != nil {
			return nil, err
		}
		return &hubRuntimeInfo{
			CreatedAt:      info.CreatedOn,
			PartitionCount: len(info.PartitionIDs),
			PartitionIDs:   info.PartitionIDs,
			Path:           info.Name,
		}, nil
	}
	return nil, errNoConfig
}

func (h *hubWrapperAzeventhubImpl) Receive(ctx context.Context, partitionID string, handler hubHandler, applyOffset bool, logger *zap.Logger) (listenerHandleWrapper, error) {
	if h.hub != nil {
		namespace, err := h.namespace()
		if err != nil {
			return nil, err
		}
		pProps, err := h.hub.GetPartitionProperties(ctx, partitionID, nil)
		if err != nil {
			return nil, err
		}
		startPos := h.getStartPos(
			applyOffset,
			namespace,
			pProps.EventHubName,
			getConsumerGroup(h.config),
			partitionID,
		)
		pc, err := h.hub.NewPartitionClient(partitionID, &azeventhubs.PartitionClientOptions{
			StartPosition: startPos,
			Prefetch:      h.config.PrefetchCount,
		})
		if err != nil {
			return nil, err
		}

		w := &partitionListener{
			done: make(chan struct{}),
		}

		go func() {
			defer close(w.done)
			defer pc.Close(ctx)

			maxPollEvents, pollRate := getPollConfig(h.config)
			for {
				if ctx.Err() != nil {
					return
				}

				timeout, cancelTimeout := context.WithTimeout(ctx, time.Second*time.Duration(pollRate))
				events, err := pc.ReceiveEvents(timeout, maxPollEvents, nil)
				cancelTimeout()
				if err != nil && !errors.Is(err, context.DeadlineExceeded) {
					logger.Error("error receiving events", zap.Error(err))
					continue
				}

				for _, ev := range events {
					if err := handler(ctx, &azureEvent{
						AzEventData: ev,
					}); err != nil {
						logger.Error("error processing event", zap.Error(err))
						continue
					}
				}

				if len(events) > 0 {
					lastEvent := events[len(events)-1]

					if h.storage != nil {
						err := h.storage.Write(
							namespace, pProps.EventHubName, getConsumerGroup(h.config), partitionID, checkpointSeqNumber{
								SequenceNumber: lastEvent.SequenceNumber,
							},
						)
						if err != nil {
							logger.Error("error writing checkpoint", zap.Error(err))
							continue
						}
					}
				}
			}
		}()

		return w, nil
	}
	return nil, errNoConfig
}

func (h *hubWrapperAzeventhubImpl) Close(ctx context.Context) error {
	if h.hub != nil {
		return h.hub.Close(ctx)
	}
	return errNoConfig
}

func (h *hubWrapperAzeventhubImpl) getStartPos(
	applyOffset bool,
	namespace string,
	eventHubName string,
	consumerGroup string,
	partitionID string,
) azeventhubs.StartPosition {
	startPos := azeventhubs.StartPosition{Latest: to.Ptr(true)}
	if applyOffset && h.config.Offset != "" {
		startPos = azeventhubs.StartPosition{Offset: &h.config.Offset}
	}
	if h.storage != nil {
		checkpoint, readErr := h.storage.Read(
			namespace,
			eventHubName,
			consumerGroup,
			partitionID,
		)
		// Only apply the checkpoint seq number offset if we have one saved
		if readErr == nil && checkpoint.SequenceNumber != -1 && checkpoint.Offset != "@latest" {
			startPos = azeventhubs.StartPosition{
				SequenceNumber: &checkpoint.SequenceNumber,
			}
		}
	}

	return startPos
}

func (h *hubWrapperAzeventhubImpl) namespace() (string, error) {
	if h.config.Auth != nil {
		return h.config.EventHub.Namespace, nil
	}
	parsed, err := azeventhubs.ParseConnectionString(h.config.Connection)
	if err != nil {
		return "", err
	}

	// Return the first part of the namespace
	// Ex: example.servicebus.windows.net => example
	n := parsed.FullyQualifiedNamespace
	if s := strings.Split(n, "."); len(s) > 0 {
		n = s[0]
	}
	return n, nil
}

type partitionListener struct {
	done chan struct{}
	mu   sync.Mutex
	err  error
}

func (p *partitionListener) Done() <-chan struct{} { return p.done }

func (p *partitionListener) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *partitionListener) setErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

// createBlobCheckpointStore creates an Azure Blob Storage-backed checkpoint store
// for use with the distributed Processor.
func createBlobCheckpointStore(config *Config, host component.Host, logger *zap.Logger) (azeventhubs.CheckpointStore, error) {
	blobCfg := config.BlobCheckpointStore

	var containerClient *container.Client

	switch {
	case blobCfg.Connection != "":
		var err error
		containerClient, err = container.NewClientFromConnectionString(blobCfg.Connection, blobCfg.ContainerName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create blob container client from connection string: %w", err)
		}
	case config.Auth != nil:
		ext, ok := host.GetExtensions()[*config.Auth]
		if !ok {
			return nil, fmt.Errorf("failed to resolve auth extension %q for blob checkpoint store", *config.Auth)
		}
		credential, ok := ext.(azcore.TokenCredential)
		if !ok {
			return nil, fmt.Errorf("extension %q does not implement azcore.TokenCredential", *config.Auth)
		}
		containerURL, err := url.JoinPath(blobCfg.StorageAccountURL, blobCfg.ContainerName)
		if err != nil {
			return nil, fmt.Errorf("failed to construct container URL: %w", err)
		}
		containerClient, err = container.NewClient(containerURL, credential, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create blob container client with auth: %w", err)
		}
	default:
		return nil, errors.New("blob checkpoint store requires either a connection string or auth extension")
	}

	store, err := checkpoints.NewBlobStore(containerClient, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create blob checkpoint store: %w", err)
	}

	logger.Info("Created blob checkpoint store", zap.String("container", blobCfg.ContainerName))
	return store, nil
}

// createProcessor creates an Azure Event Hub Processor for distributed consumption.
// It returns both the Processor and the ConsumerClient so the caller can close the
// client on shutdown (the Processor does not take ownership of closing it).
func createProcessor(config *Config, host component.Host, logger *zap.Logger) (*azeventhubs.Processor, *azeventhubs.ConsumerClient, error) {
	consumerGroup := getConsumerGroup(config)
	consumerClient, err := createConsumerClient(config, host, consumerGroup, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create consumer client for processor: %w", err)
	}

	checkpointStore, err := createBlobCheckpointStore(config, host, logger)
	if err != nil {
		consumerClient.Close(context.Background())
		return nil, nil, err
	}

	processor, err := azeventhubs.NewProcessor(consumerClient, checkpointStore, &azeventhubs.ProcessorOptions{
		Prefetch: config.PrefetchCount,
	})
	if err != nil {
		consumerClient.Close(context.Background())
		return nil, nil, fmt.Errorf("failed to create processor: %w", err)
	}

	return processor, consumerClient, nil
}

// processorPartitionClient is an interface for the subset of
// azeventhubs.ProcessorPartitionClient methods used by processPartitionEvents.
// This enables testing without requiring a real Azure connection.
type processorPartitionClient interface {
	PartitionID() string
	ReceiveEvents(ctx context.Context, count int, options *azeventhubs.ReceiveEventsOptions) ([]*azeventhubs.ReceivedEventData, error)
	UpdateCheckpoint(ctx context.Context, latestEvent *azeventhubs.ReceivedEventData, options *azeventhubs.UpdateCheckpointOptions) error
	Close(ctx context.Context) error
}

// processPartitionEvents receives and processes events from a single partition
// assigned by the Processor.
func processPartitionEvents(
	ctx context.Context,
	partitionClient processorPartitionClient,
	handler hubHandler,
	config *Config,
	logger *zap.Logger,
) {
	logger.Debug("Starting distributed processing for partition", zap.String("partition", partitionClient.PartitionID()))
	defer partitionClient.Close(context.Background())

	maxPollEvents, pollRate := getPollConfig(config)

	for {
		if ctx.Err() != nil {
			return
		}

		timeout, cancelTimeout := context.WithTimeout(ctx, time.Second*time.Duration(pollRate))
		events, err := partitionClient.ReceiveEvents(timeout, maxPollEvents, nil)
		cancelTimeout()

		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			var eventHubError *azeventhubs.Error
			if errors.As(err, &eventHubError) && eventHubError.Code == azeventhubs.ErrorCodeOwnershipLost {
				logger.Debug("Partition ownership lost, stopping processing", zap.String("partition", partitionClient.PartitionID()))
				return
			}
			logger.Error("Error receiving events in distributed mode", zap.Error(err), zap.String("partition", partitionClient.PartitionID()))
			continue
		}

		var lastSuccessful *azeventhubs.ReceivedEventData
		for _, ev := range events {
			if err := handler(ctx, &azureEvent{
				AzEventData: ev,
			}); err != nil {
				logger.Error("Error processing event in distributed mode", zap.Error(err), zap.String("partition", partitionClient.PartitionID()))
				break
			}
			lastSuccessful = ev
		}

		if lastSuccessful != nil {
			if err := partitionClient.UpdateCheckpoint(ctx, lastSuccessful, nil); err != nil {
				logger.Error("Error updating checkpoint", zap.Error(err), zap.String("partition", partitionClient.PartitionID()))
			}
		}
	}
}
