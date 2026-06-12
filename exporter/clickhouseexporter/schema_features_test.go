// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestSchemaDetector_ProbeRetriedUntilSuccess is the unit-level regression for
// #48875: a single failed DESC TABLE must not be cached as "feature absent".
// ensureDetected must retry the probe until it succeeds, leaving the detector
// in schemaUnknown after each failure so the next caller re-probes.
func TestSchemaDetector_ProbeRetriedUntilSuccess(t *testing.T) {
	var d schemaDetector
	var probeCalls atomic.Int32
	probeErr := errors.New("dial tcp clickhouse:9440: connect: connection refused")

	probe := func(_ context.Context) error {
		probeCalls.Add(1)
		// Fail the first two attempts, succeed on the third.
		if probeCalls.Load() < 3 {
			return probeErr
		}
		return nil
	}

	logger := zaptest.NewLogger(t)

	// First two calls: detection fails, error returned with the stable prefix.
	for range 2 {
		err := d.ensureDetected(t.Context(), logger, probe)
		require.Error(t, err)
		require.ErrorContains(t, err, "schema detection deferred")
		require.ErrorIs(t, err, probeErr)
	}

	// Third call: detection succeeds, transitions to schemaDetected.
	require.NoError(t, d.ensureDetected(t.Context(), logger, probe))
	assert.Equal(t, int32(3), probeCalls.Load())

	// Subsequent calls: probe is NOT re-run.
	require.NoError(t, d.ensureDetected(t.Context(), logger, probe))
	require.NoError(t, d.ensureDetected(t.Context(), logger, probe))
	assert.Equal(t, int32(3), probeCalls.Load(), "probe must not run after schemaDetected")
}

// TestSchemaDetector_ConcurrentCallsResolveOnce verifies that concurrent
// pushXData goroutines do not all run the probe — only one succeeds and the
// rest see schemaDetected immediately. This matters because every push goroutine
// (default num_consumers=10) calls ensureDetected on every batch until detection
// succeeds.
func TestSchemaDetector_ConcurrentCallsResolveOnce(t *testing.T) {
	var d schemaDetector
	var probeCalls atomic.Int32
	// probe always succeeds — error return is unused but required by the
	// schemaDetector.ensureDetected signature.
	//nolint:unparam // intentional: probe is success-only in this test
	probe := func(_ context.Context) error {
		probeCalls.Add(1)
		return nil
	}

	logger := zaptest.NewLogger(t)

	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			assert.NoError(t, d.ensureDetected(t.Context(), logger, probe))
		})
	}
	wg.Wait()

	// Exactly one probe call survives the mutex race.
	assert.Equal(t, int32(1), probeCalls.Load())
}

// TestSchemaDetector_FailureKeepsStateUnknown verifies the bug-fix invariant:
// after a probe failure the state must NOT transition to schemaDetected. This
// is the precise inversion of the #48875 bug, where a failed DESC TABLE silently
// cached "feature absent" for the life of the process.
func TestSchemaDetector_FailureKeepsStateUnknown(t *testing.T) {
	var d schemaDetector
	logger := zap.NewNop()

	err := d.ensureDetected(t.Context(), logger, func(_ context.Context) error {
		return errors.New("ch unavailable")
	})
	require.Error(t, err)
	require.Equal(t, schemaUnknown, schemaDetectionState(d.state.Load()))

	// A second probe with a different outcome resolves cleanly.
	require.NoError(t, d.ensureDetected(t.Context(), logger, func(_ context.Context) error {
		return nil
	}))
	require.Equal(t, schemaDetected, schemaDetectionState(d.state.Load()))
}
