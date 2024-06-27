// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	tracesDDL = `
CREATE TABLE IF NOT EXISTS %s
(
    service_name          VARCHAR(50),
    timestamp             DATETIME(6),
    trace_id              VARCHAR(50),
    span_id               VARCHAR(50),
    trace_state           VARCHAR(50),
    parent_span_id        VARCHAR(50),
    span_name             VARCHAR(50),
    span_kind             VARCHAR(50),
    end_time              DATETIME(6),
    duration              BIGINT,
    span_attributes       VARIANT,
    events                ARRAY<STRUCT<timestamp:DATETIME(6), name:STRING, attributes:MAP<STRING, STRING>>>,
    links                 ARRAY<STRUCT<trace_id:STRING, span_id:STRING, trace_state:STRING, attributes:MAP<STRING, STRING>>>,
    status_message        VARCHAR(50),
    status_code           VARCHAR(50),
    resource_attributes   VARIANT,
    scope_name            VARCHAR(50),
    scope_version         VARCHAR(50)
)
ENGINE = OLAP
DUPLICATE KEY(service_name, timestamp)
PARTITION BY RANGE(timestamp) ()
DISTRIBUTED BY HASH(trace_id) BUCKETS AUTO
PROPERTIES (
"bloom_filter_columns" = "resource_attributes, span_attributes",
"replication_num" = "1",
"compaction_policy" = "time_series",
"enable_single_replica_compaction" = "true",
"dynamic_partition.enable" = "true",
"dynamic_partition.create_history_partition" = "true",
"dynamic_partition.time_unit" = "DAY",
"dynamic_partition.history_partition_num" = "%d",
"dynamic_partition.end" = "1",
"dynamic_partition.prefix" = "p"
);
`
)

type tracesExporter struct {
	client *http.Client

	logger *zap.Logger
	cfg    *Config
}

func newTracesExporter(logger *zap.Logger, cfg *Config) (*tracesExporter, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			req.SetBasicAuth(cfg.Username, cfg.Password)
			return nil
		},
	}

	return &tracesExporter{
		logger: logger,
		cfg:    cfg,
		client: client,
	}, nil
}

func (e *tracesExporter) start(ctx context.Context, _ component.Host) error {
	if !e.cfg.CreateSchema {
		return nil
	}

	conn, err := createMySQLClient(e.cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	err = createAndUseDatabase(ctx, conn, e.cfg)
	if err != nil {
		return err
	}

	ddl := fmt.Sprintf(tracesDDL, e.cfg.Table.Traces, e.cfg.HistoryDays)
	_, err = conn.ExecContext(ctx, ddl)
	return err
}

func (e *tracesExporter) shutdown(ctx context.Context) error {
	e.client.CloseIdleConnections()
	return nil
}

func (e *tracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	traces := make([]*Trace, 0, 256)

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		resource := resourceSpan.Resource()
		resourceAttributes := resource.Attributes()
		serviceName := ""
		v, ok := resourceAttributes.Get(semconv.AttributeServiceName)
		if ok {
			serviceName = v.AsString()
		}

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)

				events := span.Events()
				newEvents := make([]*Event, 0, events.Len())
				for l := 0; l < events.Len(); l++ {
					event := events.At(l)

					newEvent := &Event{
						Timestamp:  event.Timestamp().AsTime().Format(TimeFormat),
						Name:       event.Name(),
						Attributes: event.Attributes().AsRaw(),
					}

					newEvents = append(newEvents, newEvent)
				}

				links := span.Links()
				newLinks := make([]*Link, 0, links.Len())
				for l := 0; l < links.Len(); l++ {
					link := links.At(l)

					newLink := &Link{
						TraceID:    traceutil.TraceIDToHexOrEmptyString(link.TraceID()),
						SpanID:     traceutil.SpanIDToHexOrEmptyString(link.SpanID()),
						TraceState: link.TraceState().AsRaw(),
						Attributes: link.Attributes().AsRaw(),
					}

					newLinks = append(newLinks, newLink)
				}

				trace := &Trace{
					ServiceName:        serviceName,
					Timestamp:          span.StartTimestamp().AsTime().Format(TimeFormat),
					TraceID:            traceutil.TraceIDToHexOrEmptyString(span.TraceID()),
					SpanID:             traceutil.SpanIDToHexOrEmptyString(span.SpanID()),
					TraceState:         span.TraceState().AsRaw(),
					ParentSpanID:       traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()),
					SpanName:           span.Name(),
					SpanKind:           traceutil.SpanKindStr(span.Kind()),
					EndTime:            span.EndTimestamp().AsTime().Format(TimeFormat),
					Duration:           span.EndTimestamp().AsTime().Sub(span.EndTimestamp().AsTime()).Microseconds(),
					SpanAttributes:     span.Attributes().AsRaw(),
					Events:             newEvents,
					Links:              newLinks,
					StatusMessage:      span.Status().Message(),
					StatusCode:         traceutil.StatusCodeStr(span.Status().Code()),
					ResourceAttributes: resourceAttributes.AsRaw(),
					ScopeName:          scopeSpan.Scope().Name(),
					ScopeVersion:       scopeSpan.Scope().Version(),
				}

				traces = append(traces, trace)
			}
		}
	}

	return e.pushTraceDataInternal(ctx, traces)
}

func (e *tracesExporter) pushTraceDataInternal(ctx context.Context, traces []*Trace) error {
	marshal, err := json.Marshal(traces)
	if err != nil {
		return err
	}

	req, err := streamLoadRequest(ctx, e.cfg, e.cfg.Table.Traces, marshal)
	if err != nil {
		return err
	}

	res, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	response := StreamLoadResponse{}
	json.Unmarshal(body, &response)

	if !response.Success() {
		return fmt.Errorf("failed to push trace data: %s", response.Message)
	}

	return nil
}
