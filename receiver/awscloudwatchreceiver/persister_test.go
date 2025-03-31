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

// MockStorageClient is a mock implementation of the storage.Client interface
type mockStorageClient struct {
	cache    map[string][]byte
	cacheMux sync.Mutex
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{
		cache: make(map[string][]byte),
	}
}

func (m *mockStorageClient) Get(_ context.Context, key string) ([]byte, error) {
	m.cacheMux.Lock()
	defer m.cacheMux.Unlock()

	if value, ok := m.cache[key]; ok {
		return value, nil
	}
	return nil, errors.New("not found")
}

func (m *mockStorageClient) Set(_ context.Context, key string, value []byte) error {
	m.cacheMux.Lock()
	defer m.cacheMux.Unlock()

	m.cache[key] = value
	return nil
}

func (m *mockStorageClient) Delete(_ context.Context, key string) error {
	m.cacheMux.Lock()
	defer m.cacheMux.Unlock()

	delete(m.cache, key)
	return nil
}

func (m *mockStorageClient) Batch(_ context.Context, ops ...*storage.Operation) error {
	m.cacheMux.Lock()
	defer m.cacheMux.Unlock()

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

func TestSetCheckpoint(t *testing.T) {
	mockClient := newMockStorageClient()
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	logGroupName := "test-log-group"
	timestamp := time.Now()

	key := persister.getCheckpointKey(logGroupName)
	data := []byte(`"` + timestamp.Format(time.RFC3339) + `"`)

	err := persister.SetCheckpoint(ctx, logGroupName, timestamp.Format(time.RFC3339))
	assert.NoError(t, err)

	storedData, err := mockClient.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, data, storedData)
}

func TestGetCheckpoint(t *testing.T) {
	mockClient := newMockStorageClient()
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	ctx := context.Background()
	logGroupName := "test-log-group"
	timestamp := time.Now()

	key := persister.getCheckpointKey(logGroupName)
	data := []byte(`"` + timestamp.Format(time.RFC3339) + `"`)
	err := mockClient.Set(ctx, key, data)
	assert.NoError(t, err)

	result, err := persister.GetCheckpoint(ctx, logGroupName)
	assert.NoError(t, err)
	assert.Equal(t, timestamp.Format(time.RFC3339), result)
}

func TestGetCheckpoint_NotFound_Or_EmptyValue(t *testing.T) {
	mockClient := newMockStorageClient()
	logger := zap.NewNop()
	persister := newCloudwatchCheckpointPersister(mockClient, logger)

	result, err := persister.GetCheckpoint(context.Background(), "test-log-group")
	assert.NoError(t, err)
	assert.Equal(t, newCheckpointTimeFromStartOfStream(), result)
}
