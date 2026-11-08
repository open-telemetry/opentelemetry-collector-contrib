// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps_test

import (
	"context"
	"errors"
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

	v, loaded, _ := m.LoadOrStore(ctx, 1, 42)
	assert.False(t, loaded)
	assert.Equal(t, 42, v)

	v, loaded, _ = m.LoadOrStore(ctx, 1, 99)
	assert.True(t, loaded)
	assert.Equal(t, 42, v)
}

func TestPersisted_LoadOrStore_ReadThrough(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	_ = client.Set(ctx, "test/"+intHashKey(1), []byte("42"))

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	v, loaded, _ := m.LoadOrStore(ctx, 1, 0)
	assert.False(t, loaded)
	assert.Equal(t, 42, v)
}

func TestPersisted_LoadOrStore_StorageMiss(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	v, loaded, _ := m.LoadOrStore(ctx, 1, 42)
	assert.False(t, loaded)
	assert.Equal(t, 42, v)
}

func TestPersisted_LoadAndDelete(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	_, _, err := m.LoadOrStore(ctx, 1, 42)
	require.NoError(t, err)
	require.NoError(t, m.Flush(ctx, m.Keys()))
	require.Equal(t, 1, client.Len())

	v, loaded, _ := m.LoadAndDelete(ctx, 1)
	assert.True(t, loaded)
	assert.Equal(t, 42, v)

	assert.Equal(t, 0, client.Len())

	_, loaded, _ = m.LoadAndDelete(ctx, 1)
	assert.False(t, loaded)
}

func TestPersisted_Flush(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	for i := range 5 {
		_, _, err := m.LoadOrStore(ctx, i, i*10)
		require.NoError(t, err)
	}

	assert.Equal(t, 0, client.Len())

	keys := m.Keys()
	require.NoError(t, m.Flush(ctx, keys))

	assert.Equal(t, 5, client.Len())

	assert.Len(t, keys, 5)

	for i := range 5 {
		data, err := client.Get(ctx, "test/"+intHashKey(i))
		require.NoError(t, err)
		v, err := intDecode(data)
		require.NoError(t, err)
		assert.Equal(t, i*10, v)
	}
}

func TestPersisted_LimitExceeded(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(2)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	v1, loaded, err := m.LoadOrStore(ctx, 1, 10)
	require.NoError(t, err)
	assert.False(t, loaded)
	assert.Equal(t, 10, v1)

	v2, loaded, err := m.LoadOrStore(ctx, 2, 20)
	require.NoError(t, err)
	assert.False(t, loaded)
	assert.Equal(t, 20, v2)

	v3, loaded, err := m.LoadOrStore(ctx, 3, 30)
	require.ErrorIs(t, err, maps.ErrLimitExceeded)
	assert.False(t, loaded)
	assert.Zero(t, v3)
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
				_, loaded, _ := m.LoadOrStore(ctx, i, i*10)
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

	limit1 := maps.Limit(100)
	m1 := maps.NewPersisted(client, "test", limit1, intHashKey, intEncode, intDecode)
	_, _, err := m1.LoadOrStore(ctx, 1, 100)
	require.NoError(t, err)
	_, _, err = m1.LoadOrStore(ctx, 2, 200)
	require.NoError(t, err)
	require.NoError(t, m1.Flush(ctx, m1.Keys()))

	limit2 := maps.Limit(100)
	m2 := maps.NewPersisted(client, "test", limit2, intHashKey, intEncode, intDecode)

	v, loaded, _ := m2.LoadOrStore(ctx, 1, 0)
	assert.False(t, loaded)
	assert.Equal(t, 100, v)

	v, loaded, _ = m2.LoadOrStore(ctx, 2, 0)
	assert.False(t, loaded)
	assert.Equal(t, 200, v)

	v, loaded, _ = m2.LoadOrStore(ctx, 3, 0)
	assert.False(t, loaded)
	assert.Equal(t, 0, v)
}

