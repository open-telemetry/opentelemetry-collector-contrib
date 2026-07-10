// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"fmt"
	"os"
	"os/signal"
	"testing"
	"time"

	"go.uber.org/goleak"
)

const supervisorRestartHelperEnv = "OTEL_SUPERVISOR_RESTART_HELPER"

func TestMain(m *testing.M) {
	switch os.Getenv(supervisorRestartHelperEnv) {
	case "sleep":
		// Re-run this test binary as a stand-in Collector process so supervisor
		// restart tests can exercise real process handling without a helper script.
		time.Sleep(time.Minute)
		os.Exit(0)
	case "emit-on-interrupt":
		// Emit a shutdown log line from the child process to verify restart paths
		// clear logs produced by the intentional stop before starting again.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		_, _ = fmt.Fprint(os.Stderr, "shutdown error")
		os.Exit(0)
	}
	goleak.VerifyTestMain(m)
}
