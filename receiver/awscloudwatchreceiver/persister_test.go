// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

const logGroupName = "test-log-group"

// MockStorageClient is a mock implementation of the storage.Client interface
type mockStorageClient struct {
	cache      map[string][]byte
	cacheMux   sync.Mutex
	forceError bool
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{
		cache: make(map[string][]byte),
	}
}

func (m *mockStorageClient) Get(_ context.Context, key string) ([]byte, error) {
	m.cacheMux.Lock()
	defer m.cacheMux.Unlock()

	if m.forceError {
		return nil, errors.New("forced storage error")
	}

	if value, ok := m.cache[key]; ok {
		return value, nil
	}

	// Get will retrieve data from storage that corresponds to the
	// specified key. It should return (nil, nil) if not found
	// ref: Storage client interface
	return nil, nil
}

func (m *mockStorageClient) Set(_ context.Context, key string, value []byte) error {
	m.cacheMux.Lock()
	defer m.cacheMux.Unlock()

	if m.forceError {
		return errors.New("forced storage error")
	}

	m.cache[key] = value

	return nil
}

func (m *mockStorageClient) Delete(_ context.Context, key string) error {
	m.cacheMux.Lock()
	defer m.cacheMux.Unlock()

	if m.forceError {
		return errors.New("forced storage error")
	}

	delete(m.cache, key)

	return nil
}

func (m *mockStorageClient) Batch(_ context.Context, ops ...*storage.Operation) error {
	m.cacheMux.Lock()
	defer m.cacheMux.Unlock()

	if m.forceError {
		return errors.New("forced storage error")
	}

	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			op.Value = m.cache[op.Key]
		case storage.Set:
			m.cache[op.Key] = op.Value
		case storage.Delete:
			delete(m.cache, op.Key)
		default:
			return errors.New("wrong operation type")
		}
	}

	return nil
}

func (m *mockStorageClient) Close(_ context.Context) error {
	m.cacheMux.Lock()
	defer m.cacheMux.Unlock()
	m.cache = nil
	return nil
}

func TestGetCheckpoint(t *testing.T) {
	mockClient := newMockStorageClient()
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	timestamp := time.Now().Format(time.RFC3339)

	err := persister.SetCheckpoint(ctx, logGroupName, timestamp)
	assert.NoError(t, err)

	result, err := persister.GetCheckpoint(ctx, logGroupName)
	assert.NoError(t, err)
	assert.Equal(t, timestamp, result)
}

func TestSetCheckpoint_NilClient(t *testing.T) {
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(nil, logger)

	ctx := context.Background()
	timestamp := time.Now().Format(time.RFC3339)

	err := persister.SetCheckpoint(ctx, logGroupName, timestamp)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "storage client is nil")
}

func TestSetCheckpoint_EmptyValue(t *testing.T) {
	mockClient := newMockStorageClient()
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	err := persister.SetCheckpoint(ctx, logGroupName, "") // Set an empty value
	assert.Error(t, err)
	assert.ErrorContains(t, err, "timestamp is empty")

	result, err := persister.GetCheckpoint(ctx, logGroupName)
	assert.NoError(t, err)
	assert.Equal(t, newCheckpointTimeFromStartOfStream(), result)
}

func TestSetCheckpoint_EmptyLogGroupName(t *testing.T) {
	mockClient := newMockStorageClient()
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	timestamp := time.Now().Format(time.RFC3339)

	err := persister.SetCheckpoint(ctx, "", timestamp)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "checkpoint key is empty")
}

func TestSetCheckpoint_StorageError(t *testing.T) {
	mockClient := newMockStorageClient()
	mockClient.forceError = true
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	timestamp := time.Now().Format(time.RFC3339)

	err := persister.SetCheckpoint(ctx, logGroupName, timestamp)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "forced storage error")
}

func TestGetCheckpoint_NilClient(t *testing.T) {
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(nil, logger)

	ctx := context.Background()
	_, err := persister.GetCheckpoint(ctx, logGroupName)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "storage client is nil")
}

func TestGetCheckpoint_StorageError(t *testing.T) {
	mockClient := newMockStorageClient()
	mockClient.forceError = true
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	result, err := persister.GetCheckpoint(ctx, logGroupName)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "forced storage error")
	assert.Empty(t, result)
}

func TestGetCheckpoint_NotFound(t *testing.T) {
	mockClient := newMockStorageClient()
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	result, err := persister.GetCheckpoint(ctx, "non-existent-group")
	assert.NoError(t, err)
	assert.Equal(t, newCheckpointTimeFromStartOfStream(), result)
}

func TestDeleteCheckpoint(t *testing.T) {
	mockClient := newMockStorageClient()
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	timestamp := time.Now().Format(time.RFC3339)

	err := persister.SetCheckpoint(ctx, logGroupName, timestamp)
	assert.NoError(t, err)

	err = persister.DeleteCheckpoint(ctx, logGroupName)
	assert.NoError(t, err)

	// no error and new checkpoint indicates not found
	result, err := persister.GetCheckpoint(ctx, logGroupName)
	assert.NoError(t, err)
	assert.Equal(t, newCheckpointTimeFromStartOfStream(), result)
}

func TestDeleteCheckpoint_NilClient(t *testing.T) {
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(nil, logger)

	ctx := context.Background()
	err := persister.DeleteCheckpoint(ctx, logGroupName)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "storage client is nil")
}

func TestDeleteCheckpoint_StorageError(t *testing.T) {
	mockClient := newMockStorageClient()
	mockClient.forceError = true
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	err := persister.DeleteCheckpoint(ctx, logGroupName)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "forced storage error")
}
