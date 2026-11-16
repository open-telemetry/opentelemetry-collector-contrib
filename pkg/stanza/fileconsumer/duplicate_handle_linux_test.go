// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package fileconsumer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestDuplicateUnmatchedFilesDoNotGrowOpenFDs(t *testing.T) {
	// Keep finalizers from masking handles discarded without an explicit Close.
	previousGCPercent := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previousGCPercent)

	dir := t.TempDir()
	cfg := NewConfig().includeDir(dir)
	cfg.StartAt = "beginning"
	cfg.PollInterval = time.Hour // Poll synchronously below.
	manager, _ := testManager(t, cfg)
	require.NoError(t, manager.Start(testutil.NewUnscopedMockPersister()))
	stopped := false
	defer func() {
		if !stopped {
			require.NoError(t, manager.Stop())
		}
	}()

	before, err := countOpenFilesInDir(dir)
	if os.IsNotExist(err) {
		t.Skip("/proc/self/fd is unavailable")
	}
	require.NoError(t, err)
	probe, err := os.CreateTemp(dir, "fd-probe-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = probe.Close() })
	withProbe, err := countOpenFilesInDir(dir)
	require.NoError(t, err)
	require.Equal(t, before+1, withProbe, "FD inspection must detect an open test file")
	require.NoError(t, probe.Close())
	require.NoError(t, os.Remove(probe.Name()))
	for i := range 12 {
		content := []byte(fmt.Sprintf("unique-pair-%02d\n", i))
		for _, suffix := range []string{".log", ".log.1"} {
			path := filepath.Join(dir, fmt.Sprintf("app-%02d%s", i, suffix))
			require.NoError(t, os.WriteFile(path, content, 0o600))
		}
		manager.poll(t.Context())
	}

	// Stop releases retained readers. Rejected duplicates must already be closed.
	require.NoError(t, manager.Stop())
	stopped = true
	after, err := countOpenFilesInDir(dir)
	require.NoError(t, err)
	t.Logf("open test-file descriptors after twelve duplicate pairs: before=%d after=%d", before, after)
	require.Equal(t, before, after, "duplicate files left descriptors open across polls")
}

func countOpenFilesInDir(dir string) (int, error) {
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if os.IsNotExist(err) { // A descriptor can close between ReadDir and Readlink.
			continue
		}
		if err != nil {
			return 0, err
		}
		if strings.HasPrefix(target, resolvedDir+string(os.PathSeparator)) {
			count++
		}
	}
	return count, nil
}
