// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"
)

func TestPersisted_LoadOrStore_CacheHit(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	// Store a new value
	v, loaded := m.LoadOrStore(ctx, 1, 42)
	assert.False(t, loaded)
	assert.Equal(t, 42, v)

	// Hit the cache
	v, loaded = m.LoadOrStore(ctx, 1, 99)
	assert.True(t, loaded)
	assert.Equal(t, 42, v) // original value, not 99
}

func TestPersisted_LoadOrStore_ReadThrough(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	// Pre-populate storage to simulate a previous session
	_ = client.Set(ctx, "test/"+intHashKey(1), []byte("42"))

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	// Should read-through from storage
	v, loaded := m.LoadOrStore(ctx, 1, 0)
	assert.False(t, loaded) // stored recovered value in cache (not "loaded" from cache)
	assert.Equal(t, 42, v)  // recovered from storage, not the default 0
}

func TestPersisted_LoadOrStore_StorageMiss(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	// Nothing in cache or storage — should store the default
	v, loaded := m.LoadOrStore(ctx, 1, 42)
	assert.False(t, loaded)
	assert.Equal(t, 42, v)
}

func TestPersisted_LoadAndDelete(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	// Store, then flush to storage
	m.LoadOrStore(ctx, 1, 42)
	require.NoError(t, m.Flush(ctx).Err)
	require.Equal(t, 1, client.Len())

	// Delete
	v, loaded := m.LoadAndDelete(ctx, 1)
	assert.True(t, loaded)
	assert.Equal(t, 42, v)

	// Storage should also be cleaned up
	assert.Equal(t, 0, client.Len())

	// Should not be in cache anymore
	_, loaded = m.LoadAndDelete(ctx, 1)
	assert.False(t, loaded)
}

func TestPersisted_Flush(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	// Store several entries
	for i := range 5 {
		m.LoadOrStore(ctx, i, i*10)
	}

	// Storage should be empty before flush
	assert.Equal(t, 0, client.Len())

	// Flush
	result := m.Flush(ctx)
	require.NoError(t, result.Err)

	// All 5 entries should be in storage
	assert.Equal(t, 5, client.Len())

	// Flush should also return all keys
	assert.Len(t, result.Keys, 5)

	// Verify values
	for i := range 5 {
		data, err := client.Get(ctx, "test/"+intHashKey(i))
		require.NoError(t, err)
		v, err := intDecode(data)
		require.NoError(t, err)
		assert.Equal(t, i*10, v)
	}
}

func TestPersisted_Exceeded(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(2)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	// Fill to capacity
	v1, loaded := m.LoadOrStore(ctx, 1, 10)
	assert.False(t, loaded)
	assert.Equal(t, 10, v1)

	v2, loaded := m.LoadOrStore(ctx, 2, 20)
	assert.False(t, loaded)
	assert.Equal(t, 20, v2)

	// Third entry should be rejected
	v3, loaded := m.LoadOrStore(ctx, 3, 30)
	assert.True(t, maps.Exceeded(v3, loaded))
}

func TestPersisted_Concurrent(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	var (
		stores = new(atomic.Int64)
		loads  = new(atomic.Int64)
	)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for i := range 100 {
				_, loaded := m.LoadOrStore(ctx, i, i*10)
				if loaded {
					loads.Add(1)
				} else {
					stores.Add(1)
				}
			}
		})
	}
	wg.Wait()

	assert.Equal(t, int64(100), stores.Load())
	assert.Equal(t, int64(900), loads.Load())
}

func TestPersisted_FlushThenRecover(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	// Session 1: store values and flush
	limit1 := maps.Limit(100)
	m1 := maps.NewPersisted(client, "test", limit1, intHashKey, intEncode, intDecode)
	m1.LoadOrStore(ctx, 1, 100)
	m1.LoadOrStore(ctx, 2, 200)
	require.NoError(t, m1.Flush(ctx).Err)

	// Session 2: new map against the same storage client (simulates restart)
	limit2 := maps.Limit(100)
	m2 := maps.NewPersisted(client, "test", limit2, intHashKey, intEncode, intDecode)

	// Should recover values via read-through
	v, loaded := m2.LoadOrStore(ctx, 1, 0)
	assert.False(t, loaded)
	assert.Equal(t, 100, v)

	v, loaded = m2.LoadOrStore(ctx, 2, 0)
	assert.False(t, loaded)
	assert.Equal(t, 200, v)

	// Key 3 was never stored — should use default
	v, loaded = m2.LoadOrStore(ctx, 3, 0)
	assert.False(t, loaded)
	assert.Equal(t, 0, v)
}

func TestPersisted_DeleteByHashKey(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	// Store and flush
	m.LoadOrStore(ctx, 1, 42)
	require.NoError(t, m.Flush(ctx).Err)
	require.Equal(t, 1, client.Len())

	// Delete by hash key (simulates orphan cleanup)
	require.NoError(t, m.DeleteByHashKey(ctx, intHashKey(1)))

	// Storage entry should be gone
	data, err := client.Get(ctx, "test/"+intHashKey(1))
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestPersisted_DirtyTracking(t *testing.T) {
	ctx := t.Context()
	client := &countClient{memClient: newMemClient()}
	limit := maps.Limit(100)

	m := maps.NewPersisted[int, int](client, "test", limit, intHashKey, intEncode, intDecode)

	// Store 5 entries and flush — all 5 are dirty
	for i := range 5 {
		m.LoadOrStore(ctx, i, i*10)
	}
	r1 := m.Flush(ctx)
	require.NoError(t, r1.Err)
	assert.Equal(t, int64(5), client.batchSets.Load())

	// Flush again without touching anything — 0 batch ops expected
	client.batchSets.Store(0)
	r2 := m.Flush(ctx)
	require.NoError(t, r2.Err)
	assert.Equal(t, int64(0), client.batchSets.Load())
	// Keys should still be returned even though nothing was dirty
	assert.Len(t, r2.Keys, 5)

	// Touch only 2 entries — only 2 should be flushed
	client.batchSets.Store(0)
	m.LoadOrStore(ctx, 0, 0) // cache hit, marks dirty
	m.LoadOrStore(ctx, 3, 0) // cache hit, marks dirty
	r3 := m.Flush(ctx)
	require.NoError(t, r3.Err)
	assert.Equal(t, int64(2), client.batchSets.Load())
}

func TestPersisted_LoadOrStore_GetErrorReturnsExceeded(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	// Pre-populate storage with an existing value
	_ = client.Set(ctx, "test/"+intHashKey(1), []byte("100"))

	// Wrap with a client that fails on Get
	failing := &failGetClient{memClient: client}
	m := maps.NewPersisted(failing, "test", limit, intHashKey, intEncode, intDecode)

	// LoadOrStore should return the Exceeded sentinel because Get failed and
	// we cannot safely distinguish "key missing" from "transient failure".
	v, loaded := m.LoadOrStore(ctx, 1, 0)
	assert.True(t, maps.Exceeded(v, loaded), "should signal exceeded on Get failure")

	// The original value in storage must be preserved (not overwritten)
	data, err := client.Get(ctx, "test/"+intHashKey(1))
	require.NoError(t, err)
	got, err := intDecode(data)
	require.NoError(t, err)
	assert.Equal(t, 100, got, "storage value must not be overwritten when Get fails")
}
