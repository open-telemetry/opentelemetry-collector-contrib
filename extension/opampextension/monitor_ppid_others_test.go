//go:build !windows

package opampextension

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestMonitorPPIDOthers(t *testing.T) {
	t.Run("Does not trigger if ppid stays as specified", func(t *testing.T) {
		statusReportFunc := func(*component.StatusEvent) {
			require.FailNow(t, "status report function should not be called")
		}

		monitorCtx, monitorCtxCancel := context.WithCancel(context.Background())
		monitorCtxCancel()

		monitorPPID(monitorCtx, os.Getppid(), statusReportFunc)
	})

	t.Run("Emits fatal status if ppid changes", func(t *testing.T) {
		numPolls := 0
		setGetPPID(t, func() int {
			numPolls++
			if numPolls > 1 {
				return 1
			}
			return os.Getppid()
		})

		setOrphanPollInterval(t, 10*time.Millisecond)

		var statusEvent *component.StatusEvent
		statusReportFunc := func(evt *component.StatusEvent) {
			if statusEvent != nil {
				require.FailNow(t, "status report function should not be called twice")
			}
			statusEvent = evt
		}

		monitorPPID(context.Background(), os.Getppid(), statusReportFunc)
		require.NotNil(t, statusEvent)
		require.Equal(t, component.StatusFatalError, statusEvent.Status())
	})

}

func setGetPPID(t *testing.T, newFunc func() int) {
	old := getppid
	getppid = newFunc
	t.Cleanup(func() {
		getppid = old
	})
}
