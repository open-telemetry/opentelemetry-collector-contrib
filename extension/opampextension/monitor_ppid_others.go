//go:build !windows

package opampextension

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
)

// getppid is a function to get ppid of the process. It is mocked in testing.
var getppid = os.Getppid

func monitorPPID(ctx context.Context, ppid int, reportStatus func(*component.StatusEvent)) {
	// On unix-based systems, when the parent process dies orphaned processes
	// are re-parented to be under the init system process (ppid becomes 1).
	for {
		if getppid() != ppid {
			err := fmt.Errorf("collector was orphaned, parent pid is no longer %d", ppid)
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
