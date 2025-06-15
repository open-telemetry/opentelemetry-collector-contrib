// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"fmt"
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
	db        driver.Conn
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
	dsn, err := e.cfg.buildDSN()
	if err != nil {
		return err
	}

	e.db, err = internal.NewClickhouseClient(dsn)
	if err != nil {
		return err
	}

	if e.cfg.shouldCreateSchema() {
		if err := internal.CreateDatabase(ctx, e.db, e.cfg.database(), e.cfg.clusterString()); err != nil {
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
	rsSpans := td.ResourceSpans()
	rsLen := rsSpans.Len()
	for i := 0; i < rsLen; i++ {
		spans := rsSpans.At(i)
		res := spans.Resource()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrMap := internal.AttributesToMap(res.Attributes())

		ssRootLen := spans.ScopeSpans().Len()
		for j := 0; j < ssRootLen; j++ {
			scopeSpanRoot := spans.ScopeSpans().At(j)
			scopeSpanScope := scopeSpanRoot.Scope()
			scopeName := scopeSpanScope.Name()
			scopeVersion := scopeSpanScope.Version()
			scopeSpans := scopeSpanRoot.Spans()

			ssLen := scopeSpans.Len()
			for k := 0; k < ssLen; k++ {
				span := scopeSpans.At(k)
				spanStatus := span.Status()
				spanDurationNanos := span.EndTimestamp() - span.StartTimestamp()
				spanAttrMap := internal.AttributesToMap(span.Attributes())

				eventTimes, eventNames, eventAttrs := convertEvents(span.Events())
				linksTraceIDs, linksSpanIDs, linksTraceStates, linksAttrs := convertLinks(span.Links())

				appendErr := batch.Append(
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
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)
		times = append(times, event.Timestamp().AsTime())
		names = append(names, event.Name())
		attrs = append(attrs, internal.AttributesToMap(event.Attributes()))
	}

	return
}

func convertLinks(links ptrace.SpanLinkSlice) (traceIDs []string, spanIDs []string, states []string, attrs []column.IterableOrderedMap) {
	for i := 0; i < links.Len(); i++ {
		link := links.At(i)
		traceIDs = append(traceIDs, traceutil.TraceIDToHexOrEmptyString(link.TraceID()))
		spanIDs = append(spanIDs, traceutil.SpanIDToHexOrEmptyString(link.SpanID()))
		states = append(states, link.TraceState().AsRaw())
		attrs = append(attrs, internal.AttributesToMap(link.Attributes()))
	}

	return
}

func renderInsertTracesSQL(cfg *Config) string {
	return fmt.Sprintf(sqltemplates.TracesInsert, cfg.database(), cfg.TracesTableName)
}

func renderCreateTracesTableSQL(cfg *Config) string {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "toDateTime(Timestamp)")
	return fmt.Sprintf(sqltemplates.TracesCreateTable,
		cfg.database(), cfg.TracesTableName, cfg.clusterString(),
		cfg.tableEngineString(),
		ttlExpr,
	)
}

func renderCreateTraceIDTsTableSQL(cfg *Config) string {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "toDateTime(Start)")
	return fmt.Sprintf(sqltemplates.TracesCreateTsTable,
		cfg.database(), cfg.TracesTableName, cfg.clusterString(),
		cfg.tableEngineString(),
		ttlExpr,
	)
}

func renderTraceIDTsMaterializedViewSQL(cfg *Config) string {
	database := cfg.database()
	return fmt.Sprintf(sqltemplates.TracesCreateTsView,
		database, cfg.TracesTableName, cfg.clusterString(),
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
