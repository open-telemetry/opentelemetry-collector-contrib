// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter/internal"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type traceSignal struct {
	ResourceSchemaURL  string            `json:"resource_schema_url"`
	ResourceAttributes map[string]string `json:"resource_attributes"`
	ServiceName        string            `json:"service_name"`
	ScopeSchemaURL     string            `json:"scope_schema_url"`
	ScopeName          string            `json:"scope_name"`
	ScopeVersion       string            `json:"scope_version"`
	ScopeAttributes    map[string]string `json:"scope_attributes"`
	TraceID            string            `json:"trace_id"`
	SpanID             string            `json:"span_id"`
	ParentSpanID       string            `json:"parent_span_id"`
	TraceState         string            `json:"trace_state"`
	TraceFlags         uint32            `json:"trace_flags"`
	SpanName           string            `json:"span_name"`
	SpanKind           string            `json:"span_kind"`
	SpanAttributes     map[string]string `json:"span_attributes"`
	StartTime          string            `json:"start_time"`
	// used when users choose the StartTime-to-EndTime approach
	EndTime string `json:"end_time,omitempty"`
	// used when users choose the StartTime-plus-Duration approach
	Duration         int64               `json:"duration,omitempty"`
	StatusCode       string              `json:"status_code"`
	StatusMessage    string              `json:"status_message"`
	EventsTimestamp  []string            `json:"events_timestamp"`
	EventsName       []string            `json:"events_name"`
	EventsAttributes []map[string]string `json:"events_attributes"`
	LinksTraceID     []string            `json:"links_trace_id"`
	LinksSpanID      []string            `json:"links_span_id"`
	LinksTraceState  []string            `json:"links_trace_state"`
	LinksAttributes  []map[string]string `json:"links_attributes"`
}

func convertEvents(events ptrace.SpanEventSlice) (timestamps, names []string, attributes []map[string]string) {
	timestamps = make([]string, events.Len())
	names = make([]string, events.Len())
	attributes = make([]map[string]string, events.Len())
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)
		timestamps[i] = event.Timestamp().AsTime().Format(time.RFC3339Nano)
		names[i] = event.Name()
		attributes[i] = convertAttributes(event.Attributes())
	}
	return timestamps, names, attributes
}

func convertLinks(links ptrace.SpanLinkSlice) (traceIDs, spanIDs, states []string, attrs []map[string]string) {
	traceIDs = make([]string, links.Len())
	spanIDs = make([]string, links.Len())
	states = make([]string, links.Len())
	attrs = make([]map[string]string, links.Len())
	for i := 0; i < links.Len(); i++ {
		link := links.At(i)
		traceIDs[i] = traceutil.TraceIDToHexOrEmptyString(link.TraceID())
		spanIDs[i] = traceutil.SpanIDToHexOrEmptyString(link.SpanID())
		states[i] = link.TraceState().AsRaw()
		attrs[i] = convertAttributes(link.Attributes())
	}
	return traceIDs, spanIDs, states, attrs
}

func ConvertTraces(td ptrace.Traces, encoder Encoder) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		schemaURL := rs.SchemaUrl()
		resource := rs.Resource()
		resourceAttributesMap := resource.Attributes()
		resourceAttributes := convertAttributes(resourceAttributesMap)
		serviceName := getServiceName(resourceAttributesMap)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			ScopeSchemaURL := ss.SchemaUrl()
			scope := ss.Scope()
			scopeAttributes := convertAttributes(scope.Attributes())
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				attributes := span.Attributes()
				eventsTimestamp, eventsName, eventsAttributes := convertEvents(span.Events())
				linksTraceID, linksSpanID, linksTraceState, linksAttributes := convertLinks(span.Links())
				traceEntry := traceSignal{
					ResourceSchemaURL:  schemaURL,
					ResourceAttributes: resourceAttributes,
					ServiceName:        serviceName,
					ScopeSchemaURL:     ScopeSchemaURL,
					ScopeName:          scope.Name(),
					ScopeVersion:       scope.Version(),
					ScopeAttributes:    scopeAttributes,
					TraceID:            traceutil.TraceIDToHexOrEmptyString(span.TraceID()),
					SpanID:             traceutil.SpanIDToHexOrEmptyString(span.SpanID()),
					ParentSpanID:       traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID()),
					TraceState:         span.TraceState().AsRaw(),
					TraceFlags:         span.Flags(),
					SpanName:           span.Name(),
					SpanKind:           span.Kind().String(),
					SpanAttributes:     convertAttributes(attributes),
					StartTime:          span.StartTimestamp().AsTime().Format(time.RFC3339Nano),
					EndTime:            span.EndTimestamp().AsTime().Format(time.RFC3339Nano),
					Duration:           span.EndTimestamp().AsTime().Sub(span.StartTimestamp().AsTime()).Nanoseconds(),
					StatusCode:         span.Status().Code().String(),
					StatusMessage:      span.Status().Message(),
					EventsTimestamp:    eventsTimestamp,
					EventsName:         eventsName,
					EventsAttributes:   eventsAttributes,
					LinksTraceID:       linksTraceID,
					LinksSpanID:        linksSpanID,
					LinksTraceState:    linksTraceState,
					LinksAttributes:    linksAttributes,
				}
				if err := encoder.Encode(traceEntry); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
