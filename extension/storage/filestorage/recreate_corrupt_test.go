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

// bboltPageSize is pinned so the synthetic corruption is byte-identical on
// hosts whose default page size differs (macOS uses 16 KiB, Linux 4 KiB).
const bboltPageSize = 4096

// seedCorruptDatabase writes a synthetic database, then points a sub-bucket's
// root page at the database's own root page. bbolt's reachability walk then
// visits that page twice and reports "page N: multiple references
// (stack: [N])", the exact shape observed on the host in #35899.
func seedCorruptDatabase(t *testing.T, path string) {
	t.Helper()

	db, err := bbolt.Open(path, 0o600, &bbolt.Options{
		Timeout:        time.Second,
		PageSize:       bboltPageSize,
		NoFreelistSync: true,
		FreelistType:   bbolt.FreelistMapType,
	})
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
		// Enough entries that the buckets are not stored inline, so the root
		// page holds real bucket headers to redirect.
		for _, name := range []string{"default", "filler"} {
			b, bErr := tx.CreateBucketIfNotExists([]byte(name))
			if bErr != nil {
				return bErr
			}
			for i := range 400 {
				if pErr := b.Put([]byte(fmt.Sprintf("synthetic-key-%05d", i)), []byte("synthetic-value")); pErr != nil {
					return pErr
				}
			}
		}
		return nil
	}))
	require.NoError(t, db.Close())

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	const (
		metaOffset     = 16 // page header length
		metaRootPgid   = metaOffset + 16
		metaTxid       = metaOffset + 48
		leafPageFlag   = 0x02
		bucketLeafFlag = 0x01
		flagsOffset    = 8
		countOffset    = 10
		elementsOffset = 16
		elementSize    = 16 // leaf element: flags(4) pos(4) ksize(4) vsize(4)
		bucketHdrSize  = 16 // bucket header: root pgid(8) sequence(8)
	)
	le := binary.LittleEndian

	// The meta page with the higher transaction id is the active one.
	root, txid := le.Uint64(raw[metaRootPgid:]), le.Uint64(raw[metaTxid:])
	if alt := le.Uint64(raw[bboltPageSize+metaTxid:]); alt > txid {
		root = le.Uint64(raw[bboltPageSize+metaRootPgid:])
	}

	page := int(root) * bboltPageSize
	require.Equal(t, uint16(leafPageFlag), le.Uint16(raw[page+flagsOffset:]),
		"root page should be a leaf holding bucket headers")

	count := int(le.Uint16(raw[page+countOffset:]))
	for i := range count {
		elem := page + elementsOffset + elementSize*i
		if le.Uint32(raw[elem:]) != bucketLeafFlag || le.Uint32(raw[elem+12:]) < bucketHdrSize {
			continue
		}
		value := elem + int(le.Uint32(raw[elem+4:])) + int(le.Uint32(raw[elem+8:]))
		if le.Uint64(raw[value:]) == 0 {
			continue // inline bucket, no root page to redirect
		}
		le.PutUint64(raw[value:], root)
		require.NoError(t, os.WriteFile(path, raw, 0o600))
		return
	}
	t.Fatal("no non-inline bucket found in seeded database; increase the seeded data size")
}
