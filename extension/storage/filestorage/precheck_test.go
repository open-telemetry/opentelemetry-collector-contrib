// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestOpenClient_RecreateDisabled_SkipsPrecheck(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.bbolt")

	require.NoError(t, seedBboltFile(dbPath))

	var precheckCalled bool
	lfs := newTestStorage(
		defaultRecreateConfig(t, tempDir, false),
		zap.NewNop(),
		func(context.Context, string, time.Duration) error {
			precheckCalled = true
			return nil
		},
	)

	client, err := lfs.openClient(ctx, dbPath)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.False(t, precheckCalled, "precheck must not run when Recreate is false")
	require.NoError(t, client.Close(ctx))
}

func TestOpenClient_RecreateEnabled_MissingFile_SkipsPrecheck(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.bbolt")

	var precheckCalled bool
	lfs := newTestStorage(
		defaultRecreateConfig(t, tempDir, true),
		zap.NewNop(),
		func(context.Context, string, time.Duration) error {
			precheckCalled = true
			return nil
		},
	)

	client, err := lfs.openClient(ctx, dbPath)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.False(t, precheckCalled, "precheck must not run when DB file is absent")
	require.NoError(t, client.Close(ctx))
}

func TestOpenClient_RecreateEnabled_PrecheckClean_PreservesData(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.bbolt")

	require.NoError(t, seedBboltFile(dbPath))
	originalSize := fileSize(t, dbPath)

	lfs := newTestStorage(
		defaultRecreateConfig(t, tempDir, true),
		zap.NewNop(),
		func(context.Context, string, time.Duration) error {
			return nil
		},
	)

	client, err := lfs.openClient(ctx, dbPath)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.Close(ctx))

	require.NoFileExists(t, findBackup(t, tempDir), "no backup should be created on clean precheck")
	require.Equal(t, originalSize, fileSize(t, dbPath))
}

func TestOpenClient_RecreateEnabled_PrecheckCorruption_RenamesFile(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.bbolt")

	require.NoError(t, seedBboltFile(dbPath))

	logCore, logObserver := observer.New(zap.WarnLevel)

	lfs := newTestStorage(
		defaultRecreateConfig(t, tempDir, true),
		zap.New(logCore),
		func(context.Context, string, time.Duration) error {
			return errDBCorruption
		},
	)

	client, err := lfs.openClient(ctx, dbPath)
	require.NoError(t, err)
	require.NotNil(t, client)
	t.Cleanup(func() { require.NoError(t, client.Close(ctx)) })

	backupPath := findBackup(t, tempDir)
	require.NotEmpty(t, backupPath, "corrupt database should be renamed")
	require.FileExists(t, backupPath)
	require.True(t, strings.HasSuffix(backupPath, ".backup"),
		"backup file must end with .backup suffix, got %s", backupPath)

	require.NotEmpty(t, logObserver.FilterMessageSnippet("Renamed corrupt bbolt database").All(),
		"expected log entry about renaming the corrupt database")
}

func TestOpenClient_RecreateEnabled_MainGoroutinePanic_Recovers(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.bbolt")

	require.NoError(t, seedBboltFile(dbPath))

	logCore, logObserver := observer.New(zap.WarnLevel)

	var calls int
	lfs := newTestStorage(
		defaultRecreateConfig(t, tempDir, true),
		zap.New(logCore),
		func(context.Context, string, time.Duration) error {
			return nil
		},
	)
	lfs.newClientFn = func(logger *zap.Logger, filePath string, timeout time.Duration, c *CompactionConfig, noSync bool) (*fileStorageClient, error) {
		calls++
		if calls == 1 {
			panic("simulated bbolt main-goroutine panic")
		}
		return newClient(logger, filePath, timeout, c, noSync)
	}

	client, err := lfs.openClient(ctx, dbPath)
	require.NoError(t, err, "openClient must swallow the panic and recover via rename + retry")
	require.NotNil(t, client)
	t.Cleanup(func() { require.NoError(t, client.Close(ctx)) })

	require.Equal(t, 2, calls, "newClientFn must be called twice: once panicking, once on the fresh file")
	require.NotEmpty(t, findBackup(t, tempDir),
		"main-goroutine panic must trigger rename of the corrupt file")
	require.NotEmpty(t,
		logObserver.FilterMessageSnippet("bbolt.Open panicked in main goroutine").All(),
		"expected log entry about the recovered panic")
	require.NotEmpty(t,
		logObserver.FilterMessageSnippet("Renamed corrupt bbolt database").All(),
		"expected log entry about the rename")
}

