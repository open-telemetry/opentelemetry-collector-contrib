// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain verifies that no goroutines leak once tests complete. The notifier
// spawns worker goroutines, so a shutdown regression that fails to drain the
// wait group or to cancel the shutdown context will surface here.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	)
}
