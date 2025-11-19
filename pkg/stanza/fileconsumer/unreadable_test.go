// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

// Verifies that permission-denied errors when opening files are logged only
// once per file per process run, and that an informational message is emitted
// when the file later becomes readable.
func TestUnreadableFileLoggedOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission manipulation tests are not reliable on Windows")
	}

	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)

	// Create a file and remove permissions so open will fail
	f := filetest.OpenTemp(t, tempDir)
	_, err := f.WriteString("abc\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, os.Chmod(f.Name(), 0))

	core, obs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = logger

	sink := emittest.NewSink()
	mgr, err := cfg.Build(set, sink.Callback)
	require.NoError(t, err)
	mgr.tracker = tracker.NewFileTracker(t.Context(), set, cfg.MaxBatches, cfg.PollsToArchive, testutil.NewUnscopedMockPersister())
	t.Cleanup(func() { mgr.tracker.ClosePreviousFiles() })

	// First poll should attempt to open and log an error once
	mgr.poll(t.Context())
	// small delay to ensure logs are recorded
	time.Sleep(10 * time.Millisecond)

	// Count error messages with the exact message
	countErrMsgs := 0
	for _, e := range obs.All() {
		if e.Level == zapcore.ErrorLevel && e.Message == "Failed to open file" {
			countErrMsgs++
		}
	}
	require.Equal(t, 1, countErrMsgs, "expected exactly one 'Failed to open file' error after first poll")

	// Second poll should not add another error-level log for the same path
	mgr.poll(t.Context())
	time.Sleep(10 * time.Millisecond)
	countErrMsgs2 := 0
	for _, e := range obs.All() {
		if e.Level == zapcore.ErrorLevel && e.Message == "Failed to open file" {
			countErrMsgs2++
		}
	}
	require.Equal(t, 1, countErrMsgs2, "expected still exactly one 'Failed to open file' error after second poll")

	// Now make the file readable again and poll; should emit an info message
	require.NoError(t, os.Chmod(f.Name(), 0o644))
	mgr.poll(t.Context())
	time.Sleep(10 * time.Millisecond)

	infoCount := 0
	for _, e := range obs.All() {
		if e.Level == zapcore.InfoLevel && e.Message == "Previously unreadable file is now readable" {
			infoCount++
		}
	}
	require.GreaterOrEqual(t, infoCount, 1, "expected at least one info message when file becomes readable")
}
