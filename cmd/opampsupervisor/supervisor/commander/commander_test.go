// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package commander

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

const passthroughTestModeEnv = "OTEL_SUPERVISOR_COMMANDER_TEST_MODE" // #nosec G101 -- Test mode selector, not a credential.

func TestMain(m *testing.M) {
	switch os.Getenv(passthroughTestModeEnv) {
	case "passthrough":
		// Re-run this test binary as the child process so the test can assert
		// Commander can drain passthrough logs after process exit is observed.
		_, _ = fmt.Fprint(os.Stderr, "final error line")
		os.Exit(1)
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
	prevGracePeriod := stopGracePeriod
	stopGracePeriod = 100 * time.Millisecond
	t.Cleanup(func() { stopGracePeriod = prevGracePeriod })

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
