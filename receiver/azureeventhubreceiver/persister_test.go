// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-event-hubs-go/v3/persist"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

func TestStorageOffsetPersisterUnknownCheckpoint(t *testing.T) {
	client := newMockClient()
	s := storageCheckpointPersister{storageClient: client}
	// check we have no match
	checkpoint, err := s.Read("foo", "bar", "foobar", "foobarfoo")
	assert.NoError(t, err)
	assert.NotNil(t, checkpoint)
	assert.Equal(t, "-1", checkpoint.Offset)
}

func TestStorageOffsetPersisterWithKnownCheckpoint(t *testing.T) {
	client := newMockClient()
	s := storageCheckpointPersister{storageClient: client}
	checkpoint := persist.Checkpoint{
		Offset:         "foo",
		SequenceNumber: 2,
		EnqueueTime:    time.Now(),
	}
	err := s.Write("foo", "bar", "foobar", "foobarfoo", checkpoint)
	assert.NoError(t, err)
	read, err := s.Read("foo", "bar", "foobar", "foobarfoo")
	assert.NoError(t, err)
	assert.Equal(t, checkpoint.Offset, read.Offset)
	assert.Equal(t, checkpoint.SequenceNumber, read.SequenceNumber)
	assert.True(t, checkpoint.EnqueueTime.Equal(read.EnqueueTime))
}

// copied from pkg/stanza/adapter/mocks_test.go
type mockClient struct {
	cache    map[string][]byte
	cacheMux sync.Mutex
}

func newMockClient() *mockClient {
	return &mockClient{
		cache: make(map[string][]byte),
	}
}

func (p *mockClient) Get(_ context.Context, key string) ([]byte, error) {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	return p.cache[key], nil
}

func (p *mockClient) Set(_ context.Context, key string, value []byte) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	p.cache[key] = value
	return nil
}

func (p *mockClient) Delete(_ context.Context, key string) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	delete(p.cache, key)
	return nil
}

func (p *mockClient) Batch(_ context.Context, ops ...storage.Operation) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()

	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			op.Value = p.cache[op.Key]
		case storage.Set:
			p.cache[op.Key] = op.Value
		case storage.Delete:
			delete(p.cache, op.Key)
		default:
			return errors.New("wrong operation type")
		}
	}

	return nil
}

func (p *mockClient) Close(_ context.Context) error {
	p.cacheMux.Lock()
	defer p.cacheMux.Unlock()
	p.cache = nil
	return nil
}
