// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/chjson"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"
)

type tracesJSONExporter struct {
	cfg       *Config
	logger    *zap.Logger
	db        driver.Conn
	insertSQL string

	resourceAttributesBufferPool *internal.ExporterStructPool[*chjson.JSONBuffer]
	spanAttributesBufferPool     *internal.ExporterStructPool[*chjson.JSONBuffer]
	eventsAttributesBufferPool   *internal.ExporterStructPool[*chjson.JSONBuffer]
	linksAttributesBufferPool    *internal.ExporterStructPool[*chjson.JSONBuffer]
	traceHexBufferPool           *internal.ExporterStructPool[[]byte]
	spanHexBufferPool            *internal.ExporterStructPool[[]byte]
	parentSpanHexBufferPool      *internal.ExporterStructPool[[]byte]
	linksHexBufferPool           *internal.ExporterStructPool[[]byte]
}

func newTracesJSONExporter(logger *zap.Logger, cfg *Config) (*tracesJSONExporter, error) {
	numConsumers := cfg.QueueSettings.NumConsumers

	newJSONBuffer := func() (*chjson.JSONBuffer, error) {
		return chjson.NewJSONBuffer(2048, 256), nil
	}
	newHexBuffer := func() ([]byte, error) {
		return make([]byte, 0, 128), nil
	}

	resourceAttributesBufferPool, _ := internal.NewExporterStructPool[*chjson.JSONBuffer](numConsumers, newJSONBuffer)
	spanAttributesBufferPool, _ := internal.NewExporterStructPool[*chjson.JSONBuffer](numConsumers, newJSONBuffer)
	eventsAttributesBufferPool, _ := internal.NewExporterStructPool[*chjson.JSONBuffer](numConsumers, newJSONBuffer)
	linksAttributesBufferPool, _ := internal.NewExporterStructPool[*chjson.JSONBuffer](numConsumers, newJSONBuffer)
	traceHexBufferPool, _ := internal.NewExporterStructPool[[]byte](numConsumers, newHexBuffer)
	spanHexBufferPool, _ := internal.NewExporterStructPool[[]byte](numConsumers, newHexBuffer)
	parentSpanHexBufferPool, _ := internal.NewExporterStructPool[[]byte](numConsumers, newHexBuffer)
	linksHexBufferPool, _ := internal.NewExporterStructPool[[]byte](numConsumers, newHexBuffer)

	return &tracesJSONExporter{
		cfg:                          cfg,
		logger:                       logger,
		insertSQL:                    renderInsertTracesJSONSQL(cfg),
		resourceAttributesBufferPool: resourceAttributesBufferPool,
		spanAttributesBufferPool:     spanAttributesBufferPool,
		eventsAttributesBufferPool:   eventsAttributesBufferPool,
		linksAttributesBufferPool:    linksAttributesBufferPool,
		traceHexBufferPool:           traceHexBufferPool,
		spanHexBufferPool:            spanHexBufferPool,
		parentSpanHexBufferPool:      parentSpanHexBufferPool,
		linksHexBufferPool:           linksHexBufferPool,
	}, nil
}

func (e *tracesJSONExporter) start(ctx context.Context, _ component.Host) error {
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

		if err := createTraceJSONTables(ctx, e.cfg, e.db); err != nil {
			return err
		}
	}

	return nil
}

