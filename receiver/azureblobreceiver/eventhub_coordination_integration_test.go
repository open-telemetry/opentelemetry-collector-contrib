// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2/checkpoints"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

func TestEventHubCoordinationIntegration(t *testing.T) {
	connection := os.Getenv("AZUREBLOB_TEST_EVENT_HUB_CONNECTION_STRING")
	storage := os.Getenv("AZUREBLOB_TEST_STORAGE_CONNECTION_STRING")
	group := os.Getenv("AZUREBLOB_TEST_CONSUMER_GROUP")
	if connection == "" || storage == "" || group == "" {
		t.Skip("requires a dedicated Event Hub consumer group and Azure Blob test credentials; see README")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	producer, err := azeventhubs.NewProducerClientFromConnectionString(connection, "", nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.WithoutCancel(t.Context()), 30*time.Second)
		defer closeCancel()
		assert.NoError(t, producer.Close(closeCtx))
	})
	properties, err := producer.GetEventHubProperties(ctx, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(properties.PartitionIDs), 2)
	connectionProperties, err := azeventhubs.ParseConnectionString(connection)
	require.NoError(t, err)
	service, err := azblob.NewClientFromConnectionString(storage, nil)
	require.NoError(t, err)
	suffix := strings.ToLower(rand.Text())
	sourceContainer := "otel-source-" + suffix
	checkpointContainer := "otel-checkpoints-" + suffix
	for _, name := range []string{sourceContainer, checkpointContainer} {
		_, err = service.CreateContainer(ctx, name, nil)
		require.NoError(t, err)
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(t.Context()), 30*time.Second)
			defer cleanupCancel()
			_, deleteErr := service.DeleteContainer(cleanupCtx, name, nil)
			assert.NoError(t, deleteErr)
		})
	}
	store, err := checkpoints.NewBlobStore(service.ServiceClient().NewContainerClient(checkpointContainer), nil)
	require.NoError(t, err)
	sink := &consumertest.LogsSink{}
	start := func() func() {
		client, clientErr := newBlobClientFromConnectionString(storage, zaptest.NewLogger(t))
		require.NoError(t, clientErr)
		handler := newEventHubEventHandler(connection, sourceContainer, "", client, zaptest.NewLogger(t))
		handler.consumerGroup = group
		handler.checkpointStore = store
		r, createErr := newReceiver(receivertest.NewNopSettings(metadata.Type), handler, EncodingOTLPJSON, EncodingOTLPJSON, CompressionNone)
		require.NoError(t, createErr)
		receiver := r.(*blobReceiver)
		receiver.setNextLogsConsumer(sink)
		require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
		stop := sync.OnceFunc(func() {
			closeCtx, closeCancel := context.WithTimeout(context.WithoutCancel(t.Context()), 30*time.Second)
			defer closeCancel()
			assert.NoError(t, receiver.Shutdown(closeCtx))
		})
		t.Cleanup(stop)
		return stop
	}
	waitForOwners := func(count int) {
		require.Eventually(t, func() bool {
			owners, listErr := store.ListOwnership(ctx, connectionProperties.FullyQualifiedNamespace, properties.Name, group, nil)
			if listErr != nil {
				return false
			}
			counts := make(map[string]int)
			total := 0
			for _, owner := range owners {
				if owner.OwnerID != "" {
					counts[owner.OwnerID]++
					total++
				}
			}
			if len(counts) != count || total != len(properties.PartitionIDs) {
				return false
			}
			for _, n := range counts {
				if n < total/count || n > (total+count-1)/count {
					return false
				}
			}
			return true
		}, 2*time.Minute, time.Second, "partition ownership did not converge")
	}
	sendRound := func(round int) {
		positions := make(map[string]int64)
		for _, partition := range properties.PartitionIDs {
			name := fmt.Sprintf("round-%d-partition-%s", round, partition)
			logs := plog.NewLogs()
			logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(name)
			payload, marshalErr := (&plog.JSONMarshaler{}).MarshalLogs(logs)
			require.NoError(t, marshalErr)
			_, uploadErr := service.UploadBuffer(ctx, sourceContainer, name, payload, nil)
			require.NoError(t, uploadErr)
			body, marshalErr := json.Marshal([]map[string]string{{"eventType": blobCreatedEventType, "subject": "/blobServices/default/containers/" + sourceContainer + "/blobs/" + name}})
			require.NoError(t, marshalErr)
			batch, batchErr := producer.NewEventDataBatch(ctx, &azeventhubs.EventDataBatchOptions{PartitionID: &partition})
			require.NoError(t, batchErr)
			require.NoError(t, batch.AddEventData(&azeventhubs.EventData{Body: body}, nil))
			require.NoError(t, producer.SendEventDataBatch(ctx, batch, nil))
			partitionProperties, partitionErr := producer.GetPartitionProperties(ctx, partition, nil)
			require.NoError(t, partitionErr)
			positions[partition] = partitionProperties.LastEnqueuedSequenceNumber
		}
		require.Eventually(t, func() bool {
			entries, listErr := store.ListCheckpoints(ctx, connectionProperties.FullyQualifiedNamespace, properties.Name, group, nil)
			if listErr != nil || len(entries) != len(positions) {
				return false
			}
			for _, entry := range entries {
				if entry.SequenceNumber == nil || *entry.SequenceNumber < positions[entry.PartitionID] {
					return false
				}
			}
			return sink.LogRecordCount() == round*len(properties.PartitionIDs)
		}, time.Minute, time.Second, "records were not consumed and checkpointed")
	}

	stopA, stopB := start(), start()
	waitForOwners(2)
	sendRound(1)
	stopB()
	waitForOwners(1)
	sendRound(2)
	stopA()
	start()
	waitForOwners(1)
	sendRound(3)
}
