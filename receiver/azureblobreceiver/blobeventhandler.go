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

package azureblobreceiver

import (
	"context"
	"encoding/json"
	"strings"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.uber.org/zap"
)

type BlobEventHandler interface {
	Run(ctx context.Context) error
	Close(ctx context.Context) error
	SetLogsConsumer(logsConsumer LogsConsumer)
}

type azureBlobEventHandler struct {
	blobClient               BlobClient
	logsConsumer             LogsConsumer
	eventHubSonnectionString string
	hub                      *eventhub.Hub
	logger                   *zap.Logger
}

const (
	blobCreatedEventType = "Microsoft.Storage.BlobCreated"
)

func (p *azureBlobEventHandler) Run(ctx context.Context) error {
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

func (p *azureBlobEventHandler) newMessageHangdler(ctx context.Context, event *eventhub.Event) error {
	p.logger.Info("New message has arrived from Event Hub")
	p.logger.Info("===========message start=========")
	p.logger.Info(string(event.Data))
	p.logger.Info("===========message end=========")

	// var event map[string]interface{}
	// json.Unmarshal((event.Data, &event)
	// p.logger.Info(event.)

	var eventDataSlice []map[string]interface{}
	json.Unmarshal(event.Data, &eventDataSlice)

	subject := eventDataSlice[0]["subject"].(string)
	containerName := strings.SplitN(strings.SplitN(subject, "containers/", -1)[1], "/", -1)[0]
	eventType := eventDataSlice[0]["eventType"].(string)
	blobName := strings.SplitN(subject, "blobs/", -1)[1]

	if eventType == blobCreatedEventType {
		blobData, err := p.blobClient.ReadBlob(ctx, containerName, blobName)
		defer p.blobClient.DeleteBlob(ctx, containerName, blobName)
		if err != nil {
			return err
		}
		if containerName == "logs" {
			p.logsConsumer.ConsumeLogsJson(ctx, blobData.Bytes())
		}
	}

	return nil
}

func (p *azureBlobEventHandler) Close(ctx context.Context) error {
	err := p.hub.Close(ctx)
	return err
}

func (p *azureBlobEventHandler) SetLogsConsumer(logsConsumer LogsConsumer) {
	p.logsConsumer = logsConsumer
}

func NewBlobEventHandler(eventHubSonnectionString string, blobClient BlobClient, logger *zap.Logger) *azureBlobEventHandler {
	return &azureBlobEventHandler{
		blobClient:               blobClient,
		eventHubSonnectionString: eventHubSonnectionString,
		logger:                   logger,
	}
}
