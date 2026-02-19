// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package credentialsfile

import (
	"os"
	"path/filepath"
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
