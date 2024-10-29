// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const clientService = "client"

func TestStoreUpsertEdge(t *testing.T) {
	key := NewKey(pcommon.TraceID([16]byte{1, 2, 3}), pcommon.SpanID([8]byte{1, 2, 3}))

	var onCompletedCount int
	var onExpireCount int

	s := NewStore(time.Hour, 1, countingCallback(&onCompletedCount), countingCallback(&onExpireCount))
	assert.Equal(t, 0, s.Len())

	// Insert first half of an edge
	isNew, err := s.UpsertEdge(key, func(e *Edge) {
		e.ClientService = clientService
	})
	require.NoError(t, err)
	require.True(t, isNew)
	assert.Equal(t, 1, s.Len())

	// Nothing should be evicted as TTL is set to 1h
	assert.False(t, s.tryEvictHead())
	assert.Equal(t, 0, onCompletedCount)
	assert.Equal(t, 0, onExpireCount)

	// Insert the second half of an edge
	isNew, err = s.UpsertEdge(key, func(e *Edge) {
		assert.Equal(t, clientService, e.ClientService)
		e.ServerService = "server"
	})
	require.NoError(t, err)
	require.False(t, isNew)
	// Edge is complete and should have been removed
	assert.Equal(t, 0, s.Len())

	assert.Equal(t, 1, onCompletedCount)
	assert.Equal(t, 0, onExpireCount)

	// Insert an edge that will immediately expire
	isNew, err = s.UpsertEdge(key, func(e *Edge) {
		e.ClientService = clientService
		e.expiration = time.UnixMicro(0)
	})
	require.NoError(t, err)
	require.True(t, isNew)
	assert.Equal(t, 1, s.Len())
	assert.Equal(t, 1, onCompletedCount)
	assert.Equal(t, 0, onExpireCount)

	assert.True(t, s.tryEvictHead())
	assert.Equal(t, 0, s.Len())
	assert.Equal(t, 1, onCompletedCount)
	assert.Equal(t, 1, onExpireCount)
}

func TestStoreUpsertEdge_errTooManyItems(t *testing.T) {
	key1 := NewKey(pcommon.TraceID([16]byte{1, 2, 3}), pcommon.SpanID([8]byte{1, 2, 3}))
	key2 := NewKey(pcommon.TraceID([16]byte{4, 5, 6}), pcommon.SpanID([8]byte{1, 2, 3}))
	var onCallbackCounter int

	s := NewStore(time.Hour, 1, countingCallback(&onCallbackCounter), countingCallback(&onCallbackCounter))
	assert.Equal(t, 0, s.Len())

	isNew, err := s.UpsertEdge(key1, func(e *Edge) {
		e.ClientService = clientService
	})
	require.NoError(t, err)
	require.True(t, isNew)
	assert.Equal(t, 1, s.Len())

	_, err = s.UpsertEdge(key2, func(e *Edge) {
		e.ClientService = clientService
	})
	require.ErrorIs(t, err, ErrTooManyItems)
	assert.Equal(t, 1, s.Len())

	isNew, err = s.UpsertEdge(key1, func(e *Edge) {
		e.ClientService = clientService
	})
	require.NoError(t, err)
	require.False(t, isNew)
	assert.Equal(t, 1, s.Len())

	assert.Equal(t, 0, onCallbackCounter)
}

func TestStoreExpire(t *testing.T) {
	const testSize = 100

	keys := map[Key]struct{}{}
	for i := 0; i < testSize; i++ {
		keys[NewKey(pcommon.TraceID([16]byte{byte(i)}), pcommon.SpanID([8]byte{1, 2, 3}))] = struct{}{}
	}

	var onCompletedCount int
	var onExpireCount int

	onComplete := func(e *Edge) {
		onCompletedCount++
		assert.Contains(t, keys, e.Key)
	}
	// New edges are immediately expired
	s := NewStore(-time.Second, testSize, onComplete, countingCallback(&onExpireCount))

	for key := range keys {
		isNew, err := s.UpsertEdge(key, noopCallback)
		require.NoError(t, err)
		require.True(t, isNew)
	}

	s.Expire()
	assert.Equal(t, 0, s.Len())
	assert.Equal(t, 0, onCompletedCount)
	assert.Equal(t, testSize, onExpireCount)
}

func TestStoreConcurrency(t *testing.T) {
	s := NewStore(10*time.Millisecond, 100000, noopCallback, noopCallback)

	end := make(chan struct{})

	accessor := func(f func()) {
		for {
			select {
			case <-end:
				return
			default:
				f()
			}
		}
	}

	go accessor(func() {
		key := NewKey(pcommon.TraceID([16]byte{byte(rand.Intn(32))}), pcommon.SpanID([8]byte{1, 2, 3}))

		_, err := s.UpsertEdge(key, func(e *Edge) {
			e.ClientService = hex.EncodeToString(key.tid[:])
		})
		assert.NoError(t, err)
	})

	go accessor(func() {
		s.Expire()
	})

	time.Sleep(100 * time.Millisecond)
	close(end)
}

func noopCallback(_ *Edge) {}

func countingCallback(counter *int) func(*Edge) {
	return func(_ *Edge) {
		*counter++
	}
}
