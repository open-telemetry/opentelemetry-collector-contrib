// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/collector/extension/xextension/storage"
)

// memClient is a minimal in-memory storage.Client for testing.
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

func (c *memClient) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data)
}

// intEncode is a trivial encode for int values in tests.
func intEncode(v int) ([]byte, error) {
	return []byte(strconv.Itoa(v)), nil
}

// intDecode is a trivial decode for int values in tests.
func intDecode(data []byte) (int, error) {
	return strconv.Atoi(string(data))
}

func intHashKey(k int) string {
	return strconv.Itoa(k)
}

// countClient wraps memClient and counts Set operations via Batch.
type countClient struct {
	*memClient
	batchSets atomic.Int64
}

func (c *countClient) Batch(ctx context.Context, ops ...*storage.Operation) error {
	for _, op := range ops {
		if op.Type == storage.Set {
			c.batchSets.Add(1)
		}
	}
	return c.memClient.Batch(ctx, ops...)
}

// failGetClient wraps memClient and makes all Get calls return an error.
type failGetClient struct {
	*memClient
}

func (*failGetClient) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("simulated storage failure")
}
