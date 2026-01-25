// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"go.uber.org/zap"
)

var (
	DEFAULT_POLL_RATE      = 5
	DEFAULT_MAX_POLL_EVENT = 100
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
	maxPollEvents            int
	pollRate                 int
	logger                   *zap.Logger
}

var _ blobEventHandler = (*azureBlobEventHandler)(nil)

const (
	blobCreatedEventType = "Microsoft.Storage.BlobCreated"
)

func (p *azureBlobEventHandler) run(ctx context.Context) error {
	if p.hub != nil {
		return nil
	}

	hub, err := azeventhubs.NewConsumerClientFromConnectionString(
		p.eventHubConnectionString,
		"",
		"",
		&azeventhubs.ConsumerClientOptions{},
	)

	p.hub = hub

	runtimeInfo, err := hub.GetEventHubProperties(ctx, &azeventhubs.GetEventHubPropertiesOptions{})
	if err != nil {
		return err
	}

	for _, partitionID := range runtimeInfo.PartitionIDs {
		go p.ReceiveEvents(ctx, hub, partitionID, p.newMessageHandler)
	}

	return nil
}

func (p *azureBlobEventHandler) ReceiveEvents(
	ctx context.Context,
	client *azeventhubs.ConsumerClient,
	partitionID string,
	handler func(ctx context.Context, event *azeventhubs.ReceivedEventData) error,
) {
	startAtLatest := true
	pc, err := client.NewPartitionClient(partitionID, &azeventhubs.PartitionClientOptions{
		StartPosition: azeventhubs.StartPosition{
			Latest: &startAtLatest,
		},
	})
	if err != nil {
		p.logger.Error("error creating partition client", zap.Error(err))
	}

	for {
		if ctx.Err() != nil {
			return
		}

		pollRate := p.pollRate
		if pollRate <= 0 {
			pollRate = DEFAULT_POLL_RATE
		}
		maxPollEvent := p.maxPollEvents
		if maxPollEvent <= 0 {
			maxPollEvent = DEFAULT_MAX_POLL_EVENT
		}

		timeoutCtx, cancelCxt := context.WithTimeout(
			ctx,
			time.Second*time.Duration(pollRate),
		)
		events, err := pc.ReceiveEvents(timeoutCtx, maxPollEvent, &azeventhubs.ReceiveEventsOptions{})
		if err != nil {
			p.logger.Error(
				"error receiving events from partition",
				zap.String("partitionID", partitionID),
				zap.Error(err),
			)
		}
		cancelCxt()

		for _, event := range events {
			handler(ctx, event)
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

func newBlobEventHandler(eventHubConnectionString, logsContainerName, tracesContainerName string, blobClient blobClient, maxPollEvent int, pollRate int, logger *zap.Logger) *azureBlobEventHandler {
	return &azureBlobEventHandler{
		blobClient:               blobClient,
		logsContainerName:        logsContainerName,
		tracesContainerName:      tracesContainerName,
		eventHubConnectionString: eventHubConnectionString,
		logger:                   logger,
		maxPollEvents:            maxPollEvent,
		pollRate:                 pollRate,
	}
}
