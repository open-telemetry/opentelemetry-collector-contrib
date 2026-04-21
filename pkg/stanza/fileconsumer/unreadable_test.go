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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

// TestUnreadableFileLoggedOnce verifies that permission-denied errors when
// opening files are logged only once per file per process run, and that an
// informational message is emitted when the file later becomes readable.
func TestUnreadableFileLoggedOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission manipulation tests are not reliable on Windows")
	}

	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	operator, _ := testManager(t, cfg)

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

	operator.set.Logger = set.Logger
	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	// First poll should attempt to open and log an error once
	operator.poll(t.Context())

	// Verify the unreadable map recorded the path and exactly one error was logged
	require.Eventually(t, func() bool {
		return len(operator.unreadable) == 1
	}, 2*time.Second, 10*time.Millisecond, "expected unreadable map to have one entry after first poll")

	require.Eventually(t, func() bool {
		countErrMsgs := 0
		for _, e := range obs.All() {
			if e.Level == zapcore.ErrorLevel && e.Message == "Failed to open file - unreadable" {
				countErrMsgs++
			}
		}
		return countErrMsgs == 1
	}, 2*time.Second, 10*time.Millisecond, "expected exactly one 'Failed to open file - unreadable' error after first poll")

	// Second poll should not add another error-level log for the same path
	operator.poll(t.Context())

	require.Eventually(t, func() bool {
		countErrMsgs := 0
		for _, e := range obs.All() {
			if e.Level == zapcore.ErrorLevel && e.Message == "Failed to open file - unreadable" {
				countErrMsgs++
			}
		}
		return countErrMsgs == 1
	}, 2*time.Second, 10*time.Millisecond, "expected still exactly one 'Failed to open file - unreadable' error after second poll")

	// Verify the unreadable map still contains the entry (no reinitialization)
	require.Len(t, operator.unreadable, 1, "expected unreadable map to still have one entry after second poll")

	// Now make the file readable again and poll; should emit an info message
	require.NoError(t, os.Chmod(f.Name(), 0o644))
	operator.poll(t.Context())

	require.Eventually(t, func() bool {
		for _, e := range obs.All() {
			if e.Level == zapcore.InfoLevel && e.Message == "Previously unreadable file is now readable" {
				return true
			}
		}
		return false
	}, 2*time.Second, 10*time.Millisecond, "expected at least one info message when file becomes readable")
}

// TestNonPermissionErrorsAlwaysLogged verifies that errors other than permission
// errors (e.g., file not found) are always logged and not suppressed.
func TestNonPermissionErrorsAlwaysLogged(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	operator, _ := testManager(t, cfg)

	core, obs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = logger

	operator.set.Logger = set.Logger
	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	// Directly call makeFingerprint with a non-existent file path
	// This simulates a file being deleted after matching but before opening
	nonExistentPath := tempDir + "/non_existent_file.log"

	// First call should log an error
	fp, file := operator.makeFingerprint(nonExistentPath)
	require.Nil(t, fp)
	require.Nil(t, file)

	require.Eventually(t, func() bool {
		for _, e := range obs.All() {
			if e.Level == zapcore.ErrorLevel && e.Message == "Failed to open file" {
				return true
			}
		}
		return false
	}, 2*time.Second, 10*time.Millisecond, "expected an error log on first call for non-existent file")

	countBeforeSecondCall := 0
	for _, e := range obs.All() {
		if e.Level == zapcore.ErrorLevel && e.Message == "Failed to open file" {
			countBeforeSecondCall++
		}
	}

	// Second call should also log an error (not suppressed like permission errors)
	fp, file = operator.makeFingerprint(nonExistentPath)
	require.Nil(t, fp)
	require.Nil(t, file)

	require.Eventually(t, func() bool {
		countAfterSecondCall := 0
		for _, e := range obs.All() {
			if e.Level == zapcore.ErrorLevel && e.Message == "Failed to open file" {
				countAfterSecondCall++
			}
		}
		// Should have at least one more error than before
		return countAfterSecondCall > countBeforeSecondCall
	}, 2*time.Second, 10*time.Millisecond, "expected another error log on second call for non-permission errors")

	// Verify the unreadable map is empty (non-permission errors shouldn't be tracked)
	require.Empty(t, operator.unreadable, "expected unreadable map to be empty for non-permission errors")
}
