// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2" // For register database driver.
	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type tracesExporter struct {
	client    *sql.DB
	insertSQL string

	logger *zap.Logger
	cfg    *Config
}

func newTracesExporter(logger *zap.Logger, cfg *Config) (*tracesExporter, error) {
	client, err := newClickhouseClient(cfg)
	if err != nil {
		return nil, err
	}

	return &tracesExporter{
		client:    client,
		insertSQL: renderInsertTracesSQL(cfg),
		logger:    logger,
		cfg:       cfg,
	}, nil
}

func (e *tracesExporter) start(ctx context.Context, _ component.Host) error {
	if !e.cfg.shouldCreateSchema() {
		return nil
	}

	if err := createDatabase(ctx, e.cfg); err != nil {
		return err
	}

	return createTracesTable(ctx, e.cfg, e.client)
}

// shutdown will shut down the exporter.
func (e *tracesExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *tracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	start := time.Now()
	err := doWithTx(ctx, e.client, func(tx *sql.Tx) error {
		statement, err := tx.PrepareContext(ctx, e.insertSQL)
		if err != nil {
			return fmt.Errorf("PrepareContext:%w", err)
		}
		defer func() {
			_ = statement.Close()
		}()
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			spans := td.ResourceSpans().At(i)
			res := spans.Resource()
			resAttr := internal.AttributesToMap(res.Attributes())
			serviceName, _ := res.Attributes().Get(conventions.AttributeServiceName)
			for j := 0; j < spans.ScopeSpans().Len(); j++ {
				rs := spans.ScopeSpans().At(j).Spans()
				scopeName := spans.ScopeSpans().At(j).Scope().Name()
				scopeVersion := spans.ScopeSpans().At(j).Scope().Version()
				for k := 0; k < rs.Len(); k++ {
					r := rs.At(k)
					spanAttr := internal.AttributesToMap(r.Attributes())
					status := r.Status()
					eventTimes, eventNames, eventAttrs := convertEvents(r.Events())
					linksTraceIDs, linksSpanIDs, linksTraceStates, linksAttrs := convertLinks(r.Links())
					_, err = statement.ExecContext(ctx,
						r.StartTimestamp().AsTime(),
						traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
						traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
						traceutil.SpanIDToHexOrEmptyString(r.ParentSpanID()),
						r.TraceState().AsRaw(),
						r.Name(),
						r.Kind().String(),
						serviceName.AsString(),
						resAttr,
						scopeName,
						scopeVersion,
						spanAttr,
						r.EndTimestamp().AsTime().Sub(r.StartTimestamp().AsTime()).Nanoseconds(),
						status.Code().String(),
						status.Message(),
						eventTimes,
						eventNames,
						eventAttrs,
						linksTraceIDs,
						linksSpanIDs,
						linksTraceStates,
						linksAttrs,
					)
					if err != nil {
						return fmt.Errorf("ExecContext:%w", err)
					}
				}
			}
		}
		return nil
	})
	duration := time.Since(start)
	e.logger.Debug("insert traces", zap.Int("records", td.SpanCount()),
		zap.String("cost", duration.String()))
	return err
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

