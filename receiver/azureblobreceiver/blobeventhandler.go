// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"encoding/json"
	"strings"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.uber.org/zap"
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
	eventHubSonnectionString string
	hub                      *eventhub.Hub
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

	hub, err := eventhub.NewHubFromConnectionString(p.eventHubSonnectionString)
	if err != nil {
		return err
	}

	p.hub = hub

	runtimeInfo, err := hub.GetRuntimeInformation(ctx)
	if err != nil {
		return err
	}

	for _, partitionID := range runtimeInfo.PartitionIDs {
		_, err := hub.Receive(ctx, partitionID, p.newMessageHandler, eventhub.ReceiveWithLatestOffset())
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *azureBlobEventHandler) newMessageHandler(ctx context.Context, event *eventhub.Event) error {

	type eventData struct {
		Topic           string
		Subject         string
		EventType       string
		ID              string
		Data            map[string]interface{}
		DataVersion     string
		MetadataVersion string
		EsventTime      string
	}
	var eventDataSlice []eventData
	marshalErr := json.Unmarshal(event.Data, &eventDataSlice)
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
		switch {
		case containerName == p.logsContainerName:
			err = p.logsDataConsumer.consumeLogsJSON(ctx, blobData.Bytes())
			if err != nil {
				return err
			}
		case containerName == p.tracesContainerName:
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

func newBlobEventHandler(eventHubSonnectionString string, logsContainerName string, tracesContainerName string, blobClient blobClient, logger *zap.Logger) *azureBlobEventHandler {
	return &azureBlobEventHandler{
		blobClient:               blobClient,
		logsContainerName:        logsContainerName,
		tracesContainerName:      tracesContainerName,
		eventHubSonnectionString: eventHubSonnectionString,
		logger:                   logger,
	}
}
