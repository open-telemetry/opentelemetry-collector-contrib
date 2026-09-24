// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"bytes"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	logsDataConsumer.AssertNumberOfCalls(t, "consumeLogs", 1)
	tracesDataConsumer.AssertNumberOfCalls(t, "consumeTraces", 1)
	blobClient.AssertNumberOfCalls(t, "readBlob", 2)
	blobClient.AssertNumberOfCalls(t, "deleteBlob", 2)
}

func TestEventHubEventHandler_ProcessBlobCreated_DoesNotDeleteOnReadError(t *testing.T) {
	blobClient := &mockBlobClient{}
	blobClient.On("readBlob", mock.Anything, mock.Anything, mock.Anything).Return((*bytes.Buffer)(nil), errors.New("read failed"))

	handler := getEventHubEventHandler(t, blobClient)
	handler.setLogsDataConsumer(newMockLogsDataConsumer())
	handler.setTracesDataConsumer(newMockTracesDataConsumer())

	err := handler.processBlobCreatedEventType(t.Context(), logsContainerName, "logs-1")
	require.Error(t, err)

	blobClient.AssertCalled(t, "readBlob", mock.Anything, logsContainerName, "logs-1")
	blobClient.AssertNotCalled(t, "deleteBlob", mock.Anything, mock.Anything, mock.Anything)
}

func TestEventHubEventHandler_ProcessBlobCreated_DoesNotDeleteOnConsumeError(t *testing.T) {
	blobClient := newMockBlobClient()

	logsConsumer := &mockLogsDataConsumer{}
	logsConsumer.On("consumeLogs", mock.Anything, mock.Anything).Return(errors.New("consume failed"))

	handler := getEventHubEventHandler(t, blobClient)
	handler.setLogsDataConsumer(logsConsumer)
	handler.setTracesDataConsumer(newMockTracesDataConsumer())

	err := handler.processBlobCreatedEventType(t.Context(), logsContainerName, "logs-1")
	require.Error(t, err)

	blobClient.AssertCalled(t, "readBlob", mock.Anything, logsContainerName, "logs-1")
	blobClient.AssertNotCalled(t, "deleteBlob", mock.Anything, mock.Anything, mock.Anything)
}

func TestEventHubEventHandler_NewMessageHandler_PropagatesConsumeError(t *testing.T) {
	blobClient := newMockBlobClient()

	logsConsumer := &mockLogsDataConsumer{}
	logsConsumer.On("consumeLogs", mock.Anything, mock.Anything).Return(errors.New("consume failed"))

	handler := getEventHubEventHandler(t, blobClient)
	handler.setLogsDataConsumer(logsConsumer)
	handler.setTracesDataConsumer(newMockTracesDataConsumer())

	err := handler.newMessageHandler(t.Context(), getEventHubEvent(logEventData))
	require.Error(t, err)

	blobClient.AssertNotCalled(t, "deleteBlob", mock.Anything, mock.Anything, mock.Anything)
}

func TestEventHubEventHandler_ProcessBlobCreated_DeletesAfterSuccessfulConsume(t *testing.T) {
	blobClient := newMockBlobClient()
	handler := getEventHubEventHandler(t, blobClient)
	handler.setLogsDataConsumer(newMockLogsDataConsumer())
	handler.setTracesDataConsumer(newMockTracesDataConsumer())

	err := handler.processBlobCreatedEventType(t.Context(), logsContainerName, "logs-1")
	require.NoError(t, err)

	blobClient.AssertCalled(t, "readBlob", mock.Anything, logsContainerName, "logs-1")
	blobClient.AssertCalled(t, "deleteBlob", mock.Anything, logsContainerName, "logs-1")
}

func TestEventHubEventHandler_NewMessageHandler_MalformedMessageDoesNotPanic(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "empty event array",
			body:    `[]`,
			wantErr: true,
		},
		{
			name:    "blob created with subject missing containers",
			body:    `[{"subject":"/some/other/resource","eventType":"Microsoft.Storage.BlobCreated"}]`,
			wantErr: true,
		},
		{
			name:    "blob created with subject missing blobs",
			body:    `[{"subject":"/blobServices/default/containers/logs","eventType":"Microsoft.Storage.BlobCreated"}]`,
			wantErr: true,
		},
		{
			name:    "other event type with unparseable subject is ignored",
			body:    `[{"subject":"/some/other/resource","eventType":"Microsoft.Storage.BlobDeleted"}]`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blobClient := newMockBlobClient()
			handler := getEventHubEventHandler(t, blobClient)

			var err error
			require.NotPanics(t, func() {
				err = handler.newMessageHandler(t.Context(), getEventHubEvent([]byte(tt.body)))
			})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			blobClient.AssertNotCalled(t, "readBlob", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestParseBlobSubject(t *testing.T) {
	tests := []struct {
		subject       string
		wantContainer string
		wantBlob      string
		wantErr       bool
	}{
		{subject: "/blobServices/default/containers/logs/blobs/logs-1", wantContainer: "logs", wantBlob: "logs-1"},
		{subject: "/blobServices/default/containers/logs/blobs/dir/sub/file.json", wantContainer: "logs", wantBlob: "dir/sub/file.json"},
		{subject: "/blobServices/default/containers/containers/blobs/x", wantContainer: "containers", wantBlob: "x"},
		{subject: "/blobServices/default/containers/blobs/blobs/x", wantContainer: "blobs", wantBlob: "x"},
		{subject: "/blobServices/default/containers/logs/blobs/a/blobs/b.json", wantContainer: "logs", wantBlob: "a/blobs/b.json"},
		{subject: "/some/other/resource", wantErr: true},
		{subject: "/blobServices/default/containers/logs", wantErr: true},
		{subject: "/blobServices/default/containers/logs/blobs/", wantErr: true},
		{subject: "/blobServices/default/containers//blobs/x", wantErr: true},
		{subject: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			container, blob, err := parseBlobSubject(tt.subject)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantContainer, container)
			assert.Equal(t, tt.wantBlob, blob)
		})
	}
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
