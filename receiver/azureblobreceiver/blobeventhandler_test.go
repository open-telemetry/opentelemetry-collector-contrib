// Copyright The OpenTelemetry Authors
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
	"testing"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
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

func TestNewBlobEventHandler(t *testing.T) {
	blobClient := newMockBlobClient()
	blobEventHandler := getBlobEventHandler(t, blobClient)

	require.NotNil(t, blobEventHandler)
	assert.Equal(t, blobClient, blobEventHandler.blobClient)
}

func TestNewMessageHangdler(t *testing.T) {
	blobClient := newMockBlobClient()
	blobEventHandler := getBlobEventHandler(t, blobClient)

	logsDataConsumer := newMockLogsDataConsumer()
	tracesDataConsumer := newMockTracesDataConsumer()
	blobEventHandler.setLogsDataConsumer(logsDataConsumer)
	blobEventHandler.setTracesDataConsumer(tracesDataConsumer)

	logEvent := getEvent(logEventData)
	err := blobEventHandler.newMessageHandler(context.Background(), logEvent)
	require.NoError(t, err)

	traceEvent := getEvent(traceEventData)
	err = blobEventHandler.newMessageHandler(context.Background(), traceEvent)
	require.NoError(t, err)

	logsDataConsumer.AssertNumberOfCalls(t, "consumeLogsJSON", 1)
	tracesDataConsumer.AssertNumberOfCalls(t, "consumeTracesJSON", 1)
	blobClient.AssertNumberOfCalls(t, "readBlob", 2)

}

func getEvent(eventData []byte) *eventhub.Event {
	return &eventhub.Event{Data: eventData}
}

func getBlobEventHandler(tb testing.TB, blobClient blobClient) *azureBlobEventHandler {
	blobEventHandler := newBlobEventHandler(eventHubString, logsContainerName, tracesContainerName, blobClient, zaptest.NewLogger(tb))
	return blobEventHandler
}
