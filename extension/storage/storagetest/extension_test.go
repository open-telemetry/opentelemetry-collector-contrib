// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package storagetest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

func TestID(t *testing.T) {
	require.Equal(t, NewStorageID("test"), NewInMemoryStorageExtension("test").ID)
	require.Equal(t, NewStorageID("test"), NewFileBackedStorageExtension("test", t.TempDir()).ID)
	require.Equal(t, NewNonStorageID("test"), NewNonStorageExtension("test").ID)
}

func TestInMemoryLifecycle(t *testing.T) {
	ext := NewInMemoryStorageExtension("test")
	require.Equal(t, component.NewIDWithName(testStorageType, "test"), ext.ID)
	runExtensionLifecycle(t, ext, false)
}

func TestFileBackedLifecycle(t *testing.T) {
	dir := t.TempDir()
	ext := NewFileBackedStorageExtension("test", dir)
	require.Equal(t, component.NewIDWithName(testStorageType, "test"), ext.ID)
	runExtensionLifecycle(t, ext, true)
}

func runExtensionLifecycle(t *testing.T, ext *TestStorage, expectPersistence bool) {
	ctx := context.Background()
	require.NoError(t, ext.Start(ctx, componenttest.NewNopHost()))

	clientOne, err := ext.GetClient(ctx, component.KindProcessor, component.NewID("foo"), "client_one")
	require.NoError(t, err)

	creatorID, err := CreatorID(ctx, clientOne)
	require.NoError(t, err)
	require.Equal(t, ext.ID, creatorID)

	// Write a value, confirm it is saved
	require.NoError(t, clientOne.Set(ctx, "foo", []byte("bar")))
	fooVal, err := clientOne.Get(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, []byte("bar"), fooVal)

	// Delete the value, confirm it is deleted
	require.NoError(t, clientOne.Delete(ctx, "foo"))
	fooVal, err = clientOne.Get(ctx, "foo")
	require.NoError(t, err)
	require.Nil(t, fooVal)

	// Write a new value, confirm it is saved
	require.NoError(t, clientOne.Set(ctx, "foo2", []byte("bar2")))
	fooVal, err = clientOne.Get(ctx, "foo2")
	require.NoError(t, err)
	require.Equal(t, []byte("bar2"), fooVal)

	// Close first client
	require.NoError(t, clientOne.Close(ctx))

	// Create new client to test persistence
	clientTwo, err := ext.GetClient(ctx, component.KindProcessor, component.NewID("foo"), "client_one")
	require.NoError(t, err)

	creatorID, err = CreatorID(ctx, clientTwo)
	require.NoError(t, err)
	require.Equal(t, ext.ID, creatorID)

	// Check if the value is accessible from another client
	fooVal, err = clientTwo.Get(ctx, "foo2")
	require.NoError(t, err)
	if expectPersistence {
		require.Equal(t, []byte("bar2"), fooVal)
	} else {
		require.Nil(t, fooVal)
	}

	// Perform some additional operations
	set := storage.SetOperation("foo3", []byte("bar3"))
	get := storage.GetOperation("foo3")
	delete := storage.DeleteOperation("foo3")
	getNil := storage.GetOperation("foo3")
	require.NoError(t, clientTwo.Batch(ctx, set, get, delete, getNil))
	require.Equal(t, get.Value, []byte("bar3"))
	require.Nil(t, getNil.Value)

	// Cleanup
	require.NoError(t, clientTwo.Close(ctx))
	require.NoError(t, ext.Shutdown(ctx))
}
