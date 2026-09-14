// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package commander

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

const passthroughTestModeEnv = "OTEL_SUPERVISOR_COMMANDER_TEST_MODE" // #nosec G101 -- Test mode selector, not a credential.

// childReadyLine is printed by the child once it is ignoring shutdown signals. The
// test waits for it so that the signal is not sent before the child has registered
// its handler, which would instead terminate the process.
const childReadyLine = "ignoring shutdown signals"

// logAppendContinueFileEnv names the env var that tells the "log-append-after-truncate"
// child where to look for the marker file signaling it to write its second line. Passed
// as an env var, like passthroughTestModeEnv, because the child is a re-exec of this same
// test binary and has no other channel to receive it over.
const logAppendContinueFileEnv = "OTEL_SUPERVISOR_COMMANDER_TEST_CONTINUE_FILE"

func TestMain(m *testing.M) {
	switch os.Getenv(passthroughTestModeEnv) {
	case "passthrough":
		// Re-run this test binary as the child process so the test can assert
		// Commander can drain passthrough logs after process exit is observed.
		_, _ = fmt.Fprint(os.Stderr, "final error line")
		os.Exit(1)
	case "ignore-signals-forever":
		// Swallow shutdown signals for good, so the process can only be terminated
		// forcibly. Used to prove Stop kills a process whose graceful shutdown
		// cannot succeed. Note this registers a handler rather than calling
		// signal.Ignore: on Windows an unhandled console control event terminates
		// the process with STATUS_CONTROL_C_EXIT instead of being ignored.
		ch := make(chan os.Signal, 8)
		signal.Notify(ch, os.Interrupt)
		_, _ = fmt.Fprintln(os.Stderr, childReadyLine)
		for {
			<-ch
		}
	case "ignore-shutdown-signal":
		// Ignore the graceful shutdown signal so Stop has to fall back to killing
		// the process forcibly. The ready line lets the parent wait until the signal
		// is actually being ignored before it asks the process to stop.
		signal.Ignore(os.Interrupt)
		_, _ = fmt.Fprintln(os.Stderr, "ready")
		time.Sleep(time.Minute)
		os.Exit(0)
	case "log-append-after-truncate":
		// Writes a line, waits for the parent to truncate the log file out from
		// under it and signal via a marker file, then writes a second line. This
		// exercises the file handle Commander.startNormal actually hands to a real
		// child through exec - as opposed to TestOpenAgentLogFileAppendsAfterExternalTruncate,
		// which only writes through the parent's own *os.File and so would not catch
		// a regression to Windows' handle-inheritance behavior.
		_, _ = fmt.Fprint(os.Stdout, "before rotation\n")
		continueFile := os.Getenv(logAppendContinueFileEnv)
		if continueFile == "" {
			os.Exit(1)
		}
		for {
			if _, err := os.Stat(continueFile); err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		_, _ = fmt.Fprint(os.Stdout, "after rotation\n")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// Stop must terminate the Agent even when graceful shutdown cannot succeed, and
// even when the caller's context is already cancelled. An error from Stop means
// the process could not be killed - not that the graceful path failed.
func TestStopKillsAgentThatIgnoresShutdownSignals(t *testing.T) {
	cmdr, err := NewCommander(
		zap.NewNop(),
		filepath.Join(t.TempDir(), "agent.log"),
		config.Agent{
			Executable:      os.Args[0],
			PassthroughLogs: true,
			Env: map[string]string{
				passthroughTestModeEnv: "ignore-signals-forever",
			},
		},
	)
	require.NoError(t, err)
	cmdr.stopGracePeriod = 2 * time.Second

	ready := make(chan struct{})
	var once sync.Once
	cmdr.SetPassthroughLogHook(func(line string) {
		if strings.Contains(line, childReadyLine) {
			once.Do(func() { close(ready) })
		}
	})

	require.NoError(t, cmdr.Start(t.Context()))

	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the agent to start ignoring shutdown signals")
	}

	// A cancelled caller context must not skip the kill.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	stopDone := make(chan error, 1)
	go func() { stopDone <- cmdr.Stop(ctx) }()

	select {
	case err := <-stopDone:
		require.NoError(t, err,
			"Stop should kill the unresponsive agent and report success")
	case <-time.After(8 * time.Second):
		t.Fatal("Stop did not terminate an agent that ignores shutdown signals")
	}
	require.False(t, cmdr.IsRunning())
}

// Concurrent Stop calls must all return: the process exit is announced only
// once, so without serialization one caller would consume it and the other
// would wait forever.
func TestStopCalledConcurrentlyBothReturn(t *testing.T) {
	cmdr, err := NewCommander(
		zap.NewNop(),
		filepath.Join(t.TempDir(), "agent.log"),
		config.Agent{
			Executable:      os.Args[0],
			PassthroughLogs: true,
			Env: map[string]string{
				passthroughTestModeEnv: "ignore-signals-forever",
			},
		},
	)
	require.NoError(t, err)
	cmdr.stopGracePeriod = 2 * time.Second

	ready := make(chan struct{})
	var once sync.Once
	cmdr.SetPassthroughLogHook(func(line string) {
		if strings.Contains(line, childReadyLine) {
			once.Do(func() { close(ready) })
		}
	})

	require.NoError(t, cmdr.Start(t.Context()))

	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the agent to start ignoring shutdown signals")
	}

	stopDone := make(chan error, 2)
	go func() { stopDone <- cmdr.Stop(t.Context()) }()
	go func() { stopDone <- cmdr.Stop(t.Context()) }()

	for range 2 {
		select {
		case err := <-stopDone:
			require.NoError(t, err)
		case <-time.After(8 * time.Second):
			t.Fatal("a concurrent Stop call never returned")
		}
	}
	require.False(t, cmdr.IsRunning())
}

func TestWaitForOutputDrainCapturesFinalPassthroughLine(t *testing.T) {
	cmdr, err := NewCommander(
		zap.NewNop(),
		filepath.Join(t.TempDir(), "agent.log"),
		config.Agent{
			Executable:      os.Args[0],
			PassthroughLogs: true,
			Env: map[string]string{
				passthroughTestModeEnv: "passthrough",
			},
		},
	)
	require.NoError(t, err)

	var mu sync.Mutex
	var lines []string
	cmdr.SetPassthroughLogHook(func(line string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, line)
	})

	require.NoError(t, cmdr.Start(t.Context()))

	select {
	case <-cmdr.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for commander exit")
	}
	require.True(t, cmdr.WaitForOutputDrain(5*time.Second))

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"final error line"}, lines)
}

