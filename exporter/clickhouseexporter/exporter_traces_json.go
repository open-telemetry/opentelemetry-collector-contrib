// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"
)

// anyTracesExporter is an interface that satisfies both the default map tracesExporter and the tracesJSONExporter
type anyTracesExporter interface {
	start(context.Context, component.Host) error
	shutdown(context.Context) error
	pushTraceData(ctx context.Context, td ptrace.Traces) error
}

type tracesJSONExporter struct {
	cfg            *Config
	logger         *zap.Logger
	db             driver.Conn
	insertSQL      string
	schemaFeatures struct {
		AttributeKeys bool
	}
}

func newTracesJSONExporter(logger *zap.Logger, cfg *Config) *tracesJSONExporter {
	return &tracesJSONExporter{
		cfg:    cfg,
		logger: logger,
	}
}

func (e *tracesJSONExporter) start(ctx context.Context, _ component.Host) error {
	opt, err := e.cfg.buildClickHouseOptions()
	if err != nil {
		return err
	}

	e.db, err = internal.NewClickhouseClientFromOptions(opt)
	if err != nil {
		return err
	}

	if e.cfg.shouldCreateSchema() {
		if createDBErr := internal.CreateDatabase(ctx, e.db, e.cfg.database(), e.cfg.clusterString()); createDBErr != nil {
			return createDBErr
		}

		if createTableErr := createTraceJSONTables(ctx, e.cfg, e.db); createTableErr != nil {
			return createTableErr
		}
	}

	err = e.detectSchemaFeatures(ctx)
	if err != nil {
		return fmt.Errorf("schema detection: %w", err)
	}

	e.renderInsertTracesJSONSQL()

	return nil
}

const (
	tracesJSONColumnResourceAttributesKeys = "ResourceAttributesKeys"
	tracesJSONColumnSpanAttributesKeys     = "SpanAttributesKeys"
)

func (e *tracesJSONExporter) detectSchemaFeatures(ctx context.Context) error {
	columnNames, err := internal.GetTableColumns(ctx, e.db, e.cfg.database(), e.cfg.TracesTableName)
	if err != nil {
		return err
	}

	for _, name := range columnNames {
		switch name {
		case tracesJSONColumnResourceAttributesKeys:
			e.schemaFeatures.AttributeKeys = true
		case tracesJSONColumnSpanAttributesKeys:
			e.schemaFeatures.AttributeKeys = true
		}
	}

	return nil
}

func (e *tracesJSONExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		if err := e.db.Close(); err != nil {
			e.logger.Warn("failed to close json traces db connection", zap.Error(err))
			return err
		}
	}

	return nil
}

