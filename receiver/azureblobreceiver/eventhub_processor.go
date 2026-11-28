// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"go.uber.org/zap"
)

type processorPartitionClient interface {
	PartitionID() string
	ReceiveEvents(context.Context, int, *azeventhubs.ReceiveEventsOptions) ([]*azeventhubs.ReceivedEventData, error)
	UpdateCheckpoint(context.Context, *azeventhubs.ReceivedEventData, *azeventhubs.UpdateCheckpointOptions) error
	Close(context.Context) error
}

type eventHubProcessor interface {
	Run(context.Context) error
	NextPartitionClient(context.Context) processorPartitionClient
}

type sdkEventHubProcessor struct {
	*azeventhubs.Processor
}

func (p sdkEventHubProcessor) NextPartitionClient(ctx context.Context) processorPartitionClient {
	client := p.Processor.NextPartitionClient(ctx)
	if client == nil {
		return nil
	}
	return client
}

func (p *eventHubEventHandler) runCoordinated(ctx context.Context) {
	delay := time.Second
	for ctx.Err() == nil {
		earliest := true
		processor, err := azeventhubs.NewProcessor(p.hub, p.checkpointStore, &azeventhubs.ProcessorOptions{
			StartPositions: azeventhubs.StartPositions{Default: azeventhubs.StartPosition{Earliest: &earliest}},
		})
		if err == nil {
			err = p.runProcessor(ctx, sdkEventHubProcessor{processor})
		}
		if ctx.Err() != nil {
			return
		}
		p.logger.Error("Event Hub processor stopped; restarting", zap.Error(err), zap.Duration("retry_delay", delay))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		delay = min(delay*2, 30*time.Second)
	}
}

func (p *eventHubEventHandler) runProcessor(ctx context.Context, processor eventHubProcessor) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- processor.Run(ctx)
		cancel()
	}()

	var workers sync.WaitGroup
	for {
		client := processor.NextPartitionClient(ctx)
		if client == nil {
			break
		}
		workers.Go(func() { p.processPartition(ctx, client) })
	}
	cancel()
	workers.Wait()
	return <-done
}

func (p *eventHubEventHandler) processPartition(ctx context.Context, client processorPartitionClient) {
	logger := p.logger.With(zap.String("partition", client.PartitionID()))
	logger.Info("Starting assigned Event Hub partition")
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Close(closeCtx); err != nil {
			logger.Warn("Failed to close partition client", zap.Error(err))
		}
		logger.Info("Stopped assigned Event Hub partition")
	}()

	for ctx.Err() == nil {
		receiveCtx, cancel := context.WithTimeout(ctx, time.Duration(p.pollRate)*time.Second)
		events, err := client.ReceiveEvents(receiveCtx, p.maxPollEvents, nil)
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			var hubErr *azeventhubs.Error
			if errors.As(err, &hubErr) && hubErr.Code == azeventhubs.ErrorCodeOwnershipLost {
				logger.Info("Event Hub partition ownership lost")
			} else if ctx.Err() == nil {
				logger.Error("Failed to receive partition events", zap.Error(err))
			}
			return
		}
		for _, event := range events {
			if ctx.Err() != nil {
				return
			}
			if err := p.newMessageHandler(ctx, event); err != nil {
				logger.Error("Failed to process event; partition will resume from its checkpoint", zap.Int64("sequence_number", event.SequenceNumber), zap.Error(err))
				return
			}
			if ctx.Err() != nil {
				return
			}
			if err := client.UpdateCheckpoint(ctx, event, nil); err != nil {
				logger.Error("Failed to checkpoint event; partition will resume from its checkpoint", zap.Int64("sequence_number", event.SequenceNumber), zap.Error(err))
				return
			}
		}
	}
}