func TestOpenAgentLogFileAppendsAfterExternalTruncate(t *testing.T) {
	// Regression test for the pre-fix behavior: opening agent.log with a plain
	// os.Create/os.OpenFile keeps writes anchored at the offset the file had
	// when it was opened. If an external tool truncates the file for
	// copytruncate-style rotation (e.g. the default policy on BOSH stemcells),
	// the next write from the still-open handle used to land at that stale
	// offset, padding the gap with NUL bytes and making the file's size snap
	// right back up instead of staying rotated. openAgentLogFile must instead
	// force every write to the file's current end-of-file, so that after an
	// external truncate the next write starts the file clean.
	path := filepath.Join(t.TempDir(), "agent.log")

	f, err := openAgentLogFile(path)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.WriteString("before rotation\n")
	require.NoError(t, err)

	// Simulate an external copytruncate-style rotation: some other process
	// truncates the file in place while our handle stays open.
	require.NoError(t, os.Truncate(path, 0))

	_, err = f.WriteString("after rotation\n")
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "after rotation\n", string(got), "write after external truncate should land at the new end-of-file, not the stale pre-truncate offset")
}

// TestStartNormalChildWritesAfterExternalTruncate goes through startNormal with a real
// child process instead of writing through openAgentLogFile's own *os.File. This matters
// on Windows, where Go's os.O_APPEND is emulated in the opening process and does not
// survive handle inheritance: a regression that dropped the FILE_APPEND_DATA-only handle
// back to a plain append-mode handle would still pass TestOpenAgentLogFileAppendsAfterExternalTruncate,
// but would fail here because the child writes through the inherited handle at its own
// stale offset.
func TestStartNormalChildWritesAfterExternalTruncate(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent.log")
	continueFile := filepath.Join(t.TempDir(), "continue")

	cmdr, err := NewCommander(
		zap.NewNop(),
		logPath,
		config.Agent{
			Executable: os.Args[0],
			Env: map[string]string{
				passthroughTestModeEnv:   "log-append-after-truncate",
				logAppendContinueFileEnv: continueFile,
			},
		},
	)
	require.NoError(t, err)

	require.NoError(t, cmdr.Start(t.Context()))
	defer cmdr.Stop(t.Context()) //nolint:errcheck

	require.Eventually(t, func() bool {
		got, readErr := os.ReadFile(logPath)
		return readErr == nil && strings.Contains(string(got), "before rotation")
	}, 5*time.Second, 20*time.Millisecond, "child never wrote its first line")

	// Simulate an external copytruncate-style rotation while the child is still alive
	// and holding its inherited handle open.
	require.NoError(t, os.Truncate(logPath, 0))

	// Let the child proceed to its second write, now that the file has been rotated
	// out from under it.
	require.NoError(t, os.WriteFile(continueFile, nil, 0o600))

	select {
	case <-cmdr.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for child process to exit")
	}

	got, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.Equal(t, "after rotation\n", string(got),
		"child's write after external truncate should land at the new end-of-file through its inherited handle, not the stale pre-truncate offset")
}

func TestStopKillsUnresponsiveProcess(t *testing.T) {
	cmdr, err := NewCommander(
		zap.NewNop(),
		filepath.Join(t.TempDir(), "agent.log"),
		config.Agent{
			Executable:      os.Args[0],
			PassthroughLogs: true,
			Env: map[string]string{
				passthroughTestModeEnv: "ignore-shutdown-signal",
			},
		},
	)
	require.NoError(t, err)
	cmdr.stopGracePeriod = 100 * time.Millisecond

	ready := make(chan struct{})
	var readyOnce sync.Once
	cmdr.SetPassthroughLogHook(func(line string) {
		if line == "ready" {
			readyOnce.Do(func() { close(ready) })
		}
	})

	require.NoError(t, cmdr.Start(t.Context()))
	require.True(t, cmdr.IsRunning())

	select {
	case <-ready:
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for agent process to ignore the shutdown signal")
	}

	require.NoError(t, cmdr.Stop(t.Context()))
	require.False(t, cmdr.IsRunning())
}