func (e *tracesJSONExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	batch, err := e.db.PrepareBatch(ctx, e.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		if closeErr := batch.Close(); closeErr != nil {
			e.logger.Warn("failed to close json traces batch", zap.Error(closeErr))
		}
	}(batch)

	processStart := time.Now()

	var spanCount int
	rsSpans := td.ResourceSpans()
	rsLen := rsSpans.Len()
	for i := range rsLen {
		spans := rsSpans.At(i)
		res := spans.Resource()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrBytes, resAttrErr := json.Marshal(resAttr.AsRaw())
		if resAttrErr != nil {
			return fmt.Errorf("failed to marshal json trace resource attributes: %w", resAttrErr)
		}

		var resAttrKeys []string
		if e.schemaFeatures.AttributeKeys {
			resAttrKeys = internal.UniqueFlattenedAttributes(resAttr)
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
				spanAttr := span.Attributes()
				spanAttrBytes, spanAttrErr := json.Marshal(spanAttr.AsRaw())
				if spanAttrErr != nil {
					return fmt.Errorf("failed to marshal json trace span attributes: %w", spanAttrErr)
				}

				eventTimes, eventNames, eventAttrs, eventsErr := convertEventsJSON(span.Events())
				if eventsErr != nil {
					return fmt.Errorf("failed to convert json trace events: %w", eventsErr)
				}
				linksTraceIDs, linksSpanIDs, linksTraceStates, linksAttrs, linksErr := convertLinksJSON(span.Links())
				if linksErr != nil {
					return fmt.Errorf("failed to convert json trace links: %w", linksErr)
				}

				columnValues := make([]any, 0, 24)
				columnValues = append(columnValues,
					span.StartTimestamp().AsTime(),
					span.TraceID().String(),
					span.SpanID().String(),
					span.ParentSpanID().String(),
					span.TraceState().AsRaw(),
					span.Name(),
					span.Kind().String(),
					serviceName,
					resAttrBytes,
					scopeName,
					scopeVersion,
					spanAttrBytes,
					spanDurationNanos,
					spanStatus.Code().String(),
					spanStatus.Message(),
					eventTimes,
					eventNames,
					eventAttrs,
					linksTraceIDs,
					linksSpanIDs,
					linksTraceStates,
					linksAttrs,
				)

				if e.schemaFeatures.AttributeKeys {
					spanAttrKeys := internal.UniqueFlattenedAttributes(spanAttr)
					columnValues = append(columnValues, resAttrKeys, spanAttrKeys)
				}

				appendErr := batch.Append(columnValues...)
				if appendErr != nil {
					return fmt.Errorf("failed to append json trace row: %w", appendErr)
				}

				spanCount++
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("traces json insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	e.logger.Debug("insert json traces",
		zap.Int("records", spanCount),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func convertEventsJSON(events ptrace.SpanEventSlice) (times []time.Time, names, attrs []string, err error) {
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)
		times = append(times, event.Timestamp().AsTime())
		names = append(names, event.Name())

		eventAttrBytes, eventAttrErr := json.Marshal(event.Attributes().AsRaw())
		if eventAttrErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to marshal json trace event attributes: %w", eventAttrErr)
		}
		attrs = append(attrs, string(eventAttrBytes))
	}

	return times, names, attrs, err
}

func convertLinksJSON(links ptrace.SpanLinkSlice) (traceIDs, spanIDs, states, attrs []string, err error) {
	for i := 0; i < links.Len(); i++ {
		link := links.At(i)
		traceIDs = append(traceIDs, link.TraceID().String())
		spanIDs = append(spanIDs, link.SpanID().String())
		states = append(states, link.TraceState().AsRaw())

		linkAttrBytes, linkAttrErr := json.Marshal(link.Attributes().AsRaw())
		if linkAttrErr != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to marshal json trace link attributes: %w", linkAttrErr)
		}
		attrs = append(attrs, string(linkAttrBytes))
	}

	return traceIDs, spanIDs, states, attrs, err
}

func (e *tracesJSONExporter) renderInsertTracesJSONSQL() {
	var featureColumnNames strings.Builder
	var featureColumnPositions strings.Builder

	if e.schemaFeatures.AttributeKeys {
		featureColumnNames.WriteString(", ")
		featureColumnNames.WriteString(tracesJSONColumnResourceAttributesKeys)
		featureColumnNames.WriteString(", ")
		featureColumnNames.WriteString(tracesJSONColumnSpanAttributesKeys)

		featureColumnPositions.WriteString(", ?, ?, ?")
	}

	e.insertSQL = fmt.Sprintf(sqltemplates.TracesJSONInsert, e.cfg.database(), e.cfg.TracesTableName, featureColumnNames.String(), featureColumnPositions.String())
}

func renderCreateTracesJSONTableSQL(cfg *Config) string {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "toDateTime(Timestamp)")
	return fmt.Sprintf(sqltemplates.TracesJSONCreateTable,
		cfg.database(), cfg.TracesTableName, cfg.clusterString(),
		cfg.tableEngineString(),
		ttlExpr,
	)
}

func createTraceJSONTables(ctx context.Context, cfg *Config, db driver.Conn) error {
	if err := db.Exec(ctx, renderCreateTracesJSONTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create json traces table sql: %w", err)
	}
	if err := db.Exec(ctx, renderCreateTraceIDTsTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create traceID timestamp table sql: %w", err)
	}
	if err := db.Exec(ctx, renderTraceIDTsMaterializedViewSQL(cfg)); err != nil {
		return fmt.Errorf("exec create traceID timestamp view sql: %w", err)
	}

	return nil
}
