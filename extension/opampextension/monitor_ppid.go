package opampextension

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"go.opentelemetry.io/collector/component"
)

var orphanPollInterval = 5 * time.Second

// monitorPPID polls for the existence of ppid.
// If the specified ppid no longer exists, a fatal error event is reported via the passed in reportStatus function.
func monitorPPID(ctx context.Context, ppid int32, reportStatus func(*component.StatusEvent)) {
	for {
		exists, err := process.PidExistsWithContext(ctx, ppid)
		if err != nil {
			statusErr := fmt.Errorf("collector was orphaned, failed to find process with pid %d: %w", ppid, err)
			status := component.NewFatalErrorEvent(statusErr)
			reportStatus(status)
			return
		}

		if !exists {
			statusErr := fmt.Errorf("collector was orphaned, process with pid %d does not exist", ppid)
			status := component.NewFatalErrorEvent(statusErr)
			reportStatus(status)
			return
		}

		select {
		case <-time.After(orphanPollInterval): // OK; Poll again to make sure PID exists
		case <-ctx.Done():
			return
		}
	}
}
