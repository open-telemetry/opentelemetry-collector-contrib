// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package checkpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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

			key := checkpointer.CheckpointKey(tt.namespace, tt.objectType)
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
	assert.False(t, cp.AlreadySeen("100", "default"))
}

func TestCheckpointerLoadAndAlreadySeen(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := New(client, zap.NewNop())
	ctx := t.Context()

	require.NoError(t, cp.SetCheckpoint(ctx, "default", "pods", "200"))
	require.NoError(t, cp.SetCheckpoint(ctx, "kube-system", "pods", "500"))
	require.NoError(t, cp.Flush(ctx))

	cp.Load(ctx, []string{"default", "kube-system"}, "pods")

	assert.True(t, cp.AlreadySeen("199", "default"), "RV below persisted should be seen")
	assert.True(t, cp.AlreadySeen("200", "default"), "RV equal to persisted should be seen (≤)")
	assert.False(t, cp.AlreadySeen("201", "default"), "RV above persisted should not be seen")

	assert.True(t, cp.AlreadySeen("500", "kube-system"))
	assert.False(t, cp.AlreadySeen("501", "kube-system"))
}

func TestCheckpointerLoadSkipsMissingAndUnparseable(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := New(client, zap.NewNop())
	ctx := t.Context()

	// Seed a parseable RV for "default" and a non-numeric RV for "weird" by
	// going around SetCheckpoint's validation — write the raw bytes directly.
	require.NoError(t, cp.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, cp.Flush(ctx))
	require.NoError(t, client.Set(ctx, cp.CheckpointKey("weird", "pods"), []byte("not-a-number")))

	cp.Load(ctx, []string{"default", "missing", "weird"}, "pods")

	assert.True(t, cp.AlreadySeen("50", "default"), "valid RV namespace loaded")
	assert.False(t, cp.AlreadySeen("50", "missing"), "namespace with no checkpoint must not be marked seen")
	assert.False(t, cp.AlreadySeen("50", "weird"), "namespace with unparseable RV must not be marked seen")
}

func TestCheckpointerLoadResetsState(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := New(client, zap.NewNop())
	ctx := t.Context()

	require.NoError(t, cp.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, cp.Flush(ctx))
	cp.Load(ctx, []string{"default"}, "pods")
	assert.True(t, cp.AlreadySeen("100", "default"))

	// Delete the persisted entry; Load should discard the previously cached value.
	require.NoError(t, cp.DeleteCheckpoint(ctx, "default", "pods"))
	cp.Load(ctx, []string{"default"}, "pods")
	assert.False(t, cp.AlreadySeen("100", "default"), "Load must reset previously loaded entries")
}

func TestCheckpointerAlreadySeenUnparseableRV(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	cp := New(client, zap.NewNop())
	ctx := t.Context()

	require.NoError(t, cp.SetCheckpoint(ctx, "default", "pods", "100"))
	require.NoError(t, cp.Flush(ctx))
	cp.Load(ctx, []string{"default"}, "pods")

	assert.False(t, cp.AlreadySeen("not-a-number", "default"), "unparseable RV must not be marked seen")
}
