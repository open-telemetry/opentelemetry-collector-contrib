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

func TestWriteThrough_LoadOrStore_ReadFromStorage(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	// Pre-populate storage (simulates another collector instance writing)
	_ = client.Set(ctx, "test/"+intHashKey(1), []byte("42"))

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	// First access: cache miss → read-through from storage → store in cache.
	// loaded=false because the value was stored into the cache (not loaded
	// from an existing cache entry). Same semantics as Persisted.
	v, loaded := m.LoadOrStore(ctx, 1, 0)
	assert.False(t, loaded)
	assert.Equal(t, 42, v) // recovered from storage, not the default 0
}

func TestWriteThrough_LoadOrStore_NewEntry(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	// Not in storage — should store default
	v, loaded := m.LoadOrStore(ctx, 1, 42)
	assert.False(t, loaded)
	assert.Equal(t, 42, v)

	// Verify it was written to storage
	data, err := client.Get(ctx, "test/"+intHashKey(1))
	require.NoError(t, err)
	got, err := intDecode(data)
	require.NoError(t, err)
	assert.Equal(t, 42, got)
}

func TestWriteThrough_CacheTakesPrecedence(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	// Store initial value
	m.LoadOrStore(ctx, 1, 10)

	// Externally update storage (simulates another collector instance)
	_ = client.Set(ctx, "test/"+intHashKey(1), []byte("99"))

	// WriteThrough caches values for concurrent safety within a process.
	// External updates are NOT visible once cached — this is the expected
	// tradeoff for safe intra-process concurrency.
	v, loaded := m.LoadOrStore(ctx, 1, 0)
	assert.True(t, loaded)
	assert.Equal(t, 10, v) // cached value, not external update
}

func TestWriteThrough_Store(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	// Create entry
	m.LoadOrStore(ctx, 1, 10)

	// Update via Store (simulates mutation after aggregation)
	require.NoError(t, m.Store(ctx, 1, 42))

	// Verify storage was updated
	data, err := client.Get(ctx, "test/"+intHashKey(1))
	require.NoError(t, err)
	got, err := intDecode(data)
	require.NoError(t, err)
	assert.Equal(t, 42, got)
}

func TestWriteThrough_LoadAndDelete(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	// Store and verify
	m.LoadOrStore(ctx, 1, 42)
	assert.Equal(t, 1, client.Len())

	// Delete — returns the cached value
	v, loaded := m.LoadAndDelete(ctx, 1)
	assert.True(t, loaded)
	assert.Equal(t, 42, v)

	// Storage should be empty
	assert.Equal(t, 0, client.Len())

	// Double delete returns false
	_, loaded = m.LoadAndDelete(ctx, 1)
	assert.False(t, loaded)
}

func TestWriteThrough_Exceeded(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(2)

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	// Fill to capacity
	m.LoadOrStore(ctx, 1, 10)
	m.LoadOrStore(ctx, 2, 20)

	// Third entry should be rejected
	v, loaded := m.LoadOrStore(ctx, 3, 30)
	assert.True(t, maps.Exceeded(v, loaded))

	// Existing entries still accessible
	v, loaded = m.LoadOrStore(ctx, 1, 0)
	assert.True(t, loaded)
	assert.Equal(t, 10, v)
}

func TestWriteThrough_Keys(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	m.LoadOrStore(ctx, 1, 10)
	m.LoadOrStore(ctx, 2, 20)
	m.LoadOrStore(ctx, 3, 30)

	keys := m.Keys()
	assert.Len(t, keys, 3)

	expected := map[string]struct{}{
		intHashKey(1): {},
		intHashKey(2): {},
		intHashKey(3): {},
	}
	for _, k := range keys {
		_, ok := expected[k]
		assert.True(t, ok, "unexpected key: %s", k)
	}
}

