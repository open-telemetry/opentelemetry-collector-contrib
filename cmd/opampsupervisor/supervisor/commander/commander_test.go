// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package commander

import (
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

// ignoreShutdownSignalFor is how long the child process in the
// "ignore-first-signal" mode ignores shutdown signals, long enough that Stop must
// re-send at least once for the child to ever act on one.
const ignoreShutdownSignalFor = 2500 * time.Millisecond

// childReadyLine is printed by the child once it is ignoring shutdown signals. The
// test waits for it so that the first signal is ignored rather than racing the
// child's startup, where it would instead terminate the process.
const childReadyLine = "ignoring shutdown signals"

func TestMain(m *testing.M) {
	switch os.Getenv(passthroughTestModeEnv) {
	case "passthrough":
		// Re-run this test binary as the child process so the test can assert
		// Commander can drain passthrough logs after process exit is observed.
		_, _ = fmt.Fprint(os.Stderr, "final error line")
		os.Exit(1)
	case "ignore-first-signal":
		// Take delivery of shutdown signals but do nothing with them for a while, the
		// way a Collector that has not finished starting up never acts on one. Note
		// this registers a handler rather than calling signal.Ignore: on Windows an
		// unhandled console control event terminates the process with
		// STATUS_CONTROL_C_EXIT instead of being ignored.
		ch := make(chan os.Signal, 8)
		signal.Notify(ch, os.Interrupt)
		_, _ = fmt.Fprintln(os.Stderr, childReadyLine)

		time.Sleep(ignoreShutdownSignalFor)

		// Discard whatever arrived during that window so that only a signal sent
		// afterwards - that is, a re-send - can end this process.
		for drained := false; !drained; {
			select {
			case <-ch:
			default:
				drained = true
			}
		}

		<-ch
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// An Agent that does not act on the first shutdown signal has to be sent another
// one. Without the re-send it survives until Stop gives up and kills it.
func TestStopResendsShutdownSignalUntilAgentExits(t *testing.T) {
	cmdr, err := NewCommander(
		zap.NewNop(),
		filepath.Join(t.TempDir(), "agent.log"),
		config.Agent{
			Executable:      os.Args[0],
			PassthroughLogs: true,
			Env: map[string]string{
				passthroughTestModeEnv: "ignore-first-signal",
			},
		},
	)
	require.NoError(t, err)

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
		t.Fatal("timed out waiting for the agent to start dropping shutdown signals")
	}

	start := time.Now()
	require.NoError(t, cmdr.Stop(t.Context()))
	elapsed := time.Since(start)

	// Without the re-send the only signal is the one that gets dropped, leaving the
	// agent to be killed once Stop's deadline expires.
	require.Equal(t, 0, cmdr.ExitCode(),
		"agent should have exited on a re-sent shutdown signal rather than being killed")
	require.Less(t, elapsed, 8*time.Second,
		"agent should have exited well before Stop's kill deadline")
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
