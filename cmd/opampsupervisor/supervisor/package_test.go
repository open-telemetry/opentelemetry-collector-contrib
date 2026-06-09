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
		time.Sleep(time.Minute)
		os.Exit(0)
	case "emit-on-interrupt":
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		_, _ = fmt.Fprint(os.Stderr, "shutdown error")
		os.Exit(0)
	}
	goleak.VerifyTestMain(m)
}
