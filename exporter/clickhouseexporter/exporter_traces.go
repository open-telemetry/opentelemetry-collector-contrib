// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
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
	opt, err := e.cfg.buildClickHouseOptions()
	if err != nil {
		return err
	}

	e.db, err = internal.NewClickhouseClientFromOptions(opt, e.cfg.shouldCreateSchema())
	if err != nil {
		return err
	}

	if e.cfg.shouldCreateSchema() {
		if err := internal.CreateDatabase(ctx, e.db, e.cfg.database(), e.cfg.clusterString()); err != nil {
			return err
		}

		if err := createTraceTables(ctx, e.cfg, e.db, e.logger); err != nil {
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

			ssLen := scopeSpans.Len()
			for k := range ssLen {
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

func renderInsertTracesSQL(cfg *Config) string {
	return fmt.Sprintf(sqltemplates.TracesInsert, cfg.database(), cfg.TracesTableName)
}

func renderCreateTracesTableSQL(cfg *Config, hasFullTextSearch bool) (string, error) {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "toDateTime(Timestamp)")
	data := sqltemplates.CreateTableData{
		Database:          cfg.database(),
		TableName:         cfg.TracesTableName,
		ClusterString:     cfg.clusterString(),
		Engine:            cfg.tableEngineString(),
		TTL:               ttlExpr,
		HasFullTextSearch: hasFullTextSearch,
	}

	var buf bytes.Buffer
	if err := sqltemplates.TracesCreateTableTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute traces create table template: %w", err)
	}

	return buf.String(), nil
}

func renderCreateTraceIDTsTableSQL(cfg *Config) (string, error) {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "toDateTime(Start)")
	data := sqltemplates.CreateTableData{
		Database:      cfg.database(),
		TableName:     cfg.TracesTableName + "_trace_id_ts",
		ClusterString: cfg.clusterString(),
		Engine:        cfg.traceIDTsTableEngineString(),
		TTL:           ttlExpr,
	}

	var buf bytes.Buffer
	if err := sqltemplates.TracesCreateTsTableTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute trace ID timestamp table template: %w", err)
	}

	return buf.String(), nil
}

func renderTraceIDTsMaterializedViewSQL(cfg *Config) (string, error) {
	data := sqltemplates.TracesTsMVData{
		Database:        cfg.database(),
		ViewName:        cfg.TracesTableName + "_trace_id_ts_mv",
		TableName:       cfg.TracesTableName + "_trace_id_ts",
		SourceTableName: cfg.TracesTableName,
		ClusterString:   cfg.clusterString(),
	}

	var buf bytes.Buffer
	if err := sqltemplates.TracesCreateTsViewTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute trace ID timestamp view template: %w", err)
	}

	return buf.String(), nil
}

func createTraceIDTsTable(ctx context.Context, cfg *Config, db driver.Conn) error {
	tsTableSQL, err := renderCreateTraceIDTsTableSQL(cfg)
	if err != nil {
		return err
	}
	tsViewSQL, err := renderTraceIDTsMaterializedViewSQL(cfg)
	if err != nil {
		return err
	}

	if err := db.Exec(ctx, tsTableSQL); err != nil {
		return fmt.Errorf("exec create traceID timestamp table sql: %w", err)
	}
	if err := db.Exec(ctx, tsViewSQL); err != nil {
		return fmt.Errorf("exec create traceID timestamp view sql: %w", err)
	}

	return nil
}

func createTraceTables(ctx context.Context, cfg *Config, db driver.Conn, logger *zap.Logger) error {
	hasFullTextSearch := false
	sv, err := db.ServerVersion()
	if err != nil {
		logger.Warn("failed to get ClickHouse server version, falling back to bloom filter indexes", zap.Error(err))
	} else {
		hasFullTextSearch = proto.CheckMinVersion(versionFullTextSearch, sv.Version)
	}

	tracesSQL, err := renderCreateTracesTableSQL(cfg, hasFullTextSearch)
	if err != nil {
		return err
	}

	if err := db.Exec(ctx, tracesSQL); err != nil {
		return fmt.Errorf("exec create traces table sql: %w", err)
	}

	return createTraceIDTsTable(ctx, cfg, db)
}
