// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
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
		err := d.ensureDetected(t.Context(), logger, probe, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "schema detection deferred")
		require.ErrorIs(t, err, probeErr)
	}

	// Third call: detection succeeds, transitions to schemaDetected.
	require.NoError(t, d.ensureDetected(t.Context(), logger, probe, nil))
	assert.Equal(t, int32(3), probeCalls.Load())

	// Subsequent calls: probe is NOT re-run.
	require.NoError(t, d.ensureDetected(t.Context(), logger, probe, nil))
	require.NoError(t, d.ensureDetected(t.Context(), logger, probe, nil))
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
			assert.NoError(t, d.ensureDetected(t.Context(), logger, probe, nil))
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
	}, nil)
	require.Error(t, err)
	require.Equal(t, schemaUnknown, schemaDetectionState(d.state.Load()))

	// A second probe with a different outcome resolves cleanly.
	require.NoError(t, d.ensureDetected(t.Context(), logger, func(_ context.Context) error {
		return nil
	}, nil))
	require.Equal(t, schemaDetected, schemaDetectionState(d.state.Load()))
}

// TestSchemaDetector_PermanentFailureFallsBack verifies that a permanent
// ClickHouse error (ACCESS_DENIED, UNKNOWN_TABLE, etc.) triggers the fallback
// renderer instead of deferring every batch forever. This is the fix for the
// regression reported in PR #48902: a write-only ClickHouse user (INSERT
// granted, SELECT/DESC denied) must keep delivering rows with a degraded
// INSERT rather than silently losing all data.
func TestSchemaDetector_PermanentFailureFallsBack(t *testing.T) {
	tests := []struct {
		name string
		code int32
	}{
		{"ACCESS_DENIED", chCodeAccessDenied},
		{"UNKNOWN_TABLE", chCodeUnknownTable},
		{"UNKNOWN_DATABASE", chCodeUnknownDatabase},
		{"DATABASE_ACCESS_DENIED", chCodeDatabaseAccessDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d schemaDetector
			logger := zaptest.NewLogger(t)
			fallbackCalled := false

			probe := func(_ context.Context) error {
				return &proto.Exception{
					Code:    tt.code,
					Name:    tt.name,
					Message: "Not enough privileges",
				}
			}
			fallback := func() { fallbackCalled = true }

			err := d.ensureDetected(t.Context(), logger, probe, fallback)
			require.NoError(t, err, "permanent failure with fallback must not return an error")
			require.True(t, fallbackCalled, "fallback must be called on permanent failure")
			require.Equal(t, schemaDetected, schemaDetectionState(d.state.Load()),
				"state must transition to schemaDetected after fallback")

			// Subsequent calls must not re-probe.
			fallbackCalled = false
			require.NoError(t, d.ensureDetected(t.Context(), logger, probe, fallback))
			require.False(t, fallbackCalled, "fallback must not be called again after schemaDetected")
		})
	}
}

// TestSchemaDetector_PermanentFailureWithoutFallback verifies that when no
// fallback is provided (fallback == nil), permanent errors are treated the
// same as transient errors — they defer and retry on the next push.
func TestSchemaDetector_PermanentFailureWithoutFallback(t *testing.T) {
	var d schemaDetector
	logger := zaptest.NewLogger(t)

	probe := func(_ context.Context) error {
		return &proto.Exception{
			Code:    chCodeAccessDenied,
			Name:    "ACCESS_DENIED",
			Message: "Not enough privileges",
		}
	}

	err := d.ensureDetected(t.Context(), logger, probe, nil)
	require.Error(t, err, "permanent failure without fallback must return an error")
	require.ErrorContains(t, err, "schema detection deferred")
	require.Equal(t, schemaUnknown, schemaDetectionState(d.state.Load()))
}

// TestIsPermanentDESCFailure verifies the error classification function.
func TestIsPermanentDESCFailure(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		permanent bool
	}{
		{
			name:      "ACCESS_DENIED exception",
			err:       &proto.Exception{Code: chCodeAccessDenied, Message: "Not enough privileges"},
			permanent: true,
		},
		{
			name:      "UNKNOWN_TABLE exception",
			err:       &proto.Exception{Code: chCodeUnknownTable, Message: "Table does not exist"},
			permanent: true,
		},
		{
			name:      "UNKNOWN_DATABASE exception",
			err:       &proto.Exception{Code: chCodeUnknownDatabase, Message: "Database does not exist"},
			permanent: true,
		},
		{
			name:      "DATABASE_ACCESS_DENIED exception",
			err:       &proto.Exception{Code: chCodeDatabaseAccessDenied, Message: "Not enough privileges"},
			permanent: true,
		},
		{
			name:      "wrapped permanent exception",
			err:       fmt.Errorf("get table columns: %w", &proto.Exception{Code: chCodeAccessDenied, Message: "denied"}),
			permanent: true,
		},
		{
			name:      "transient network error",
			err:       errors.New("dial tcp clickhouse:9440: connect: connection refused"),
			permanent: false,
		},
		{
			name:      "transient ClickHouse exception (MEMORY_LIMIT_EXCEEDED)",
			err:       &proto.Exception{Code: 241, Message: "Memory limit exceeded"},
			permanent: false,
		},
		{
			name:      "nil error",
			err:       nil,
			permanent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				// isPermanentDESCFailure is only called on non-nil errors in
				// production, but verify it does not panic.
				require.False(t, isPermanentDESCFailure(tt.err))
				return
			}
			require.Equal(t, tt.permanent, isPermanentDESCFailure(tt.err))
		})
	}
}
