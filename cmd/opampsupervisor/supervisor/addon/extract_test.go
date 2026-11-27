// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tarEntry is one entry to place in a test archive.
type tarEntry struct {
	name     string
	body     string
	mode     int64
	typeflag byte
	linkname string
}

// buildTarGz builds a gzipped tarball from entries.
func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, e := range entries {
		typeflag := e.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		mode := e.mode
		if mode == 0 {
			mode = 0o600
		}
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     e.name,
			Mode:     mode,
			Size:     int64(len(e.body)),
			Typeflag: typeflag,
			Linkname: e.linkname,
		}))
		if typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()

	pkg := buildTarGz(t, []tarEntry{
		{name: "README", body: "geo data\n"},
		{name: "data/geo.mmdb", body: "binary-ish"},
		{name: "run.sh", body: "#!/bin/sh\n", mode: 0o755},
	})

	require.NoError(t, ExtractTarGz(pkg, dir))

	readme, err := os.ReadFile(filepath.Join(dir, "README"))
	require.NoError(t, err)
	assert.Equal(t, "geo data\n", string(readme))

	body, err := os.ReadFile(filepath.Join(dir, "data", "geo.mmdb"))
	require.NoError(t, err)
	assert.Equal(t, "binary-ish", string(body))

	// The executable bit must survive extraction or an entrypoint cannot run.
	info, err := os.Stat(filepath.Join(dir, "run.sh"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode().Perm()&0o100, "executable bit should be preserved")

	// Non-executable entries must not become executable.
	info, err = os.Stat(filepath.Join(dir, "data", "geo.mmdb"))
	require.NoError(t, err)
	assert.Zero(t, info.Mode().Perm()&0o100, "data file should not be executable")
}

// TestExtractTarGzRejectsHostileArchives covers the package-escape cases. A
// package that tries to write outside its directory is rejected outright
// rather than partially extracted, so a hostile package cannot land some of
// its files and then fail.
func TestExtractTarGzRejectsHostileArchives(t *testing.T) {
	tests := []struct {
		name          string
		entries       []tarEntry
		expectedError string
	}{
		{
			name:          "parent traversal",
			entries:       []tarEntry{{name: "../escaped", body: "x"}},
			expectedError: "escapes the addon directory",
		},
		{
			name:          "nested parent traversal",
			entries:       []tarEntry{{name: "a/b/../../../escaped", body: "x"}},
			expectedError: "escapes the addon directory",
		},
		{
			name:          "absolute path",
			entries:       []tarEntry{{name: "/etc/passwd", body: "x"}},
			expectedError: "must not be an absolute path",
		},
		{
			name:          "symlink",
			entries:       []tarEntry{{name: "link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"}},
			expectedError: "unsupported type",
		},
		{
			name:          "hardlink",
			entries:       []tarEntry{{name: "link", typeflag: tar.TypeLink, linkname: "/etc/passwd"}},
			expectedError: "unsupported type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			err := ExtractTarGz(buildTarGz(t, tt.entries), dir)
			require.ErrorContains(t, err, tt.expectedError)

			// Nothing from a rejected package should be left behind outside it.
			_, statErr := os.Stat(filepath.Join(filepath.Dir(dir), "escaped"))
			assert.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

func TestExtractTarGzRejectsTooManyFiles(t *testing.T) {
	entries := make([]tarEntry, 0, maxAddonFiles+1)
	for i := 0; i <= maxAddonFiles; i++ {
		entries = append(entries, tarEntry{name: filepath.Join("f", string(rune('a'+i%26)), "x"), body: "y"})
	}

	err := ExtractTarGz(buildTarGz(t, entries), t.TempDir())
	require.ErrorContains(t, err, "more than")
}

func TestExtractTarGzRejectsNonGzip(t *testing.T) {
	err := ExtractTarGz([]byte("not a gzip stream"), t.TempDir())
	require.ErrorContains(t, err, "create gzip reader")
}
