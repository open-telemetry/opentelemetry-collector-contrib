// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starrocksexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/sqltemplates"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type tracesExporter struct {
	db        *sql.DB
	insertSQL string

	logger *zap.Logger
	cfg    *Config
}

func newTracesExporter(logger *zap.Logger, cfg *Config) *tracesExporter {
	return &tracesExporter{
		insertSQL: renderInsertTracesSQL(cfg),
		logger:    logger,
		cfg:       cfg,
	}
}

func (e *tracesExporter) start(ctx context.Context, _ component.Host) error {
	db, err := e.cfg.buildStarRocksDB()
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.shouldCreateSchema() {
		if err := internal.CreateDatabase(ctx, e.db, e.cfg.database()); err != nil {
			return err
		}

		if err := createTraceTables(ctx, e.cfg, e.db); err != nil {
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
	processStart := time.Now()

	var spanCount int
	rsSpans := td.ResourceSpans()
	rsLen := rsSpans.Len()
	for i := range rsLen {
		spans := rsSpans.At(i)
		res := spans.Resource()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrJSON, err := internal.AttributesToJSON(res.Attributes())
		if err != nil {
			return fmt.Errorf("failed to convert resource attributes to JSON: %w", err)
		}

		ssRootLen := spans.ScopeSpans().Len()
		for j := range ssRootLen {
			scopeSpanRoot := spans.ScopeSpans().At(j)
			scopeSpanScope := scopeSpanRoot.Scope()
			scopeName := scopeSpanScope.Name()
			scopeVersion := scopeSpanScope.Version()
			scopeSpans := scopeSpanRoot.Spans()

			ssLen := scopeSpans.Len()
			for k := range ssLen {
				span := scopeSpans.At(k)
				spanStatus := span.Status()
				spanDurationNanos := span.EndTimestamp() - span.StartTimestamp()
				spanAttrJSON, err := internal.AttributesToJSON(span.Attributes())
				if err != nil {
					return fmt.Errorf("failed to convert span attributes to JSON: %w", err)
				}

				eventsJSON, err := convertEventsJSON(span.Events())
				if err != nil {
					return fmt.Errorf("failed to convert events: %w", err)
				}
				linksJSON, err := convertLinksJSON(span.Links())
				if err != nil {
					return fmt.Errorf("failed to convert links: %w", err)
				}

				values := []interface{}{
					serviceName,
					span.Name(),
					span.StartTimestamp().AsTime(),
					traceutil.TraceIDToHexOrEmptyString(span.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(span.SpanID()),
					traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()),
					span.TraceState().AsRaw(),
					span.Kind().String(),
					resAttrJSON,
					scopeName,
					scopeVersion,
					spanAttrJSON,
					spanDurationNanos,
					spanStatus.Code().String(),
					spanStatus.Message(),
					eventsJSON,
					linksJSON,
				}

				// Build complete INSERT statement with formatted values
				// Replace the VALUES (?, ?, ...) part with actual values
				insertSQL := strings.Replace(e.insertSQL, "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", "VALUES "+internal.BuildValuesClause(values), 1)
				_, execErr := e.db.ExecContext(ctx, insertSQL)
				if execErr != nil {
					return fmt.Errorf("failed to execute trace insert: %w", execErr)
				}

				spanCount++
			}
		}
	}

	processDuration := time.Since(processStart)
	e.logger.Debug("insert traces",
		zap.Int("records", spanCount),
		zap.String("process_cost", processDuration.String()))

	return nil
}

func convertEventsJSON(events ptrace.SpanEventSlice) (string, error) {
	eventList := make([]map[string]interface{}, 0, events.Len())
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)
		eventMap := map[string]interface{}{
			"timestamp": event.Timestamp().AsTime(),
			"name":      event.Name(),
		}
		attrs, err := internal.AttributesToJSON(event.Attributes())
		if err == nil {
			var attrsMap map[string]interface{}
			if json.Unmarshal([]byte(attrs), &attrsMap) == nil {
				eventMap["attributes"] = attrsMap
			}
		}
		eventList = append(eventList, eventMap)
	}
	jsonBytes, err := json.Marshal(eventList)
	if err != nil {
		return "[]", err
	}
	return string(jsonBytes), nil
}

func convertLinksJSON(links ptrace.SpanLinkSlice) (string, error) {
	linkList := make([]map[string]interface{}, 0, links.Len())
	for i := 0; i < links.Len(); i++ {
		link := links.At(i)
		linkMap := map[string]interface{}{
			"trace_id":    traceutil.TraceIDToHexOrEmptyString(link.TraceID()),
			"span_id":     traceutil.SpanIDToHexOrEmptyString(link.SpanID()),
			"trace_state": link.TraceState().AsRaw(),
		}
		attrs, err := internal.AttributesToJSON(link.Attributes())
		if err == nil {
			var attrsMap map[string]interface{}
			if json.Unmarshal([]byte(attrs), &attrsMap) == nil {
				linkMap["attributes"] = attrsMap
			}
		}
		linkList = append(linkList, linkMap)
	}
	jsonBytes, err := json.Marshal(linkList)
	if err != nil {
		return "[]", err
	}
	return string(jsonBytes), nil
}

func renderInsertTracesSQL(cfg *Config) string {
	return fmt.Sprintf(sqltemplates.TracesInsert, cfg.database(), cfg.TracesTableName)
}

func renderCreateTracesTableSQL(cfg *Config) string {
	return fmt.Sprintf(sqltemplates.TracesCreateTable,
		cfg.database(), cfg.TracesTableName,
	)
}

func createTraceTables(ctx context.Context, cfg *Config, db *sql.DB) error {
	sql := renderCreateTracesTableSQL(cfg)
	_, err := db.ExecContext(ctx, sql)
	if err != nil {
		return fmt.Errorf("exec create traces table sql: %w", err)
	}

	return nil
}
