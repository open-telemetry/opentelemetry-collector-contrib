// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"go.uber.org/zap"
)

var (
	defaultPollRate      = 5
	defaultMaxPollEvents = 100
	defaultConsumerGroup = "$Default"
)

type blobEventHandler interface {
	run(ctx context.Context) error
	close(ctx context.Context) error
	setLogsDataConsumer(logsDataConsumer logsDataConsumer)
	setTracesDataConsumer(tracesDataConsumer tracesDataConsumer)
}

type azureBlobEventHandler struct {
	blobClient               blobClient
	logsDataConsumer         logsDataConsumer
	tracesDataConsumer       tracesDataConsumer
	logsContainerName        string
	tracesContainerName      string
	eventHubConnectionString string
	hub                      *azeventhubs.ConsumerClient
	logger                   *zap.Logger
	wg                       sync.WaitGroup
	cancelFunc               context.CancelFunc
}

var _ blobEventHandler = (*azureBlobEventHandler)(nil)

const (
	blobCreatedEventType = "Microsoft.Storage.BlobCreated"
)

func (p *azureBlobEventHandler) run(ctx context.Context) error {
	if p.hub != nil {
		return nil
	}

	// The event hub name is empty because it's extracted from the connection string's EntityPath.
	// The consumer group "$Default" is the default consumer group for Event Hubs.
	hub, err := azeventhubs.NewConsumerClientFromConnectionString(
		p.eventHubConnectionString,
		"",
		defaultConsumerGroup,
		&azeventhubs.ConsumerClientOptions{},
	)
	if err != nil {
		return err
	}

	p.hub = hub

	runtimeInfo, err := hub.GetEventHubProperties(ctx, &azeventhubs.GetEventHubPropertiesOptions{})
	if err != nil {
		return err
	}

	startAtLatest := true
	pcCtx, cancelFunc := context.WithCancel(ctx)
	p.cancelFunc = cancelFunc
	for _, partitionID := range runtimeInfo.PartitionIDs {
		p.wg.Add(1)
		pc, err := p.hub.NewPartitionClient(partitionID, &azeventhubs.PartitionClientOptions{
			StartPosition: azeventhubs.StartPosition{
				Latest: &startAtLatest,
			},
		})
		if err != nil {
			p.logger.Error("error creating partition client", zap.Error(err))
			return err
		}
		go p.receiveEvents(pcCtx, pc, p.newMessageHandler)
	}

	return nil
}

func (p *azureBlobEventHandler) receiveEvents(
	ctx context.Context,
	pc *azeventhubs.PartitionClient,
	handler func(ctx context.Context, event *azeventhubs.ReceivedEventData) error,
) {
	defer p.wg.Done()
	defer pc.Close(ctx)
	defer p.logger.Info("partition client closed")

	for {
		if ctx.Err() != nil {
			return
		}

		timeoutCtx, cancelCtx := context.WithTimeout(
			ctx,
			time.Second*time.Duration(defaultPollRate),
		)
		events, err := pc.ReceiveEvents(timeoutCtx, defaultMaxPollEvents, &azeventhubs.ReceiveEventsOptions{})
		cancelCtx()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			p.logger.Error(
				"error receiving events from partition",
				zap.Error(err),
			)
		}

		for _, event := range events {
			if handlerErr := handler(ctx, event); handlerErr != nil {
				p.logger.Error("error handling event", zap.Error(handlerErr))
			}
		}
	}
}

func (p *azureBlobEventHandler) newMessageHandler(ctx context.Context, event *azeventhubs.ReceivedEventData) error {
	type eventData struct {
		Topic           string
		Subject         string
		EventType       string
		ID              string
		Data            map[string]any
		DataVersion     string
		MetadataVersion string
		EventTime       string
	}
	var eventDataSlice []eventData
	marshalErr := json.Unmarshal(event.Body, &eventDataSlice)
	if marshalErr != nil {
		return marshalErr
	}
	subject := eventDataSlice[0].Subject
	containerName := strings.Split(strings.Split(subject, "containers/")[1], "/")[0]
	eventType := eventDataSlice[0].EventType
	blobName := strings.Split(subject, "blobs/")[1]

	if eventType == blobCreatedEventType {
		blobData, err := p.blobClient.readBlob(ctx, containerName, blobName)
		if err != nil {
			return err
		}
		switch containerName {
		case p.logsContainerName:
			err = p.logsDataConsumer.consumeLogsJSON(ctx, blobData.Bytes())
			if err != nil {
				return err
			}
		case p.tracesContainerName:
			err = p.tracesDataConsumer.consumeTracesJSON(ctx, blobData.Bytes())
			if err != nil {
				return err
			}
		default:
			p.logger.Debug("Unknown container name", zap.String("containerName", containerName))
		}
	}

	return nil
}

func (p *azureBlobEventHandler) close(ctx context.Context) error {
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	// Wait for all partition receiver goroutines to finish
	p.wg.Wait()
	if p.hub != nil {
		err := p.hub.Close(ctx)
		if err != nil {
			return err
		}
		p.hub = nil
	}
	return nil
}

func (p *azureBlobEventHandler) setLogsDataConsumer(logsDataConsumer logsDataConsumer) {
	p.logsDataConsumer = logsDataConsumer
}

func (p *azureBlobEventHandler) setTracesDataConsumer(tracesDataConsumer tracesDataConsumer) {
	p.tracesDataConsumer = tracesDataConsumer
}

func newBlobEventHandler(eventHubConnectionString, logsContainerName, tracesContainerName string, blobClient blobClient, logger *zap.Logger) *azureBlobEventHandler {
	return &azureBlobEventHandler{
		blobClient:               blobClient,
		logsContainerName:        logsContainerName,
		tracesContainerName:      tracesContainerName,
		eventHubConnectionString: eventHubConnectionString,
		logger:                   logger,
	}
}