func TestWriteThrough_LoadOrStore_GetErrorReturnsExceeded(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	// Pre-populate storage with an existing value
	_ = client.Set(ctx, "test/"+intHashKey(1), []byte("100"))

	// Wrap with a client that fails on Get
	failing := &failGetClient{memClient: client}
	m := maps.NewWriteThrough(failing, "test", limit, intHashKey, intEncode, intDecode)

	// LoadOrStore should return the Exceeded sentinel because Get failed and
	// we cannot safely distinguish "key missing" from "transient failure".
	v, loaded := m.LoadOrStore(ctx, 1, 0)
	assert.True(t, maps.Exceeded(v, loaded), "should signal exceeded on Get failure")

	// The original value in storage must be preserved (not overwritten with "0")
	data, err := client.Get(ctx, "test/"+intHashKey(1))
	require.NoError(t, err)
	got, err := intDecode(data)
	require.NoError(t, err)
	assert.Equal(t, 100, got, "storage value must not be overwritten when Get fails")
}

func TestWriteThrough_LoadOrStore_DecodeErrorPreservesStorage(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	// Pre-populate storage with corrupted data (not decodable as int)
	_ = client.Set(ctx, "test/"+intHashKey(1), []byte("not-a-number"))

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	// LoadOrStore should return default because decode failed.
	// The value is cached so subsequent calls return the same default.
	v, loaded := m.LoadOrStore(ctx, 1, 0)
	assert.False(t, loaded)
	assert.Equal(t, 0, v)

	// Corrupted data in storage is NOT overwritten by LoadOrStore (data was
	// non-empty, so the "write default" branch is skipped). It will be
	// overwritten on the next explicit Store call after aggregation.
	data, err := client.Get(ctx, "test/"+intHashKey(1))
	require.NoError(t, err)
	assert.Equal(t, []byte("not-a-number"), data, "corrupted storage must not be overwritten by LoadOrStore")
}

func TestWriteThrough_DeleteByHashKey(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	// Store and verify
	m.LoadOrStore(ctx, 1, 42)
	assert.Equal(t, 1, client.Len())

	// Delete by hash key (simulates orphan cleanup)
	require.NoError(t, m.DeleteByHashKey(ctx, intHashKey(1)))

	// Storage should be empty
	data, err := client.Get(ctx, "test/"+intHashKey(1))
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestWriteThrough_Concurrent(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

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

	// Exactly 100 unique keys should be stored, 900 loads
	assert.Equal(t, int64(100), stores.Load())
	assert.Equal(t, int64(900), loads.Load())
}

func TestWriteThrough_ConcurrentSamePointer(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewWriteThrough(client, "test", limit, intHashKey, intEncode, intDecode)

	// Store a key first
	first, _ := m.LoadOrStore(ctx, 1, 42)

	// Concurrent LoadOrStore calls must all return the same pointer
	ptrs := make([]int, 10)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Go(func() {
			v, loaded := m.LoadOrStore(ctx, 1, 0)
			assert.True(t, loaded)
			ptrs[i] = v
		})
	}
	wg.Wait()

	for i := range 10 {
		assert.Equal(t, first, ptrs[i], "goroutine %d got different value", i)
	}
}

func TestWriteThrough_FlushThenRecover(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	// Session 1: store values via write-through
	limit1 := maps.Limit(100)
	m1 := maps.NewWriteThrough(client, "test", limit1, intHashKey, intEncode, intDecode)
	m1.LoadOrStore(ctx, 1, 100)
	m1.LoadOrStore(ctx, 2, 200)

	// Write-through: values are already in storage immediately
	assert.Equal(t, 2, client.Len())

	// Update via Store (simulates aggregation mutation)
	require.NoError(t, m1.Store(ctx, 1, 150))

	// Session 2: new map against the same storage client (simulates restart)
	limit2 := maps.Limit(100)
	m2 := maps.NewWriteThrough(client, "test", limit2, intHashKey, intEncode, intDecode)

	// Should recover values via read-through
	v, loaded := m2.LoadOrStore(ctx, 1, 0)
	assert.False(t, loaded) // first access in this session
	assert.Equal(t, 150, v) // recovered the updated value

	v, loaded = m2.LoadOrStore(ctx, 2, 0)
	assert.False(t, loaded)
	assert.Equal(t, 200, v)

	// Key 3 was never stored — should use default
	v, loaded = m2.LoadOrStore(ctx, 3, 0)
	assert.False(t, loaded)
	assert.Equal(t, 0, v)
}
