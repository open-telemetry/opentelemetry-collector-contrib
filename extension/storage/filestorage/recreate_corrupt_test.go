// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage

import (
	"encoding/binary"
	"fmt"
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
)

// TestRecreateRecoversFreepagesPanic opens a genuinely corrupt database whose
// page tree contains a duplicated child reference, the condition that makes
// bbolt raise "freepages: failed to get all reachable pages ... multiple
// references". Before bbolt v1.5.0 that panic was raised on a bbolt-spawned
// goroutine and killed the collector regardless of the recreate option.
func TestRecreateRecoversFreepagesPanic(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = dir
	cfg.Recreate = true

	dbPath := filepath.Join(dir, "receiver_file_log_")
	seedCorruptDatabase(t, dbPath)

	ext, err := f.Create(ctx, extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	se, ok := ext.(storage.Extension)
	require.True(t, ok)

	client, err := se.GetClient(ctx, component.KindReceiver, component.MustNewID("file_log"), "")
	require.NoError(t, err)
	require.NotNil(t, client)

	// The fresh database must be usable.
	require.NoError(t, client.Set(ctx, "key", []byte("val")))
	val, err := client.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, []byte("val"), val)
	require.NoError(t, client.Close(ctx))
	require.NoError(t, ext.Shutdown(ctx))

	// The corrupt file must be preserved next to the new one, not deleted.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var backups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".backup") {
			backups = append(backups, e.Name())
		}
	}
	require.Len(t, backups, 1, "corrupt database should be renamed aside, got %v", entries)
}

const bboltPageSize = 4096

// seedCorruptDatabase writes a bbolt database large enough to have a branch
// page, then points two of that page's entries at the same child page so
// bbolt's reachability walk sees a page referenced twice.
func seedCorruptDatabase(t *testing.T, path string) {
	t.Helper()

	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Second, NoFreelistSync: true})
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
		for i := range 8 {
			b, bErr := tx.CreateBucketIfNotExists([]byte(fmt.Sprintf("bucket%d", i)))
			if bErr != nil {
				return bErr
			}
			for j := range 500 {
				if pErr := b.Put([]byte(fmt.Sprintf("k%04d", j)), make([]byte, 200)); pErr != nil {
					return pErr
				}
			}
		}
		return nil
	}))
	require.NoError(t, db.Close())

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	// Page header: id(8) flags(2) count(2) overflow(4); branch elements follow
	// as pos(4) ksize(4) pgid(8).
	const (
		branchPageFlag  = 0x01
		flagsOffset     = 8
		countOffset     = 10
		elementsOffset  = 16
		elementSize     = 16
		elementPgidOffs = 8
	)
	for off := 0; off+bboltPageSize <= len(raw); off += bboltPageSize {
		if binary.LittleEndian.Uint16(raw[off+flagsOffset:]) != branchPageFlag ||
			binary.LittleEndian.Uint16(raw[off+countOffset:]) < 2 {
			continue
		}
		first := off + elementsOffset
		second := first + elementSize
		child := binary.LittleEndian.Uint64(raw[first+elementPgidOffs:])
		binary.LittleEndian.PutUint64(raw[second+elementPgidOffs:], child)
		require.NoError(t, os.WriteFile(path, raw, 0o600))
		return
	}
	t.Fatal("no branch page found in seeded database; increase the seeded data size")
}
