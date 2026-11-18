// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type tracesExporter struct {
	db             driver.Conn
	insertSQL      string
	schemaFeatures struct {
		ScopeAttributes bool
	}

	logger *zap.Logger
	cfg    *Config
}

func newTracesExporter(logger *zap.Logger, cfg *Config) *tracesExporter {
	return &tracesExporter{
		logger: logger,
		cfg:    cfg,
	}
}

func (e *tracesExporter) start(ctx context.Context, _ component.Host) error {
	opt, err := e.cfg.buildClickHouseOptions()
	if err != nil {
		return err
	}

	e.db, err = internal.NewClickhouseClientFromOptions(opt, e.cfg.shouldCreateSchema())
	if err != nil {
		return err
	}

	if e.cfg.shouldCreateSchema() {
		if createDBErr := internal.CreateDatabase(ctx, e.db, e.cfg.database(), e.cfg.clusterString(), e.cfg.databaseEngineString()); createDBErr != nil {
			return createDBErr
		}

		if createTableErr := createTraceTables(ctx, e.cfg, e.db); createTableErr != nil {
			return createTableErr
		}
	}

	err = e.detectSchemaFeatures(ctx)
	if err != nil {
		e.logger.Error("schema detection failed", zap.Error(err))
	}

	e.renderInsertTracesSQL()

	return nil
}

const (
	tracesColumnScopeAttributes = "ScopeAttributes"
)

