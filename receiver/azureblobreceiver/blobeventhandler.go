// Copyright OpenTelemetry Authors
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

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.uber.org/zap"
)

type BlobEventHandler interface {
	Run(ctx context.Context) error
	Close(ctx context.Context) error
	SetLogsDataConsumer(logsDataConsumer LogsDataConsumer)
	SetTracesDataConsumer(tracesDataConsumer TracesDataConsumer)
}

type AzureBlobEventHandler struct {
	blobClient               BlobClient
	logsDataConsumer         LogsDataConsumer
	tracesDataConsumer       TracesDataConsumer
	logsContainerName        string
	tracesContainerName      string
	eventHubSonnectionString string
	hub                      *eventhub.Hub
	logger                   *zap.Logger
}

const (
	blobCreatedEventType = "Microsoft.Storage.BlobCreated"
)

func (p *AzureBlobEventHandler) Run(ctx context.Context) error {

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
		_, err := hub.Receive(ctx, partitionID, p.newMessageHangdler, eventhub.ReceiveWithLatestOffset())
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *AzureBlobEventHandler) newMessageHangdler(ctx context.Context, event *eventhub.Event) error {
	p.logger.Debug(fmt.Sprintf("New event: %s", string(event.Data)))

	var eventDataSlice []map[string]interface{}
	json.Unmarshal(event.Data, &eventDataSlice)

	subject := eventDataSlice[0]["subject"].(string)
	containerName := strings.SplitN(strings.SplitN(subject, "containers/", -1)[1], "/", -1)[0]
	eventType := eventDataSlice[0]["eventType"].(string)
	blobName := strings.SplitN(subject, "blobs/", -1)[1]

	p.logger.Debug(fmt.Sprintf("containerName: %s", containerName))
	p.logger.Debug(fmt.Sprintf("blobName: %s", blobName))

	if eventType == blobCreatedEventType {
		blobData, err := p.blobClient.ReadBlob(ctx, containerName, blobName)

		if err != nil {
			p.logger.Error(err.Error())
			return err
		}
		if containerName == p.logsContainerName {
			p.logsDataConsumer.ConsumeLogsJSON(ctx, blobData.Bytes())
		} else if containerName == p.tracesContainerName {
			p.tracesDataConsumer.ConsumeTracesJSON(ctx, blobData.Bytes())
		} else {
			p.logger.Debug(fmt.Sprintf("Unknown container name %s", containerName))
		}
	}

	return nil
}

func (p *AzureBlobEventHandler) Close(ctx context.Context) error {

	if p.hub != nil {
		err := p.hub.Close(ctx)
		if err != nil {
			return err
		}
		p.hub = nil
	}
	return nil
}

func (p *AzureBlobEventHandler) SetLogsDataConsumer(logsDataConsumer LogsDataConsumer) {
	p.logsDataConsumer = logsDataConsumer
}

func (p *AzureBlobEventHandler) SetTracesDataConsumer(tracesDataConsumer TracesDataConsumer) {
	p.tracesDataConsumer = tracesDataConsumer
}

func NewBlobEventHandler(eventHubSonnectionString string, logsContainerName string, tracesContainerName string, blobClient BlobClient, logger *zap.Logger) *AzureBlobEventHandler {
	return &AzureBlobEventHandler{
		blobClient:               blobClient,
		logsContainerName:        logsContainerName,
		tracesContainerName:      tracesContainerName,
		eventHubSonnectionString: eventHubSonnectionString,
		logger:                   logger,
	}
}