func TestPersisted_DeleteByHashKey(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	m := maps.NewPersisted(client, "test", limit, intHashKey, intEncode, intDecode)

	_, _, err := m.LoadOrStore(ctx, 1, 42)
	require.NoError(t, err)
	require.NoError(t, m.Flush(ctx, m.Keys()))
	require.Equal(t, 1, client.Len())

	require.NoError(t, m.DeleteByHashKey(ctx, intHashKey(1)))

	data, err := client.Get(ctx, "test/"+intHashKey(1))
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestPersisted_FlushRetriesAndSnapshotsMutations(t *testing.T) {
	ctx := t.Context()
	client := &retryClient{memClient: newMemClient(), fail: true}
	encode := func(v *int) ([]byte, error) { return intEncode(*v) }
	decode := func(b []byte) (*int, error) { v, err := intDecode(b); return &v, err }
	m := maps.NewPersisted(client, "test", maps.Limit(10), intHashKey, encode, decode)
	value, _, _ := m.LoadOrStore(ctx, 1, new(42))
	require.Error(t, m.Flush(ctx, m.Keys()))
	client.fail = false
	require.NoError(t, m.Flush(ctx, m.Keys()))
	got, err := client.Get(ctx, "test/1")
	require.NoError(t, err)
	assert.Equal(t, []byte("42"), got)
	// A caller may mutate a returned value after a flush has already run.
	*value = 43
	require.NoError(t, m.Flush(ctx, m.Keys()))
	got, err = client.Get(ctx, "test/1")
	require.NoError(t, err)
	assert.Equal(t, []byte("43"), got)
}

func TestPersisted_LoadOrStore_GetErrorPreservesState(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	limit := maps.Limit(100)

	_ = client.Set(ctx, "test/"+intHashKey(1), []byte("100"))

	failing := &failGetClient{memClient: client}
	m := maps.NewPersisted(failing, "test", limit, intHashKey, intEncode, intDecode)

	_, _, err := m.LoadOrStore(ctx, 1, 0)
	require.ErrorContains(t, err, "simulated storage failure")
	require.NoError(t, m.Flush(ctx, m.Keys()))

	data, err := client.Get(ctx, "test/"+intHashKey(1))
	require.NoError(t, err)
	got, err := intDecode(data)
	require.NoError(t, err)
	assert.Equal(t, 100, got, "storage value must not be overwritten when Get fails")
}

func TestPersisted_CorruptStateIsNotOverwritten(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()
	require.NoError(t, client.Set(ctx, "test/1", []byte("invalid integer")))
	m := maps.NewPersisted(client, "test", maps.Limit(10), intHashKey, intEncode, intDecode)
	_, _, err := m.LoadOrStore(ctx, 1, 42)
	require.ErrorContains(t, err, "decode state")
	require.NoError(t, m.Flush(ctx, m.Keys()))
	got, err := client.Get(ctx, "test/1")
	require.NoError(t, err)
	require.Equal(t, []byte("invalid integer"), got)
}

type deleteRetryClient struct {
	*memClient
	fail bool
}

func (c *deleteRetryClient) Delete(ctx context.Context, key string) error {
	if c.fail {
		return errors.New("delete unavailable")
	}
	return c.memClient.Delete(ctx, key)
}

func TestPersisted_DeleteFailureIsRetryable(t *testing.T) {
	ctx := t.Context()
	client := &deleteRetryClient{memClient: newMemClient(), fail: true}
	m := maps.NewPersisted(client, "test", maps.Limit(10), intHashKey, intEncode, intDecode)
	_, _, err := m.LoadOrStore(ctx, 1, 42)
	require.NoError(t, err)
	require.NoError(t, m.Flush(ctx, m.Keys()))
	_, _, err = m.LoadAndDelete(ctx, 1)
	require.Error(t, err)
	require.NoError(t, m.Flush(ctx, m.Keys()))
	client.fail = false
	value, loaded, err := m.LoadAndDelete(ctx, 1)
	require.NoError(t, err)
	require.True(t, loaded)
	require.Equal(t, 42, value)
	require.NoError(t, m.Flush(ctx, m.Keys()))
	require.Zero(t, client.Len())
}

func TestPersistedFlushDefersUnindexedKeys(t *testing.T) {
	client := newMemClient()
	m := maps.NewPersisted(client, "test", maps.Limit(10), intHashKey, intEncode, intDecode)
	_, _, err := m.LoadOrStore(t.Context(), 1, 10)
	require.NoError(t, err)
	indexed := m.Keys()
	_, _, err = m.LoadOrStore(t.Context(), 2, 20)
	require.NoError(t, err)
	require.NoError(t, m.Flush(t.Context(), indexed))
	first, err := client.Get(t.Context(), "test/1")
	require.NoError(t, err)
	require.Equal(t, []byte("10"), first)
	second, err := client.Get(t.Context(), "test/2")
	require.NoError(t, err)
	require.Nil(t, second)
	require.NoError(t, m.Flush(t.Context(), m.Keys()))
	second, err = client.Get(t.Context(), "test/2")
	require.NoError(t, err)
	require.Equal(t, []byte("20"), second)
}
