package opampextension

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestMonitorPPIDOthers(t *testing.T) {
	t.Run("Does not trigger if process with ppid never stops", func(t *testing.T) {
		cmdContext, cmdCancel := context.WithCancel(context.Background())
		cmd := longRunningComand(cmdContext)
		cmd.Stdout = os.Stdout
		require.NoError(t, cmd.Start())

		t.Cleanup(func() {
			cmdCancel()
			_ = cmd.Wait()
		})

		statusReportFunc := func(se *component.StatusEvent) {
			t.Logf("Status event error: %s", se.Err())
			require.FailNow(t, "status report function should not be called")
		}

		monitorCtx, monitorCtxCancel := context.WithCancel(context.Background())

		setOrphanPollInterval(t, 10*time.Millisecond)

		go func() {
			time.Sleep(500 * time.Millisecond)
			monitorCtxCancel()
		}()

		monitorPPID(monitorCtx, int32(cmd.Process.Pid), statusReportFunc)
	})

	t.Run("Emits fatal status if ppid changes", func(t *testing.T) {
		cmdContext, cmdCancel := context.WithCancel(context.Background())
		cmd := longRunningComand(cmdContext)
		require.NoError(t, cmd.Start())

		t.Cleanup(func() {
			cmdCancel()
			_ = cmd.Wait()
		})

		var status *component.StatusEvent
		statusReportFunc := func(evt *component.StatusEvent) {
			if status != nil {
				require.FailNow(t, "status report function should not be called twice")
			}
			status = evt
		}

		setOrphanPollInterval(t, 10*time.Millisecond)

		go func() {
			time.Sleep(500 * time.Millisecond)
			cmdCancel()
			_ = cmd.Wait()
		}()

		monitorPPID(context.Background(), int32(cmd.Process.Pid), statusReportFunc)
		require.NotNil(t, status)
		require.Equal(t, component.StatusFatalError, status.Status())
	})

}

func longRunningComand(ctx context.Context) *exec.Cmd {
	switch runtime.GOOS {
	case "windows":
		// Would prefer to use timeout.exe here, but it doesn't seem to work in
		// a CMD-less context.
		return exec.CommandContext(ctx, "ping", "-n", "1000", "localhost")
	default:
		return exec.CommandContext(ctx, "sleep", "1000")
	}
}

func setOrphanPollInterval(t *testing.T, newInterval time.Duration) {
	old := orphanPollInterval
	orphanPollInterval = newInterval
	t.Cleanup(func() {
		orphanPollInterval = old
	})
}
