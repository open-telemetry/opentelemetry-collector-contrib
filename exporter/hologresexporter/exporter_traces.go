// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

// maxInsertParams is the maximum number of parameters in a single INSERT statement.
// PostgreSQL protocol supports up to 65535 parameters.
const maxInsertParams = 65535

// traceColumnsCount is the number of columns in the traces table.
const traceColumnsCount = 17

// maxSpansPerBatch is the maximum number of spans per INSERT batch.
const maxSpansPerBatch = maxInsertParams / traceColumnsCount

// traceSpanRow holds the arguments for a single span row INSERT.
type traceSpanRow struct {
	args []any
}

type tracesExporter struct {
	logger *zap.Logger
	cfg    *Config
	db     *sql.DB
}

func newTracesExporter(logger *zap.Logger, cfg *Config) *tracesExporter {
	return &tracesExporter{
		logger: logger,
		cfg:    cfg,
	}
}

func (e *tracesExporter) start(ctx context.Context, _ component.Host) error {
	db, err := openDB(e.cfg.DSN)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.CreateSchema {
		if err := createTracesTable(ctx, e.db, e.cfg.TracesTableName, e.cfg.TTL); err != nil {
			return err
		}
	}
	return nil
}

func (e *tracesExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func (e *tracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	start := time.Now()

	// Collect all span rows.
	var rows []traceSpanRow

	rsSpans := td.ResourceSpans()
	for i := range rsSpans.Len() {
		rs := rsSpans.At(i)
		serviceName := getServiceName(rs.Resource())
		resourceAttrs, err := attributesToJSON(rs.Resource().Attributes())
		if err != nil {
			return fmt.Errorf("failed to marshal resource attributes: %w", err)
		}

		scopeSpans := rs.ScopeSpans()
		for j := range scopeSpans.Len() {
			ss := scopeSpans.At(j)
			scopeName := ss.Scope().Name()
			scopeVersion := ss.Scope().Version()

			spans := ss.Spans()
			for k := range spans.Len() {
				span := spans.At(k)

				spanAttrs, err := attributesToJSON(span.Attributes())
				if err != nil {
					return fmt.Errorf("failed to marshal span attributes: %w", err)
				}

				eventsJSON, err := convertEvents(span.Events())
				if err != nil {
					return fmt.Errorf("failed to marshal events: %w", err)
				}

				linksJSON, err := convertLinks(span.Links())
				if err != nil {
					return fmt.Errorf("failed to marshal links: %w", err)
				}

				rows = append(rows, traceSpanRow{
					args: []any{
						span.StartTimestamp().AsTime(),                   // timestamp
						traceutil.TraceIDToHexOrEmptyString(span.TraceID()),   // trace_id
						traceutil.SpanIDToHexOrEmptyString(span.SpanID()),     // span_id
						traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()), // parent_span_id
						span.TraceState().AsRaw(),                        // trace_state
						span.Name(),                                      // span_name
						traceutil.SpanKindStr(span.Kind()),               // span_kind
						serviceName,                                      // service_name
						resourceAttrs,                                    // resource_attributes
						scopeName,                                        // scope_name
						scopeVersion,                                     // scope_version
						spanAttrs,                                        // span_attributes
						int64(span.EndTimestamp() - span.StartTimestamp()), // duration (nanoseconds)
						traceutil.StatusCodeStr(span.Status().Code()),    // status_code
						span.Status().Message(),                          // status_message
						eventsJSON,                                       // events
						linksJSON,                                        // links
					},
				})
			}
		}
	}

	if len(rows) == 0 {
		return nil
	}

	// Insert in batches to stay within PostgreSQL parameter limit.
	for batchStart := 0; batchStart < len(rows); batchStart += maxSpansPerBatch {
		batchEnd := batchStart + maxSpansPerBatch
		if batchEnd > len(rows) {
			batchEnd = len(rows)
		}
		batch := rows[batchStart:batchEnd]

		if err := e.insertBatch(ctx, batch); err != nil {
			return err
		}
	}

	e.logger.Debug("inserted traces",
		zap.Int("span_count", len(rows)),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

func (e *tracesExporter) insertBatch(ctx context.Context, rows []traceSpanRow) error {
	values := make([]string, len(rows))
	args := make([]any, 0, len(rows)*traceColumnsCount)
	argIdx := 1

	for i, row := range rows {
		placeholders := make([]string, traceColumnsCount)
		for p := range traceColumnsCount {
			placeholders[p] = fmt.Sprintf("$%d", argIdx+p)
		}
		values[i] = fmt.Sprintf("(%s)", strings.Join(placeholders, ","))
		argIdx += traceColumnsCount
		args = append(args, row.args...)
	}

	query := fmt.Sprintf(
		`INSERT INTO %s ("timestamp", trace_id, span_id, parent_span_id, trace_state, span_name, span_kind, service_name, resource_attributes, scope_name, scope_version, span_attributes, duration, status_code, status_message, events, links) VALUES %s`,
		e.cfg.TracesTableName,
		strings.Join(values, ","),
	)

	_, err := e.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert traces: %w", err)
	}

	return nil
}

// convertEvents serializes span events to a JSON array for JSONB storage.
func convertEvents(events ptrace.SpanEventSlice) ([]byte, error) {
	if events.Len() == 0 {
		return []byte("[]"), nil
	}
	result := make([]map[string]any, events.Len())
	for i := range events.Len() {
		e := events.At(i)
		attrs, err := attributesToJSON(e.Attributes())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal event attributes: %w", err)
		}
		result[i] = map[string]any{
			"timestamp":  e.Timestamp().AsTime(),
			"name":       e.Name(),
			"attributes": json.RawMessage(attrs),
		}
	}
	return json.Marshal(result)
}

// convertLinks serializes span links to a JSON array for JSONB storage.
func convertLinks(links ptrace.SpanLinkSlice) ([]byte, error) {
	if links.Len() == 0 {
		return []byte("[]"), nil
	}
	result := make([]map[string]any, links.Len())
	for i := range links.Len() {
		l := links.At(i)
		attrs, err := attributesToJSON(l.Attributes())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal link attributes: %w", err)
		}
		result[i] = map[string]any{
			"trace_id":    traceutil.TraceIDToHexOrEmptyString(l.TraceID()),
			"span_id":     traceutil.SpanIDToHexOrEmptyString(l.SpanID()),
			"trace_state": l.TraceState().AsRaw(),
			"attributes":  json.RawMessage(attrs),
		}
	}
	return json.Marshal(result)
}
