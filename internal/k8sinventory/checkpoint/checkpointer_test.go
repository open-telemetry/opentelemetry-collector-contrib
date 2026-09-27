// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package checkpoint

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

func TestCheckpointerGetAndSet(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	err := checkpointer.SetCheckpoint(ctx, "default", "pods", "12345")
	require.NoError(t, err)

	require.NoError(t, checkpointer.Flush(ctx))

	rv, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "12345", rv)
}

func TestCheckpointerKeyFormat(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	tests := []struct {
		name            string
		namespace       string
		objectType      string
		expectedKey     string
		resourceVersion string
	}{
		{
			name:            "cluster-scoped resource (nodes)",
			namespace:       "",
			objectType:      "nodes",
			expectedKey:     "latestResourceVersion/nodes",
			resourceVersion: "100",
		},
		{
			name:            "cluster-scoped resource (namespaces)",
			namespace:       "",
			objectType:      "namespaces",
			expectedKey:     "latestResourceVersion/namespaces",
			resourceVersion: "200",
		},
		{
			name:            "namespaced resource in default",
			namespace:       "default",
			objectType:      "pods",
			expectedKey:     "latestResourceVersion/pods.default",
			resourceVersion: "300",
		},
		{
			name:            "namespaced resource in kube-system",
			namespace:       "kube-system",
			objectType:      "events",
			expectedKey:     "latestResourceVersion/events.kube-system",
			resourceVersion: "400",
		},
		{
			name:            "cluster-wide watch of pods",
			namespace:       "",
			objectType:      "pods",
			expectedKey:     "latestResourceVersion/pods",
			resourceVersion: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkpointer.SetCheckpoint(ctx, tt.namespace, tt.objectType, tt.resourceVersion)
			require.NoError(t, err)

			require.NoError(t, checkpointer.Flush(ctx))

			rv, err := checkpointer.GetCheckpoint(ctx, tt.namespace, tt.objectType)
			require.NoError(t, err)
			assert.Equal(t, tt.resourceVersion, rv)

			key := checkpointer.checkpointKey(tt.namespace, tt.objectType)
			assert.Equal(t, tt.expectedKey, key)
		})
	}
}

func TestCheckpointerGetNonExistent(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	rv, err := checkpointer.GetCheckpoint(t.Context(), "default", "pods")
	require.NoError(t, err)
	assert.Empty(t, rv)
}

func TestCheckpointerUpdate(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	// Buffer two updates — only the latest should be written on Flush.
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "200"))
	require.NoError(t, checkpointer.Flush(ctx))

	rv, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv)
}

func TestCheckpointerFlushBatches(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	// Simulate multiple watch streams writing without flushing.
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "150"))
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "kube-system", "pods", "200"))
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "", "nodes", "300"))

	// Single Flush writes the latest value for each key.
	require.NoError(t, checkpointer.Flush(ctx))

	rv1, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "150", rv1)

	rv2, err := checkpointer.GetCheckpoint(ctx, "kube-system", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv2)

	rv3, err := checkpointer.GetCheckpoint(ctx, "", "nodes")
	require.NoError(t, err)
	assert.Equal(t, "300", rv3)
}

func TestCheckpointerFlushClearsPending(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, checkpointer.Flush(ctx))

	// Second flush with no new writes should be a no-op.
	require.NoError(t, checkpointer.Flush(ctx))

	rv, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "100", rv)
}

func TestCheckpointerMultipleNamespaces(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "kube-system", "pods", "200"))
	require.NoError(t, checkpointer.Flush(ctx))

	rv1, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "100", rv1)

	rv2, err := checkpointer.GetCheckpoint(ctx, "kube-system", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv2)
}

func TestCheckpointerNilClient(t *testing.T) {
	checkpointer := New(nil, zap.NewNop())

	ctx := t.Context()

	// Get with nil client should return error.
	_, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage client is nil")

	// SetCheckpoint only buffers in memory — no error even with nil client.
	err = checkpointer.SetCheckpoint(ctx, "default", "pods", "100")
	assert.NoError(t, err)

	// Flush with nil client should return error.
	err = checkpointer.Flush(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage client is nil")

	// Delete with nil client should return error.
	err = checkpointer.DeleteCheckpoint(ctx, "default", "pods")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage client is nil")
}

func TestCheckpointerDelete(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "12345"))
	require.NoError(t, checkpointer.Flush(ctx))

	rv, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "12345", rv)

	require.NoError(t, checkpointer.DeleteCheckpoint(ctx, "default", "pods"))

	rv, err = checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Empty(t, rv)
}

