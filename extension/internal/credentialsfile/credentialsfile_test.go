// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package credentialsfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestNewValueResolver_Static(t *testing.T) {
	t.Parallel()
	r, err := NewValueResolver("myvalue", "", zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context()))
	assert.Equal(t, "myvalue", r.Value())
	require.NoError(t, r.Shutdown())
}

func TestNewValueResolver_EmptyBoth(t *testing.T) {
	t.Parallel()
	_, err := NewValueResolver("", "", zap.NewNop())
	require.ErrorIs(t, err, errNoValueProvided)
}

func TestNewValueResolver_FilePreferredOverInline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(f, []byte("fromfile"), 0o600))

	r, err := NewValueResolver("inline", f, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context()))
	defer func() { require.NoError(t, r.Shutdown()) }()

	assert.Equal(t, "fromfile", r.Value())
}

func TestFileWatcher_StartFailsMissingFile(t *testing.T) {
	t.Parallel()
	r, err := NewValueResolver("", "/nonexistent/path/secret", zaptest.NewLogger(t))
	require.NoError(t, err)
	require.Error(t, r.Start(t.Context()))
}

func TestFileWatcher_StartFailsEmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(f, []byte("  \n  "), 0o600))

	r, err := NewValueResolver("", f, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.Error(t, r.Start(t.Context()))
}

func TestFileWatcher_TrimsWhitespace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(f, []byte("  mytoken\n"), 0o600))

	r, err := NewValueResolver("", f, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context()))
	defer func() { require.NoError(t, r.Shutdown()) }()

	assert.Equal(t, "mytoken", r.Value())
}

func TestFileWatcher_UpdatesOnWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(f, []byte("original"), 0o600))

	r, err := NewValueResolver("", f, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context()))
	defer func() { require.NoError(t, r.Shutdown()) }()

	assert.Equal(t, "original", r.Value())

	require.NoError(t, os.WriteFile(f, []byte("updated"), 0o600))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "updated", r.Value())
	}, 5*time.Second, 50*time.Millisecond)
}

func TestFileWatcher_UpdatesOnAtomicReplace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(f, []byte("original"), 0o600))

	r, err := NewValueResolver("", f, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context()))
	defer func() { require.NoError(t, r.Shutdown()) }()

	// Simulate atomic replacement: write to temp file, then rename
	tmp := filepath.Join(dir, "secret.tmp")
	require.NoError(t, os.WriteFile(tmp, []byte("replaced"), 0o600))
	require.NoError(t, os.Rename(tmp, f))

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "replaced", r.Value())
	}, 5*time.Second, 50*time.Millisecond)
}

func TestFileWatcher_KeepsLastValueOnDelete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(f, []byte("keepme"), 0o600))

	r, err := NewValueResolver("", f, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context()))
	defer func() { require.NoError(t, r.Shutdown()) }()

	require.NoError(t, os.Remove(f))
	// Give the watcher time to process the event
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, "keepme", r.Value())
}

// TestFileWatcher_UpdatesOnKubernetesSymlinkSwap verifies that the watcher
// picks up changes when the backing file is rotated using the Kubernetes-style
// atomic symlink swap used for projected volumes (ConfigMaps, Secrets,
// serviceAccountToken).
//
// Layout:
//
//	mountDir/
//	  .data -> .ts_1   (symlink to timestamped dir)
//	  .ts_1/
//	    secret          (actual file)
//	  secret -> .data/secret  (symlink through .data)
//
// On rotation the kubelet creates a new timestamped dir, writes new content,
// atomically swaps .data via rename, then removes the old timestamped dir.
func TestFileWatcher_UpdatesOnKubernetesSymlinkSwap(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Atomic symlink swaps with rename(2) are not supported on Windows")
	}

	mountDir := t.TempDir()

	// Initial timestamped directory with secret file
	ts1Dir := filepath.Join(mountDir, ".ts_1")
	require.NoError(t, os.Mkdir(ts1Dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ts1Dir, "secret"), []byte("original"), 0o600))

	// .data -> .ts_1
	require.NoError(t, os.Symlink(".ts_1", filepath.Join(mountDir, ".data")))
	// secret -> .data/secret
	secretFile := filepath.Join(mountDir, "secret")
	require.NoError(t, os.Symlink(filepath.Join(".data", "secret"), secretFile))

	// Sanity check
	got, err := os.ReadFile(secretFile)
	require.NoError(t, err)
	require.Equal(t, "original", string(got))

	r, err := NewValueResolver("", secretFile, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context()))
	defer func() { require.NoError(t, r.Shutdown()) }()

	assert.Equal(t, "original", r.Value())

	// Simulate Kubernetes rotation: new timestamped dir, atomic .data swap, old dir removal
	ts2Dir := filepath.Join(mountDir, ".ts_2")
	require.NoError(t, os.Mkdir(ts2Dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ts2Dir, "secret"), []byte("rotated"), 0o600))

	tmpLink := filepath.Join(mountDir, ".data_tmp")
	require.NoError(t, os.Symlink(".ts_2", tmpLink))
	require.NoError(t, os.Rename(tmpLink, filepath.Join(mountDir, ".data")))
	require.NoError(t, os.RemoveAll(ts1Dir))

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "rotated", r.Value())
	}, 5*time.Second, 50*time.Millisecond, "value was not refreshed after symlink rotation")
}
