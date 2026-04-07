// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudmysqlduckdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter"

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal/sqltemplates"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const tracesColumnCount = 17

type tracesExporter struct {
	db               *sql.DB
	insertPrefix     string
	valuePlaceholder string

	logger *zap.Logger
	cfg    *Config
}

func newTracesExporter(logger *zap.Logger, cfg *Config) *tracesExporter {
	return &tracesExporter{
		logger:           logger,
		cfg:              cfg,
		insertPrefix:     internal.BuildInsertPrefix(sqltemplates.TracesInsert, cfg.Database, cfg.TracesTableName),
		valuePlaceholder: internal.BuildValuePlaceholder(tracesColumnCount),
	}
}

func (e *tracesExporter) start(ctx context.Context, _ component.Host) error {
	dsn := e.cfg.buildDSN()

	if e.cfg.CreateSchema {
		initDB, err := internal.NewMySQLClientNoDB(dsn)
		if err != nil {
			return err
		}
		if err := internal.CreateDatabase(ctx, initDB, e.cfg.Database); err != nil {
			_ = initDB.Close()
			return err
		}
		_ = initDB.Close()
	}

	db, err := internal.NewMySQLClient(dsn)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.CreateSchema {
		ddl := renderCreateTableSQL(sqltemplates.TracesCreateTable, e.cfg.Database, e.cfg.TracesTableName)
		if err := internal.CreateTable(ctx, e.db, ddl); err != nil {
			return fmt.Errorf("create traces table: %w", err)
		}
	}

	if e.cfg.TTL > 0 {
		if err := internal.CreateTTLEvent(ctx, e.db, e.cfg.Database, e.cfg.TracesTableName, "timestamp", e.cfg.TTL); err != nil {
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

	rows := make([][]any, 0, td.SpanCount())

	rsSpans := td.ResourceSpans()
	for i := 0; i < rsSpans.Len(); i++ {
		spans := rsSpans.At(i)
		res := spans.Resource()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrJSON := internal.AttributesToJSON(resAttr)

		for j := 0; j < spans.ScopeSpans().Len(); j++ {
			scopeSpan := spans.ScopeSpans().At(j)
			scope := scopeSpan.Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				spanStatus := span.Status()
				durationNanos := span.EndTimestamp() - span.StartTimestamp()
				spanAttrJSON := internal.AttributesToJSON(span.Attributes())
				eventsJSON := convertEventsToJSON(span.Events())
				linksJSON := convertLinksToJSON(span.Links())

				rows = append(rows, []any{
					span.StartTimestamp().AsTime(),
					traceutil.TraceIDToHexOrEmptyString(span.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(span.SpanID()),
					traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()),
					span.TraceState().AsRaw(),
					span.Name(),
					span.Kind().String(),
					serviceName,
					resAttrJSON,
					scopeName,
					scopeVersion,
					spanAttrJSON,
					durationNanos,
					spanStatus.Code().String(),
					spanStatus.Message(),
					eventsJSON,
					linksJSON,
				})
			}
		}
	}

	if err := internal.BatchInsert(ctx, e.db, e.insertPrefix, e.valuePlaceholder, rows, internal.DefaultBatchSize); err != nil {
		return fmt.Errorf("batch insert traces: %w", err)
	}

	duration := time.Since(start)
	e.logger.Debug("insert traces",
		zap.Int("records", len(rows)),
		zap.String("cost", duration.String()))

	return nil
}

type eventJSON struct {
	Timestamp  time.Time         `json:"timestamp"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type linkJSON struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	TraceState string            `json:"trace_state"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func convertEventsToJSON(events ptrace.SpanEventSlice) []byte {
	n := events.Len()
	if n == 0 {
		return nil
	}
	result := make([]eventJSON, 0, n)
	for i := 0; i < n; i++ {
		event := events.At(i)
		attrs := make(map[string]string)
		event.Attributes().Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})
		result = append(result, eventJSON{
			Timestamp:  event.Timestamp().AsTime(),
			Name:       event.Name(),
			Attributes: attrs,
		})
	}
	b, _ := json.Marshal(result)
	return b
}

func convertLinksToJSON(links ptrace.SpanLinkSlice) []byte {
	n := links.Len()
	if n == 0 {
		return nil
	}
	result := make([]linkJSON, 0, n)
	for i := 0; i < n; i++ {
		link := links.At(i)
		attrs := make(map[string]string)
		link.Attributes().Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})
		result = append(result, linkJSON{
			TraceID:    traceutil.TraceIDToHexOrEmptyString(link.TraceID()),
			SpanID:     traceutil.SpanIDToHexOrEmptyString(link.SpanID()),
			TraceState: link.TraceState().AsRaw(),
			Attributes: attrs,
		})
	}
	b, _ := json.Marshal(result)
	return b
}
