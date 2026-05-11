// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestPersistStaleIndex_RoundTrip(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	// Persist keys from 3 maps
	err := persistStaleIndex(ctx, client, []string{"aaa", "bbb"}, []string{"ccc"}, []string{"ddd", "eee"})
	require.NoError(t, err)

	// Load them back
	loaded := loadStaleIndex(ctx, client)
	require.Len(t, loaded, 5)
	for _, k := range []string{"aaa", "bbb", "ccc", "ddd", "eee"} {
		_, ok := loaded[k]
		assert.True(t, ok, "expected key %q in loaded index", k)
	}
}

func TestLoadStaleIndex_Empty(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	loaded := loadStaleIndex(ctx, client)
	assert.Nil(t, loaded)
}

func TestPersistStaleIndex_EmptyKeys(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	err := persistStaleIndex(ctx, client, nil, nil, nil)
	require.NoError(t, err)

	// Empty buffer is written — loadStaleIndex should return nil
	loaded := loadStaleIndex(ctx, client)
	assert.Nil(t, loaded)
}

// fakePersisted implements deletableByHash for testing scheduleOrphanCleanup.
type fakePersisted struct {
	mu      sync.Mutex
	active  []string
	deleted []string
}

func (f *fakePersisted) Keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.active...)
}

func (f *fakePersisted) DeleteByHashKey(_ context.Context, hashKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, hashKey)
	return nil
}

func (f *fakePersisted) getDeleted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleted...)
}

func TestScheduleOrphanCleanup_DeletesOrphans(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	// Simulate previous session: persisted 5 keys in the stale index
	require.NoError(t, persistStaleIndex(ctx, client, []string{"a", "b", "c", "d", "e"}))

	// Current session: only keys "a" and "c" are still active
	p := &fakePersisted{active: []string{"a", "c"}}

	done := make(chan struct{})
	scheduleOrphanCleanup(ctx, client, done, 10*time.Millisecond, p)

	require.Eventually(t, func() bool {
		return len(p.getDeleted()) == 3
	}, 5*time.Second, 10*time.Millisecond)
	assert.ElementsMatch(t, []string{"b", "d", "e"}, p.getDeleted())
}

func TestScheduleOrphanCleanup_NoPreviousIndex(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	// No stale index in storage — scheduleOrphanCleanup returns immediately
	// without spawning a goroutine, so no deletions will ever happen.
	p := &fakePersisted{active: []string{"x"}}

	done := make(chan struct{})
	scheduleOrphanCleanup(ctx, client, done, 10*time.Millisecond, p)

	// Give extra time to confirm nothing happens
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, p.getDeleted())
}

func TestScheduleOrphanCleanup_AllKeysActive(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	require.NoError(t, persistStaleIndex(ctx, client, []string{"a", "b"}))

	// All previous keys are still active — nothing to delete
	p := &fakePersisted{active: []string{"a", "b"}}

	done := make(chan struct{})
	scheduleOrphanCleanup(ctx, client, done, 10*time.Millisecond, p)

	// The goroutine must fire and complete with 0 deletions. We wait long
	// enough for it to run, then verify nothing was deleted.
	time.Sleep(100 * time.Millisecond)
	assert.Empty(t, p.getDeleted())
}

func TestScheduleOrphanCleanup_CancelledBeforeTimer(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	require.NoError(t, persistStaleIndex(ctx, client, []string{"a", "b", "c"}))

	p := &fakePersisted{active: []string{}}

	done := make(chan struct{})
	// Long maxStale — goroutine should be waiting on timer
	scheduleOrphanCleanup(ctx, client, done, time.Hour, p)

	// Close done channel before timer fires — goroutine should exit
	close(done)
	time.Sleep(50 * time.Millisecond)

	deleted := p.getDeleted()
	assert.Empty(t, deleted, "cleanup should not have run because done was closed before timer")
}

func TestScheduleOrphanCleanup_MultiplePersistedMaps(t *testing.T) {
	ctx := t.Context()
	client := newMemClient()

	// Previous session had keys from 2 maps
	require.NoError(t, persistStaleIndex(ctx, client, []string{"a", "b", "c"}))

	// 2 persisted maps: map1 has "a" active, map2 has "c" active
	p1 := &fakePersisted{active: []string{"a"}}
	p2 := &fakePersisted{active: []string{"c"}}

	done := make(chan struct{})
	scheduleOrphanCleanup(ctx, client, done, 10*time.Millisecond, p1, p2)

	// "b" is orphaned — should be deleted from both maps
	require.Eventually(t, func() bool {
		return len(p1.getDeleted()) == 1 && len(p2.getDeleted()) == 1
	}, 5*time.Second, 10*time.Millisecond)
	assert.ElementsMatch(t, []string{"b"}, p1.getDeleted())
	assert.ElementsMatch(t, []string{"b"}, p2.getDeleted())
}
