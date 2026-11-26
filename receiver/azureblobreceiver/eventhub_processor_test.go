// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPartitionCheckpointsOnlySuccessfulEvents(t *testing.T) {
	failure := errors.New("operation failed")
	for _, failAt := range []string{"", "read-b", "consume-b", "delete-b", "checkpoint-2"} {
		t.Run(failAt, func(t *testing.T) {
			var operations []string
			op := func(name string) error {
				operations = append(operations, name)
				if name == failAt {
					return failure
				}
				return nil
			}
			blobs := &processorTestBlobs{
				read: func(_ context.Context, _, name string) (*bytes.Buffer, error) {
					return bytes.NewBufferString(name), op("read-" + name)
				},
				delete: func(_ context.Context, _, name string) error { return op("delete-" + name) },
			}
			handler := getEventHubEventHandler(t, blobs)
			handler.checkpointStore = &unusedCheckpointStore{}
			consumer := &mockLogsDataConsumer{}
			consumer.On("consumeLogs", mock.Anything, mock.Anything).Return(func(_ context.Context, data []byte) error {
				return op("consume-" + string(data))
			})
			handler.setLogsDataConsumer(consumer)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			checkpoint := int64(0)
			partition := &processorTestPartition{
				receive: func(context.Context) ([]*azeventhubs.ReceivedEventData, error) {
					calls++
					if calls > 1 {
						cancel()
						return nil, context.Canceled
					}
					return []*azeventhubs.ReceivedEventData{blobNotification(1, "a"), blobNotification(2, "b"), blobNotification(3, "c")}, context.DeadlineExceeded
				},
				checkpoint: func(_ context.Context, event *azeventhubs.ReceivedEventData) error {
					if err := op(fmt.Sprintf("checkpoint-%d", event.SequenceNumber)); err != nil {
						return err
					}
					checkpoint = event.SequenceNumber
					return nil
				},
			}
			handler.processPartition(ctx, partition)
			want := []string{"read-a", "consume-a", "delete-a", "checkpoint-1", "read-b", "consume-b", "delete-b", "checkpoint-2", "read-c", "consume-c", "delete-c", "checkpoint-3"}
			if failAt != "" {
				for i, step := range want {
					if step == failAt {
						want = want[:i+1]
						break
					}
				}
				assert.Equal(t, int64(1), checkpoint)
			} else {
				assert.Equal(t, int64(3), checkpoint)
			}
			assert.Equal(t, want, operations)
			assert.Equal(t, 1, partition.closes)
		})
	}
}

func TestPartitionStopsOnReceiveFailure(t *testing.T) {
	for _, err := range []error{
		errors.New("connection failed"),
		&azeventhubs.Error{Code: azeventhubs.ErrorCodeOwnershipLost},
	} {
		t.Run(err.Error(), func(t *testing.T) {
			handler := getEventHubEventHandler(t, &mockBlobClient{})
			partition := &processorTestPartition{receive: func(context.Context) ([]*azeventhubs.ReceivedEventData, error) {
				return []*azeventhubs.ReceivedEventData{blobNotification(1, "a")}, err
			}}
			handler.processPartition(t.Context(), partition)
			assert.Equal(t, 1, partition.closes)
		})
	}
}

func TestPartitionDoesNotCheckpointAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	blobs := newMockBlobClient()
	handler := getEventHubEventHandler(t, blobs)
	consumer := &mockLogsDataConsumer{}
	consumer.On("consumeLogs", mock.Anything, mock.Anything).Run(func(mock.Arguments) { cancel() }).Return(nil)
	handler.setLogsDataConsumer(consumer)
	partition := &processorTestPartition{receive: func(context.Context) ([]*azeventhubs.ReceivedEventData, error) {
		return []*azeventhubs.ReceivedEventData{getEventHubEvent(logEventData)}, nil
	}}
	handler.processPartition(ctx, partition)
	assert.Equal(t, 1, partition.closes)
}

func TestPartitionDoesNotCheckpointInvalidNotification(t *testing.T) {
	for _, body := range []string{"invalid JSON", "[]", `[{"eventType":"Microsoft.Storage.BlobCreated","subject":"invalid subject"}]`} {
		t.Run(body, func(t *testing.T) {
			handler := getEventHubEventHandler(t, &mockBlobClient{})
			partition := &processorTestPartition{receive: func(context.Context) ([]*azeventhubs.ReceivedEventData, error) {
				return []*azeventhubs.ReceivedEventData{getEventHubEvent([]byte(body)), blobNotification(2, "later")}, nil
			}}
			handler.processPartition(t.Context(), partition)
			assert.Equal(t, 1, partition.closes)
		})
	}
}

