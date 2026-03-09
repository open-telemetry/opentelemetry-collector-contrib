// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

func TestCheckpointerGetAndSet(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := newCheckpointer(client, zap.NewNop())

	ctx := context.Background()

	// Test Set
	err := checkpointer.SetResourceVersion(ctx, "default", "pods", "12345")
	require.NoError(t, err)

	// Test Get
	rv, err := checkpointer.GetResourceVersion(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "12345", rv)
}

func TestCheckpointerKeyFormat(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := newCheckpointer(client, zap.NewNop())

	ctx := context.Background()

	tests := []struct {
		name         string
		namespace    string
		objectType   string
		expectedKey  string
		resourceVersion string
	}{
		{
			name:         "cluster-scoped resource (nodes)",
			namespace:    "",
			objectType:   "nodes",
			expectedKey:  "latestResourceVersion/nodes",
			resourceVersion: "100",
		},
		{
			name:         "cluster-scoped resource (namespaces)",
			namespace:    "",
			objectType:   "namespaces",
			expectedKey:  "latestResourceVersion/namespaces",
			resourceVersion: "200",
		},
		{
			name:         "namespaced resource in default",
			namespace:    "default",
			objectType:   "pods",
			expectedKey:  "latestResourceVersion/pods.default",
			resourceVersion: "300",
		},
		{
			name:         "namespaced resource in kube-system",
			namespace:    "kube-system",
			objectType:   "events",
			expectedKey:  "latestResourceVersion/events.kube-system",
			resourceVersion: "400",
		},
		{
			name:         "cluster-wide watch of pods",
			namespace:    "",
			objectType:   "pods",
			expectedKey:  "latestResourceVersion/pods",
			resourceVersion: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the resourceVersion
			err := checkpointer.SetResourceVersion(ctx, tt.namespace, tt.objectType, tt.resourceVersion)
			require.NoError(t, err)

			// Verify the key format by getting it back
			rv, err := checkpointer.GetResourceVersion(ctx, tt.namespace, tt.objectType)
			require.NoError(t, err)
			assert.Equal(t, tt.resourceVersion, rv)

			// Verify the actual key in storage
			key := checkpointer.getCheckpointKey(tt.namespace, tt.objectType)
			assert.Equal(t, tt.expectedKey, key)
		})
	}
}

func TestCheckpointerGetNonExistent(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := newCheckpointer(client, zap.NewNop())

	ctx := context.Background()

	// Get non-existent key
	rv, err := checkpointer.GetResourceVersion(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "", rv)
}

func TestCheckpointerUpdate(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := newCheckpointer(client, zap.NewNop())

	ctx := context.Background()

	// Set initial value
	err := checkpointer.SetResourceVersion(ctx, "default", "pods", "100")
	require.NoError(t, err)

	// Update to new value
	err = checkpointer.SetResourceVersion(ctx, "default", "pods", "200")
	require.NoError(t, err)

	// Verify updated value
	rv, err := checkpointer.GetResourceVersion(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv)
}

func TestCheckpointerMultipleNamespaces(t *testing.T) {
	client := storagetest.NewInMemoryClient(component.KindReceiver, component.MustNewID("test"), "test")
	checkpointer := newCheckpointer(client, zap.NewNop())

	ctx := context.Background()

	// Set different versions for different namespaces
	err := checkpointer.SetResourceVersion(ctx, "default", "pods", "100")
	require.NoError(t, err)

	err = checkpointer.SetResourceVersion(ctx, "kube-system", "pods", "200")
	require.NoError(t, err)

	// Verify they are independent
	rv1, err := checkpointer.GetResourceVersion(ctx, "default", "pods")
	require.NoError(t, err)
	assert.Equal(t, "100", rv1)

	rv2, err := checkpointer.GetResourceVersion(ctx, "kube-system", "pods")
	require.NoError(t, err)
	assert.Equal(t, "200", rv2)
}

func TestCheckpointerNilClient(t *testing.T) {
	checkpointer := newCheckpointer(nil, zap.NewNop())

	ctx := context.Background()

	// Get with nil client should return error
	_, err := checkpointer.GetResourceVersion(ctx, "default", "pods")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage client is nil")

	// Set with nil client should return error
	err = checkpointer.SetResourceVersion(ctx, "default", "pods", "100")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage client is nil")
}
