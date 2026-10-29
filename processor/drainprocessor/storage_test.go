// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/metadata"
)

func fileBackedStorageHost(t *testing.T) *storagetest.StorageHost {
	t.Helper()
	return storagetest.NewStorageHost().WithFileBackedStorageExtension("test", t.TempDir())
}

func storageID() *component.ID {
	id := storagetest.NewStorageID("test")
	return &id
}

// newManualProcessor creates a processor without auto-cleanup, for tests that
// need to control the Start/Shutdown lifecycle explicitly.
func newManualProcessor(t *testing.T, cfg *Config) *drainProcessor {
	t.Helper()
	set := processortest.NewNopSettings(metadata.Type)
	p, err := newDrainProcessor(set, cfg)
	require.NoError(t, err)
	return p
}

// TestStartShutdownWithoutStorage verifies that the processor works when
// storage is not configured (the default stateless mode).
func TestStartShutdownWithoutStorage(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	// Storage is nil by default.
	p := newTestProcessor(t, cfg)
	_ = p // Start/Shutdown handled by cleanup in newTestProcessor
}

// TestSnapshotPersistAndReload verifies the full save-on-shutdown /
// load-on-start round-trip: train lines, shutdown, create a new processor
// pointing at the same storage, start, and verify the tree was restored.
func TestSnapshotPersistAndReload(t *testing.T) {
	host := fileBackedStorageHost(t)
	sid := storageID()
	ctx := t.Context()

	// First instance: train lines, then shutdown to save.
	cfg1 := createDefaultConfig().(*Config)
	cfg1.Storage = sid
	p1 := newManualProcessor(t, cfg1)
	require.NoError(t, p1.Start(ctx, host))

	lines := []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
		"connected to host 172.16.0.1 on port 80",
	}
	for _, line := range lines {
		_, err := p1.processLogs(ctx, makeLogRecord(line))
		require.NoError(t, err)
	}
	require.NoError(t, p1.Shutdown(ctx))

	// Second instance: start with the same storage, verify loaded tree.
	cfg2 := createDefaultConfig().(*Config)
	cfg2.Storage = sid
	p2 := newManualProcessor(t, cfg2)
	require.NoError(t, p2.Start(ctx, host))

	// Match a new line that fits the trained pattern.
	out, err := p2.processLogs(ctx, makeLogRecord("connected to host 10.10.10.10 on port 9000"))
	require.NoError(t, err)

	tmpl := templateAttr(t, out)
	assert.Contains(t, tmpl, "<*>", "restored tree should match the trained pattern")

	require.NoError(t, p2.Shutdown(ctx))
}

// TestLoadedSnapshotSkipsSeed verifies that when a snapshot is loaded, seed
// templates/logs from the config are not applied (the snapshot already
// incorporates them).
func TestLoadedSnapshotSkipsSeed(t *testing.T) {
	host := fileBackedStorageHost(t)
	sid := storageID()
	ctx := t.Context()

	// First instance: train with specific pattern, shutdown.
	cfg1 := createDefaultConfig().(*Config)
	cfg1.Storage = sid
	cfg1.SeedTemplates = []string{"connected to host <*> on port <*>"}
	p1 := newManualProcessor(t, cfg1)
	require.NoError(t, p1.Start(ctx, host))
	_, err := p1.processLogs(ctx, makeLogRecord("disk write error on device sda"))
	require.NoError(t, err)
	require.NoError(t, p1.Shutdown(ctx))

	// Second instance: different seeds. If snapshot wins, the "disk" cluster
	// should exist; the new seed should NOT create a fresh cluster.
	cfg2 := createDefaultConfig().(*Config)
	cfg2.Storage = sid
	cfg2.SeedTemplates = []string{"new pattern that should not appear <*>"}
	p2 := newManualProcessor(t, cfg2)
	require.NoError(t, p2.Start(ctx, host))

	// The "disk" cluster from the first run should be present.
	out, err := p2.processLogs(ctx, makeLogRecord("disk write error on device sdb"))
	require.NoError(t, err)
	tmpl := templateAttr(t, out)
	assert.Contains(t, tmpl, "disk", "loaded snapshot should have the trained disk cluster")

	require.NoError(t, p2.Shutdown(ctx))
}

// TestLoadedSnapshotSkipsWarmup verifies that when a snapshot is loaded with
// enough clusters, warmup is skipped and records are annotated immediately.
func TestLoadedSnapshotSkipsWarmup(t *testing.T) {
	host := fileBackedStorageHost(t)
	sid := storageID()
	ctx := t.Context()

	// First instance: train enough distinct clusters.
	cfg1 := createDefaultConfig().(*Config)
	cfg1.Storage = sid
	p1 := newManualProcessor(t, cfg1)
	require.NoError(t, p1.Start(ctx, host))

	distinctLines := []string{
		"connected to host 10.0.0.1 on port 443",
		"disk write error on device sda",
		"user alice logged in from 10.0.0.1",
		"database query took 150ms",
		"HTTP GET /api/v1/users returned 200",
	}
	for _, line := range distinctLines {
		_, err := p1.processLogs(ctx, makeLogRecord(line))
		require.NoError(t, err)
	}
	require.NoError(t, p1.Shutdown(ctx))

	// Second instance: warmup threshold of 3. Loaded snapshot has 5 clusters.
	cfg2 := createDefaultConfig().(*Config)
	cfg2.Storage = sid
	cfg2.WarmupMinClusters = 3
	p2 := newManualProcessor(t, cfg2)
	require.NoError(t, p2.Start(ctx, host))

	assert.True(t, p2.warmedUp, "warmup should be skipped when loaded clusters >= threshold")

	// First record should be annotated immediately.
	out, err := p2.processLogs(ctx, makeLogRecord("connected to host 10.10.10.10 on port 9000"))
	require.NoError(t, err)
	_, ok := getFirstRecord(out).Attributes().Get("log.record.template")
	assert.True(t, ok, "first record should be annotated when warmup is skipped")

	require.NoError(t, p2.Shutdown(ctx))
}