func TestPartitionCheckpointsWholeNotification(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var consumed []string
	blobs := &processorTestBlobs{
		read: func(_ context.Context, _, name string) (*bytes.Buffer, error) {
			return bytes.NewBufferString(name), nil
		},
		delete: func(context.Context, string, string) error { return nil },
	}
	handler := getEventHubEventHandler(t, blobs)
	consumer := &mockLogsDataConsumer{}
	consumer.On("consumeLogs", mock.Anything, mock.Anything).Return(func(_ context.Context, data []byte) error {
		consumed = append(consumed, string(data))
		return nil
	})
	handler.setLogsDataConsumer(consumer)
	event := getEventHubEvent([]byte(`[
		{"eventType":"Microsoft.Storage.BlobDeleted"},
		{"eventType":"Microsoft.Storage.BlobCreated","subject":"/blobServices/default/containers/logs/blobs/a"},
		{"eventType":"Microsoft.Storage.BlobCreated","subject":"/blobServices/default/containers/logs/blobs/b"}
	]`))
	checkpoints := 0
	partition := &processorTestPartition{
		receive: func(context.Context) ([]*azeventhubs.ReceivedEventData, error) {
			return []*azeventhubs.ReceivedEventData{event}, nil
		},
		checkpoint: func(_ context.Context, got *azeventhubs.ReceivedEventData) error {
			assert.Same(t, event, got)
			assert.Equal(t, []string{"a", "b"}, consumed)
			checkpoints++
			cancel()
			return nil
		},
	}
	handler.processPartition(ctx, partition)
	assert.Equal(t, 1, checkpoints)
	assert.Equal(t, 1, partition.closes)
}

func TestEventArrayReplayAfterPartialFailure(t *testing.T) {
	missing := &azcore.ResponseError{StatusCode: http.StatusNotFound, ErrorCode: string(bloberror.BlobNotFound)}
	remaining := map[string]bool{"a": true, "b": true, "c": true}
	var consumed []string
	blobs := &processorTestBlobs{
		read: func(_ context.Context, _, name string) (*bytes.Buffer, error) {
			if !remaining[name] {
				return nil, missing
			}
			return bytes.NewBufferString(name), nil
		},
		delete: func(_ context.Context, _, name string) error {
			delete(remaining, name)
			return nil
		},
	}
	handler := getEventHubEventHandler(t, blobs)
	handler.checkpointStore = &unusedCheckpointStore{}
	failure := errors.New("downstream unavailable")
	consumer := &mockLogsDataConsumer{}
	consumer.On("consumeLogs", mock.Anything, mock.Anything).Return(func(_ context.Context, data []byte) error {
		if string(data) == "b" && failure != nil {
			return failure
		}
		consumed = append(consumed, string(data))
		return nil
	})
	handler.setLogsDataConsumer(consumer)
	event := blobNotification(1, "a", "b", "c")
	require.ErrorIs(t, handler.newMessageHandler(t.Context(), event), failure)
	assert.Equal(t, []string{"a"}, consumed)
	assert.Equal(t, map[string]bool{"b": true, "c": true}, remaining)

	failure = nil
	require.ErrorIs(t, handler.newMessageHandler(t.Context(), event), missing)
	handler.ignoreMissingBlobs = true
	require.NoError(t, handler.newMessageHandler(t.Context(), event))
	assert.Equal(t, []string{"a", "b", "c"}, consumed)
	assert.Empty(t, remaining)
}

func TestMissingBlobPolicy(t *testing.T) {
	for _, code := range []bloberror.Code{bloberror.BlobNotFound, bloberror.ContainerNotFound, bloberror.AuthenticationFailed} {
		for _, ignore := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/ignore=%v", code, ignore), func(t *testing.T) {
				failure := &azcore.ResponseError{StatusCode: http.StatusNotFound, ErrorCode: string(code)}
				blobs := &processorTestBlobs{read: func(context.Context, string, string) (*bytes.Buffer, error) { return nil, failure }}
				handler := getEventHubEventHandler(t, blobs)
				handler.ignoreMissingBlobs = ignore
				handler.setLogsDataConsumer(newMockLogsDataConsumer())
				err := handler.newMessageHandler(t.Context(), blobNotification(1, "a"))
				if ignore && code == bloberror.BlobNotFound {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, failure)
				}
			})
		}
	}
}

