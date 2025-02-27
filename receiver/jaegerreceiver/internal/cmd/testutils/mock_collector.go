// Copyright The OpenTelemetry Authors
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"net"
	"sync"
	"testing"

	"github.com/jaegertracing/jaeger-idl/model/v1"
	"github.com/stretchr/testify/require"
)

// MockCollector is a mock collector for tests that stores batches
type MockCollector struct {
	listener net.Listener
	batches  []*model.Batch
	mu       sync.Mutex
}

// StartMockCollector starts a mock collector on a random port
func StartMockCollector(t *testing.T) *MockCollector {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	collector := &MockCollector{
		listener: lis,
		batches:  make([]*model.Batch, 0),
	}

	return collector
}

// Listener returns server's listener
func (c *MockCollector) Listener() net.Listener {
	return c.listener
}

// Close closes the collector
func (c *MockCollector) Close() error {
	return c.listener.Close()
}

// StoreBatch allows direct storing of batches for testing
func (c *MockCollector) StoreBatch(batch *model.Batch) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.batches = append(c.batches, batch)
}

// GetJaegerBatches returns accumulated Jaeger batches
func (c *MockCollector) GetJaegerBatches() []*model.Batch {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.batches
}