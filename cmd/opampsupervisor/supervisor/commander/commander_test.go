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
