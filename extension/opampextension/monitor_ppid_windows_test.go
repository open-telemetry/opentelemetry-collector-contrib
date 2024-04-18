//go:build windows

package opampextension

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestMonitorPPIDWindows(t *testing.T) {
	cmdContext, cmdContextCancel := context.WithCancel(context.Background())
	t.Cleanup(cmdContextCancel)

	cmd := exec.CommandContext(cmdContext, "/bin/sh", "-c", "sleep 1000")
	err := cmd.Start()
	require.NoError(t, err)

	statusReportFunc := func(*component.StatusEvent) {
		require.FailNow(t, "status report function should not be called")
	}

	monitorCtx, monitorCtxCancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(1 * time.Second)
		monitorCtxCancel()
	}()

	monitorPPID(monitorCtx, cmd.Process.Pid, statusReportFunc)
}
