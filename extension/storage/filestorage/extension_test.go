// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestExtensionIntegrity(t *testing.T) {
	ctx := context.Background()
	se := newTestExtension(t)

	type mockComponent struct {
		kind component.Kind
		name component.ID
	}

	components := []mockComponent{
		{kind: component.KindReceiver, name: newTestEntity("receiver_one")},
		{kind: component.KindReceiver, name: newTestEntity("receiver_two")},
		{kind: component.KindProcessor, name: newTestEntity("processor_one")},
		{kind: component.KindProcessor, name: newTestEntity("processor_two")},
		{kind: component.KindExporter, name: newTestEntity("exporter_one")},
		{kind: component.KindExporter, name: newTestEntity("exporter_two")},
		{kind: component.KindExtension, name: newTestEntity("extension_one")},
		{kind: component.KindExtension, name: newTestEntity("extension_two")},
	}

	// Make a client for each component
	clients := make(map[component.ID]storage.Client)
	for _, c := range components {
		client, err := se.GetClient(ctx, c.kind, c.name, "")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, client.Close(ctx))
		})

		clients[c.name] = client
	}

	thrashClient := func(wg *sync.WaitGroup, n component.ID, c storage.Client) {
		// keys and values
		keys := []string{"a", "b", "c", "d", "e"}
		myBytes := []byte(n.Name())

		// Set my values
		for i := 0; i < len(keys); i++ {
			err := c.Set(ctx, keys[i], myBytes)
			require.NoError(t, err)
		}

		// Repeatedly thrash client
		for j := 0; j < 100; j++ {

			// Make sure my values are still mine
			for i := 0; i < len(keys); i++ {
				v, err := c.Get(ctx, keys[i])
				require.NoError(t, err)
				require.Equal(t, myBytes, v)
			}

			// Delete my values
			for i := 0; i < len(keys); i++ {
				err := c.Delete(ctx, keys[i])
				require.NoError(t, err)
			}

			// Reset my values
			for i := 0; i < len(keys); i++ {
				err := c.Set(ctx, keys[i], myBytes)
				require.NoError(t, err)
			}
		}
		wg.Done()
	}

	// Use clients concurrently
	var wg sync.WaitGroup
	for name, client := range clients {
		wg.Add(1)
		go thrashClient(&wg, name, client)
	}
	wg.Wait()
}

func TestClientHandlesSimpleCases(t *testing.T) {
	ctx := context.Background()
	se := newTestExtension(t)

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)

	myBytes := []byte("value")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// Set the data
	err = client.Set(ctx, "key", myBytes)
	require.NoError(t, err)

	// Set it again (nop does not error)
	err = client.Set(ctx, "key", myBytes)
	require.NoError(t, err)

	// Get actual data
	data, err := client.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes, data)

	// Delete the data
	err = client.Delete(ctx, "key")
	require.NoError(t, err)

	// Delete it again (nop does not error)
	err = client.Delete(ctx, "key")
	require.NoError(t, err)

	// Get missing data
	data, err = client.Get(ctx, "key")
	require.NoError(t, err)
	require.Nil(t, data)

}

func TestTwoClientsWithDifferentNames(t *testing.T) {
	ctx := context.Background()
	se := newTestExtension(t)

	client1, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"foo",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client1.Close(ctx))
	})

	client2, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"bar",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client2.Close(ctx))
	})

	myBytes1 := []byte("value1")
	myBytes2 := []byte("value2")

	// Set the data
	err = client1.Set(ctx, "key", myBytes1)
	require.NoError(t, err)

	err = client2.Set(ctx, "key", myBytes2)
	require.NoError(t, err)

	// Check it was associated accordingly
	data, err := client1.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes1, data)

	data, err = client2.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes2, data)
}

func TestGetClientErrorsOnDeletedDirectory(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir

	extension, err := f.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	// Delete the directory before getting client
	err = os.RemoveAll(tempDir)
	require.NoError(t, err)

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)

	require.Error(t, err)
	require.Nil(t, client)
}

func newTestExtension(t *testing.T) storage.Extension {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = t.TempDir()

	extension, err := f.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	return se
}

func newTestEntity(name string) component.ID {
	return component.NewIDWithName("nop", name)
}

func TestCompaction(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir

	extension, err := f.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Equal(t, 1, len(files))

	file := files[0]
	path := filepath.Join(tempDir, file.Name())
	stats, err := os.Stat(path)
	require.NoError(t, err)

	var key string
	i := 0

	// magic numbers giving enough data to force bbolt to allocate a new page
	// see https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/9004 for some discussion
	numEntries := 50
	entrySize := 512
	entry := make([]byte, entrySize)

	// add the data to the db
	for i = 0; i < numEntries; i++ {
		key = fmt.Sprintf("key_%d", i)
		err = client.Set(ctx, key, entry)
		require.NoError(t, err)
	}

	// compact the db
	c, ok := client.(*fileStorageClient)
	require.True(t, ok)
	err = c.Compact(tempDir, cfg.Timeout, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check size after compaction
	newStats, err := os.Stat(path)
	require.NoError(t, err)
	require.Less(t, stats.Size(), newStats.Size())

	// remove data from database
	for i = 0; i < numEntries; i++ {
		key = fmt.Sprintf("key_%d", i)
		err = c.Delete(ctx, key)
		require.NoError(t, err)
	}

	// compact after data removal
	c, ok = client.(*fileStorageClient)
	require.True(t, ok)
	err = c.Compact(tempDir, cfg.Timeout, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check size
	stats = newStats
	newStats, err = os.Stat(path)
	require.NoError(t, err)
	require.Less(t, newStats.Size(), stats.Size())
}

// TestCompactionRemoveTemp validates if temporary db used for compaction is removed afterwards
// test is performed for both: the same and different than storage directories
func TestCompactionRemoveTemp(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir

	extension, err := f.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check if only db exists in tempDir
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Equal(t, 1, len(files))
	fileName := files[0].Name()

	// perform compaction in the same directory
	c, ok := client.(*fileStorageClient)
	require.True(t, ok)
	err = c.Compact(tempDir, cfg.Timeout, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check if only db exists in tempDir
	files, err = os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Equal(t, 1, len(files))
	require.Equal(t, fileName, files[0].Name())

	// perform compaction in different directory
	emptyTempDir := t.TempDir()

	c, ok = client.(*fileStorageClient)
	require.True(t, ok)
	err = c.Compact(emptyTempDir, cfg.Timeout, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check if emptyTempDir is empty after compaction
	files, err = os.ReadDir(emptyTempDir)
	require.NoError(t, err)
	require.Equal(t, 0, len(files))
}