func TestDeleteCheckpointClearsPendingCheckpoint(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "12345"))
	require.NoError(t, checkpointer.DeleteCheckpoint(ctx, "default", "pods"))
	require.NoError(t, checkpointer.Flush(ctx))

	rv, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Empty(t, rv)
}

func TestDeleteCheckpointWinsAgainstInFlightFlush(t *testing.T) {
	client := newBlockingSetClient(
		storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test"),
	)
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "12345"))

	flushDone := make(chan error, 1)
	go func() {
		flushDone <- checkpointer.Flush(ctx)
	}()
	<-client.setStarted

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- checkpointer.DeleteCheckpoint(ctx, "default", "pods")
	}()

	key := checkpointer.checkpointKey("default", "pods")
	require.Eventually(t, func() bool {
		checkpointer.mu.Lock()
		defer checkpointer.mu.Unlock()
		return checkpointer.deleted[key]
	}, time.Second, time.Millisecond)

	client.releaseSet()
	require.NoError(t, <-flushDone)
	require.NoError(t, <-deleteDone)

	rv, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Empty(t, rv, "delete must win against an in-flight flush")
}

// TestDeleteSetsTombstoneClearedBySet verifies the tombstone lifecycle:
// DeleteCheckpoint sets it (and clears pending), and a subsequent SetCheckpoint
// clears it so the fresh resourceVersion is allowed to flush.
func TestDeleteSetsTombstoneClearedBySet(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()
	key := checkpointer.checkpointKey("default", "pods")

	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, checkpointer.DeleteCheckpoint(ctx, "default", "pods"))

	checkpointer.mu.Lock()
	_, tombstoned := checkpointer.deleted[key]
	_, stillPending := checkpointer.pending[key]
	checkpointer.mu.Unlock()
	assert.True(t, tombstoned, "DeleteCheckpoint should set a tombstone")
	assert.False(t, stillPending, "DeleteCheckpoint should clear pending")

	// A fresh resourceVersion marks the key alive again and clears the tombstone.
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "200"))
	checkpointer.mu.Lock()
	_, tombstoned = checkpointer.deleted[key]
	checkpointer.mu.Unlock()
	assert.False(t, tombstoned, "SetCheckpoint should clear the tombstone")

	require.NoError(t, checkpointer.Flush(ctx))
	rv, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv, "post-delete SetCheckpoint value should flush")
}

func TestCheckpointerDeleteNonExistent(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	err := checkpointer.DeleteCheckpoint(t.Context(), "default", "pods")
	require.NoError(t, err)
}

func TestCheckpointerDeleteMultipleNamespaces(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "kube-system", "pods", "200"))
	require.NoError(t, checkpointer.Flush(ctx))

	require.NoError(t, checkpointer.DeleteCheckpoint(ctx, "default", "pods"))

	rv1, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Empty(t, rv1)

	rv2, err := checkpointer.GetCheckpoint(ctx, "kube-system", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv2)
}

func TestCheckpointerDeleteClusterWide(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	require.NoError(t, checkpointer.SetCheckpoint(ctx, "", "nodes", "500"))
	require.NoError(t, checkpointer.Flush(ctx))

	rv, err := checkpointer.GetCheckpoint(ctx, "", "nodes")
	require.NoError(t, err)
	assert.Equal(t, "500", rv)

	require.NoError(t, checkpointer.DeleteCheckpoint(ctx, "", "nodes"))

	rv, err = checkpointer.GetCheckpoint(ctx, "", "nodes")
	require.NoError(t, err)
	assert.Empty(t, rv)
}

func TestCheckpointerHighWatermark(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	// Simulate out-of-order resourceVersions from List() API.
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "500"))
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "100")) // lower — should be ignored
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "300")) // lower — should be ignored
	require.NoError(t, checkpointer.Flush(ctx))

	rv, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "500", rv)
}

func TestCheckpointerHighWatermarkFirstEntry(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	ctx := t.Context()

	// Key absent — any value should be accepted.
	require.NoError(t, checkpointer.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, checkpointer.Flush(ctx))

	rv, err := checkpointer.GetCheckpoint(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "100", rv)
}

func TestCheckpointerInvalidResourceVersion(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := New(client, zap.NewNop())

	err := checkpointer.SetCheckpoint(t.Context(), "default", "pods", "not-a-number")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid resourceVersion")
}

