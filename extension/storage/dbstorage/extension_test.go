// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"

	ctypes "github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
)

func TestExtensionIntegrityWithSqlite(t *testing.T) {
	if runtime.GOOS == "windows" && os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test on Windows GH runners: test requires Docker to be running Linux containers")
	}

	testExtensionIntegrity(t, newSqliteTestExtension(t))
}

func TestExtensionIntegrityWithPostgres(t *testing.T) {
	if runtime.GOOS == "windows" && os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test on Windows GH runners: test requires Docker to be running Linux containers")
	}

	testExtensionIntegrity(t, newPostgresTestExtension(t))
}

func testExtensionIntegrity(t *testing.T, se storage.Extension) {
	ctx := context.Background()
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

func newSqliteTestExtension(t *testing.T) storage.Extension {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.DriverName = "sqlite3"
	cfg.DataSource = fmt.Sprintf("file:%s/foo.db?_busy_timeout=10000&_journal=WAL&_sync=NORMAL", t.TempDir())

	extension, err := f.Create(context.Background(), extensiontest.NewNopSettings(), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	return se
}

func newPostgresTestExtension(t *testing.T) storage.Extension {
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:14",
			HostConfigModifier: func(config *ctypes.HostConfig) {
				ports := nat.PortMap{}
				ports[nat.Port("5432")] = []nat.PortBinding{
					{HostPort: "5432"},
				}
				config.PortBindings = ports
			},
			Env: map[string]string{
				"POSTGRES_PASSWORD": "passwd",
				"POSTGRES_USER":     "root",
				"POSTGRES_DB":       "db",
			},
			WaitingFor: wait.ForListeningPort("5432"),
		},
		Started: true,
	}

	ctr, err := testcontainers.GenericContainer(context.Background(), req)
	require.NoError(t, err)
	port, err := ctr.MappedPort(context.Background(), "5432")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ctr.Terminate(context.Background()))
	})
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.DriverName = "pgx"
	cfg.DataSource = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", "127.0.0.1", port.Port(), "root", "passwd", "db")

	extension, err := f.Create(context.Background(), extensiontest.NewNopSettings(), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	return se
}

func newTestEntity(name string) component.ID {
	return component.MustNewIDWithName("nop", name)
}
