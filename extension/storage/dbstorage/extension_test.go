// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Skip tests on Windows temporarily, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11451
//go:build !windows
// +build !windows

package dbstorage

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestExtensionIntegrity(t *testing.T) {
	ctx := context.Background()
	se := newTestExtension(t)
	err := se.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	defer func() {
		err = se.Shutdown(context.Background())
		assert.NoError(t, err)
	}()

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
		c.Close(ctx)
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

func newTestExtension(t *testing.T) storage.Extension {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.DriverName = "sqlite3"
	cfg.DataSource = fmt.Sprintf("file:%s/foo.db?_busy_timeout=10000&_journal=WAL&_sync=NORMAL", t.TempDir())

	extension, err := f.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	return se
}

func newTestEntity(name string) component.ID {
	return component.NewIDWithName("nop", name)
}