func TestCheckpointerAlreadySeenBeforeLoad(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := New(client, zap.NewNop())

	// Without Load, no namespace is known — AlreadySeen must not panic or return true.
	seen, err := cp.AlreadySeen("100", "default")
	require.NoError(t, err)
	assert.False(t, seen)
}

func TestCheckpointerLoadAndAlreadySeen(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := New(client, zap.NewNop())
	ctx := t.Context()

	require.NoError(t, cp.SetCheckpoint(ctx, "default", "pods", "200"))
	require.NoError(t, cp.SetCheckpoint(ctx, "kube-system", "pods", "500"))
	require.NoError(t, cp.Flush(ctx))

	require.NoError(t, cp.Load(ctx, []string{"default", "kube-system"}, "pods"))

	mustSeen := func(rv, ns string) bool {
		seen, err := cp.AlreadySeen(rv, ns)
		require.NoError(t, err)
		return seen
	}

	assert.True(t, mustSeen("199", "default"), "RV below persisted should be seen")
	assert.True(t, mustSeen("200", "default"), "RV equal to persisted should be seen (≤)")
	assert.False(t, mustSeen("201", "default"), "RV above persisted should not be seen")
	assert.True(t, mustSeen("500", "kube-system"))
	assert.False(t, mustSeen("501", "kube-system"))
}

func TestCheckpointerLoadSkipsMissingAndUnparseable(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := New(client, zap.NewNop())
	ctx := t.Context()

	// Seed a parseable RV for "default" and a non-numeric RV for "weird" by
	// going around SetCheckpoint's validation — write the raw bytes directly.
	require.NoError(t, cp.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, cp.Flush(ctx))
	require.NoError(t, client.Set(ctx, cp.checkpointKey("weird", "pods"), []byte("not-a-number")))

	// Load returns valid namespaces cleanly; missing keys are not an error.
	require.NoError(t, cp.Load(ctx, []string{"default", "missing"}, "pods"))

	seen, err := cp.AlreadySeen("50", "default")
	require.NoError(t, err)
	assert.True(t, seen, "valid RV namespace loaded")

	seen, err = cp.AlreadySeen("50", "missing")
	require.NoError(t, err)
	assert.False(t, seen, "namespace with no checkpoint must not be marked seen")

	// An unparseable persisted RV surfaces as a Load error rather than being silently skipped.
	require.Error(t, cp.Load(ctx, []string{"default", "weird"}, "pods"), "unparseable persisted RV should fail Load")
}

func TestCheckpointerLoadResetsState(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := New(client, zap.NewNop())
	ctx := t.Context()

	require.NoError(t, cp.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, cp.Flush(ctx))
	require.NoError(t, cp.Load(ctx, []string{"default"}, "pods"))
	seen, err := cp.AlreadySeen("100", "default")
	require.NoError(t, err)
	assert.True(t, seen)

	// Delete the persisted entry; Load should discard the previously cached value.
	require.NoError(t, cp.DeleteCheckpoint(ctx, "default", "pods"))
	require.NoError(t, cp.Load(ctx, []string{"default"}, "pods"))
	seen, err = cp.AlreadySeen("100", "default")
	require.NoError(t, err)
	assert.False(t, seen, "Load must reset previously loaded entries")
}

func TestCheckpointerAlreadySeenUnparseableRV(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := New(client, zap.NewNop())
	ctx := t.Context()

	require.NoError(t, cp.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, cp.Flush(ctx))
	require.NoError(t, cp.Load(ctx, []string{"default"}, "pods"))

	seen, err := cp.AlreadySeen("not-a-number", "default")
	require.Error(t, err, "unparseable RV must surface a parse error")
	assert.False(t, seen, "unparseable RV must not be marked seen")
}

type blockingSetClient struct {
	storage.Client

	setStarted chan struct{}
	setRelease chan struct{}
	startOnce  sync.Once
	release    sync.Once
}

func newBlockingSetClient(client storage.Client) *blockingSetClient {
	return &blockingSetClient{
		Client:     client,
		setStarted: make(chan struct{}),
		setRelease: make(chan struct{}),
	}
}

func (c *blockingSetClient) Set(ctx context.Context, key string, value []byte) error {
	c.startOnce.Do(func() {
		close(c.setStarted)
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.setRelease:
		return c.Client.Set(ctx, key, value)
	}
}

func (c *blockingSetClient) releaseSet() {
	c.release.Do(func() {
		close(c.setRelease)
	})
}
