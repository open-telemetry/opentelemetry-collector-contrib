// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewBlobEventHandler(t *testing.T) {
	blobClient := newMockBlobClient()
	blobEventHandler := getBlobEventHandler(t, blobClient)

	require.NotNil(t, blobEventHandler)
	assert.Equal(t, blobClient, blobEventHandler.blobClient)
}

func TestBlobEventHandler_SetConsumers(t *testing.T) {
	blobClient := newMockBlobClient()
	handler := getBlobEventHandler(t, blobClient)

	logsConsumer := newMockLogsDataConsumer()
	tracesConsumer := newMockTracesDataConsumer()

	handler.setLogsDataConsumer(logsConsumer)
	handler.setTracesDataConsumer(tracesConsumer)

	assert.Equal(t, logsConsumer, handler.logsDataConsumer)
	assert.Equal(t, tracesConsumer, handler.tracesDataConsumer)
}

func TestBlobEventHandler_RunAndClose(t *testing.T) {
	blobClient := newMockBlobClient()
	handler := getBlobEventHandler(t, blobClient)

	ctx := t.Context()
	err := handler.run(ctx)
	require.NoError(t, err)
	require.NotNil(t, handler.cancelFunc)

	err = handler.close(ctx)
	require.NoError(t, err)
}

func TestBlobEventHandler_ProcessContainers(t *testing.T) {
	blobClient := &mockBlobClient{}
	blobClient.On("listBlobs", mock.Anything, logsContainerName).Return([]string{"log1.json", "log2.json"}, nil)
	blobClient.On("listBlobs", mock.Anything, tracesContainerName).Return([]string{"trace1.json"}, nil)
	blobClient.On("readBlob", mock.Anything, mock.Anything, mock.Anything).Return(bytes.NewBufferString("{}"), nil)

	handler := getBlobEventHandler(t, blobClient)

	logsConsumer := newMockLogsDataConsumer()
	tracesConsumer := newMockTracesDataConsumer()
	handler.setLogsDataConsumer(logsConsumer)
	handler.setTracesDataConsumer(tracesConsumer)

	handler.processContainers(t.Context())

	logsConsumer.AssertNumberOfCalls(t, "consumeLogsJSON", 2)
	tracesConsumer.AssertNumberOfCalls(t, "consumeTracesJSON", 1)
	blobClient.AssertNumberOfCalls(t, "readBlob", 3)
}

func TestBlobEventHandler_ProcessContainersWithNoConsumers(t *testing.T) {
	blobClient := newMockBlobClient()
	handler := getBlobEventHandler(t, blobClient)

	handler.processContainers(t.Context())

	blobClient.AssertNotCalled(t, "listBlobs", mock.Anything, mock.Anything)
}

func TestBlobEventHandler_DefaultPollInterval(t *testing.T) {
	blobClient := newMockBlobClient()
	handler := getBlobEventHandler(t, blobClient)

	assert.Equal(t, 10*time.Second, handler.pollInterval)
}

func getBlobEventHandler(tb testing.TB, blobClient blobClient) *blobEventHandler {
	blobEventHandler := newBlobEventHandler(
		logsContainerName,
		tracesContainerName,
		blobClient,
		zaptest.NewLogger(tb),
	)
	return blobEventHandler
}
