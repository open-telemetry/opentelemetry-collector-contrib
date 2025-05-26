// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	_ "embed" // for SQL file embedding
	"encoding/json"
	"fmt"
	"io"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

//go:embed sql/traces_ddl.sql
var tracesDDL string

//go:embed sql/traces_view.sql
var tracesView string

//go:embed sql/traces_graph_ddl.sql
var tracesGraphDDL string

//go:embed sql/traces_graph_job.sql
var tracesGraphJob string

// dTrace Trace to Doris
type dTrace struct {
	ServiceName        string         `json:"service_name"`
	Timestamp          string         `json:"timestamp"`
	ServiceInstanceID  string         `json:"service_instance_id"`
	TraceID            string         `json:"trace_id"`
	SpanID             string         `json:"span_id"`
	TraceState         string         `json:"trace_state"`
	ParentSpanID       string         `json:"parent_span_id"`
	SpanName           string         `json:"span_name"`
	SpanKind           string         `json:"span_kind"`
	EndTime            string         `json:"end_time"`
	Duration           int64          `json:"duration"`
	SpanAttributes     map[string]any `json:"span_attributes"`
	Events             []*dEvent      `json:"events"`
	Links              []*dLink       `json:"links"`
	StatusMessage      string         `json:"status_message"`
	StatusCode         string         `json:"status_code"`
	ResourceAttributes map[string]any `json:"resource_attributes"`
	ScopeName          string         `json:"scope_name"`
	ScopeVersion       string         `json:"scope_version"`
}

// dEvent Event to Doris
type dEvent struct {
	Timestamp  string         `json:"timestamp"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
}

// dLink Link to Doris
type dLink struct {
	TraceID    string         `json:"trace_id"`
	SpanID     string         `json:"span_id"`
	TraceState string         `json:"trace_state"`
	Attributes map[string]any `json:"attributes"`
}

type tracesExporter struct {
	*commonExporter
}

func newTracesExporter(logger *zap.Logger, cfg *Config, set component.TelemetrySettings) *tracesExporter {
	return &tracesExporter{
		commonExporter: newExporter(logger, cfg, set, "TRACE"),
	}
}

func (e *tracesExporter) start(ctx context.Context, host component.Host) error {
	client, err := createDorisHTTPClient(ctx, e.cfg, host, e.TelemetrySettings)
	if err != nil {
		return err
	}
	e.client = client

	if e.cfg.CreateSchema {
		conn, err := createDorisMySQLClient(e.cfg)
		if err != nil {
			return err
		}
		defer conn.Close()

		err = createAndUseDatabase(ctx, conn, e.cfg)
		if err != nil {
			return err
		}

		ddl := fmt.Sprintf(tracesDDL, e.cfg.Traces, e.cfg.propertiesStr())
		_, err = conn.ExecContext(ctx, ddl)
		if err != nil {
			return err
		}

		view := fmt.Sprintf(tracesView, e.cfg.Traces, e.cfg.Traces)
		_, err = conn.ExecContext(ctx, view)
		if err != nil {
			e.logger.Warn("failed to create materialized view", zap.Error(err))
		}

		ddl = fmt.Sprintf(tracesGraphDDL, e.cfg.Traces, e.cfg.propertiesStr())
		_, err = conn.ExecContext(ctx, ddl)
		if err != nil {
			return err
		}

		dropJob := e.formatDropTraceGraphJob()
		_, err = conn.ExecContext(ctx, dropJob)
		if err != nil {
			e.logger.Warn("failed to drop job", zap.Error(err))
		}

		job := e.formatTraceGraphJob()
		_, err = conn.ExecContext(ctx, job)
		if err != nil {
			e.logger.Warn("failed to create job", zap.Error(err))
		}
	}

	go e.reporter.report()
	return nil
}

func (e *tracesExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	return nil
}

func (e *tracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	label := generateLabel(e.cfg, e.cfg.Traces)
	traces := make([]*dTrace, 0, td.SpanCount())

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		resource := resourceSpan.Resource()
		resourceAttributes := resource.Attributes()
		serviceName := ""
		v, ok := resourceAttributes.Get(string(semconv.ServiceNameKey))
		if ok {
			serviceName = v.AsString()
		}
		serviceInstance := ""
		v, ok = resourceAttributes.Get(string(semconv.ServiceInstanceIDKey))
		if ok {
			serviceInstance = v.AsString()
		}

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)

				events := span.Events()
				newEvents := make([]*dEvent, 0, events.Len())
				for l := 0; l < events.Len(); l++ {
					event := events.At(l)

					newEvent := &dEvent{
						Timestamp:  e.formatTime(event.Timestamp().AsTime()),
						Name:       event.Name(),
						Attributes: event.Attributes().AsRaw(),
					}

					newEvents = append(newEvents, newEvent)
				}

				links := span.Links()
				newLinks := make([]*dLink, 0, links.Len())
				for l := 0; l < links.Len(); l++ {
					link := links.At(l)

					newLink := &dLink{
						TraceID:    traceutil.TraceIDToHexOrEmptyString(link.TraceID()),
						SpanID:     traceutil.SpanIDToHexOrEmptyString(link.SpanID()),
						TraceState: link.TraceState().AsRaw(),
						Attributes: link.Attributes().AsRaw(),
					}

					newLinks = append(newLinks, newLink)
				}

				trace := &dTrace{
					ServiceName:        serviceName,
					Timestamp:          e.formatTime(span.StartTimestamp().AsTime()),
					ServiceInstanceID:  serviceInstance,
					TraceID:            traceutil.TraceIDToHexOrEmptyString(span.TraceID()),
					SpanID:             traceutil.SpanIDToHexOrEmptyString(span.SpanID()),
					TraceState:         span.TraceState().AsRaw(),
					ParentSpanID:       traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()),
					SpanName:           span.Name(),
					SpanKind:           traceutil.SpanKindStr(span.Kind()),
					EndTime:            e.formatTime(span.EndTimestamp().AsTime()),
					Duration:           span.EndTimestamp().AsTime().Sub(span.StartTimestamp().AsTime()).Microseconds(),
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

	return e.pushTraceDataInternal(ctx, traces, label)
}

func (e *tracesExporter) pushTraceDataInternal(ctx context.Context, traces []*dTrace, label string) error {
	marshal, err := toJSONLines(traces)
	if err != nil {
		return err
	}

	req, err := streamLoadRequest(ctx, e.cfg, e.cfg.Traces, marshal, label)
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

	response := streamLoadResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	if response.success() {
		e.reporter.incrTotalRows(int64(len(traces)))
		e.reporter.incrTotalBytes(int64(len(marshal)))

		if response.duplication() {
			e.logger.Warn("label already exists", zap.String("label", label), zap.Int("skipped", len(traces)))
		}

		if e.cfg.LogResponse {
			e.logger.Info("trace response:\n" + string(body))
		} else {
			e.logger.Debug("trace response:\n" + string(body))
		}
		return nil
	}

	return fmt.Errorf("failed to push trace data, response:%s", string(body))
}

func (e *tracesExporter) formatDropTraceGraphJob() string {
	return fmt.Sprintf(
		"DROP JOB where jobName = '%s:%s_graph_job';",
		e.cfg.Database,
		e.cfg.Traces,
	)
}

func (e *tracesExporter) formatTraceGraphJob() string {
	return fmt.Sprintf(
		tracesGraphJob,
		e.cfg.Database,
		e.cfg.Traces,
		e.cfg.Traces,
		e.cfg.Traces,
		e.cfg.Traces,
	)
}
