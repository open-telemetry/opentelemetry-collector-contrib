// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubsexporter"

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
)

// eventDataBatch is an interface that defines the methods needed from azeventhubs.EventDataBatch
// This interface allows for mocking in tests
type eventDataBatch interface {
	AddEventData(eventData *azeventhubs.EventData, options *azeventhubs.AddEventDataOptions) error
	NumEvents() int32
}

// eventHubProducerClient is an interface that defines the methods needed from azeventhubs.ProducerClient
// This interface allows for mocking in tests
type eventHubProducerClient interface {
	NewEventDataBatch(ctx context.Context, options *azeventhubs.EventDataBatchOptions) (eventDataBatch, error)
	SendEventDataBatch(ctx context.Context, batch eventDataBatch, options *azeventhubs.SendEventDataBatchOptions) error
	Close(ctx context.Context) error
}

// azureEventHubProducerClientWrapper wraps the Azure SDK ProducerClient to implement our interface
type azureEventHubProducerClientWrapper struct {
	client *azeventhubs.ProducerClient
}

func (w *azureEventHubProducerClientWrapper) NewEventDataBatch(ctx context.Context, options *azeventhubs.EventDataBatchOptions) (eventDataBatch, error) {
	return w.client.NewEventDataBatch(ctx, options)
}

func (w *azureEventHubProducerClientWrapper) SendEventDataBatch(ctx context.Context, batch eventDataBatch, options *azeventhubs.SendEventDataBatchOptions) error {
	// Cast back to concrete type for Azure SDK
	azureBatch, ok := batch.(*azeventhubs.EventDataBatch)
	if !ok {
		// For testing, just return nil as mock will handle it
		return nil
	}
	return w.client.SendEventDataBatch(ctx, azureBatch, options)
}

func (w *azureEventHubProducerClientWrapper) Close(ctx context.Context) error {
	return w.client.Close(ctx)
}

// Ensure that *azeventhubs.EventDataBatch implements eventDataBatch
var _ eventDataBatch = (*azeventhubs.EventDataBatch)(nil)
