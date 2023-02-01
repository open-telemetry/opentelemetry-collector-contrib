// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"fmt"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

type client struct {
	settings receiver.CreateSettings
	consumer consumer.Logs
	config   *Config
	obsrecv  *obsreport.Receiver
	hub      hubWrapper
	convert  eventConverter
}

type hubWrapper interface {
	GetRuntimeInformation(ctx context.Context) (*eventhub.HubRuntimeInformation, error)
	Receive(ctx context.Context, partitionID string, handler eventhub.Handler, opts ...eventhub.ReceiveOption) (listerHandleWrapper, error)
	Close(ctx context.Context) error
}

type eventConverter interface {
	ToLogs(event *eventhub.Event) (plog.Logs, error)
}

type listerHandleWrapper interface {
	Done() <-chan struct{}
	Err() error
}

type hubWrapperImpl struct {
	hub *eventhub.Hub
}

func (h *hubWrapperImpl) GetRuntimeInformation(ctx context.Context) (*eventhub.HubRuntimeInformation, error) {
	return h.hub.GetRuntimeInformation(ctx)
}

func (h *hubWrapperImpl) Receive(ctx context.Context, partitionID string, handler eventhub.Handler, opts ...eventhub.ReceiveOption) (listerHandleWrapper, error) {
	l, err := h.hub.Receive(ctx, partitionID, handler, opts...)
	return l, err
}

func (h *hubWrapperImpl) Close(ctx context.Context) error {
	return h.hub.Close(ctx)
}

func (c *client) Start(ctx context.Context, host component.Host) error {
	storageClient, err := adapter.GetStorageClient(ctx, host, c.config.StorageID, c.settings.ID)
	if err != nil {
		return err
	}
	if c.hub == nil { // set manually for testing.
		hub, newHubErr := eventhub.NewHubFromConnectionString(c.config.Connection, eventhub.HubWithOffsetPersistence(&storageCheckpointPersister{storageClient: storageClient}))
		if newHubErr != nil {
			return newHubErr
		}
		c.hub = &hubWrapperImpl{
			hub: hub,
		}
	}

	if c.config.Partition == "" {
		// listen to each partition of the Event Hub
		var runtimeInfo *eventhub.HubRuntimeInformation
		runtimeInfo, err = c.hub.GetRuntimeInformation(ctx)
		if err != nil {
			return err
		}

		for _, partitionID := range runtimeInfo.PartitionIDs {
			err = c.setUpOnePartition(ctx, partitionID, false)
			if err != nil {
				return err
			}
		}
	} else {
		err = c.setUpOnePartition(ctx, c.config.Partition, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *client) setUpOnePartition(ctx context.Context, partitionID string, applyOffset bool) error {
	offsetOption := eventhub.ReceiveWithLatestOffset()
	if applyOffset && c.config.Offset != "" {
		offsetOption = eventhub.ReceiveWithStartingOffset(c.config.Offset)
	}

	handle, err := c.hub.Receive(ctx, partitionID, c.handle, offsetOption)
	if err != nil {
		return err
	}
	go func() {
		<-handle.Done()
		err := handle.Err()
		if err != nil {
			c.settings.Logger.Error("Error reported by event hub", zap.Error(err))
		}
	}()

	return nil
}

func (c *client) handle(ctx context.Context, event *eventhub.Event) error {
	logs, err := c.convert.ToLogs(event)
	if err != nil {
		return fmt.Errorf("failed to convert logs: %w", err)
	}
	c.obsrecv.StartLogsOp(ctx)
	consumerErr := c.consumer.ConsumeLogs(ctx, logs)
	c.obsrecv.EndLogsOp(ctx, "azureeventhub", logs.LogRecordCount(), consumerErr)
	return consumerErr
}

func (c *client) Shutdown(ctx context.Context) error {
	if c.hub == nil {
		return nil
	}
	return c.hub.Close(ctx)
}
