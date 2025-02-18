// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage/internal/metadata"
)

func TestExtensionIntegrityWithSqlite(t *testing.T) {
	if runtime.GOOS == "windows" && os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test on Windows GH runners: test requires Docker to be running Linux containers")
	}

	dbPath := filepath.Join(t.TempDir(), "foo.db")
	se, err := newSqliteTestExtension(dbPath)
	require.NoError(t, err)
	testExtensionIntegrity(t, se)
}

func TestExtensionIntegrityWithPostgres(t *testing.T) {
	if runtime.GOOS == "windows" && os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test on Windows GH runners: test requires Docker to be running Linux containers")
	}

	se, ctr, err := newPostgresTestExtension()
	t.Cleanup(func() {
		if ctr != nil {
			require.NoError(t, ctr.Terminate(context.Background()))
		}
	})
	require.NoError(t, err)

	testExtensionIntegrity(t, se)
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
	clients := make(map[component.ID]storage.Client, len(components))
	for _, c := range components {
		client, err := se.GetClient(ctx, c.kind, c.name, "")
		require.NoError(t, err)
		clients[c.name] = client
	}

	thrashClient := func(wg *sync.WaitGroup, n component.ID, c storage.Client) {
		// keys and values
		keys := []string{"a", "b", "c", "d", "e"}
		myBytes := []byte(n.Name())

		// Test Batch interface
		// Make ops for testing...
		opsSet := make([]*storage.Operation, 0, len(keys))
		opsGet := make([]*storage.Operation, 0, len(keys))
		opsDelete := make([]*storage.Operation, 0, len(keys))
		for i := 0; i < len(keys); i++ {
			opsSet = append(opsSet, &storage.Operation{
				Type:  storage.Set,
				Key:   keys[i],
				Value: append(myBytes, []byte("_batch_"+keys[i])...),
			})
			opsGet = append(opsGet, &storage.Operation{
				Type: storage.Get,
				Key:  keys[i],
			})
			opsDelete = append(opsDelete, &storage.Operation{
				Type: storage.Delete,
				Key:  keys[i],
			})
		}
		// Set in Batch
		err := c.Batch(ctx, opsSet...)
		require.NoError(t, err)
		// Get in Batch
		err = c.Batch(ctx, opsGet...)
		require.NoError(t, err)
		// validate values
		for _, v := range opsGet {
			assert.Equal(t, append(myBytes, []byte("_batch_"+v.Key)...), v.Value)
		}
		// Delete in Batch
		err = c.Batch(ctx, opsDelete...)
		require.NoError(t, err)

		// All 3 operations in single batch
		ops := []*storage.Operation{
			{
				Type:  storage.Set,
				Key:   "op",
				Value: []byte("set"),
			},
			{
				Type: storage.Get,
				Key:  "op",
			},
			{
				Type: storage.Delete,
				Key:  "op",
			},
		}
		err = c.Batch(ctx, ops...)
		require.NoError(t, err)
		// validate value
		assert.Equal(t, ops[0].Value, ops[1].Value)

		// Single-operation interfaces
		// Reset my values
		for i := 0; i < len(keys); i++ {
			err := c.Set(ctx, keys[i], append(myBytes, []byte("_"+keys[i])...))
			require.NoError(t, err)
		}

		// Make sure my values are still mine
		for i := 0; i < len(keys); i++ {
			v, err := c.Get(ctx, keys[i])
			require.NoError(t, err)
			require.Equal(t, append(myBytes, []byte("_"+keys[i])...), v)
		}

		// Delete my values
		for i := 0; i < len(keys); i++ {
			err := c.Delete(ctx, keys[i])
			require.NoError(t, err)
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

func BenchmarkBatchGet(b *testing.B) {
	recordSize := 10
	benchmarks := []struct {
		name        string
		backend     string
		singleBatch bool
		records     int
	}{}
	backends := map[string]storage.Extension{}

	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "benchBatchGet.sqlite")
	seSQLite, err := newSqliteTestExtension(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	backends[driverSQLite] = seSQLite

	sePostgreSQL, ctr, err := newPostgresTestExtension()
	b.Cleanup(func() {
		if ctrErr := ctr.Terminate(context.Background()); ctrErr != nil {
			b.Fatal(ctrErr)
		}
	})
	if err != nil {
		b.Fatal(err)
	}
	backends[driverPostgreSQL] = sePostgreSQL

	for _, se := range backends {
		err = se.Start(context.Background(), componenttest.NewNopHost())
		if err != nil {
			b.Fatal(err)
		}
		defer func() {
			err = se.Shutdown(context.Background())
			if err != nil {
				b.Fatal(err)
			}
		}()
	}

	for d := range backends {
		for _, n := range []int{2, 10, 100, 200, 500, 1000} {
			for _, bt := range []string{"MultiQuery", "SingleQuery"} {
				benchmarks = append(benchmarks,
					struct {
						name        string
						backend     string
						singleBatch bool
						records     int
					}{
						name:        bt,
						backend:     d,
						singleBatch: bt == "SingleQuery",
						records:     n,
					},
				)
			}
		}
	}

	for _, bench := range benchmarks {
		b.Run(fmt.Sprintf("%s/%s/%d", bench.backend, bench.name, bench.records), func(b *testing.B) {
			client, err := backends[bench.backend].GetClient(context.Background(), component.KindExporter, newTestEntity(bench.name), fmt.Sprintf("%d", bench.records))
			if err != nil {
				b.Fatal(err)
			}
			defer client.Close(context.Background())

			// Populate records and create set of Get Operations
			ops := make([]*storage.Operation, 0, bench.records)
			// if it's multi-query bench - prepend single different operation
			if !bench.singleBatch {
				ops = append(ops, &storage.Operation{
					Type: storage.Delete,
					Key:  "-1",
				})
			}
			for r := range bench.records {
				key := strconv.Itoa(r)
				if err := client.Set(context.Background(), key, randBytes(recordSize)); err != nil {
					b.Fatal(err)
				}
				ops = append(ops, &storage.Operation{
					Type: storage.Get,
					Key:  key,
				})
			}

			// Reset benchmark timer
			b.ResetTimer()

			// Run Benchmark
			for i := 0; i < b.N; i++ {
				if err := client.Batch(context.Background(), ops...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBatchSet(b *testing.B) {
	recordSize := 10
	benchmarks := []struct {
		name        string
		backend     string
		singleBatch bool
		records     int
	}{}
	backends := map[string]storage.Extension{}

	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "benchBatchSet.sqlite")
	seSQLite, err := newSqliteTestExtension(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	backends[driverSQLite] = seSQLite

	sePostgreSQL, ctr, err := newPostgresTestExtension()
	b.Cleanup(func() {
		if ctrErr := ctr.Terminate(context.Background()); ctrErr != nil {
			b.Fatal(ctrErr)
		}
	})
	if err != nil {
		b.Fatal(err)
	}
	backends[driverPostgreSQL] = sePostgreSQL

	for _, se := range backends {
		err = se.Start(context.Background(), componenttest.NewNopHost())
		if err != nil {
			b.Fatal(err)
		}
		defer func() {
			err = se.Shutdown(context.Background())
			if err != nil {
				b.Fatal(err)
			}
		}()
	}

	for d := range backends {
		for _, n := range []int{2, 10, 100, 200, 500, 1000} {
			for _, bt := range []string{"MultiQuery", "SingleQuery"} {
				benchmarks = append(benchmarks,
					struct {
						name        string
						backend     string
						singleBatch bool
						records     int
					}{
						name:        bt,
						backend:     d,
						singleBatch: bt == "SingleQuery",
						records:     n,
					},
				)
			}
		}
	}

	for _, bench := range benchmarks {
		b.Run(fmt.Sprintf("%s/%s/%d", bench.backend, bench.name, bench.records), func(b *testing.B) {
			client, err := backends[bench.backend].GetClient(context.Background(), component.KindExporter, newTestEntity(bench.name), fmt.Sprintf("%d", bench.records))
			if err != nil {
				b.Fatal(err)
			}
			defer client.Close(context.Background())

			// Populate records and create set of Get Operations
			ops := make([]*storage.Operation, 0, bench.records)
			// if it's multi-query bench - prepend single different operation
			if !bench.singleBatch {
				ops = append(ops, &storage.Operation{
					Type: storage.Delete,
					Key:  "-1",
				})
			}
			for r := range bench.records {
				key := strconv.Itoa(r)
				ops = append(ops, &storage.Operation{
					Type:  storage.Set,
					Key:   key,
					Value: randBytes(recordSize),
				})
			}

			// Reset benchmark timer
			b.ResetTimer()

			// Run Benchmark
			for i := 0; i < b.N; i++ {
				if err := client.Batch(context.Background(), ops...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBatchDelete(b *testing.B) {
	recordSize := 10
	benchmarks := []struct {
		name        string
		backend     string
		singleBatch bool
		records     int
	}{}
	backends := map[string]storage.Extension{}

	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "benchBatchDelete.sqlite")
	seSQLite, err := newSqliteTestExtension(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	backends[driverSQLite] = seSQLite

	sePostgreSQL, ctr, err := newPostgresTestExtension()
	b.Cleanup(func() {
		if ctrErr := ctr.Terminate(context.Background()); ctrErr != nil {
			b.Fatal(ctrErr)
		}
	})
	if err != nil {
		b.Fatal(err)
	}
	backends[driverPostgreSQL] = sePostgreSQL

	for _, se := range backends {
		err = se.Start(context.Background(), componenttest.NewNopHost())
		if err != nil {
			b.Fatal(err)
		}
		defer func() {
			err = se.Shutdown(context.Background())
			if err != nil {
				b.Fatal(err)
			}
		}()
	}

	for d := range backends {
		for _, n := range []int{2, 10, 100, 200, 500, 1000} {
			for _, bt := range []string{"MultiQuery", "SingleQuery"} {
				benchmarks = append(benchmarks,
					struct {
						name        string
						backend     string
						singleBatch bool
						records     int
					}{
						name:        bt,
						backend:     d,
						singleBatch: bt == "SingleQuery",
						records:     n,
					},
				)
			}
		}
	}

	for _, bench := range benchmarks {
		b.Run(fmt.Sprintf("%s/%s/%d", bench.backend, bench.name, bench.records), func(b *testing.B) {
			client, err := backends[bench.backend].GetClient(context.Background(), component.KindExporter, newTestEntity(bench.name), fmt.Sprintf("%d", bench.records))
			if err != nil {
				b.Fatal(err)
			}
			defer client.Close(context.Background())

			// Populate records and create set of Get Operations
			ops := make([]*storage.Operation, 0, bench.records)
			// if it's multi-query bench - prepend single different operation
			if !bench.singleBatch {
				ops = append(ops, &storage.Operation{
					Type: storage.Get,
					Key:  "-1",
				})
			}
			for r := range bench.records {
				key := strconv.Itoa(r)
				if err := client.Set(context.Background(), key, randBytes(recordSize)); err != nil {
					b.Fatal(err)
				}
				ops = append(ops, &storage.Operation{
					Type: storage.Delete,
					Key:  key,
				})
			}

			// Reset benchmark timer
			b.ResetTimer()

			// Run Benchmark
			for i := 0; i < b.N; i++ {
				if err := client.Batch(context.Background(), ops...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func randBytes(n int) []byte {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	bytes := make([]byte, n)
	//nolint:errcheck
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}

	return bytes
}

func newSqliteTestExtension(dbPath string) (storage.Extension, error) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.DriverName = driverSQLite
	cfg.DataSource = fmt.Sprintf("%s?_busy_timeout=10000&_journal=WAL&_sync=NORMAL", dbPath)

	extension, err := f.Create(context.Background(), extensiontest.NewNopSettingsWithType(metadata.Type), cfg)
	if err != nil {
		return nil, err
	}

	se, ok := extension.(storage.Extension)
	if !ok {
		return nil, fmt.Errorf("created extension is not a storage extension")
	}

	return se, nil
}

func newPostgresTestExtension() (storage.Extension, testcontainers.Container, error) {
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
	if err != nil {
		return nil, nil, err
	}
	port, err := ctr.MappedPort(context.Background(), "5432")
	if err != nil {
		return nil, nil, err
	}
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.DriverName = driverPostgreSQL
	cfg.DataSource = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", "127.0.0.1", port.Port(), "root", "passwd", "db")

	extension, err := f.Create(context.Background(), extensiontest.NewNopSettingsWithType(metadata.Type), cfg)
	if err != nil {
		return nil, nil, err
	}

	se, ok := extension.(storage.Extension)
	if !ok {
		return nil, nil, fmt.Errorf("created extension is not a storage extension")
	}

	return se, ctr, nil
}

func newTestEntity(name string) component.ID {
	return component.MustNewIDWithName("nop", name)
}
