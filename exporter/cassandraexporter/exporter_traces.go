// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"context"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type tracesExporter struct {
	client *gocql.Session
	exec   queryExecutor
	logger *zap.Logger
	cfg    *Config
}

func newTracesExporter(logger *zap.Logger, cfg *Config) *tracesExporter {
	return &tracesExporter{logger: logger, cfg: cfg}
}

func (e *tracesExporter) Start(ctx context.Context, _ component.Host) error {
	if err := initializeTraceKernel(ctx, e.cfg); err != nil {
		return err
	}
	session, err := newDataSession(e.cfg)
	if err != nil {
		return err
	}
	e.client = session
	e.exec = gocqlExecutor{session: session}
	return nil
}

func (e *tracesExporter) Shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.Close()
	}
	return nil
}

func (e *tracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	start := time.Now()
	if err := writeRows(ctx, e.exec, parseInsertSpanSQL(e.cfg), spanRows(td)); err != nil {
		return err
	}
	e.logger.Debug("insert traces", zap.Int("records", td.SpanCount()), zap.String("cost", time.Since(start).String()))
	return nil
}

func spanRows(td ptrace.Traces) []insertRow {
	rows := make([]insertRow, 0, td.SpanCount())
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpans := td.ResourceSpans().At(i)
		resourceAttrs := attributesToMap(resourceSpans.Resource().Attributes().AsRaw())
		for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
			spans := resourceSpans.ScopeSpans().At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				traceID := traceutil.TraceIDToHexOrEmptyString(span.TraceID())
				spanID := traceutil.SpanIDToHexOrEmptyString(span.SpanID())
				partition := traceID
				if partition == "" {
					partition = spanID
				}
				status := span.Status()
				rows = append(rows, insertRow{
					partition: partition,
					args: []any{
						span.StartTimestamp().AsTime(),
						traceID,
						spanID,
						traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()),
						span.TraceState().AsRaw(),
						span.Name(),
						traceutil.SpanKindStr(span.Kind()),
						resourceAttrs,
						attributesToMap(span.Attributes().AsRaw()),
						span.EndTimestamp().AsTime().Sub(span.StartTimestamp().AsTime()).Nanoseconds(),
						traceutil.StatusCodeStr(status.Code()),
						status.Message(),
						eventsFromSpan(span),
						linksFromSpan(span),
					},
				})
			}
		}
	}
	return rows
}
