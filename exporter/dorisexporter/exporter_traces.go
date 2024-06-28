// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

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
    scope_version         VARCHAR(50),

    INDEX idx_service_name(service_name) USING INVERTED,
    INDEX idx_timestamp(timestamp) USING INVERTED,
    INDEX idx_trace_id(trace_id) USING INVERTED,
    INDEX idx_span_id(span_id) USING INVERTED,
    INDEX idx_trace_state(trace_state) USING INVERTED,
    INDEX idx_parent_span_id(parent_span_id) USING INVERTED,
    INDEX idx_span_name(span_name) USING INVERTED,
    INDEX idx_span_kind(span_kind) USING INVERTED,
    INDEX idx_end_time(end_time) USING INVERTED,
    INDEX idx_duration(duration) USING INVERTED,
    INDEX idx_span_attributes(span_attributes) USING INVERTED,
    INDEX idx_status_message(status_message) USING INVERTED,
    INDEX idx_status_code(status_code) USING INVERTED,
    INDEX idx_resource_attributes(resource_attributes) USING INVERTED,
    INDEX idx_scope_name(scope_name) USING INVERTED,
    INDEX idx_scope_version(scope_version) USING INVERTED
)
ENGINE = OLAP
DUPLICATE KEY(service_name, timestamp)
PARTITION BY RANGE(timestamp) ()
DISTRIBUTED BY HASH(trace_id) BUCKETS AUTO
PROPERTIES (
"replication_num" = "1",
"compaction_policy" = "time_series",
"enable_single_replica_compaction" = "true",
"dynamic_partition.enable" = "true",
"dynamic_partition.create_history_partition" = "true",
"dynamic_partition.time_unit" = "DAY",
"dynamic_partition.start" = "%d",
"dynamic_partition.history_partition_num" = "%d",
"dynamic_partition.end" = "1",
"dynamic_partition.prefix" = "p"
);
`
)

type Trace struct {
	ServiceName        string         `json:"service_name"`
	Timestamp          string         `json:"timestamp"`
	TraceID            string         `json:"trace_id"`
	SpanID             string         `json:"span_id"`
	TraceState         string         `json:"trace_state"`
	ParentSpanID       string         `json:"parent_span_id"`
	SpanName           string         `json:"span_name"`
	SpanKind           string         `json:"span_kind"`
	EndTime            string         `json:"end_time"`
	Duration           int64          `json:"duration"`
	SpanAttributes     map[string]any `json:"span_attributes"`
	Events             []*Event       `json:"events"`
	Links              []*Link        `json:"links"`
	StatusMessage      string         `json:"status_message"`
	StatusCode         string         `json:"status_code"`
	ResourceAttributes map[string]any `json:"resource_attributes"`
	ScopeName          string         `json:"scope_name"`
	ScopeVersion       string         `json:"scope_version"`
}

type Event struct {
	Timestamp  string         `json:"timestamp"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
}

type Link struct {
	TraceID    string         `json:"trace_id"`
	SpanID     string         `json:"span_id"`
	TraceState string         `json:"trace_state"`
	Attributes map[string]any `json:"attributes"`
}

type tracesExporter struct {
	*commonExporter
}

func newTracesExporter(logger *zap.Logger, cfg *Config) (*tracesExporter, error) {
	commonExporter, err := newExporter(logger, cfg)
	if err != nil {
		return nil, err
	}

	return &tracesExporter{
		commonExporter: commonExporter,
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

	start, historyDays := e.cfg.startAndHistoryDays()
	ddl := fmt.Sprintf(tracesDDL, e.cfg.Table.Traces, start, historyDays)
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
						Timestamp:  e.formatTime(event.Timestamp().AsTime()),
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
					Timestamp:          e.formatTime(span.StartTimestamp().AsTime()),
					TraceID:            traceutil.TraceIDToHexOrEmptyString(span.TraceID()),
					SpanID:             traceutil.SpanIDToHexOrEmptyString(span.SpanID()),
					TraceState:         span.TraceState().AsRaw(),
					ParentSpanID:       traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()),
					SpanName:           span.Name(),
					SpanKind:           traceutil.SpanKindStr(span.Kind()),
					EndTime:            e.formatTime(span.EndTimestamp().AsTime()),
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