const (
	// language=ClickHouse SQL
	createTracesTableSQL = `
CREATE TABLE IF NOT EXISTS %s %s (
	Timestamp DateTime64(9) CODEC(Delta, ZSTD(1)),
	TraceId String CODEC(ZSTD(1)),
	SpanId String CODEC(ZSTD(1)),
	ParentSpanId String CODEC(ZSTD(1)),
	TraceState String CODEC(ZSTD(1)),
	SpanName LowCardinality(String) CODEC(ZSTD(1)),
	SpanKind LowCardinality(String) CODEC(ZSTD(1)),
	ServiceName LowCardinality(String) CODEC(ZSTD(1)),
	ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	ScopeName String CODEC(ZSTD(1)),
	ScopeVersion String CODEC(ZSTD(1)),
	SpanAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	Duration UInt64 CODEC(ZSTD(1)),
	StatusCode LowCardinality(String) CODEC(ZSTD(1)),
	StatusMessage String CODEC(ZSTD(1)),
	Events Nested (
		Timestamp DateTime64(9),
		Name LowCardinality(String),
		Attributes Map(LowCardinality(String), String)
	) CODEC(ZSTD(1)),
	Links Nested (
		TraceId String,
		SpanId String,
		TraceState String,
		Attributes Map(LowCardinality(String), String)
	) CODEC(ZSTD(1)),
	INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
	INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_span_attr_value mapValues(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
	INDEX idx_duration Duration TYPE minmax GRANULARITY 1
) ENGINE = %s
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SpanName, toDateTime(Timestamp))
%s
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
`
	// language=ClickHouse SQL
	insertTracesSQLTemplate = `INSERT INTO %s (
                        Timestamp,
                        TraceId,
                        SpanId,
                        ParentSpanId,
                        TraceState,
                        SpanName,
                        SpanKind,
                        ServiceName,
					    ResourceAttributes,
						ScopeName,
						ScopeVersion,
                        SpanAttributes,
                        Duration,
                        StatusCode,
                        StatusMessage,
                        Events.Timestamp,
                        Events.Name,
                        Events.Attributes,
                        Links.TraceId,
                        Links.SpanId,
                        Links.TraceState,
                        Links.Attributes
                        ) VALUES (
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?
                                  )`
)

const (
	createTraceIDTsTableSQL = `
CREATE TABLE IF NOT EXISTS %s_trace_id_ts %s (
     TraceId String CODEC(ZSTD(1)),
     Start DateTime CODEC(Delta, ZSTD(1)),
     End DateTime CODEC(Delta, ZSTD(1)),
     INDEX idx_trace_id TraceId TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE = %s
PARTITION BY toDate(Start)
ORDER BY (TraceId, Start)
%s
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
`
	createTraceIDTsMaterializedViewSQL = `
CREATE MATERIALIZED VIEW IF NOT EXISTS %s_trace_id_ts_mv %s
TO %s.%s_trace_id_ts
AS SELECT
	TraceId,
	min(Timestamp) as Start,
	max(Timestamp) as End
FROM
%s.%s
WHERE TraceId != ''
GROUP BY TraceId;
`
)

func createTracesTable(ctx context.Context, cfg *Config, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, renderCreateTracesTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create traces table sql: %w", err)
	}
	if _, err := db.ExecContext(ctx, renderCreateTraceIDTsTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create traceID timestamp table sql: %w", err)
	}
	if _, err := db.ExecContext(ctx, renderTraceIDTsMaterializedViewSQL(cfg)); err != nil {
		return fmt.Errorf("exec create traceID timestamp view sql: %w", err)
	}
	return nil
}

func renderInsertTracesSQL(cfg *Config) string {
	return fmt.Sprintf(strings.ReplaceAll(insertTracesSQLTemplate, "'", "`"), cfg.TracesTableName)
}

func renderCreateTracesTableSQL(cfg *Config) string {
	ttlExpr := generateTTLExpr(cfg.TTL, "toDate(Timestamp)")
	return fmt.Sprintf(createTracesTableSQL, cfg.TracesTableName, cfg.clusterString(), cfg.tableEngineString(), ttlExpr)
}

func renderCreateTraceIDTsTableSQL(cfg *Config) string {
	ttlExpr := generateTTLExpr(cfg.TTL, "toDate(Start)")
	return fmt.Sprintf(createTraceIDTsTableSQL, cfg.TracesTableName, cfg.clusterString(), cfg.tableEngineString(), ttlExpr)
}

func renderTraceIDTsMaterializedViewSQL(cfg *Config) string {
	return fmt.Sprintf(createTraceIDTsMaterializedViewSQL, cfg.TracesTableName,
		cfg.clusterString(), cfg.Database, cfg.TracesTableName, cfg.Database, cfg.TracesTableName)
}