func TestOpenClient_RecreateEnabled_PrecheckTransientError_DoesNotRename(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.bbolt")

	require.NoError(t, seedBboltFile(dbPath))
	originalSize := fileSize(t, dbPath)

	logCore, logObserver := observer.New(zap.WarnLevel)

	lfs := newTestStorage(
		defaultRecreateConfig(t, tempDir, true),
		zap.New(logCore),
		func(context.Context, string, time.Duration) error {
			return os.ErrPermission
		},
	)

	client, err := lfs.openClient(ctx, dbPath)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.Close(ctx))

	require.NoFileExists(t, findBackup(t, tempDir),
		"transient errors must not trigger rename")
	require.Equal(t, originalSize, fileSize(t, dbPath))
	require.NotEmpty(t,
		logObserver.FilterMessage("Database pre-check returned non-corruption error; proceeding with open").All(),
		"expected log entry about transient precheck error")
}

func TestRunPrecheck_RealSubprocess_OnCleanDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "clean.bbolt")
	require.NoError(t, seedBboltFile(dbPath))

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	require.NoError(t, runPrecheck(ctx, dbPath, time.Second),
		"precheck on a healthy bbolt file should return nil")
}

func TestRunPrecheck_RealSubprocess_OnMissingFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	tempDir := t.TempDir()
	missingPath := filepath.Join(tempDir, "does-not-exist.bbolt")

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	// bbolt.Open creates the file when it does not exist, so this should succeed.
	require.NoError(t, runPrecheck(ctx, missingPath, time.Second))
}

func TestExtensionRecreate_FactoryWiresPrecheck(t *testing.T) {
	ctx := t.Context()
	f := NewFactory()

	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = t.TempDir()
	cfg.Recreate = true

	ext, err := f.Create(ctx, extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)

	lfs, ok := ext.(*localFileStorage)
	require.True(t, ok, "factory must return *localFileStorage")
	require.NotNil(t, lfs.precheckFn, "factory must wire a default precheckFn")
}

func TestExtensionRecreate_EndToEndWithInjectedPrecheck(t *testing.T) {
	ctx := t.Context()
	tempDir := t.TempDir()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir

	{
		ext, err := f.Create(ctx, extensiontest.NewNopSettings(f.Type()), cfg)
		require.NoError(t, err)

		lfs := ext.(*localFileStorage)
		lfs.precheckFn = func(context.Context, string, time.Duration) error { return nil }

		se := ext.(storage.Extension)
		client, err := se.GetClient(ctx, component.KindReceiver, component.MustNewID("filelog"), "")
		require.NoError(t, err)
		require.NoError(t, client.Set(ctx, "key", []byte("val")))
		require.NoError(t, client.Close(ctx))
		require.NoError(t, ext.Shutdown(ctx))
	}

	cfg.Recreate = true
	{
		ext, err := f.Create(ctx, extensiontest.NewNopSettings(f.Type()), cfg)
		require.NoError(t, err)

		lfs := ext.(*localFileStorage)
		lfs.precheckFn = func(context.Context, string, time.Duration) error {
			return errDBCorruption
		}

		se := ext.(storage.Extension)
		client, err := se.GetClient(ctx, component.KindReceiver, component.MustNewID("filelog"), "")
		require.NoError(t, err)

		val, err := client.Get(ctx, "key")
		require.NoError(t, err)
		require.Nil(t, val, "data should be gone after corruption-triggered recreate")

		require.NoError(t, client.Close(ctx))
		require.NoError(t, ext.Shutdown(ctx))
	}

	require.NotEmpty(t, findBackup(t, tempDir), "a .backup file must exist after corruption recovery")
}

func defaultRecreateConfig(t *testing.T, dir string, recreate bool) *Config {
	t.Helper()
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Directory = dir
	cfg.Recreate = recreate
	return cfg
}

func newTestStorage(cfg *Config, logger *zap.Logger, precheck func(context.Context, string, time.Duration) error) *localFileStorage {
	return &localFileStorage{
		cfg:         cfg,
		logger:      logger,
		precheckFn:  precheck,
		newClientFn: newClient,
	}
}

func seedBboltFile(path string) error {
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{
		Timeout:        time.Second,
		NoFreelistSync: true,
		FreelistType:   bbolt.FreelistMapType,
	})
	if err != nil {
		return err
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, e := tx.CreateBucketIfNotExists([]byte("default"))
		return e
	}); err != nil {
		_ = db.Close()
		return err
	}
	return db.Close()
}

func findBackup(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".backup") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size()
}
