// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// perScopeBatcher is a consumer.Logs that rebatches logs by a type found in the scope name: profiling or regular logs.
type perScopeBatcher struct {
	logsEnabled      bool
	profilingEnabled bool
	logger           *zap.Logger
	next             consumer.Logs
}

func (rb *perScopeBatcher) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (rb *perScopeBatcher) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	var profilingFound bool
	var otherLogsFound bool

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rs := logs.ResourceLogs().At(i)
		for j := 0; j < rs.ScopeLogs().Len(); j++ {
			if isProfilingData(rs.ScopeLogs().At(j)) {
				profilingFound = true
			} else {
				otherLogsFound = true
			}
		}
		if profilingFound && otherLogsFound {
			break
		}
	}

	// if we don't have both types of logs, just call next if enabled
	if !profilingFound || !otherLogsFound {
		if otherLogsFound {
			if rb.logsEnabled {
				return rb.next.ConsumeLogs(ctx, logs)
			}
			rb.logger.Debug("Log data is not allowed", zap.Int("dropped_records", logs.LogRecordCount()))
		}
		if profilingFound {
			if rb.profilingEnabled {
				return rb.next.ConsumeLogs(ctx, logs)
			}
			rb.logger.Debug("Profiling data is not allowed", zap.Int("dropped_records", logs.LogRecordCount()))
		}
		return nil
	}

	profilingLogs := plog.NewLogs()
	otherLogs := plog.NewLogs()

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rs := logs.ResourceLogs().At(i)
		profilingFound = false
		otherLogsFound = false
		for j := 0; j < rs.ScopeLogs().Len(); j++ {
			sl := rs.ScopeLogs().At(j)
			if isProfilingData(sl) {
				profilingFound = true
			} else {
				otherLogsFound = true
			}
		}
		switch {
		case profilingFound && otherLogsFound:
			if rb.profilingEnabled {
				copyResourceLogs(rs, profilingLogs.ResourceLogs().AppendEmpty(), true)
			}
			if rb.logsEnabled {
				copyResourceLogs(rs, otherLogs.ResourceLogs().AppendEmpty(), false)
			}
		case profilingFound && rb.profilingEnabled:
			rs.CopyTo(profilingLogs.ResourceLogs().AppendEmpty())
		case otherLogsFound && rb.logsEnabled:
			rs.CopyTo(otherLogs.ResourceLogs().AppendEmpty())
		}
	}

	var err error
	if rb.logsEnabled {
		err = multierr.Append(err, rb.next.ConsumeLogs(ctx, otherLogs))
	} else {
		rb.logger.Debug("Log data is not allowed", zap.Int("dropped_records",
			logs.LogRecordCount()-profilingLogs.LogRecordCount()))
	}
	if rb.profilingEnabled {
		err = multierr.Append(err, rb.next.ConsumeLogs(ctx, profilingLogs))
	} else {
		rb.logger.Debug("Profiling data is not allowed", zap.Int("dropped_records",
			logs.LogRecordCount()-otherLogs.LogRecordCount()))
	}
	return err
}

func copyResourceLogs(src plog.ResourceLogs, dest plog.ResourceLogs, isProfiling bool) {
	src.Resource().CopyTo(dest.Resource())
	for j := 0; j < src.ScopeLogs().Len(); j++ {
		sl := src.ScopeLogs().At(j)
		if isProfilingData(sl) == isProfiling {
			sl.CopyTo(dest.ScopeLogs().AppendEmpty())
		}
	}
}
