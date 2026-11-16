// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestDuplicateUnmatchedFileClosesDiscardedHandle(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "app.log")
	duplicatePath := filepath.Join(dir, "app.log.1")
	for _, path := range []string{firstPath, duplicatePath} {
		require.NoError(t, os.WriteFile(path, []byte("same log content\n"), 0o600))
	}

	first, err := os.Open(firstPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Close() })
	duplicate, err := os.Open(duplicatePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = duplicate.Close() })

	tr := NewFileTracker(t.Context(), componenttest.NewNopTelemetrySettings(), 10, 0, testutil.NewUnscopedMockPersister())
	firstFP, err := fingerprint.NewFromFile(first, fingerprint.DefaultSize, false, zap.NewNop())
	require.NoError(t, err)
	duplicateFP, err := fingerprint.NewFromFile(duplicate, fingerprint.DefaultSize, false, zap.NewNop())
	require.NoError(t, err)
	tr.AddUnmatched(first, firstFP)
	tr.AddUnmatched(duplicate, duplicateFP)

	files, _, _ := tr.LookupArchive(t.Context())
	require.Len(t, files, 1, "duplicate fingerprint should be discarded")
	require.Same(t, first, files[0])
	_, err = first.ReadAt(make([]byte, 1), 0)
	require.NoError(t, err, "retained file handle should remain open")
	_, err = duplicate.ReadAt(make([]byte, 1), 0)
	require.True(t, errors.Is(err, os.ErrClosed), "discarded file handle remains open: %v", err)
}
