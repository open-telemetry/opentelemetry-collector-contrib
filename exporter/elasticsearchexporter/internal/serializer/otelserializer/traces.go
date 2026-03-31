// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func (*Serializer) SerializeSpanEvent(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span, spanEvent ptrace.SpanEvent, idx elasticsearch.Index, buf *bytes.Buffer) {
	w := newJSONWriter(buf)
	w.startObject()
	first := true
	first = w.writeTimestampField("@timestamp", spanEvent.Timestamp(), first)
	first = w.writeDataStream(idx, first)
	first = w.writeTraceIDField(span.TraceID(), first)
	first = w.writeSpanIDField("span_id", span.SpanID(), first)
	first = w.writeIntFieldSkipDefault("dropped_attributes_count", int64(spanEvent.DroppedAttributesCount()), first)
	first = w.writeStringFieldSkipDefault("event_name", spanEvent.Name(), first)

	var attributes pcommon.Map
	if spanEvent.Name() != "" {
		attributes = pcommon.NewMap()
		spanEvent.Attributes().CopyTo(attributes)
		attributes.PutStr("event.name", spanEvent.Name())
	} else {
		attributes = spanEvent.Attributes()
	}
	first = w.writeAttributes(attributes, false, first)
	first = w.writeResource(resource, resourceSchemaURL, false, first)
	_ = w.writeScope(scope, scopeSchemaURL, false, first)
	w.endObject()
}

func (*Serializer) SerializeSpan(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, span ptrace.Span, idx elasticsearch.Index, buf *bytes.Buffer) error {
	w := newJSONWriter(buf)
	w.startObject()
	first := true
	first = w.writeTimestampField("@timestamp", span.StartTimestamp(), first)
	first = w.writeDataStream(idx, first)
	first = w.writeTraceIDField(span.TraceID(), first)
	first = w.writeSpanIDField("span_id", span.SpanID(), first)
	first = w.writeStringFieldSkipDefault("trace_state", span.TraceState().AsRaw(), first)
	first = w.writeSpanIDField("parent_span_id", span.ParentSpanID(), first)
	first = w.writeStringFieldSkipDefault("name", span.Name(), first)
	first = w.writeStringFieldSkipDefault("kind", span.Kind().String(), first)
	first = w.writeUIntField("duration", uint64(span.EndTimestamp()-span.StartTimestamp()), first)
	first = w.writeAttributes(span.Attributes(), false, first)
	first = w.writeIntFieldSkipDefault("dropped_attributes_count", int64(span.DroppedAttributesCount()), first)
	first = w.writeIntFieldSkipDefault("dropped_events_count", int64(span.DroppedEventsCount()), first)
	first = writeSpanLinks(&w, span, first)
	first = w.writeIntFieldSkipDefault("dropped_links_count", int64(span.DroppedLinksCount()), first)
	first = writeStatus(&w, span.Status(), first)
	first = w.writeResource(resource, resourceSchemaURL, false, first)
	_ = w.writeScope(scope, scopeSchemaURL, false, first)
	w.endObject()
	return nil
}

func writeStatus(w *jsonWriter, status ptrace.Status, first bool) bool {
	first = w.key("status", first)
	w.startObject()
	firstField := true
	firstField = w.writeStringFieldSkipDefault("message", status.Message(), firstField)
	if code := status.Code(); code != ptrace.StatusCodeUnset {
		_ = w.writeStringFieldSkipDefault("code", code.String(), firstField)
	}
	w.endObject()
	return first
}

func writeSpanLinks(w *jsonWriter, span ptrace.Span, first bool) bool {
	first = w.key("links", first)
	w.startArray()
	firstElem := true
	for _, spanLink := range span.Links().All() {
		firstElem = w.arrayComma(firstElem)
		w.startObject()
		firstField := true
		firstField = w.writeStringFieldSkipDefault("trace_id", spanLink.TraceID().String(), firstField)
		firstField = w.writeStringFieldSkipDefault("span_id", spanLink.SpanID().String(), firstField)
		firstField = w.writeStringFieldSkipDefault("trace_state", spanLink.TraceState().AsRaw(), firstField)
		firstField = w.writeAttributes(spanLink.Attributes(), false, firstField)
		_ = w.writeIntFieldSkipDefault("dropped_attributes_count", int64(spanLink.DroppedAttributesCount()), firstField)
		w.endObject()
	}
	w.endArray()
	return first
}
