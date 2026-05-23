// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fixture // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/fixture"

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/race"
)

// ParallelRaceCompute starts `count` number of go routines that calls the provided function `fn`
// at the same to allow the race detector greater opportunity to capture known race conditions.
// This method blocks until each count number of fn has completed, any returned errors is considered
// a failing test method.
// If the race detector is not enabled, the function then skips with an notice.
// This is intended to show that a test was intentionally skipped instead of just missing.
func ParallelRaceCompute(tb testing.TB, count int, fn func() error) {
	tb.Helper()
	if !race.Enabled {
		tb.Skip(
			"This test requires the Race Detector to be enabled.",
			"Please run again with -race to run this test.",
		)
		return
	}
	require.NotNil(tb, fn, "Must have a valid function")

	var (
		start = make(chan struct{})
		wg    sync.WaitGroup
	)
	for range count {
		wg.Go(func() {
			<-start
			assert.NoError(tb, fn(), "Must not error when executing function")
		})
	}
	close(start)

	wg.Wait()
}