// TestCorruptSnapshotFallsBackToSeed verifies that a corrupt snapshot in
// storage is handled gracefully: a warning is logged, seeds are applied, and
// the processor functions normally.
func TestCorruptSnapshotFallsBackToSeed(t *testing.T) {
	host := fileBackedStorageHost(t)
	sid := storageID()
	ctx := t.Context()

	// Write corrupt data to storage via the getStorageClient helper.
	corruptClient, err := getStorageClient(ctx, host, sid, component.MustNewID("drain"))
	require.NoError(t, err)
	require.NoError(t, corruptClient.Set(ctx, storageKey, []byte("not valid json")))
	require.NoError(t, corruptClient.Close(ctx))

	// New processor should fall back to seeds.
	cfg := createDefaultConfig().(*Config)
	cfg.Storage = sid
	cfg.SeedTemplates = []string{"disk write error on device <*>"}
	p := newManualProcessor(t, cfg)
	require.NoError(t, p.Start(ctx, host))

	// Seed should have been applied — match a disk pattern.
	out, err := p.processLogs(ctx, makeLogRecord("disk write error on device sda"))
	require.NoError(t, err)
	tmpl := templateAttr(t, out)
	assert.Contains(t, tmpl, "disk", "seed should have been applied after corrupt snapshot")

	require.NoError(t, p.Shutdown(ctx))
}

// TestStorageExtensionNotFound verifies that Start returns an error when the
// configured storage extension does not exist in the host.
func TestStorageExtensionNotFound(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	badID := component.MustNewID("nonexistent")
	cfg.Storage = &badID

	p := newManualProcessor(t, cfg)
	err := p.Start(t.Context(), componenttest.NewNopHost())
	assert.Error(t, err, "Start should fail when storage extension is not found")
	assert.Contains(t, err.Error(), "not found")
}

// TestPeriodicSave verifies that the tree is saved to storage at the
// configured interval.
func TestPeriodicSave(t *testing.T) {
	host := fileBackedStorageHost(t)
	sid := storageID()
	ctx := t.Context()

	cfg := createDefaultConfig().(*Config)
	cfg.Storage = sid
	cfg.SaveInterval = 50 * time.Millisecond

	p := newManualProcessor(t, cfg)
	require.NoError(t, p.Start(ctx, host))

	// Train some lines to create state worth saving.
	_, err := p.processLogs(ctx, makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)

	// Wait for at least one periodic save tick.
	time.Sleep(150 * time.Millisecond)
	require.NoError(t, p.Shutdown(ctx))

	// Verify storage has data by starting a second processor.
	cfg2 := createDefaultConfig().(*Config)
	cfg2.Storage = sid
	p2 := newManualProcessor(t, cfg2)
	require.NoError(t, p2.Start(ctx, host))

	assert.Positive(t, p2.drain.ClusterCount(), "periodic save should have persisted the tree")
	require.NoError(t, p2.Shutdown(ctx))
}

// TestSaveSkipsWhenUnchanged verifies that saveSnapshot does not write to
// storage when the tree has not changed since the last save.
func TestSaveSkipsWhenUnchanged(t *testing.T) {
	host := fileBackedStorageHost(t)
	sid := storageID()
	ctx := t.Context()

	cfg := createDefaultConfig().(*Config)
	cfg.Storage = sid
	p := newManualProcessor(t, cfg)
	require.NoError(t, p.Start(ctx, host))

	// Train a line and save.
	_, err := p.processLogs(ctx, makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)
	require.NoError(t, p.saveSnapshot(ctx))

	hashAfterFirstSave := p.lastSnapshotHash.Load()
	assert.NotZero(t, hashAfterFirstSave, "hash should be set after first save")

	// Save again without any changes — hash should remain the same.
	require.NoError(t, p.saveSnapshot(ctx))
	assert.Equal(t, hashAfterFirstSave, p.lastSnapshotHash.Load(), "hash should not change when tree is unchanged")

	require.NoError(t, p.Shutdown(ctx))
}

// TestConfigValidateSaveIntervalWithoutStorage verifies that setting
// save_interval without storage produces a validation error.
func TestConfigValidateSaveIntervalWithoutStorage(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.SaveInterval = 5 * time.Minute
	// Storage is nil.
	assert.Error(t, cfg.Validate(), "save_interval without storage should be an error")
}
