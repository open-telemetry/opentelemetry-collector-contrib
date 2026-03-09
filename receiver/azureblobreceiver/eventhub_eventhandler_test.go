// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	eventHubString = "Endpoint=sb://oteldata.servicebus.windows.net/;SharedAccessKeyName=oteldatahubpolicy;SharedAccessKey=sharedAccessKey;EntityPath=otelddatahub"
)

var (
	logEventData   = []byte(`[{"topic":"someTopic","subject":"/blobServices/default/containers/logs/blobs/logs-1","eventType":"Microsoft.Storage.BlobCreated","id":"1","data":{"api":"PutBlob","clientRequestId":"1","requestId":"1","eTag":"1","contentType":"text","contentLength":10,"blobType":"BlockBlob","url":"https://oteldata.blob.core.windows.net/logs/logs-1","sequencer":"1","storageDiagnostics":{"batchId":"1"}},"dataVersion":"","metadataVersion":"1","eventTime":"2022-03-25T15:59:50.9251748Z"}]`)
	traceEventData = []byte(`[{"topic":"someTopic","subject":"/blobServices/default/containers/traces/blobs/traces-1","eventType":"Microsoft.Storage.BlobCreated","id":"1","data":{"api":"PutBlob","clientRequestId":"1","requestId":"1","eTag":"1","contentType":"text","contentLength":10,"blobType":"BlockBlob","url":"https://oteldata.blob.core.windows.net/traces/traces-1","sequencer":"1","storageDiagnostics":{"batchId":"1"}},"dataVersion":"","metadataVersion":"1","eventTime":"2022-03-25T15:59:50.9251748Z"}]`)
)

func TestNewEventHubEventHandler(t *testing.T) {
	blobClient := newMockBlobClient()
	blobEventHandler := getEventHubEventHandler(t, blobClient)

	require.NotNil(t, blobEventHandler)
	assert.Equal(t, blobClient, blobEventHandler.blobClient)
}

func TestNewEventHubMessageHandler(t *testing.T) {
	blobClient := newMockBlobClient()
	blobEventHandler := getEventHubEventHandler(t, blobClient)

	logsDataConsumer := newMockLogsDataConsumer()
	tracesDataConsumer := newMockTracesDataConsumer()
	blobEventHandler.setLogsDataConsumer(logsDataConsumer)
	blobEventHandler.setTracesDataConsumer(tracesDataConsumer)

	logEvent := getEventHubEvent(logEventData)
	err := blobEventHandler.newMessageHandler(t.Context(), logEvent)
	require.NoError(t, err)

	traceEvent := getEventHubEvent(traceEventData)
	err = blobEventHandler.newMessageHandler(t.Context(), traceEvent)
	require.NoError(t, err)

	logsDataConsumer.AssertNumberOfCalls(t, "consumeLogsJSON", 1)
	tracesDataConsumer.AssertNumberOfCalls(t, "consumeTracesJSON", 1)
	blobClient.AssertNumberOfCalls(t, "readBlob", 2)
}

func getEventHubEvent(eventData []byte) *azeventhubs.ReceivedEventData {
	return &azeventhubs.ReceivedEventData{
		EventData: azeventhubs.EventData{
			Body: eventData,
		},
	}
}

func getEventHubEventHandler(tb testing.TB, blobClient blobClient) *eventHubEventHandler {
	blobEventHandler := newEventHubEventHandler(
		eventHubString,
		logsContainerName,
		tracesContainerName,
		blobClient,
		zaptest.NewLogger(tb),
	)
	return blobEventHandler
}
