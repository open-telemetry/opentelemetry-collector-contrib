// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

// logColumnsCount is the number of columns in the logs table.
const logColumnsCount = 13

// maxLogsPerBatch is the maximum number of log records per INSERT batch.
const maxLogsPerBatch = maxInsertParams / logColumnsCount

// logRow holds the arguments for a single log row INSERT.
type logRow struct {
	args []any
}

type logsExporter struct {
	logger *zap.Logger
	cfg    *Config
	db     *sql.DB
}

func newLogsExporter(logger *zap.Logger, cfg *Config) *logsExporter {
	return &logsExporter{
		logger: logger,
		cfg:    cfg,
	}
}

func (e *logsExporter) start(ctx context.Context, _ component.Host) error {
	db, err := openDB(e.cfg.DSN)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.CreateSchema {
		if err := createLogsTable(ctx, e.db, e.cfg.LogsTableName, e.cfg.TTL); err != nil {
			return err
		}
	}
	return nil
}

func (e *logsExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func (e *logsExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	start := time.Now()

	// Collect all log rows.
	var rows []logRow

	rls := ld.ResourceLogs()
	for i := range rls.Len() {
		rl := rls.At(i)
		serviceName := getServiceName(rl.Resource())
		resourceAttrs, err := attributesToJSON(rl.Resource().Attributes())
		if err != nil {
			return fmt.Errorf("failed to marshal resource attributes: %w", err)
		}

		scopeLogs := rl.ScopeLogs()
		for j := range scopeLogs.Len() {
			sl := scopeLogs.At(j)
			scopeName := sl.Scope().Name()
			scopeVersion := sl.Scope().Version()
			scopeAttrs, err := attributesToJSON(sl.Scope().Attributes())
			if err != nil {
				return fmt.Errorf("failed to marshal scope attributes: %w", err)
			}

			records := sl.LogRecords()
			for k := range records.Len() {
				log := records.At(k)

				logAttrs, err := attributesToJSON(log.Attributes())
				if err != nil {
					return fmt.Errorf("failed to marshal log attributes: %w", err)
				}

				// Use Timestamp if set; otherwise fall back to ObservedTimestamp.
				ts := log.Timestamp()
				if ts == 0 {
					ts = log.ObservedTimestamp()
				}

				rows = append(rows, logRow{
					args: []any{
						ts.AsTime(),                                         // timestamp
						traceutil.TraceIDToHexOrEmptyString(log.TraceID()),  // trace_id
						traceutil.SpanIDToHexOrEmptyString(log.SpanID()),    // span_id
						int32(log.Flags()),                                  // trace_flags
						log.SeverityText(),                                  // severity_text
						int32(log.SeverityNumber()),                         // severity_number
						serviceName,                                         // service_name
						log.Body().AsString(),                               // body
						resourceAttrs,                                       // resource_attributes
						scopeName,                                           // scope_name
						scopeVersion,                                        // scope_version
						scopeAttrs,                                          // scope_attributes
						logAttrs,                                            // log_attributes
					},
				})
			}
		}
	}

	if len(rows) == 0 {
		return nil
	}

	// Insert in batches to stay within PostgreSQL parameter limit.
	for batchStart := 0; batchStart < len(rows); batchStart += maxLogsPerBatch {
		batchEnd := batchStart + maxLogsPerBatch
		if batchEnd > len(rows) {
			batchEnd = len(rows)
		}
		batch := rows[batchStart:batchEnd]

		if err := e.insertBatch(ctx, batch); err != nil {
			return err
		}
	}

	e.logger.Debug("inserted logs",
		zap.Int("log_count", len(rows)),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

func (e *logsExporter) insertBatch(ctx context.Context, rows []logRow) error {
	values := make([]string, len(rows))
	args := make([]any, 0, len(rows)*logColumnsCount)
	argIdx := 1

	for i, row := range rows {
		placeholders := make([]string, logColumnsCount)
		for p := range logColumnsCount {
			placeholders[p] = fmt.Sprintf("$%d", argIdx+p)
		}
		values[i] = fmt.Sprintf("(%s)", strings.Join(placeholders, ","))
		argIdx += logColumnsCount
		args = append(args, row.args...)
	}

	query := fmt.Sprintf(
		`INSERT INTO %s ("timestamp", trace_id, span_id, trace_flags, severity_text, severity_number, service_name, body, resource_attributes, scope_name, scope_version, scope_attributes, log_attributes) VALUES %s`,
		e.cfg.LogsTableName,
		strings.Join(values, ","),
	)

	_, err := e.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert logs: %w", err)
	}

	return nil
}
