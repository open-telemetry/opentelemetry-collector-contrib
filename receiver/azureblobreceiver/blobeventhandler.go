// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"encoding/json"
	"strings"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type blobEventHandler interface {
	run(ctx context.Context) error
	close(ctx context.Context) error
	setLogsDataConsumer(logsDataConsumer logsDataConsumer)
	setTracesDataConsumer(tracesDataConsumer tracesDataConsumer)
	getLogsEncoding() *component.ID
	getTracesEncoding() *component.ID
}

type azureBlobEventHandler struct {
	blobClient               blobClient
	logsDataConsumer         logsDataConsumer
	tracesDataConsumer       tracesDataConsumer
	logsContainerName        string
	logsEncoding             *component.ID
	tracesContainerName      string
	tracesEncoding           *component.ID
	eventHubConnectionString string
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

	hub, err := eventhub.NewHubFromConnectionString(p.eventHubConnectionString)
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
		Data            map[string]any
		DataVersion     string
		MetadataVersion string
		EventTime       string
	}
	var eventDataSlice []eventData
	marshalErr := json.Unmarshal(event.Data, &eventDataSlice)
	if marshalErr != nil {
		return marshalErr
	}
	for _, event := range eventDataSlice {
		subject := event.Subject
		containerName := strings.Split(strings.Split(subject, "containers/")[1], "/")[0]
		eventType := event.EventType
		blobName := strings.Split(subject, "blobs/")[1]

		if eventType == blobCreatedEventType {
			p.logger.Info("Staring reading blob", zap.String("containerName", containerName), zap.String("blobName", blobName))
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

func (p *azureBlobEventHandler) getLogsEncoding() *component.ID {
	return p.logsEncoding
}

func (p *azureBlobEventHandler) getTracesEncoding() *component.ID {
	return p.tracesEncoding
}

func newBlobEventHandler(cfg *Config, blobClient blobClient, logger *zap.Logger) *azureBlobEventHandler {
	return &azureBlobEventHandler{
		blobClient:               blobClient,
		logsContainerName:        cfg.Logs.ContainerName,
		logsEncoding:             cfg.Logs.Encodings,
		tracesContainerName:      cfg.Traces.ContainerName,
		tracesEncoding:           cfg.Traces.Encodings,
		eventHubConnectionString: cfg.EventHub.EndPoint,
		logger:                   logger,
	}
}