func TestDeleteFailurePolicy(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		for _, ignore := range []bool{false, true} {
			for _, code := range []bloberror.Code{bloberror.BlobNotFound, bloberror.AuthorizationFailure} {
				t.Run(fmt.Sprintf("coordinated=%v/ignore=%v/%s", coordinated, ignore, code), func(t *testing.T) {
					failure := &azcore.ResponseError{ErrorCode: string(code)}
					blobs := &processorTestBlobs{
						read:   func(context.Context, string, string) (*bytes.Buffer, error) { return bytes.NewBufferString("a"), nil },
						delete: func(context.Context, string, string) error { return failure },
					}
					handler := getEventHubEventHandler(t, blobs)
					if coordinated {
						handler.checkpointStore = &unusedCheckpointStore{}
					}
					handler.ignoreMissingBlobs = ignore
					handler.setLogsDataConsumer(newMockLogsDataConsumer())
					err := handler.newMessageHandler(t.Context(), blobNotification(1, "a"))
					if !coordinated || (ignore && code == bloberror.BlobNotFound) {
						require.NoError(t, err)
					} else {
						require.ErrorIs(t, err, failure)
					}
				})
			}
		}
	}
}

func TestProcessorRunsAssignedPartitionsConcurrently(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan string, 2)
	clients := make(chan processorPartitionClient, 2)
	var partitions []*processorTestPartition
	for _, id := range []string{"0", "3"} {
		partition := &processorTestPartition{id: id, receive: func(ctx context.Context) ([]*azeventhubs.ReceivedEventData, error) {
			started <- id
			<-ctx.Done()
			return nil, ctx.Err()
		}}
		partitions = append(partitions, partition)
		clients <- partition
	}
	processor := processorTestProcessor{clients: clients}
	handler := getEventHubEventHandler(t, &mockBlobClient{})
	done := make(chan error, 1)
	go func() { done <- handler.runProcessor(ctx, processor) }()
	var assigned []string
	for range partitions {
		select {
		case id := <-started:
			assigned = append(assigned, id)
		case <-time.After(5 * time.Second):
			t.Fatal("assigned partitions did not start concurrently")
		}
	}
	assert.ElementsMatch(t, []string{"0", "3"}, assigned)
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("processor did not stop")
	}
	for _, partition := range partitions {
		assert.Equal(t, 1, partition.closes)
	}
}

func TestProcessorFailureCancelsWorkers(t *testing.T) {
	failure := errors.New("checkpoint store unavailable")
	processor := processorTestProcessor{run: func(context.Context) error { return failure }}
	handler := getEventHubEventHandler(t, &mockBlobClient{})
	require.ErrorIs(t, handler.runProcessor(t.Context(), processor), failure)
}

func blobNotification(sequence int64, names ...string) *azeventhubs.ReceivedEventData {
	var body bytes.Buffer
	body.WriteByte('[')
	for i, name := range names {
		if i > 0 {
			body.WriteByte(',')
		}
		fmt.Fprintf(&body, `{"subject":"/blobServices/default/containers/logs/blobs/%s","eventType":"Microsoft.Storage.BlobCreated"}`, name)
	}
	body.WriteByte(']')
	event := getEventHubEvent(body.Bytes())
	event.SequenceNumber = sequence
	return event
}

type processorTestBlobs struct {
	read   func(context.Context, string, string) (*bytes.Buffer, error)
	delete func(context.Context, string, string) error
}

func (b *processorTestBlobs) readBlob(ctx context.Context, container, name string) (*bytes.Buffer, error) {
	return b.read(ctx, container, name)
}

func (b *processorTestBlobs) deleteBlob(ctx context.Context, container, name string) error {
	return b.delete(ctx, container, name)
}

func (*processorTestBlobs) listBlobs(context.Context, string) ([]string, error) {
	panic("unexpected polling")
}

type processorTestPartition struct {
	id         string
	receive    func(context.Context) ([]*azeventhubs.ReceivedEventData, error)
	checkpoint func(context.Context, *azeventhubs.ReceivedEventData) error
	closes     int
}

func (p *processorTestPartition) PartitionID() string { return p.id }

func (p *processorTestPartition) ReceiveEvents(ctx context.Context, _ int, _ *azeventhubs.ReceiveEventsOptions) ([]*azeventhubs.ReceivedEventData, error) {
	return p.receive(ctx)
}

func (p *processorTestPartition) UpdateCheckpoint(ctx context.Context, event *azeventhubs.ReceivedEventData, _ *azeventhubs.UpdateCheckpointOptions) error {
	return p.checkpoint(ctx, event)
}

func (p *processorTestPartition) Close(context.Context) error {
	p.closes++
	return nil
}

type processorTestProcessor struct {
	clients <-chan processorPartitionClient
	run     func(context.Context) error
}

func (p processorTestProcessor) Run(ctx context.Context) error {
	if p.run != nil {
		return p.run(ctx)
	}
	<-ctx.Done()
	return nil
}

func (p processorTestProcessor) NextPartitionClient(ctx context.Context) processorPartitionClient {
	select {
	case client := <-p.clients:
		return client
	case <-ctx.Done():
		return nil
	}
}

// These tests invoke the partition worker directly; checkpoint I/O belongs to its client.
type unusedCheckpointStore struct {
	azeventhubs.CheckpointStore
}
