// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
	"go.uber.org/zap"
)

// ClickHouse server error codes for permanent DESC TABLE failures.
// These indicate the probe will never succeed without external intervention
// (schema change, permission grant, table creation), so the exporter should
// fall back to a degraded INSERT rather than deferring every batch forever.
const (
	chCodeUnknownTable         int32 = 60  // UNKNOWN_TABLE
	chCodeUnknownDatabase      int32 = 81  // UNKNOWN_DATABASE
	chCodeDatabaseAccessDenied int32 = 291 // DATABASE_ACCESS_DENIED
	chCodeAccessDenied         int32 = 497 // ACCESS_DENIED
)

// schemaDetectionState tracks whether a DESC TABLE probe has produced a definitive
// answer about which optional feature-columns the target table has.
//
// The bug fixed by #48875 was that the original code conflated "DESC TABLE failed
// (table unreachable, transient outage, missing table)" with "feature absent" — a
// single failed probe at startup permanently cached a degraded INSERT statement
// that omitted columns the user actually had in their schema. The exporter would
// silently write rows without those columns for the life of the process.
//
// schemaDetectionState distinguishes the three outcomes so the caller can defer
// the INSERT rendering until detection succeeds at least once.
type schemaDetectionState int32

const (
	// schemaUnknown means no successful DESC TABLE has run yet. Any cached
	// INSERT must be treated as not-yet-rendered; push paths should re-attempt
	// detection and, on continued failure, return a retryable error so the
	// sending queue / retry_on_failure machinery defers the batch.
	schemaUnknown schemaDetectionState = iota
	// schemaDetected means DESC TABLE returned a definitive column set and the
	// per-feature flags have been populated.
	schemaDetected
)

// schemaDetector serializes lazy re-detection across concurrent push goroutines.
// It is embedded into each *JSON / non-JSON exporter that previously cached an
// INSERT based on a single startup probe.
//
// state is read atomically on the fast path so steady-state push goroutines
// (10× num_consumers by default) do not serialize on the mutex once detection
// has succeeded. The mutex is only contended on the rare error-retry path.
type schemaDetector struct {
	state atomic.Int32
	mu    sync.Mutex
}

// isPermanentDESCFailure returns true when the error wraps a ClickHouse server
// exception whose error code indicates that DESC TABLE will never succeed
// without external intervention (e.g. missing privileges, unknown table). In
// that case the exporter should fall back to a degraded INSERT (omitting
// optional columns) instead of deferring every batch forever.
//
// Transient failures (connection refused, timeout, server overloaded) are Go-
// level network errors that do NOT wrap *proto.Exception, so they return false
// and trigger the normal retry path.
func isPermanentDESCFailure(err error) bool {
	var ex *proto.Exception
	if !errors.As(err, &ex) {
		return false
	}
	switch ex.Code {
	case chCodeUnknownTable, chCodeUnknownDatabase,
		chCodeDatabaseAccessDenied, chCodeAccessDenied:
		return true
	}
	return false
}

// ensureDetected runs probe under the mutex if detection is not yet successful.
// probe is expected to (a) run DESC TABLE via internal.GetTableColumns,
// (b) populate the caller's feature flags from the column set, and (c) render
// the cached INSERT statement. probe is only called when state == schemaUnknown.
//
// fallback is called when DESC TABLE fails with a permanent ClickHouse error
// (unknown table, access denied). It must render a degraded INSERT statement
// with all optional feature flags at their zero values, matching the pre-#48902
// behavior for write-only users. When fallback is nil the permanent-failure
// path behaves the same as the transient-failure path (retry on every push).
//
// On success the state transitions to schemaDetected and probe is never called
// again. On transient error the state stays schemaUnknown and the wrapped error
// is returned with a stable prefix so callers can recognize the deferred-
// detection path and surface a retryable (non-permanent) error to exporterhelper.
func (d *schemaDetector) ensureDetected(ctx context.Context, logger *zap.Logger, probe func(context.Context) error, fallback func()) error {
	// Fast path: once detection has succeeded (or fallen back), every push
	// goroutine sees schemaDetected atomically without contending on the mutex.
	if schemaDetectionState(d.state.Load()) == schemaDetected {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Recheck under the lock: another goroutine may have completed detection
	// between our atomic load and the Lock.
	if schemaDetectionState(d.state.Load()) == schemaDetected {
		return nil
	}

	if err := probe(ctx); err != nil {
		if fallback != nil && isPermanentDESCFailure(err) {
			// Permanent failure: the user lacks SELECT/DESC privileges or the
			// table does not exist (with create_schema=false). Fall back to a
			// degraded INSERT that omits the optional feature columns. This
			// preserves the pre-#48902 behavior where write-only ClickHouse
			// users kept delivering rows without the keys/EventName columns.
			fallback()
			d.state.Store(int32(schemaDetected))
			logger.Warn("clickhouseexporter DESC TABLE permanently denied; using degraded INSERT without optional columns",
				zap.Error(err))
			return nil
		}

		// Stay schemaUnknown so the next push retries. The caller wraps this
		// in a plain (non-permanent) error to engage retry_on_failure. Log at
		// Debug — Warn would spam at 10× num_consumers per batch under a
		// sustained outage.
		logger.Debug("clickhouseexporter schema detection still deferred", zap.Error(err))
		return fmt.Errorf("schema detection deferred: %w", err)
	}

	d.state.Store(int32(schemaDetected))
	logger.Info("clickhouseexporter schema detection succeeded; INSERT cached")
	return nil
}
