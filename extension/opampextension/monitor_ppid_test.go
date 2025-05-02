// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func TestMonitorPPID(t *testing.T) {
	t.Run("Does not trigger if process with ppid never stops", func(t *testing.T) {
		t.Parallel()

		cmdContext, cmdCancel := context.WithCancel(context.Background())
		cmd := longRunningCommand(cmdContext)
		cmd.Stdout = os.Stdout
		require.NoError(t, cmd.Start())
		cmdPid := cmd.Process.Pid

		t.Cleanup(func() {
			cmdCancel()
			_ = cmd.Wait()
		})

		statusReportFunc := func(se *componentstatus.Event) {
			t.Logf("Status event error: %s", se.Err())
			require.FailNow(t, "status report function should not be called")
		}

		monitorCtx, monitorCtxCancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(50 * time.Millisecond)
			monitorCtxCancel()
		}()

		monitorPPID(monitorCtx, 1*time.Millisecond, int32(cmdPid), statusReportFunc)
	})

	t.Run("Emits fatal status if ppid changes", func(t *testing.T) {
		t.Parallel()

		cmdContext, cmdCancel := context.WithCancel(context.Background())
		cmd := longRunningCommand(cmdContext)
		require.NoError(t, cmd.Start())
		cmdPid := cmd.Process.Pid

		var status *componentstatus.Event
		statusReportFunc := func(evt *componentstatus.Event) {
			if status != nil {
				require.FailNow(t, "status report function should not be called twice")
			}
			status = evt
		}

		cmdDoneChan := make(chan struct{})
		go func() {
			time.Sleep(50 * time.Millisecond)
			cmdCancel()
			_ = cmd.Wait()
			close(cmdDoneChan)
		}()

		monitorPPID(context.Background(), 1*time.Millisecond, int32(cmdPid), statusReportFunc)
		require.NotNil(t, status)
		require.Equal(t, componentstatus.StatusFatalError, status.Status())

		// wait for command stop goroutine to actually finish
		select {
		case <-cmdDoneChan:
		case <-time.After(5 * time.Second):
			t.Fatalf("Timed out waiting for command to stop")
		}
	})
}

func longRunningCommand(ctx context.Context) *exec.Cmd {
	switch runtime.GOOS {
	case "windows":
		// Would prefer to use timeout.exe here, but it doesn't seem to work in
		// a CMD-less context.
		return exec.CommandContext(ctx, "ping", "-n", "1000", "localhost")
	default:
		return exec.CommandContext(ctx, "sleep", "1000")
	}
}