func (e *tracesExporter) detectSchemaFeatures(ctx context.Context) error {
	columnNames, err := internal.GetTableColumns(ctx, e.db, e.cfg.database(), e.cfg.TracesTableName)
	if err != nil {
		return err
	}

	for _, name := range columnNames {
		if name == tracesColumnScopeAttributes {
			e.schemaFeatures.ScopeAttributes = true
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
	batch, err := e.db.PrepareBatch(ctx, e.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		if closeErr := batch.Close(); closeErr != nil {
			e.logger.Warn("failed to close traces batch", zap.Error(closeErr))
		}
	}(batch)

	processStart := time.Now()

	var spanCount int
	// 23 matches the max number of columns in the insert statement.
	// If you add or remove columns, update this value.
	columnValues := make([]any, 0, 23)
	rsSpans := td.ResourceSpans()
	rsLen := rsSpans.Len()
	for i := range rsLen {
		spans := rsSpans.At(i)
		res := spans.Resource()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrMap := internal.AttributesToMap(res.Attributes())

		ssRootLen := spans.ScopeSpans().Len()
		for j := range ssRootLen {
			scopeSpanRoot := spans.ScopeSpans().At(j)
			scopeSpanScope := scopeSpanRoot.Scope()
			scopeName := scopeSpanScope.Name()
			scopeVersion := scopeSpanScope.Version()
			scopeSpans := scopeSpanRoot.Spans()

			var scopeAttrMap column.IterableOrderedMap
			if e.schemaFeatures.ScopeAttributes {
				scopeAttrMap = internal.AttributesToMap(scopeSpanScope.Attributes())
			}

			ssLen := scopeSpans.Len()
			for k := range ssLen {
				span := scopeSpans.At(k)
				spanStatus := span.Status()
				spanDurationNanos := span.EndTimestamp() - span.StartTimestamp()
				spanAttrMap := internal.AttributesToMap(span.Attributes())

				eventTimes, eventNames, eventAttrs := convertEvents(span.Events())
				linksTraceIDs, linksSpanIDs, linksTraceStates, linksAttrs := convertLinks(span.Links())

				columnValues = columnValues[:0]
				columnValues = append(columnValues,
					span.StartTimestamp().AsTime(),
					traceutil.TraceIDToHexOrEmptyString(span.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(span.SpanID()),
					traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()),
					span.TraceState().AsRaw(),
					span.Name(),
					span.Kind().String(),
					serviceName,
					resAttrMap,
					scopeName,
					scopeVersion,
					spanAttrMap,
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

				if e.schemaFeatures.ScopeAttributes {
					columnValues = append(columnValues, scopeAttrMap)
				}

				appendErr := batch.Append(columnValues...)
				if appendErr != nil {
					return fmt.Errorf("failed to append trace row: %w", appendErr)
				}

				spanCount++
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("traces insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	e.logger.Debug("insert traces",
		zap.Int("records", spanCount),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func convertEvents(events ptrace.SpanEventSlice) (times []time.Time, names []string, attrs []column.IterableOrderedMap) {
	n := events.Len()
	if n == 0 {
		return nil, nil, nil
	}
	times = make([]time.Time, 0, n)
	names = make([]string, 0, n)
	attrs = make([]column.IterableOrderedMap, 0, n)
	for i := range n {
		event := events.At(i)
		times = append(times, event.Timestamp().AsTime())
		names = append(names, event.Name())
		attrs = append(attrs, internal.AttributesToMap(event.Attributes()))
	}

	return times, names, attrs
}

func convertLinks(links ptrace.SpanLinkSlice) (traceIDs, spanIDs, states []string, attrs []column.IterableOrderedMap) {
	n := links.Len()
	if n == 0 {
		return nil, nil, nil, nil
	}
	traceIDs = make([]string, 0, n)
	spanIDs = make([]string, 0, n)
	states = make([]string, 0, n)
	attrs = make([]column.IterableOrderedMap, 0, n)
	for i := range n {
		link := links.At(i)
		traceIDs = append(traceIDs, traceutil.TraceIDToHexOrEmptyString(link.TraceID()))
		spanIDs = append(spanIDs, traceutil.SpanIDToHexOrEmptyString(link.SpanID()))
		states = append(states, link.TraceState().AsRaw())
		attrs = append(attrs, internal.AttributesToMap(link.Attributes()))
	}

	return traceIDs, spanIDs, states, attrs
}

func (e *tracesExporter) renderInsertTracesSQL() {
	var featureColumnNames strings.Builder
	var featureColumnPositions strings.Builder

	if e.schemaFeatures.ScopeAttributes {
		featureColumnNames.WriteString(", ")
		featureColumnNames.WriteString(tracesColumnScopeAttributes)

		featureColumnPositions.WriteString(", ?")
	}

	e.insertSQL = fmt.Sprintf(sqltemplates.TracesInsert, e.cfg.database(), e.cfg.TracesTableName, featureColumnNames.String(), featureColumnPositions.String())
}

func renderCreateTracesTableSQL(cfg *Config) string {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "toDateTime(Timestamp)")
	return fmt.Sprintf(sqltemplates.TracesCreateTable,
		cfg.database(), cfg.TracesTableName, cfg.tableClusterString(),
		cfg.tableEngineString(),
		ttlExpr,
	)
}

func renderCreateTraceIDTsTableSQL(cfg *Config) string {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "toDateTime(Start)")
	return fmt.Sprintf(sqltemplates.TracesCreateTsTable,
		cfg.database(), cfg.TracesTableName, cfg.tableClusterString(),
		cfg.tableEngineString(),
		ttlExpr,
	)
}

func renderTraceIDTsMaterializedViewSQL(cfg *Config) string {
	database := cfg.database()
	return fmt.Sprintf(sqltemplates.TracesCreateTsView,
		database, cfg.TracesTableName, cfg.tableClusterString(),
		database, cfg.TracesTableName,
		database, cfg.TracesTableName,
	)
}

func createTraceTables(ctx context.Context, cfg *Config, db driver.Conn) error {
	if err := db.Exec(ctx, renderCreateTracesTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create traces table sql: %w", err)
	}
	if err := db.Exec(ctx, renderCreateTraceIDTsTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create traceID timestamp table sql: %w", err)
	}
	if err := db.Exec(ctx, renderTraceIDTsMaterializedViewSQL(cfg)); err != nil {
		return fmt.Errorf("exec create traceID timestamp view sql: %w", err)
	}

	return nil
}
