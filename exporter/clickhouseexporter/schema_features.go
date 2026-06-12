// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
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

// ensureDetected runs probe under the mutex if detection is not yet successful.
// probe is expected to (a) run DESC TABLE via internal.GetTableColumns,
// (b) populate the caller's feature flags from the column set, and (c) render
// the cached INSERT statement. probe is only called when state == schemaUnknown.
//
// On success the state transitions to schemaDetected and probe is never called
// again. On error the state stays schemaUnknown and the wrapped error is
// returned with a stable prefix so callers can recognize the deferred-detection
// path and surface a retryable (non-permanent) error to exporterhelper.
func (d *schemaDetector) ensureDetected(ctx context.Context, logger *zap.Logger, probe func(context.Context) error) error {
	// Fast path: once detection has succeeded, every push goroutine sees
	// schemaDetected atomically without contending on the mutex.
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