func (e *tracesJSONExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		if err := e.db.Close(); err != nil {
			e.logger.Warn("failed to close json traces db connection", zap.Error(err))
		}
	}

	e.resourceAttributesBufferPool.Destroy()
	e.spanAttributesBufferPool.Destroy()
	e.eventsAttributesBufferPool.Destroy()
	e.linksAttributesBufferPool.Destroy()
	e.traceHexBufferPool.Destroy()
	e.spanHexBufferPool.Destroy()
	e.parentSpanHexBufferPool.Destroy()
	e.linksHexBufferPool.Destroy()

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

	resourceAttributesBuffer := e.resourceAttributesBufferPool.Acquire()
	defer e.resourceAttributesBufferPool.Release(resourceAttributesBuffer)
	spanAttributesBuffer := e.spanAttributesBufferPool.Acquire()
	defer e.spanAttributesBufferPool.Release(spanAttributesBuffer)
	eventsAttributesBuffer := e.eventsAttributesBufferPool.Acquire()
	defer e.eventsAttributesBufferPool.Release(eventsAttributesBuffer)
	linksAttributesBuffer := e.linksAttributesBufferPool.Acquire()
	defer e.linksAttributesBufferPool.Release(linksAttributesBuffer)
	traceHexBuffer := e.traceHexBufferPool.Acquire()
	defer e.traceHexBufferPool.Release(traceHexBuffer)
	spanHexBuffer := e.spanHexBufferPool.Acquire()
	defer e.spanHexBufferPool.Release(spanHexBuffer)
	parentSpanHexBuffer := e.parentSpanHexBufferPool.Acquire()
	defer e.parentSpanHexBufferPool.Release(parentSpanHexBuffer)
	linksHexBuffer := e.linksHexBufferPool.Acquire()
	defer e.linksHexBufferPool.Release(linksHexBuffer)

	processStart := time.Now()

	var spanCount int
	rsSpans := td.ResourceSpans()
	rsLen := rsSpans.Len()
	for i := 0; i < rsLen; i++ {
		spans := rsSpans.At(i)
		res := spans.Resource()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resourceAttributesBuffer.Reset()
		chjson.AttributesToJSON(resourceAttributesBuffer, resAttr)

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
				spanAttributesBuffer.Reset()
				chjson.AttributesToJSON(spanAttributesBuffer, span.Attributes())

				eventTimes, eventNames, eventAttrs := convertEventsJSON(span.Events(), eventsAttributesBuffer)
				linksTraceIDs, linksSpanIDs, linksTraceStates, linksAttrs := convertLinksJSON(span.Links(), linksHexBuffer, linksAttributesBuffer)

				traceHexBuffer = chjson.AppendTraceIDToHex(traceHexBuffer[:0], span.TraceID())
				spanHexBuffer = chjson.AppendSpanIDToHex(spanHexBuffer[:0], span.SpanID())
				parentSpanHexBuffer = chjson.AppendSpanIDToHex(parentSpanHexBuffer[:0], span.ParentSpanID())

				appendErr := batch.Append(
					span.StartTimestamp().AsTime(),
					traceHexBuffer,
					spanHexBuffer,
					parentSpanHexBuffer,
					span.TraceState().AsRaw(),
					span.Name(),
					span.Kind().String(),
					serviceName,
					resourceAttributesBuffer.Bytes(),
					scopeName,
					scopeVersion,
					spanAttributesBuffer.Bytes(),
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

func convertEventsJSON(events ptrace.SpanEventSlice, attrBuffer *chjson.JSONBuffer) (times []time.Time, names []string, attrs []string) {
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)
		times = append(times, event.Timestamp().AsTime())
		names = append(names, event.Name())

		attrBuffer.Reset()
		chjson.AttributesToJSON(attrBuffer, event.Attributes())
		attrs = append(attrs, string(attrBuffer.Bytes()))
	}

	return
}

func convertLinksJSON(links ptrace.SpanLinkSlice, linksHexBuffer []byte, attrBuffer *chjson.JSONBuffer) (traceIDs []string, spanIDs []string, states []string, attrs []string) {
	for i := 0; i < links.Len(); i++ {
		link := links.At(i)
		linksHexBuffer = chjson.AppendTraceIDToHex(linksHexBuffer[:0], link.TraceID())
		traceIDs = append(traceIDs, string(linksHexBuffer))
		linksHexBuffer = chjson.AppendSpanIDToHex(linksHexBuffer[:0], link.SpanID())
		spanIDs = append(spanIDs, string(linksHexBuffer))
		states = append(states, link.TraceState().AsRaw())

		attrBuffer.Reset()
		chjson.AttributesToJSON(attrBuffer, link.Attributes())
		attrs = append(attrs, string(attrBuffer.Bytes()))
	}

	return
}

func renderInsertTracesJSONSQL(cfg *Config) string {
	return fmt.Sprintf(sqltemplates.TracesJSONInsert, cfg.database(), cfg.TracesTableName)
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
