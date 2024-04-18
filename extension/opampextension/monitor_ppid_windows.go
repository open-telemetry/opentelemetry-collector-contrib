//go:build windows

package opampextension

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
)

func monitorPPID(ctx context.Context, ppid int, reportStatus func(*component.StatusEvent)) {
	// On Windows systems, we can look up and synchronously wait for the process to exit.
	// This is not possible on other systems, since Wait doesn't work on most systems unless the
	// process is a child of the current one (see doc on process.Wait).
	for {
		process, err := os.FindProcess(ppid)
		if err != nil {
			err := fmt.Errorf("collector was orphaned, error finding process %d: %w", ppid, err)
			status := component.NewFatalErrorEvent(err)
			reportStatus(status)
			return
		}

		if process == nil {
			err := fmt.Errorf("collector was orphaned, process %d does not exist", ppid)
			status := component.NewFatalErrorEvent(err)
			reportStatus(status)
			return
		}

		select {
		case <-time.After(orphanPollInterval):
		case <-ctx.Done():
			return
		}
	}
}
