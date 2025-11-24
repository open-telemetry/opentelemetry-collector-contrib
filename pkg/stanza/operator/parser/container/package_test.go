// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// Ignore goroutines from the atomicLimiter background ticker
		// These goroutines are properly stopped when cache.stop() is called,
		// but may still be exiting when goleak checks
		goleak.IgnoreTopFunction("github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/container.(*atomicLimiter).init.func1.1"),
	)
}
