// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/xextension/storage"
)

type memClient struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemClient() *memClient {
	return &memClient{data: make(map[string][]byte)}
}

func (c *memClient) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[key], nil
}

func (c *memClient) Set(_ context.Context, key string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	return nil
}

func (c *memClient) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *memClient) Batch(_ context.Context, ops ...*storage.Operation) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			op.Value = c.data[op.Key]
		case storage.Set:
			c.data[op.Key] = op.Value
		case storage.Delete:
			delete(c.data, op.Key)
		}
	}
	return nil
}

func (*memClient) Close(_ context.Context) error { return nil }

func TestPersistStaleIndex_RoundTrip(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	err := persistStaleIndex(ctx, client, []string{"aaa", "bbb"}, []string{"ccc"}, []string{"ddd", "eee"})
	require.NoError(t, err)

	loaded, err := loadStaleIndex(ctx, client)
	require.NoError(t, err)
	require.Len(t, loaded, 5)
	for _, k := range []string{"aaa", "bbb", "ccc", "ddd", "eee"} {
		_, ok := loaded[k]
		assert.True(t, ok, "expected key %q in loaded index", k)
	}
}

func TestLoadStaleIndex_Empty(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	loaded, err := loadStaleIndex(ctx, client)
	require.NoError(t, err)
	assert.Empty(t, loaded)
}

func TestPersistStaleIndex_EmptyKeys(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	err := persistStaleIndex(ctx, client, nil, nil, nil)
	require.NoError(t, err)

	loaded, err := loadStaleIndex(ctx, client)
	require.NoError(t, err)
	assert.Empty(t, loaded)
}
