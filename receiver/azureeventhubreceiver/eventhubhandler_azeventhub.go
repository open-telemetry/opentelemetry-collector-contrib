// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"go.uber.org/zap"
)

type checkpointSeqNumber struct {
	SeqNumber int64 `json:"seqNumber"`
}

type azPartitionClient interface {
	Close(ctx context.Context) error
	ReceiveEvents(ctx context.Context, maxBatchSize int, options *azeventhubs.ReceiveEventsOptions) ([]*azeventhubs.ReceivedEventData, error)
}

func newAzeventhubWrapper(h *eventhubHandler) (*hubWrapperAzeventhubImpl, error) {
	hub, newHubErr := azeventhubs.NewConsumerClientFromConnectionString(
		h.config.Connection,
		"",
		h.config.ConsumerGroup,
		&azeventhubs.ConsumerClientOptions{},
	)

	if newHubErr != nil {
		h.settings.Logger.Debug("Error connecting to Event Hub", zap.Error(newHubErr))
		return nil, newHubErr
	}

	var storage *storageCheckpointPersister[checkpointSeqNumber]
	if h.storageClient != nil {
		storage = &storageCheckpointPersister[checkpointSeqNumber]{
			storageClient: h.storageClient,
			defaultValue: checkpointSeqNumber{
				SeqNumber: -1,
			},
		}
	}

	return &hubWrapperAzeventhubImpl{
		hub:     azEventHubWrapper{hub},
		config:  h.config,
		storage: storage,
	}, nil
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

func (h *hubWrapperAzeventhubImpl) Receive(ctx context.Context, partitionID string, handler hubHandler, applyOffset bool) (listenerHandleWrapper, error) {
	if h.hub != nil {
		namespace, err := h.namespace()
		if err != nil {
			return nil, err
		}
		pProps, err := h.hub.GetPartitionProperties(ctx, partitionID, nil)
		if err != nil {
			return nil, err
		}
		startPos := azeventhubs.StartPosition{Latest: to.Ptr(true)}
		if applyOffset && h.config.Offset != "" {
			startPos = azeventhubs.StartPosition{Offset: &h.config.Offset}
		}
		if h.storage != nil {
			checkpoint, readErr := h.storage.Read(namespace, pProps.EventHubName, h.config.ConsumerGroup, partitionID)
			if readErr == nil {
				startPos = azeventhubs.StartPosition{
					SequenceNumber: &checkpoint.SeqNumber,
				}
			}
		}

		pc, err := h.hub.NewPartitionClient(partitionID, &azeventhubs.PartitionClientOptions{
			StartPosition: startPos,
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

			for {
				if ctx.Err() != nil {
					return
				}

				maxPollEvents := 100
				pollRate := 5
				if h.config != nil {
					if h.config.MaxPollEvents != 0 {
						maxPollEvents = h.config.MaxPollEvents
					}
					if h.config.PollRate != 0 {
						pollRate = h.config.PollRate
					}
				}
				timeout, cancelTimeout := context.WithTimeout(ctx, time.Second*time.Duration(pollRate))
				events, err := pc.ReceiveEvents(timeout, maxPollEvents, nil)
				cancelTimeout()
				if err != nil && !errors.Is(err, context.DeadlineExceeded) {
					w.setErr(err)
					return
				}

				for _, ev := range events {
					if err := handler(ctx, &azureEvent{
						AzEventData: ev,
					}); err != nil {
						w.setErr(err)
						return
					}
				}

				if len(events) > 0 {
					lastEvent := events[len(events)-1]

					if h.storage != nil {
						err := h.storage.Write(
							namespace, pProps.EventHubName, h.config.ConsumerGroup, partitionID, checkpointSeqNumber{
								SeqNumber: lastEvent.SequenceNumber,
							},
						)
						if err != nil {
							w.setErr(err)
							return
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

func (h *hubWrapperAzeventhubImpl) namespace() (string, error) {
	parsed, err := azeventhubs.ParseConnectionString(h.config.Connection)
	if err != nil {
		return "", err
	}

	return parsed.FullyQualifiedNamespace, nil
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
